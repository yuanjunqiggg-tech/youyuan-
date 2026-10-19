"""Real local TLS/socket/process tests; never contact a cloud or game account.

The public localhost certificate/key fixture is test-only and is not a service
credential. Tests explicitly trust it only for their local HTTPS server.
"""
import importlib.util
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import os
from pathlib import Path
import queue
import signal
import ssl
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location('direct_tunnel_fixture', Path(__file__).with_name('direct_tunnel.py'))
tunnel = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tunnel)
ORIGIN = 'https://fixture-live.trycloudflare.com'


class HTTPSProbeTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        fixtures = Path(__file__).with_name('test_direct_tunnel_fixtures')
        cls.context = ssl.create_default_context(cafile=str(fixtures / 'localhost-cert.pem'))
        server_context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        server_context.load_cert_chain(fixtures / 'localhost-cert.pem', fixtures / 'localhost-key.pem')

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_args):
                pass

            def do_GET(self):
                self.server.requests.append((self.path, dict(self.headers)))
                self.send_response(self.server.response_status)
                if self.server.response_status == 302:
                    self.send_header('Location', '/another-path')
                self.send_header('Content-Length', '0')
                self.end_headers()

        cls.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        cls.server.daemon_threads = True
        cls.server.socket = server_context.wrap_socket(cls.server.socket, server_side=True)
        cls.server.requests = []
        cls.origin = 'https://localhost:%d' % cls.server.server_port
        cls.thread = threading.Thread(target=cls.server.serve_forever, kwargs={'poll_interval': 0.02}, daemon=True)
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join(timeout=1)

    def test_real_verified_https_accepts_health_200_and_expected_401_without_credentials(self):
        for status in [200, 401]:
            with self.subTest(status=status):
                self.server.response_status = status
                self.assertTrue(tunnel.probe_https(self.origin, 1, self.context))
                path, headers = self.server.requests[-1]
                self.assertEqual(path, '/health')
                self.assertNotIn('Authorization', headers)
                self.assertNotIn('Cookie', headers)

    def test_default_certificate_verification_rejects_untrusted_certificate(self):
        self.server.response_status = 401
        self.assertFalse(tunnel.probe_https(self.origin, 1))

    def test_redirects_errors_plain_http_and_unreachable_endpoint_are_unhealthy(self):
        for status in [302, 403, 404, 500, 502, 530]:
            with self.subTest(status=status):
                self.server.response_status = status
                count = len(self.server.requests)
                self.assertFalse(tunnel.probe_https(self.origin, 1, self.context))
                self.assertEqual(len(self.server.requests), count + 1)
        self.assertFalse(tunnel.probe_https('http://127.0.0.1:1', 0.1))
        self.assertFalse(tunnel.probe_https('https://127.0.0.1:1', 0.1))


class SupervisorTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.destination = Path(self.tmp.name) / 'linux-direct.json'
        self.children = []
        self.logs = []
        # Windows cannot replace a file while a Python reader has it open.
        # Serialize the test observer with writes to reproduce Linux's atomic
        # replace/read semantics, while still executing the real publish code.
        self.file_lock = threading.Lock()
        actual_publish = tunnel.publish

        def serialize_publish(*args):
            with self.file_lock:
                return actual_publish(*args)

        publisher = patch.object(tunnel, 'publish', side_effect=serialize_publish)
        publisher.start()
        self.addCleanup(publisher.stop)
        actual_popen = subprocess.Popen

        def capture(*args, **kwargs):
            child = actual_popen(*args, **kwargs)
            self.children.append(child)
            return child

        self.patcher = patch.object(tunnel.subprocess, 'Popen', side_effect=capture)
        self.patcher.start()
        self.addCleanup(self.patcher.stop)

    def start(self, health, script=None, **options):
        script = script or "import time\nprint('registered edge; keep this diagnostic',flush=True)\nprint('%s',flush=True)\ntime.sleep(30)" % ORIGIN
        stop, finished, logs = threading.Event(), queue.Queue(), []
        self.logs = logs
        settings = dict(interval=0.03, probe_timeout=0.2, startup_timeout=1,
                        min_restart_interval=0, poll_interval=0.005, shutdown_timeout=0.2)
        settings.update(options)

        def run():
            finished.put(tunnel.supervise(self.destination, [sys.executable, '-u', '-c', script], stop, health, log=logs.append, **settings))

        thread = threading.Thread(target=run, daemon=True)
        thread.start()

        def cleanup():
            stop.set()
            thread.join(timeout=2)
            for child in self.children:
                if child.poll() is None:
                    child.kill()
                    child.wait(timeout=1)

        self.addCleanup(cleanup)
        return stop, finished, logs, thread

    def wait_for(self, predicate, timeout=2):
        until = time.monotonic() + timeout
        while time.monotonic() < until:
            if predicate():
                return
            time.sleep(0.005)
        self.fail('expected supervisor state was not reached: %r' % self.logs)

    def advertised(self):
        with self.file_lock:
            return json.loads(self.destination.read_text(encoding='utf-8'))['url'] if self.destination.exists() else None

    def test_old_url_cleared_and_candidate_only_published_after_health_then_clean_stop(self):
        self.destination.write_text(json.dumps({'url': 'https://stale.trycloudflare.com'}))
        ready, checked = threading.Event(), threading.Event()

        def health(*_args):
            checked.set()
            return ready.is_set()

        stop, finished, logs, _ = self.start(health)
        self.assertTrue(checked.wait(1))
        self.assertEqual(self.advertised(), '')
        ready.set()
        self.wait_for(lambda: self.advertised() == ORIGIN)
        self.assertTrue(any('registered edge' in line for line in logs))
        stop.set()
        self.assertEqual(finished.get(timeout=2), 0)
        self.assertEqual(self.advertised(), '')
        self.assertIsNotNone(self.children[0].poll())

    def test_first_failure_revokes_advertisement_and_threshold_requests_one_systemd_restart(self):
        failing = threading.Event()
        stop, finished, logs, _ = self.start(lambda *_args: not failing.is_set(), interval=0.08)
        self.wait_for(lambda: self.advertised() == ORIGIN)
        failing.set()
        self.wait_for(lambda: self.advertised() == '')
        self.assertEqual(finished.get(timeout=2), 1)
        self.assertEqual(len(self.children), 1)
        self.assertEqual(sum('HTTPS health failed' in line for line in logs), 3)
        self.assertIsNotNone(self.children[0].poll())
        self.assertFalse(stop.is_set())

    def test_startup_timeout_cleans_child_and_throttles_restart(self):
        started = time.monotonic()
        _, finished, logs, _ = self.start(lambda *_args: False, startup_timeout=0.12, min_restart_interval=0.4)
        self.assertEqual(finished.get(timeout=2), 1)
        self.assertGreaterEqual(time.monotonic() - started, 0.38)
        self.assertEqual(self.advertised(), '')
        self.assertIsNotNone(self.children[0].poll())
        self.assertTrue(any('startup health deadline' in line for line in logs))
        self.assertTrue(any('allowing systemd restart' in line for line in logs))

    def test_silent_child_has_bounded_startup_and_preserves_diagnostics(self):
        _, finished, _, _ = self.start(lambda *_args: True, script='import time\nprint("edge connection failed",flush=True)\ntime.sleep(30)', startup_timeout=0.15)
        self.assertEqual(finished.get(timeout=2), 1)
        self.assertEqual(self.advertised(), '')
        self.assertIsNotNone(self.children[0].poll())

    def test_stuck_dns_or_probe_cannot_block_supervision_or_accumulate_threads(self):
        release, calls = threading.Event(), []
        self.addCleanup(release.set)

        def stuck(*args):
            calls.append(args)
            release.wait(2)
            return True

        _, finished, logs, _ = self.start(stuck, probe_timeout=0.08)
        self.assertEqual(finished.get(timeout=1), 1)
        self.assertEqual(self.advertised(), '')
        self.assertEqual(len(calls), 1)
        self.assertTrue(any('probe deadline' in line for line in logs))
        self.assertIsNotNone(self.children[0].poll())

    def test_child_exit_does_not_leave_advertisement_or_spawn_internal_retry(self):
        _, finished, logs, _ = self.start(lambda *_args: True, script="print('%s',flush=True)\nraise SystemExit(7)" % ORIGIN)
        self.assertEqual(finished.get(timeout=2), 1)
        self.assertEqual(self.advertised(), '')
        self.assertEqual(len(self.children), 1)
        self.assertTrue(any('status 7' in line for line in logs))

    def test_sigterm_handler_requests_cancellable_shutdown(self):
        calls = []

        def stop_supervision(**kwargs):
            calls.append(kwargs['stop'])
            signal.getsignal(signal.SIGTERM)(signal.SIGTERM, None)
            self.assertTrue(kwargs['stop'].is_set())
            return 0

        old_term, old_int = signal.getsignal(signal.SIGTERM), signal.getsignal(signal.SIGINT)
        try:
            with patch.object(tunnel, 'supervise', side_effect=stop_supervision):
                self.assertEqual(tunnel.main(), 0)
        finally:
            signal.signal(signal.SIGTERM, old_term)
            signal.signal(signal.SIGINT, old_int)
        self.assertEqual(len(calls), 1)

    @unittest.skipUnless(os.name == 'posix', 'real POSIX SIGTERM/process-group verification runs on Linux')
    def test_real_sigterm_reaps_owned_child_and_leaves_unrelated_sibling_alive(self):
        child_pid = Path(self.tmp.name) / 'owned.pid'
        child_script = "import os,time\nfrom pathlib import Path\nPath(%r).write_text(str(os.getpid()))\nprint(%r,flush=True)\ntime.sleep(30)" % (str(child_pid), ORIGIN)
        launcher = (
            "import importlib.util\nfrom pathlib import Path\n"
            "spec=importlib.util.spec_from_file_location('supervised',%r)\n"
            "module=importlib.util.module_from_spec(spec)\nspec.loader.exec_module(module)\n"
            "original=module.supervise\n"
            "def run(**kwargs):\n"
            " return original(destination=Path(%r),command=%r,health=lambda *_:True,"
            "interval=.03,probe_timeout=.2,poll_interval=.005,shutdown_timeout=.2,min_restart_interval=10,**kwargs)\n"
            "module.supervise=run\nraise SystemExit(module.main())\n"
        ) % (str(Path(tunnel.__file__).resolve()), str(self.destination), [sys.executable, '-u', '-c', child_script])
        sibling = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(30)'])
        controller = subprocess.Popen([sys.executable, '-u', '-c', launcher], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)

        def cleanup():
            for process in [controller, sibling]:
                if process.poll() is None:
                    process.kill()
                process.wait(timeout=2)
            if controller.stdout is not None:
                controller.stdout.close()

        self.addCleanup(cleanup)
        self.wait_for(lambda: self.advertised() == ORIGIN)
        owned_pid = int(child_pid.read_text())
        controller.send_signal(signal.SIGTERM)
        output, _ = controller.communicate(timeout=2)
        self.assertEqual(controller.returncode, 0, output)
        self.assertEqual(self.advertised(), '')
        with self.assertRaises(ProcessLookupError):
            os.kill(owned_pid, 0)
        self.assertIsNone(sibling.poll())

    @unittest.skipUnless(os.name == 'posix', 'SIGKILL escalation verification runs on Linux')
    def test_owned_child_ignoring_sigterm_is_killed_within_deadline(self):
        script = "import signal,time\nsignal.signal(signal.SIGTERM,signal.SIG_IGN)\nprint(%r,flush=True)\ntime.sleep(30)" % ORIGIN
        stop, finished, _, _ = self.start(lambda *_: True, script=script, shutdown_timeout=0.1)
        self.wait_for(lambda: self.advertised() == ORIGIN)
        stop.set()
        self.assertEqual(finished.get(timeout=1), 0)
        self.assertEqual(self.children[0].returncode, -signal.SIGKILL)
        self.assertEqual(self.advertised(), '')


if __name__ == '__main__':
    unittest.main()
