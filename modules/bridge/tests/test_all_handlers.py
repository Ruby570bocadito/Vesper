#!/usr/bin/env python3
"""Smoke tests for the remaining Vesper bridge handler modules.

Imports every register_routes function, registers the handlers into a
unified registry and calls each one with empty params inside a
TEMPORARY LAB SANDBOX (VESPER_LAB_ROOT points at a pytest tmp dir), so
no test can mutate repository files — the failure mode that motivated
the Vesper safety remodel.
"""
import os
import sys
import tempfile
import traceback

# isolate: sandbox every file operation into a throwaway dir
_TMP = tempfile.mkdtemp(prefix="vesper_test_lab_")
os.environ["VESPER_LAB_ROOT"] = _TMP
os.environ.pop("VESPER_AUTHORIZED", None)  # force lab-only posture

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "handlers"))

EXPECTED_MODULES = [
    "attacks",
    "phase_1_4",
    "cred_dump",
    "bloodhound",
    "attack_navigator",
]

def build_registry():
    registry = {}
    for mod_name in EXPECTED_MODULES:
        try:
            mod = __import__(mod_name)
            if hasattr(mod, "register_routes"):
                mod.register_routes(registry)
        except Exception as e:
            print(f"  [!] {mod_name}: import failed: {e}")
            traceback.print_exc()
    return registry

def iter_handlers(registry):
    for group, handlers in registry.items():
        if isinstance(handlers, dict):
            for name, handler in handlers.items():
                yield f"{group}.{name}", handler
        elif callable(handlers):
            yield str(group), handlers

def main():
    registry = build_registry()
    handlers = list(iter_handlers(registry))
    print(f"[+] registered handlers: {len(handlers)}")
    assert len(handlers) >= 20, f"expected >=20 handlers, got {len(handlers)}"

    failures = []
    for name, fn in handlers:
        try:
            result = fn({})
            assert isinstance(result, dict), f"{name}: returned {type(result).__name__}, expected dict"
        except PermissionError:
            pass  # sandbox refusals are valid outcomes in lab-only mode
        except Exception as e:
            failures.append((name, repr(e)))

    if failures:
        print(f"[-] {len(failures)} handler failures:")
        for name, err in failures[:10]:
            print(f"    {name}: {err}")
        sys.exit(1)

    print(f"[+] all {len(handlers)} handlers executed cleanly inside the sandbox: {_TMP}")
    print("[+] HANDLER SMOKE TESTS PASSED")

if __name__ == "__main__":
    main()
