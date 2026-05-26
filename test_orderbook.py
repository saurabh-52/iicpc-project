#!/usr/bin/env python3
"""Minimal orderbook engine for sandbox testing.

Listens on port 8080 and responds with JSON acknowledgments
for every incoming order (POST) or health check (GET).
"""

import http.server
import json

PORT = 8080


class OrderBookHandler(http.server.BaseHTTPRequestHandler):
    """Simple HTTP handler that ACKs every request."""

    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        order = {}
        if body:
            try:
                order = json.loads(body)
            except json.JSONDecodeError:
                pass

        response = json.dumps(
            {"status": "ok", "action": order.get("action", "unknown")}
        ).encode()

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(response)))
        self.end_headers()
        self.wfile.write(response)

    def do_GET(self):
        response = json.dumps({"status": "ok"}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(response)))
        self.end_headers()
        self.wfile.write(response)

    def log_message(self, format, *args):
        # Suppress per-request logging to avoid I/O bottleneck during stress tests
        pass


if __name__ == "__main__":
    with http.server.HTTPServer(("0.0.0.0", PORT), OrderBookHandler) as server:
        print("Orderbook Engine Initialized...")
        print(f"Listening on 0.0.0.0:{PORT} - Waiting for orders...")
        server.serve_forever()