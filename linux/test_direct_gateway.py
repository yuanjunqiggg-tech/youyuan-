"""Loopback socket regressions; fixture credentials, no cloud/game requests."""
import importlib.util
import io
import json
import os
from pathlib import Path
import socket
import tempfile
import threading
import time
import unittest
from unittest.mock import patch


class GatewaySocketTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        config = Path(cls.tmp.name) / 'gateway.json'
        config.write_text(json.dumps({'console_token': 'fixture-only', 'owner_id': 'fixture', 'owner_username': 'fixture', 'mcp_key': 'fixture'}))
        spec = importlib.util.spec_from_file_location('gateway_fixture', Path(__file__).with_name('direct_gateway.py'))
        cls.gateway = importlib.util.module_from_spec(spec)
        with patch.dict(os.environ, {'YY_GATEWAY_CONFIG': str(config)}):
            spec.loader.exec_module(cls.gateway)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def setUp(self):
        class ShortIdleHandler(self.gateway.Handler):
            timeout = 0.25
        self.server = self.gateway.Server(('127.0.0.1', 0), ShortIdleHandler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)

    def socket(self):
        return socket.create_connection(self.server.server_address, timeout=2)

    def exchange(self, message):
        with self.socket() as sock:
            sock.sendall(message)
            out = bytearray()
            while True:
                chunk = sock.recv(65536)
                if not chunk:
                    return bytes(out)
                out.extend(chunk)

    def wait_slots(self):
        until = time.monotonic() + 2
        while time.monotonic() < until:
            if self.server.slots._value == 32:
                return
            time.sleep(0.01)
        self.fail(f'connections retained {32 - self.server.slots._value} slots')

    def test_forty_preflights_and_denials_release_connections(self):
        for _ in range(40):
            response = self.exchange(b'OPTIONS /api/files/upload HTTP/1.1\r\nHost: localhost\r\nOrigin: https://toolbox.youyuanqi.dpdns.org\r\nConnection: keep-alive\r\n\r\n')
            self.assertTrue(response.startswith(b'HTTP/1.1 204'), response[:80])
            self.assertIn(b'Connection: close', response)
        for _ in range(40):
            response = self.exchange(b'GET /health HTTP/1.1\r\nHost: localhost\r\nConnection: keep-alive\r\n\r\n')
            self.assertTrue(response.startswith(b'HTTP/1.1 401'), response[:80])
            self.assertIn(b'Connection: close', response)
        self.wait_slots()

    def test_idle_client_is_closed_and_slot_released(self):
        with self.socket() as idle:
            idle.sendall(b'GET /health HTTP/1.1\r\n')
            self.assertEqual(idle.recv(1), b'')
        self.wait_slots()

    def test_midstream_failure_never_appends_second_http_response(self):
        class BrokenUpstream:
            status = 200
            headers = {'Content-Type': 'application/octet-stream', 'Content-Length': '100'}
            reads = 0
            def __enter__(self): return self
            def __exit__(self, *args): return False
            def read(self, size):
                self.reads += 1
                if self.reads == 1: return b'partial-fixture'
                raise OSError('fixture upstream disconnect')
        with patch.object(self.gateway.urllib.request, 'urlopen', return_value=BrokenUpstream()):
            response = self.exchange(b'GET /api/files/download HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer fixture-only\r\n\r\n')
        headers, body = response.split(b'\r\n\r\n', 1)
        self.assertTrue(headers.startswith(b'HTTP/1.1 200'))
        self.assertEqual(body, b'partial-fixture')
        self.assertEqual(response.count(b'HTTP/1.1'), 1)
        self.wait_slots()


if __name__ == '__main__':
    unittest.main(verbosity=2)
