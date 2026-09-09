#!/usr/bin/env python3
"""kekkai-mpr: in-sandbox model-provider request capture (specs/025).

Modes:
  serve   loopback reverse proxy 127.0.0.1:4141 -> https://api.anthropic.com.
          Forwards everything verbatim (auth headers included, never recorded)
          and publishes one JSON event per request and per completed response
          to subscribers on an abstract unix socket. Live-only: no subscriber,
          no storage. Capture failures never affect forwarding.
  wait    readiness probe for the container CMD (exit 1 after 5s).
  follow  subscriber relaying event lines to stdout; reconnects for 10s when
          the proxy restarts. `kekkai mpr` runs this via docker exec.

stdlib only; contract in specs/025-mpr-inspect/contracts/mpr-wire.md.
"""
import collections
import http.client
import http.server
import json
import socket
import sys
import threading
import time
from datetime import datetime, timezone

UPSTREAM = "api.anthropic.com"
LISTEN = ("127.0.0.1", 4141)
SOCKET_NAME = b"\0kekkai-mpr"
SEQ_FILE = "/dev/shm/kekkai-mpr.seq"
BODY_CAP = 16 * 1024 * 1024
QUEUE_LEN = 64
UPSTREAM_TIMEOUT = 600  # streamed replies can run for minutes
WAIT_TIMEOUT = 5.0
FOLLOW_RETRY = 10.0

HOP_BY_HOP = {
    "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
    "te", "trailer", "transfer-encoding", "upgrade",
}
# accept-encoding is dropped so upstream answers identity-encoded: the
# capture reads bodies as-is and the client never receives an encoding it
# did not negotiate with us.
DROP_REQUEST = HOP_BY_HOP | {"host", "accept-encoding"}
STALE = (
    http.client.RemoteDisconnected, http.client.CannotSendRequest,
    http.client.ResponseNotReady, ConnectionResetError, BrokenPipeError,
)


def now_iso():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


def text(buf):
    return bytes(buf).decode("utf-8", errors="replace")


# --- capture: sequence + pub/sub --------------------------------------------

class Sequence:
    """Exchange counter persisted in tmpfs so a restarted proxy keeps counting."""

    def __init__(self):
        self.lock = threading.Lock()
        try:
            with open(SEQ_FILE) as f:
                self.value = int(f.read().strip() or 0)
        except (OSError, ValueError):
            self.value = 0

    def next(self):
        with self.lock:
            self.value += 1
            try:
                with open(SEQ_FILE, "w") as f:
                    f.write(str(self.value))
            except OSError:
                pass
            return self.value


class Subscriber:
    """One follow client: bounded queue, drop-oldest, drop marker before the
    next delivered event."""

    def __init__(self, conn, on_close):
        self.conn = conn
        self.on_close = on_close
        self.queue = collections.deque()
        self.dropped = 0
        self.cv = threading.Condition()
        self.alive = True
        threading.Thread(target=self._writer, daemon=True).start()
        threading.Thread(target=self._watch_eof, daemon=True).start()

    def offer(self, line):
        with self.cv:
            if len(self.queue) >= QUEUE_LEN:
                self.queue.popleft()
                self.dropped += 1
            self.queue.append(line)
            self.cv.notify()

    def close(self):
        with self.cv:
            if not self.alive:
                return
            self.alive = False
            self.cv.notify()
        try:
            self.conn.close()
        except OSError:
            pass
        self.on_close(self)

    def _writer(self):
        while True:
            with self.cv:
                while self.alive and not self.queue:
                    self.cv.wait()
                if not self.alive:
                    return
                marker = None
                if self.dropped:
                    marker = json.dumps({"type": "dropped", "count": self.dropped}) + "\n"
                    self.dropped = 0
                line = self.queue.popleft()
            try:
                if marker:
                    self.conn.sendall(marker.encode())
                self.conn.sendall(line.encode())
            except OSError:
                self.close()
                return

    def _watch_eof(self):
        # A reader that hangs up while the sandbox is idle would otherwise
        # linger until the next publish fails.
        try:
            while self.conn.recv(1024):
                pass
        except OSError:
            pass
        self.close()


class Publisher:
    def __init__(self):
        self.lock = threading.Lock()
        self.subs = []
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.bind(SOCKET_NAME)
        self.sock.listen(8)
        threading.Thread(target=self._accept, daemon=True).start()

    def _accept(self):
        while True:
            try:
                conn, _ = self.sock.accept()
            except OSError:
                return
            sub = Subscriber(conn, self._remove)
            with self.lock:
                self.subs.append(sub)

    def _remove(self, sub):
        with self.lock:
            if sub in self.subs:
                self.subs.remove(sub)

    def publish(self, event):
        with self.lock:
            subs = list(self.subs)
        if not subs:
            return  # nobody listening: discard, never accumulate
        line = json.dumps(event, ensure_ascii=False, separators=(",", ":")) + "\n"
        for sub in subs:
            sub.offer(line)


SEQ = None
PUB = None


def publish(event):
    try:
        PUB.publish(event)
    except Exception:
        pass  # capture must never affect forwarding


# --- proxy -------------------------------------------------------------------

_local = threading.local()


def upstream_request(method, path, headers, body):
    """Send on this thread's kept-alive upstream connection, reconnecting
    once when it went stale. Returns the HTTPResponse."""
    for attempt in (1, 2):
        conn = getattr(_local, "conn", None)
        if conn is None:
            conn = http.client.HTTPSConnection(UPSTREAM, 443, timeout=UPSTREAM_TIMEOUT)
            _local.conn = conn
        try:
            conn.request(method, path, body=body, headers=headers)
            return conn.getresponse()
        except STALE:
            drop_upstream()
            if attempt == 2:
                raise


def drop_upstream():
    conn = getattr(_local, "conn", None)
    _local.conn = None
    if conn is not None:
        try:
            conn.close()
        except OSError:
            pass


class Proxy(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *args):
        pass  # stdout/stderr is claude's terminal; stay silent

    def _forward_headers(self):
        out = {}
        for k, v in self.headers.items():
            if k.lower() in DROP_REQUEST:
                continue
            out[k] = out[k] + ", " + v if k in out else v
        out["Host"] = UPSTREAM
        return out

    def _read_body(self):
        if "chunked" in self.headers.get("Transfer-Encoding", "").lower():
            data = bytearray()
            while True:
                size = int(self.rfile.readline().split(b";")[0].strip() or b"0", 16)
                if size == 0:
                    while self.rfile.readline() not in (b"\r\n", b"\n", b""):
                        pass
                    return bytes(data)
                data += self.rfile.read(size)
                self.rfile.readline()
        length = int(self.headers.get("Content-Length") or 0)
        if length:
            return self.rfile.read(length)
        return None if self.command in ("GET", "HEAD", "DELETE", "OPTIONS") else b""

    def _proxy(self):
        started = time.monotonic()
        body = self._read_body()
        ex_id = SEQ.next()
        req_capture = body or b""
        publish({
            "type": "request", "id": ex_id, "ts": now_iso(),
            "method": self.command, "path": self.path,
            "body": text(req_capture[:BODY_CAP]), "truncated": len(req_capture) > BODY_CAP,
        })
        try:
            resp = upstream_request(self.command, self.path, self._forward_headers(), body)
        except Exception as e:  # noqa: BLE001 - any upstream failure is a 502
            self._upstream_error(ex_id, started, "%s: %s" % (type(e).__name__, e))
            return

        self.send_response_only(resp.status, resp.reason)
        content_length = None
        content_type = ""
        for k, v in resp.getheaders():
            kl = k.lower()
            if kl in HOP_BY_HOP:
                continue
            if kl == "content-length":
                content_length = int(v)
            elif kl == "content-type":
                content_type = v
            self.send_header(k, v)
        has_body = not (self.command == "HEAD" or resp.status in (204, 304) or resp.status < 200)
        chunked = has_body and content_length is None
        if chunked:
            self.send_header("Transfer-Encoding", "chunked")
        self.end_headers()

        captured = bytearray()
        truncated = False
        aborted = False
        try:
            while has_body:
                chunk = resp.read1(65536)
                if not chunk:
                    break
                if chunked:
                    self.wfile.write(b"%x\r\n%s\r\n" % (len(chunk), chunk))
                else:
                    self.wfile.write(chunk)
                self.wfile.flush()
                if len(captured) < BODY_CAP:
                    captured += chunk[:BODY_CAP - len(captured)]
                else:
                    truncated = True
            if chunked:
                self.wfile.write(b"0\r\n\r\n")
                self.wfile.flush()
        except OSError:
            # Client went away mid-stream: the upstream response is now
            # unreadable, so the kept-alive connection is unusable.
            aborted = True
            drop_upstream()
            self.close_connection = True
        if not has_body:
            resp.read()
        publish({
            "type": "response", "id": ex_id, "ts": now_iso(),
            "status": resp.status, "content_type": content_type,
            "duration_ms": int((time.monotonic() - started) * 1000),
            "body": text(captured), "truncated": truncated or aborted,
        })

    def _upstream_error(self, ex_id, started, reason):
        payload = json.dumps({"type": "error", "error": {
            "type": "kekkai_mpr_upstream_error", "message": reason}}).encode()
        try:
            self.send_response_only(502, "Bad Gateway")
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
        except OSError:
            self.close_connection = True
        publish({
            "type": "response", "id": ex_id, "ts": now_iso(),
            "status": 502, "content_type": "application/json",
            "duration_ms": int((time.monotonic() - started) * 1000),
            "body": payload.decode(), "truncated": False,
        })

    do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = do_HEAD = do_OPTIONS = _proxy


class Server(http.server.ThreadingHTTPServer):
    daemon_threads = True
    allow_reuse_address = True

    def handle_error(self, request, client_address):
        pass  # never print tracebacks over claude's TUI


def serve():
    global SEQ, PUB
    SEQ = Sequence()
    PUB = Publisher()
    server = Server(LISTEN, Proxy)
    print("[kekkai] model-provider capture active (kekkai mpr)", flush=True)
    server.serve_forever()


def wait():
    deadline = time.monotonic() + WAIT_TIMEOUT
    while time.monotonic() < deadline:
        try:
            socket.create_connection(LISTEN, timeout=0.5).close()
            return 0
        except OSError:
            time.sleep(0.1)
    print("[kekkai] ERROR: mpr proxy failed to start", file=sys.stderr)
    return 1


def follow():
    out = sys.stdout.buffer
    first = True
    while True:
        deadline = time.monotonic() + FOLLOW_RETRY
        sock = None
        while sock is None:
            try:
                sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                sock.connect(SOCKET_NAME)
            except OSError:
                sock = None
                if time.monotonic() >= deadline:
                    print("mpr proxy unavailable", file=sys.stderr)
                    return 1
                time.sleep(0.25)
        if not first:
            out.write((json.dumps({"type": "notice", "ts": now_iso(),
                                   "text": "mpr proxy restarted, reattached"}) + "\n").encode())
            out.flush()
        first = False
        with sock.makefile("rb") as f:
            for line in f:
                out.write(line)
                out.flush()
        sock.close()


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else ""
    if mode == "serve":
        serve()
        return 0
    if mode == "wait":
        return wait()
    if mode == "follow":
        return follow()
    print("usage: kekkai-mpr serve|wait|follow", file=sys.stderr)
    return 2


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(0)
