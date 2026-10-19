"""Advertise only a reachable, certificate-verified direct HTTPS tunnel.

One invocation owns one cloudflared process. Failure clears the advertisement
and exits after a rate limit; systemd owns creation of the next tunnel.
"""
import json
import os
from pathlib import Path
import queue
import re
import signal
import ssl
import subprocess
import threading
import time
import urllib.error
import urllib.request


DESTINATION = Path('/var/lib/youyuan/linux-direct.json')
COMMAND = ['/opt/youyuan/cloudflared', 'tunnel', '--no-autoupdate', '--protocol',
           'http2', '--url', 'http://127.0.0.1:19430']
TUNNEL_URL = re.compile(r'https://[a-z0-9-]+\.trycloudflare\.com(?=[\s/|]|$)')


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def probe_https(origin, timeout=8, context=None):
    """Probe with verified TLS and no credentials, redirects or proxy settings.

    /health is protected by the gateway: its 401 is expected and demonstrates
    HTTPS reachability without weakening API authentication. A successful
    public health response is also acceptable; other HTTP statuses are not.
    """
    if not origin.startswith('https://'):
        return False
    opener = urllib.request.build_opener(
        urllib.request.ProxyHandler({}),
        urllib.request.HTTPSHandler(context=context or ssl.create_default_context()),
        NoRedirect(),
    )
    request = urllib.request.Request(origin + '/health', headers={
        'User-Agent': 'YouYuan-Tunnel-Health/1', 'Connection': 'close',
    })
    try:
        with opener.open(request, timeout=timeout) as response:
            return response.status == 200
    except urllib.error.HTTPError as error:
        try:
            return error.code == 401
        finally:
            error.close()
    except (OSError, urllib.error.URLError, ValueError):
        return False


def publish(destination, origin):
    destination = Path(destination)
    temporary = destination.with_name(destination.name + '.tmp.' + str(os.getpid()))
    try:
        temporary.write_text(json.dumps({'url': origin}), encoding='utf-8')
        os.replace(temporary, destination)
    finally:
        temporary.unlink(missing_ok=True)


def stop_child(child, timeout=5):
    """Signal only this invocation's child/session, then reap it with a deadline."""
    if child.poll() is not None:
        child.wait()
        return

    def send(sig):
        try:
            if os.name == 'posix' and os.getpgid(child.pid) == child.pid:
                os.killpg(child.pid, sig)
            elif sig == signal.SIGTERM:
                child.terminate()
            else:
                child.kill()
        except ProcessLookupError:
            pass

    send(signal.SIGTERM)
    try:
        child.wait(timeout=timeout)
    except subprocess.TimeoutExpired:
        send(signal.SIGKILL if os.name == 'posix' else signal.SIGTERM)
        if os.name != 'posix':
            child.kill()
        child.wait(timeout=timeout)


def supervise(destination=DESTINATION, command=COMMAND, stop=None, health=probe_https,
              interval=15, probe_timeout=8, startup_timeout=120,
              failure_limit=3, min_restart_interval=60, poll_interval=0.2,
              shutdown_timeout=5, log=None):
    """Return 0 for requested stop, 1 for systemd-managed reconstruction.

    A DNS resolver can outlive a socket timeout. Only one daemon probe is
    allowed at a time; its overall deadline ends this invocation rather than
    accumulating unbounded probe threads while continuing to advertise.
    """
    stop = stop or threading.Event()
    log = log or (lambda message: print(message, flush=True))
    started = time.monotonic()
    child = None
    reader = None
    reason = ''
    try:
        publish(destination, '')
        if stop.is_set():
            return 0
        child = subprocess.Popen(command, stdout=subprocess.PIPE,
                                 stderr=subprocess.STDOUT, text=True,
                                 encoding='utf-8', errors='replace',
                                 start_new_session=os.name == 'posix')
        candidates = queue.Queue(maxsize=1)

        def read_output():
            for line in child.stdout:
                log('cloudflared: ' + line.rstrip())
                match = TUNNEL_URL.search(line)
                if match:
                    try:
                        candidates.get_nowait()
                    except queue.Empty:
                        pass
                    candidates.put_nowait(match[0])

        reader = threading.Thread(target=read_output, daemon=True)
        reader.start()
        candidate, advertised = '', ''
        failures, ever_ready = 0, False
        result = queue.Queue(maxsize=1)
        probing = False
        next_probe = time.monotonic()
        deadline = 0
        while not stop.is_set():
            now = time.monotonic()
            if child.poll() is not None:
                reason = 'cloudflared exited (status %s)' % child.returncode
                break
            try:
                discovered = candidates.get_nowait()
                if discovered != candidate:
                    candidate = discovered
                    if advertised:
                        publish(destination, '')
                        advertised = ''
                    next_probe = now
            except queue.Empty:
                pass
            if not ever_ready and now - started >= startup_timeout:
                reason = 'startup health deadline exceeded'
                break
            if probing:
                try:
                    checked, healthy = result.get_nowait()
                    probing = False
                    if checked == candidate:
                        if healthy:
                            failures, ever_ready = 0, True
                            if advertised != candidate:
                                publish(destination, candidate)
                                advertised = candidate
                                log('Linux direct URL verified: ' + candidate)
                        else:
                            failures += 1
                            if advertised:
                                publish(destination, '')
                                advertised = ''
                            log('Linux direct HTTPS health failed (%d/%d)' % (failures, failure_limit))
                            if ever_ready and failures >= failure_limit:
                                reason = 'consecutive HTTPS health failures'
                                break
                    next_probe = now + interval
                except queue.Empty:
                    if now >= deadline:
                        reason = 'HTTPS health probe deadline exceeded'
                        break
            if candidate and not probing and now >= next_probe:
                checking = candidate

                def check(origin=checking):
                    try:
                        healthy = bool(health(origin, probe_timeout))
                    except Exception:
                        healthy = False
                    result.put((origin, healthy))

                probing = True
                deadline = now + probe_timeout
                threading.Thread(target=check, daemon=True).start()
            stop.wait(poll_interval)
    except Exception as error:
        reason = 'tunnel supervisor failed: ' + type(error).__name__
    finally:
        # Clear before waiting for termination or a restart delay.
        try:
            publish(destination, '')
        except OSError as error:
            log('Could not clear direct advertisement: ' + type(error).__name__)
        try:
            if child is not None:
                stop_child(child, shutdown_timeout)
        except (OSError, subprocess.TimeoutExpired) as error:
            log('Tunnel child cleanup failed: ' + type(error).__name__)
        if reader is not None:
            reader.join(timeout=1)
        if child is not None and child.stdout is not None and not (reader and reader.is_alive()):
            child.stdout.close()
    if stop.is_set():
        return 0
    log('Linux direct unavailable: ' + (reason or 'unknown failure'))
    delay = max(0, min_restart_interval - (time.monotonic() - started))
    if delay:
        log('Waiting %.1fs before allowing systemd restart' % delay)
        stop.wait(delay)
    return 0 if stop.is_set() else 1


def main():
    stop = threading.Event()

    def request_stop(*_args):
        stop.set()

    signal.signal(signal.SIGTERM, request_stop)
    signal.signal(signal.SIGINT, request_stop)
    return supervise(stop=stop)


if __name__ == '__main__':
    raise SystemExit(main())
