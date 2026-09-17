"""Stage 28 OCSP mock responder: serves an OCSP response fixture.

Usage: python3 testdata/ocsp_responder.py <port> <fixture.der>
Example: python3 testdata/ocsp_responder.py 9999 testdata/ocsp-good.der

Accepts both GET and POST (the Go checker POSTs); returns the fixture
bytes verbatim regardless of the request CertID.
"""
import http.server
import socketserver
import sys

class OCSPResponder(http.server.BaseHTTPRequestHandler):
    fixture = "testdata/ocsp-good.der"

    def _serve(self):
        self.send_response(200)
        self.send_header("Content-Type", "application/ocsp-response")
        self.end_headers()
        with open(self.fixture, "rb") as f:
            self.wfile.write(f.read())

    def do_POST(self):
        self._serve()

    def do_GET(self):
        self._serve()

    def log_message(self, format, *args):
        pass

if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 9999
    if len(sys.argv) > 2:
        OCSPResponder.fixture = sys.argv[2]
    with socketserver.TCPServer(("127.0.0.1", port), OCSPResponder) as httpd:
        print(f"OCSP responder on 127.0.0.1:{port} serving {OCSPResponder.fixture}", file=sys.stderr)
        httpd.serve_forever()
