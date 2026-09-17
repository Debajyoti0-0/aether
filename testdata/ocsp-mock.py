"""Stage 27 OCSP/CRL mock: serves testdata files for GET and POST (OCSP responders require POST)."""
import os
from http.server import BaseHTTPRequestHandler, HTTPServer

DATA = os.path.dirname(os.path.abspath(__file__))

class H(BaseHTTPRequestHandler):
    def _serve(self):
        name = self.path.strip('/').split('?')[0]
        p = os.path.join(DATA, name)
        if not os.path.isfile(p):
            self.send_response(404); self.end_headers(); return
        body = open(p, 'rb').read()
        self.send_response(200)
        self.send_header('Content-Type', 'application/ocsp-response' if name.endswith('.der') else 'application/pkix-crl')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self): self._serve()

    def do_POST(self):
        n = int(self.headers.get('Content-Length', 0) or 0)
        if n: self.rfile.read(n)
        self._serve()

    def log_message(self, *a): pass

HTTPServer(('127.0.0.1', 9999), H).serve_forever()
