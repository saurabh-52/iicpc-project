from http.server import BaseHTTPRequestHandler, HTTPServer
import json

class RequestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        self._send_response()

    def do_POST(self):
        self._send_response()

    def _send_response(self):
        self.send_response(200)
        self.send_header('Content-type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps({"status": "ok"}).encode('utf-8'))

    def log_message(self, format, *args):
        # Suppress logging to avoid cluttering stdout during stress testing
        pass

def run(server_class=HTTPServer, handler_class=RequestHandler, port=8080):
    server_address = ('0.0.0.0', port)
    httpd = server_class(server_address, handler_class)
    print("Orderbook Engine Initialized...")
    print(f"Listening on 0.0.0.0:{port} - Waiting for orders...")
    httpd.serve_forever()

if __name__ == '__main__':
    run()
