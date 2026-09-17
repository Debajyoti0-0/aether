"""Stage 27 GitLab 200-mock: /api/v4/user for PRIVATE-TOKEN or Bearer auth."""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        auth = self.headers.get('PRIVATE-TOKEN', '') or self.headers.get('Authorization', '')
        if self.path == '/api/v4/user':
            if not auth:
                self.send_response(401); self.end_headers(); return
            payload = json.dumps({"id": 1, "username": "test", "name": "Test User"}).encode()
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers(); self.wfile.write(payload)
        else:
            self.send_response(404); self.end_headers()

    def log_message(self, *a): pass

HTTPServer(('127.0.0.1', 8082), H).serve_forever()
