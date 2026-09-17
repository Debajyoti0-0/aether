"""Stage 27 Kubernetes 200-mock: SelfSubjectReview POST + namespaces GET."""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

def body(obj):
    return json.dumps(obj).encode()

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        if not self.headers.get('Authorization', '').startswith('Bearer '):
            self.send_response(401); self.end_headers(); return
        if self.path == '/api/v1/namespaces':
            payload = body({"kind": "NamespaceList",
                            "items": [{"metadata": {"name": "default"}}]})
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers(); self.wfile.write(payload)
        else:
            self.send_response(404); self.end_headers()

    def do_POST(self):
        if not self.headers.get('Authorization', '').startswith('Bearer '):
            self.send_response(401); self.end_headers(); return
        n = int(self.headers.get('Content-Length', 0) or 0)
        if n: self.rfile.read(n)
        if self.path == '/apis/authentication.k8s.io/v1/selfsubjectreviews':
            payload = body({"apiVersion": "authentication.k8s.io/v1", "kind": "SelfSubjectReview",
                            "status": {"userInfo": {"username": "test", "uid": "00u_test"}}})
            self.send_response(201)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers(); self.wfile.write(payload)
        else:
            self.send_response(404); self.end_headers()

    def log_message(self, *a): pass

HTTPServer(('127.0.0.1', 8083), H).serve_forever()
