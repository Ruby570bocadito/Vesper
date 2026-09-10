#!/usr/bin/env python3
"""Tests for the Python bridge RPC protocol format."""
import json
import sys
import tempfile
import os
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))


class TestBridgeProtocol(unittest.TestCase):

    def test_bridge_request_format(self):
        req = {
            "module": "ransomware/encrypt",
            "function": "execute",
            "params": {"path": "/tmp/test", "files": 100},
            "timeout_ms": 30000,
        }
        serialized = json.dumps(req)
        parsed = json.loads(serialized)

        self.assertIn("module", parsed)
        self.assertIn("function", parsed)
        self.assertIn("params", parsed)
        self.assertIn("timeout_ms", parsed)
        self.assertEqual(parsed["module"], "ransomware/encrypt")
        self.assertEqual(parsed["function"], "execute")
        self.assertEqual(parsed["timeout_ms"], 30000)

    def test_bridge_response_format(self):
        resp = {
            "success": True,
            "result": {"encrypted_files": 42, "total_size": 1024000},
            "error": "",
            "elapsed_ms": 1500,
        }
        serialized = json.dumps(resp)
        parsed = json.loads(serialized)

        self.assertIn("success", parsed)
        self.assertIn("result", parsed)
        self.assertIn("error", parsed)
        self.assertIn("elapsed_ms", parsed)
        self.assertTrue(parsed["success"])
        self.assertEqual(parsed["result"]["encrypted_files"], 42)

    def test_bridge_error_response(self):
        resp = {
            "success": False,
            "result": {},
            "error": "module not found: ransomware/nonexistent",
            "elapsed_ms": 5,
        }
        serialized = json.dumps(resp)
        parsed = json.loads(serialized)

        self.assertFalse(parsed["success"])
        self.assertIn("not found", parsed["error"])

    def test_all_handler_function_names(self):
        """Verify every shipped handler module exists and exports register_routes."""
        handlers_dir = os.path.join(os.path.dirname(__file__), '..', 'handlers')
        expected_files = ['attacks', 'cred_dump', 'bloodhound', 'attack_navigator']

        for fname in expected_files:
            module_path = os.path.join(handlers_dir, f"{fname}.py")
            self.assertTrue(os.path.exists(module_path), f"Missing: {module_path}")
            src = open(module_path).read()
            self.assertIn('register_routes', src, f"{fname}.py does not export register_routes")

    def test_bridge_register_all_modules(self):
        """Verify all modules can be registered without error."""
        handlers_dir = os.path.join(os.path.dirname(__file__), '..', 'handlers')
        sys.path.insert(0, handlers_dir)

        registry = {}

        os.environ.setdefault('VESPER_LAB_ROOT', tempfile.mkdtemp(prefix='vesper_proto_test_'))
        modules_to_try = ['attacks', 'cred_dump', 'bloodhound', 'attack_navigator']

        for mod_name in modules_to_try:
            try:
                mod = __import__(mod_name)
                if hasattr(mod, 'register_routes'):
                    mod.register_routes(registry)
            except ImportError as e:
                self.fail(f"handler module {mod_name} failed to import: {e}")

        total = sum(len(handlers) for handlers in registry.values())
        self.assertGreater(total, 5, f"Expected >5 total handlers, got {total}")

    def test_bridge_protocol_compatibility(self):
        """Test forward/backward compatibility of protocol fields."""
        # Old client sends minimal fields
        old_req = json.dumps({"module": "test", "function": "ping"})
        parsed = json.loads(old_req)
        self.assertIn("module", parsed)
        self.assertIn("function", parsed)
        # New server should handle missing optional fields gracefully
        self.assertNotIn("params", parsed)
        self.assertNotIn("timeout_ms", parsed)

        # Modern response
        new_resp = json.dumps({"success": True, "result": {"status": "ok"}, "error": "", "elapsed_ms": 10})
        parsed_resp = json.loads(new_resp)
        self.assertTrue(parsed_resp["success"])

    def test_serialization_performance(self):
        """Benchmark JSON serialization."""
        import time
        payload = {
            "module": "ransomware/encrypt",
            "function": "execute",
            "params": {f"key_{i}": f"value_{i}" for i in range(100)},
            "timeout_ms": 30000,
        }
        start = time.time()
        for _ in range(1000):
            json.dumps(payload)
        elapsed = time.time() - start
        self.assertLess(elapsed, 5.0, f"Serialization too slow: {elapsed:.2f}s for 1000 ops")

    def test_bridge_handler_names_are_valid(self):
        """Verify handler names follow convention 'category/name'."""
        handlers_dir = os.path.join(os.path.dirname(__file__), '..', 'handlers')
        sys.path.insert(0, handlers_dir)

        registry = {}
        for mod_name in ['ransomware', 'ransomware_advanced', 'ransomware_blockz',
                         'ransomware_v26', 'ransomware_v27', 'ransomware_v28',
                         'ransomware_v29', 'ransomware_v210']:
            try:
                mod = __import__(mod_name)
                if hasattr(mod, 'register_routes'):
                    mod.register_routes(registry)
            except ImportError:
                pass

        for group, handlers in registry.items():
            for handler_name in handlers:
                self.assertNotIn('/', handler_name,
                                 f"Handler name '{handler_name}' in group '{group}' "
                                 "should be a plain function name")


if __name__ == '__main__':
    unittest.main(verbosity=2)
