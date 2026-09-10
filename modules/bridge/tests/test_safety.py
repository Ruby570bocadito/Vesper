"""Tests for the bridge safety sandbox (modules/bridge/safety.py).

These tests pin the security contract added in the Vesper remodel:
file operations resolve inside the lab root, escapes are refused in
lab-only mode, and the refusal payload is self-explanatory.
"""
import os
import sys
import tempfile
import unittest
from pathlib import Path

_TMP = tempfile.mkdtemp(prefix="vesper_safety_test_")
os.environ["VESPER_LAB_ROOT"] = _TMP
os.environ.pop("VESPER_AUTHORIZED", None)

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from safety import is_authorized, is_lab_only, lab_root, resolve_in_lab, sandbox_write


class TestLabSandbox(unittest.TestCase):
    def test_lab_root_is_created_and_pinned(self):
        root = lab_root()
        self.assertTrue(root.exists())
        self.assertEqual(root, Path(_TMP).resolve())

    def test_relative_path_resolves_inside_root(self):
        p = resolve_in_lab("tmp/example.json")
        self.assertTrue(str(p).startswith(str(lab_root())))

    def test_escape_is_refused_in_lab_only_mode(self):
        with self.assertRaises(PermissionError):
            resolve_in_lab("/etc/passwd")
        with self.assertRaises(PermissionError):
            resolve_in_lab("../../outside.txt")

    def test_traversal_via_dots_refused(self):
        sneaky = str(lab_root() / ".." / ".." / "escaped.txt")
        with self.assertRaises(PermissionError):
            resolve_in_lab(sneaky)

    def test_sandbox_write_returns_refusal_dict_not_exception(self):
        result = sandbox_write("/etc/evil.conf")
        self.assertIsInstance(result, dict)
        self.assertFalse(result["success"])
        self.assertTrue(result["sandboxed"])
        self.assertIn("hint", result)

    def test_sandbox_write_allows_inside_root(self):
        result = sandbox_write("reports/layer.json")
        self.assertNotIsInstance(result, dict)
        self.assertTrue(str(result).startswith(str(lab_root())))

    def test_default_posture_is_lab_only(self):
        # without VESPER_AUTHORIZED the default must be lab-only
        self.assertFalse(is_authorized())
        self.assertTrue(is_lab_only())


if __name__ == "__main__":
    unittest.main()
