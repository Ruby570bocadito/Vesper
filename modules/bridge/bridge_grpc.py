#!/usr/bin/env python3
"""Vesper — Python gRPC Bridge Server (v3.2)

Replaces the raw TCP/JSON bridge with full gRPC using protobuf schemas.
Implements BridgeService: ExecuteModule, AIAnalyze, ReconStream, HealthCheck.

Protocol: gRPC with X25519+XChaCha20-Poly1305 (handled by Go side).
Modules: 4 handler groups (recon, attacks, cred_dump, bloodhound) with 8
handlers, plus inline stubs that fail honestly (see registry).

Usage:
  python3 bridge.py --host 127.0.0.1 --port 9100
"""

import argparse
import ipaddress
import json
import logging
import os
import re
import signal
import subprocess
import sys
import time
from concurrent import futures
from dataclasses import dataclass, field
from typing import Callable, Dict, List

import grpc

# Add project root and proto stubs to path
PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
if PROJECT_ROOT not in sys.path:
    sys.path.insert(0, PROJECT_ROOT)

PROTO_DIR = os.path.join(os.path.dirname(__file__), "proto")
if PROTO_DIR not in sys.path:
    sys.path.insert(0, PROTO_DIR)

import bridge_pb2
import bridge_pb2_grpc

logging.basicConfig(level=logging.INFO, format="[Bridge-gRPC] %(message)s")
log = logging.getLogger("vesper.bridge")


# ============================================================
# SECURITY HELPERS
# ============================================================

def validate_target(target: str) -> str:
    """Validate and sanitize a target (IP or hostname) to prevent command injection."""
    if not target or not isinstance(target, str):
        raise ValueError("Target must be a non-empty string")
    target = target.strip()
    try:
        ipaddress.ip_address(target)
        return target
    except ValueError:
        pass
    if re.match(r'^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$', target):
        return target
    raise ValueError(f"Invalid target format: {target}")


def safe_run(cmd: List[str], timeout: int = 30) -> subprocess.CompletedProcess:
    """Run a subprocess safely, killing the entire process group on timeout."""
    proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           start_new_session=True, text=True)
    try:
        stdout, stderr = proc.communicate(timeout=timeout)
        return subprocess.CompletedProcess(cmd, proc.returncode, stdout, stderr)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(os.getpgid(proc.pid), signal.SIGKILL)
        except (ProcessLookupError, OSError):
            pass
        proc.wait()
        raise


class SensitiveString:
    """Wrapper that prevents sensitive data from appearing in logs."""
    __slots__ = ("_value",)

    def __init__(self, value: str):
        self._value = value

    def __str__(self) -> str:
        return "***REDACTED***"

    def __repr__(self) -> str:
        return "SensitiveString(***)"

    def raw(self) -> str:
        return self._value

# ============================================================
# MODULE REGISTRY
# ============================================================

@dataclass
class ModuleInfo:
    name: str
    description: str
    version: str
    phase: str
    handler: Callable
    requires: List[str] = field(default_factory=list)


class ModuleRegistry:
    def __init__(self):
        self._modules: Dict[str, ModuleInfo] = {}

    def register(self, name, description, version, phase, requires=None):
        def decorator(func):
            self._modules[name] = ModuleInfo(
                name=name, description=description, version=version,
                phase=phase, handler=func, requires=requires or []
            )
            return func
        return decorator

    def get(self, name):
        return self._modules.get(name)

    def list(self):
        return list(self._modules.values())

    def execute(self, name, params):
        mod = self._modules.get(name)
        if not mod:
            return {"success": False, "error": f"Module '{name}' not found"}
        try:
            start = time.time()
            result = mod.handler(params)
            elapsed = int((time.time() - start) * 1000)
            return {"success": True, "result": result, "elapsed_ms": elapsed}
        except Exception as e:
            return {"success": False, "error": str(e), "elapsed_ms": 0}


# ============================================================
# HANDLER REGISTRY (nested dict of real handler groups)
# ============================================================

_handler_registry: Dict[str, Dict[str, Callable]] = {}

def _load_all_handlers():
    """Load the real bridge handler modules into _handler_registry."""
    handler_path = os.path.join(os.path.dirname(__file__), "handlers")
    if handler_path not in sys.path:
        sys.path.insert(0, handler_path)

    modules = [
        "attacks",
        "cred_dump",
        "bloodhound",
        "attack_navigator",
    ]

    for mod_name in modules:
        try:
            mod = __import__(mod_name)
            if hasattr(mod, "register_routes"):
                mod.register_routes(_handler_registry)
        except ImportError:
            log.warning(f"Failed to load handler module: {mod_name}")

    total = sum(len(v) for v in _handler_registry.values())
    log.info(f"Loaded {total} handlers across {len(_handler_registry)} groups")


def _dispatch_handler(module_name, function_name, params):
    """Dispatch to either the ModuleRegistry or the handler registry."""
    # Try ModuleRegistry first (inline modules)
    result = registry.execute(module_name, params)
    if not ("error" in result and "Module" in str(result.get("error", ""))):
        return result

    # Try handler registry (nested dict format)
    group = _handler_registry.get(module_name)
    if group and function_name in group:
        try:
            start = time.time()
            handler_result = group[function_name](params)
            elapsed = int((time.time() - start) * 1000)
            return {"success": True, "result": handler_result, "elapsed_ms": elapsed}
        except Exception as e:
            return {"success": False, "error": str(e), "elapsed_ms": 0}

    # Try searching all groups for the function name
    for group_name, handlers in _handler_registry.items():
        if function_name in handlers:
            try:
                start = time.time()
                handler_result = handlers[function_name](params)
                elapsed = int((time.time() - start) * 1000)
                return {"success": True, "result": handler_result, "elapsed_ms": elapsed}
            except Exception as e:
                return {"success": False, "error": str(e), "elapsed_ms": 0}

    return {"success": False, "error": f"Handler '{module_name}.{function_name}' not found"}


# ============================================================
# INLINE MODULE HANDLERS (keep from original bridge.py)
# ============================================================

registry = ModuleRegistry()


@registry.register("recon", "Network reconnaissance", "1.0", "recon")
def recon_handler(params):
    try:
        target = validate_target(params.get("target", "127.0.0.1"))
    except ValueError as e:
        return {"success": False, "error": str(e)}

    mode = params.get("mode", "basic")
    tools = params.get("tools", ["nmap"])
    result = {"target": target, "mode": mode, "hosts_found": 0, "ports_open": [], "services": []}

    if "nmap" in tools and not params.get("simulation", True):
        try:
            cmd = ["nmap", "-sV", "-F", "-T4", "--max-retries", "2", target]
            if mode == "stealth":
                cmd = ["nmap", "-sS", "-T2", "--max-retries", "1", "--max-rtt-timeout", "500ms", target]
            proc = safe_run(cmd, timeout=20)
            result["raw_output"] = proc.stdout[:2000]
        except subprocess.TimeoutExpired:
            result["error"] = "nmap timed out"
        except FileNotFoundError:
            pass

    return result


@registry.register("ai_analyze", "AI analysis via local Ollama", "0.0", "stub")
def ai_analyze_handler(params):
    # honest stub: not implemented — an empty "success" would be fabrication
    return {"success": False, "error": "ai analysis is not implemented (stub)"}

@registry.register("privesc", "Privilege escalation", "0.0", "stub")
def privesc_handler(params):
    # honest stub: not implemented — an empty "success" would be fabrication
    return {"success": False, "error": "privilege escalation is not implemented (stub)"}

@registry.register("persist", "Persistence mechanisms", "0.0", "stub")
def persist_handler(params):
    # honest stub: not implemented — an empty "success" would be fabrication
    return {"success": False, "error": "persistence is not implemented (stub)"}

@registry.register("worm", "Network worm propagation", "0.0", "stub")
def worm_handler(params):
    # honest stub: not implemented — an empty "success" would be fabrication
    return {"success": False, "error": "worm propagation is not implemented (stub)"}

@registry.register("blue", "BlueForge defense metrics", "0.0", "stub")
def blue_handler(params):
    # honest stub: not implemented — an empty "success" would be fabrication
    return {"success": False, "error": "blue-team metrics is not implemented (stub)"}

@registry.register("evasion", "AMSI/ETW evasion", "0.0", "stub")
def evasion_handler(params):
    # honest stub: not implemented — an empty "success" would be fabrication
    return {"success": False, "error": "evasion is not implemented (stub)"}

@registry.register("report", "Campaign report generator", "0.0", "stub")
def report_handler(params):
    # honest stub: not implemented — an empty "success" would be fabrication
    return {"success": False, "error": "report generation is not implemented (stub)"}

@registry.register("exfil", "Data exfiltration", "0.0", "stub")
def exfil_handler(params):
    # honest stub: not implemented — an empty "success" would be fabrication
    return {"success": False, "error": "exfiltration is not implemented (stub)"}

@registry.register("health", "Health check + module listing", "1.0", "c2")
def health_handler(params):
    modules = [{"name": m.name, "version": m.version, "phase": m.phase} for m in registry.list()]
    handler_count = sum(len(v) for v in _handler_registry.values())
    return {"status": "ok", "inline_modules": len(modules), "handler_groups": len(_handler_registry),
            "modules": modules}


# ============================================================
# gRPC SERVICE IMPLEMENTATION
# ============================================================

class BridgeServiceServicer(bridge_pb2_grpc.BridgeServiceServicer):

    def ExecuteModule(self, request, context):
        module_name = request.module_name
        function_name = request.function_name
        params = {}
        if request.payload:
            try:
                params = json.loads(request.payload.decode("utf-8"))
            except json.JSONDecodeError:
                return bridge_pb2.ModuleResponse(
                    success=False,
                    error="invalid JSON payload",
                )

        if not isinstance(params, dict):
            params = {}

        result = _dispatch_handler(module_name, function_name, params)

        return bridge_pb2.ModuleResponse(
            success=result.get("success", False),
            result=json.dumps(result.get("result", {})).encode("utf-8"),
            error=result.get("error", ""),
            elapsed_ms=result.get("elapsed_ms", 0),
        )

    def AIAnalyze(self, request, context):
        prompt = request.context
        target_data = request.target_data
        model = request.model or "llama3.1:8b"
        options = dict(request.options)

        result = registry.execute("ai_analyze", {
            "prompt": f"{prompt}\n\nTarget data:\n{target_data}",
            "model": model,
            **options,
        })

        if result.get("success"):
            yield bridge_pb2.AIAnalyzeResponse(
                suggestion=result.get("result", {}).get("response", ""),
                tactic="",
                technique="",
                mitre_id="",
                confidence=0.0,
                reasoning="",
                raw_response=result.get("result", {}).get("response", ""),
            )

    def ReconStream(self, request, context):
        target = request.target
        mode = request.mode
        tools = list(request.tools)

        result = registry.execute("recon", {
            "target": target,
            "mode": mode,
            "tools": tools,
        })

        if result.get("success"):
            data = result.get("result", {})
            yield bridge_pb2.ReconResponse(
                host=data.get("target", target),
                port=0,
                service="",
                version="",
                vulnerability="",
                cve="",
            )

    def HealthCheck(self, request, context):
        result = registry.execute("health", {})
        data = result.get("result", {})
        inline = data.get("inline_modules", 0)
        handlers = data.get("handler_groups", 0)
        return bridge_pb2.HealthCheckResponse(
            ok=result.get("success", False),
            module_name="vesper-bridge",
            version=f"3.2-grpc ({inline} inline modules, {handlers} handlers)",
        )


# ============================================================
# SERVER ENTRYPOINT
# ============================================================

def serve(host="127.0.0.1", port=9100, max_workers=20):
    endpoint = f"[::]:{port}" if host == "0.0.0.0" else f"{host}:{port}"

    _load_all_handlers()

    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=max_workers),
        options=[
            ("grpc.max_send_message_length", 50 * 1024 * 1024),
            ("grpc.max_receive_message_length", 50 * 1024 * 1024),
        ],
    )

    bridge_pb2_grpc.add_BridgeServiceServicer_to_server(BridgeServiceServicer(), server)

    server.add_insecure_port(endpoint)

    server.start()
    log.info(f"Vesper Bridge gRPC v3.2 listening on {endpoint}")
    log.info(f"Modules: {len(registry.list())} inline + {sum(len(v) for v in _handler_registry.values())} handlers")

    try:
        while True:
            time.sleep(86400)
    except KeyboardInterrupt:
        log.info("Shutting down...")
        server.stop(grace=5)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Vesper Python gRPC Bridge")
    parser.add_argument("--host", default="127.0.0.1", help="Listen host")
    parser.add_argument("--port", type=int, default=9100, help="Listen port")
    parser.add_argument("--workers", type=int, default=20, help="Thread pool size")
    args = parser.parse_args()

    serve(host=args.host, port=args.port, max_workers=args.workers)
