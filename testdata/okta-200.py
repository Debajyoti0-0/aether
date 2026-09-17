"""Stage 27 Okta 200-mock: identity endpoints used by pkg/plugins/okta + CLI validateOkta."""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

def body(obj):
    return json.dumps(obj).encode()

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        auth = self.headers.get('Authorization', '')
        if self.path == '/api/v1/users/me' or self.path.startswith('/api/v1/users?') or self.path == '/api/v1/users':
            if not auth.startswith('SSWS '):
                self.send_response(401); self.end_headers(); return
            me = {"id": "00u_test", "status": "ACTIVE",
                  "profile": {"login": "test@example.com", "email": "test@example.com"}}
            if self.path == '/api/v1/users/me':
                payload = body(me)
            else:
                payload = body([{"id": "00u_test", "login": "test@example.com", "status": "ACTIVE"}])
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers(); self.wfile.write(payload)
        elif self.path == '/.well-known/openid-configuration':
            payload = body({"issuer": "http://127.0.0.1:8081",
                            "jwks_uri": "http://127.0.0.1:8081/.well-known/jwks.json"})
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers(); self.wfile.write(payload)
        else:
            self.send_response(404); self.end_headers()

    def log_message(self, *a): pass

HTTPServer(('127.0.0.1', 8081), H).serve_forever()
