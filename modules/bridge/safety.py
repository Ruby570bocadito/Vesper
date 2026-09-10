"""Vesper bridge safety — lab sandbox + authorization gate.

Every file-system-touching handler in the bridge MUST route paths
through this module. Two controls are enforced:

1. LAB_ROOT sandbox: all reads/writes are resolved inside a single
   lab directory (env ``VESPER_LAB_ROOT``, default ``./lab_root``).
   Escapes (``..``, absolute paths outside the root, symlinks) are
   rejected.

2. Authorization gate: handlers flagged ``lab_only=True`` refuse to
   touch anything outside the sandbox unless ``VESPER_AUTHORIZED=1``
   is set AND the operator passed the CLI auth gate (which also sets
   ``VESPER_LAB_ONLY=1`` to force the sandbox back on).
"""

from __future__ import annotations

import os
from pathlib import Path
from typing import Union

_LAB_ROOT_ENV = "VESPER_LAB_ROOT"
_AUTH_ENV = "VESPER_AUTHORIZED"
_LAB_ONLY_ENV = "VESPER_LAB_ONLY"


def lab_root() -> Path:
    """Return (and create) the sandbox root directory."""
    root = Path(os.environ.get(_LAB_ROOT_ENV, "./lab_root")).resolve()
    root.mkdir(parents=True, exist_ok=True)
    return root


def is_authorized() -> bool:
    """True when the operator explicitly authorized a live engagement."""
    return os.environ.get(_AUTH_ENV, "").strip().lower() in ("1", "true", "yes")


def is_lab_only() -> bool:
    """True when the sandbox must be enforced (default unless authorized)."""
    if os.environ.get(_LAB_ONLY_ENV, "").strip().lower() in ("1", "true", "yes"):
        return True
    return not is_authorized()


def sandbox_result(message: str) -> dict:
    """Standard refusal payload for blocked operations."""
    return {
        "success": False,
        "sandboxed": True,
        "error": message,
        "hint": (
            "Vesper runs LAB-ONLY by default. Set VESPER_LAB_ROOT to your "
            "lab directory and rerun the CLI with --yes-i-am-authorized "
            "for a live engagement."
        ),
    }


def resolve_in_lab(path: Union[str, os.PathLike], must_exist: bool = False) -> Path:
    """Resolve *path* inside the lab root and refuse escapes.

    Raises PermissionError when the path escapes the sandbox (unless the
    operator is authorized AND lab-only mode is off).
    """
    root = lab_root()
    p = Path(path)
    if not p.is_absolute():
        p = root / p
    p = p.resolve()
    try:
        p.relative_to(root)
    except ValueError:
        if is_lab_only() or not is_authorized():
            raise PermissionError(
                f"path {str(path)!r} escapes the lab sandbox ({str(root)!r})"
            )
    if must_exist and not p.exists():
        raise FileNotFoundError(f"{str(p)!r} does not exist")
    return p


def sandbox_write(path: Union[str, os.PathLike]) -> "Path | dict":
    """Validate a write target; return a refusal dict instead of raising."""
    try:
        return resolve_in_lab(path)
    except PermissionError as exc:
        return sandbox_result(str(exc))
