import http.server
import socketserver
import json

PORT = 8080

class MyHandler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        response = {
            "status": "ok",
            "best_bid": 100.0,
            "best_ask": 101.0
        }
        self.wfile.write(json.dumps(response).encode('utf-8'))

    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-Type', 'text/plain')
        self.end_headers()
        self.wfile.write(b"OK")

handler = MyHandler
with socketserver.TCPServer(("", PORT), handler) as httpd:
    print("serving at port", PORT)
    httpd.serve_forever()
