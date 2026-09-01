#!/usr/bin/env python3
import argparse
import http.server
import json
import os
import re
import socketserver
import sys
import time
from pathlib import Path
from urllib.parse import urlparse

PS_RE = re.compile(r"(PlayStation\s*4|PlayStation|PS4)", re.I)

class Ron1nHandler(http.server.SimpleHTTPRequestHandler):
    server_version = "ron1n/1.3.0"

    def log_message(self, fmt, *args):
        # Keep normal HTTP logs in the journal, but make them concise.
        msg = "%s - %s" % (self.client_address[0], fmt % args)
        print(msg, flush=True)

    def _state_paths(self):
        state_dir = Path(self.server.state_dir)
        return (
            state_dir / "console.json",
            state_dir / "events.log",
            state_dir / "ps4.ip",
        )

    def _record(self, path):
        ua = self.headers.get("User-Agent", "")
        ip = self.client_address[0]
        now = int(time.time())
        is_ps4 = bool(PS_RE.search(ua))

        console_json, events_log, ip_file = self._state_paths()

        event = {
            "ts": now,
            "ip": ip,
            "path": path,
            "ua": ua,
            "is_ps4": is_ps4,
        }

        if is_ps4:
            stage = "browser"
            lower = path.lower()

            if "goldhen.bin" in lower:
                stage = "goldhen-payload-served"
            elif "lapse" in lower:
                stage = "kernel-exploit-activity"
            elif "psfree" in lower:
                stage = "webkit-exploit-activity"
            elif "cache" in lower:
                stage = "offline-cache"

            event["stage"] = stage

            tmp = console_json.with_suffix(".tmp")
            tmp.write_text(json.dumps(event, indent=2))
            tmp.replace(console_json)
            ip_file.write_text(ip + "\n")

        with events_log.open("a", encoding="utf-8") as f:
            f.write(json.dumps(event, separators=(",", ":")) + "\n")

    def do_GET(self):
        path = urlparse(self.path).path
        self._record(path)

        if path == "/_ron1n/status":
            console_json, _, _ = self._state_paths()
            payload = {}
            if console_json.exists():
                try:
                    payload = json.loads(console_json.read_text())
                except Exception:
                    payload = {}
            data = json.dumps(payload, indent=2).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Cache-Control", "no-store")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return

        super().do_GET()

    def do_HEAD(self):
        self._record(urlparse(self.path).path)
        super().do_HEAD()


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--port", type=int, default=8080)
    p.add_argument("--bind", default="0.0.0.0")
    p.add_argument("--directory", required=True)
    p.add_argument("--state-dir", required=True)
    args = p.parse_args()

    Path(args.state_dir).mkdir(parents=True, exist_ok=True)

    handler = lambda *a, **kw: Ron1nHandler(
        *a, directory=args.directory, **kw
    )

    class ThreadingServer(socketserver.ThreadingTCPServer):
        allow_reuse_address = True
        daemon_threads = True

    with ThreadingServer((args.bind, args.port), handler) as httpd:
        httpd.state_dir = args.state_dir
        print(
            f"ron1n HTTP host listening on {args.bind}:{args.port}",
            flush=True,
        )
        httpd.serve_forever()


if __name__ == "__main__":
    main()
