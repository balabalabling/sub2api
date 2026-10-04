"""Offline checks for the recovery CLI. No network or real credentials."""
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("seed_image_replay", Path(__file__).with_name("seed_image_replay_cache.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class SeedImageReplayTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=os.environ.get("TEMP"))
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.payload = self.root / "seed.json"
        self.payload.write_text(json.dumps({"model": "gpt-6.1-sol", "input": [{"type": "image_generation_call", "id": "ig_fixture", "result": "cG5n"}, {"role": "user", "content": "Reply OK"}]}), encoding="utf-8")
        self.config = self.root / "config.toml"
        self.config.write_text('model_provider="custom"\n[model_providers.custom]\nbase_url="https://gateway.example/v1"\nexperimental_bearer_token="fixture-token"\n', encoding="utf-8")

    def run_cli(self, *extra):
        argv = ["seed", "--payload", str(self.payload), "--codex-home", str(self.root), *extra]
        with patch.object(sys, "argv", argv), contextlib.redirect_stdout(io.StringIO()):
            return module.main()

    def test_default_never_reads_credentials_or_opens_network(self):
        self.config.unlink()
        with patch.object(module.urllib.request, "build_opener", side_effect=AssertionError("network called")):
            self.assertEqual(0, self.run_cli())

    def test_send_verifies_missing_result_after_seeding(self):
        requests = []
        class Opener:
            def open(self, request, timeout):
                requests.append(json.loads(request.data))
                response = io.BytesIO(b'data: {"type":"response.completed","response":{}}\n\n')
                response.status = 200
                return response
        with patch.object(module.urllib.request, "build_opener", return_value=Opener()):
            self.assertEqual(0, self.run_cli("--send"))
        self.assertEqual(2, len(requests))
        self.assertIn("result", requests[0]["input"][0])
        self.assertNotIn("result", requests[1]["input"][0])
        for request in requests:
            self.assertFalse(request["store"])
            self.assertEqual([], request["tools"])
            self.assertEqual("none", request["tool_choice"])

    def test_first_failure_stops_without_retry(self):
        class Opener:
            calls = 0
            def open(self, request, timeout):
                self.calls += 1
                response = io.BytesIO(b'data: {"type":"response.failed","response":{}}\n\n')
                response.status = 200
                return response
        opener = Opener()
        with patch.object(module.urllib.request, "build_opener", return_value=opener):
            with self.assertRaisesRegex(RuntimeError, "no successful completed response"):
                self.run_cli("--send")
        self.assertEqual(1, opener.calls)

    def test_redirects_do_not_forward_credentials(self):
        with self.assertRaisesRegex(RuntimeError, "must not redirect"):
            module.NoRedirect().redirect_request(None, None, 302, "", {}, "https://other.example")


if __name__ == "__main__":
    unittest.main()
