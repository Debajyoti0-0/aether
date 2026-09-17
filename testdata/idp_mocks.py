import http.server
import socketserver
import json

class OktaHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/api/v1/users/me":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"id":"test","profile":{"login":"test@example.com","email":"test@example.com"}}')
        elif self.path == "/.well-known/openid-configuration":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"issuer":"http://localhost:8081","authorization_endpoint":"http://localhost:8081/oauth2/v1/authorize","token_endpoint":"http://localhost:8081/oauth2/v1/token","jwks_uri":"http://localhost:8081/oauth2/v1/keys","userinfo_endpoint":"http://localhost:8081/oauth2/v1/userinfo","scopes_supported":["openid","email","profile"],"response_types_supported":["code","token","id_token"],"subject_types_supported":["public"],"id_token_signing_alg_values_supported":["RS256"]}')
        else:
            self.send_response(404)
            self.end_headers()
    
    def log_message(self, format, *args):
        pass

class GitLabHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/api/v4/user":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"id":1,"username":"testuser","name":"Test User","email":"test@example.com"}')
        elif self.path == "/api/v4/version":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"version":"15.0.0","revision":"abc123"}')
        else:
            self.send_response(404)
            self.end_headers()
    
    def log_message(self, format, *args):
        pass

class KubernetesHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/api/v1/namespaces":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"kind":"NamespaceList","items":[{"metadata":{"name":"default"}},{"metadata":{"name":"kube-system"}}]}')
        elif self.path == "/version":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"major":"1","minor":"28","gitVersion":"v1.28.0","gitCommit":"abc123","gitTreeState":"clean","buildDate":"2023-01-01T00:00:00Z","goVersion":"go1.20.0","compiler":"gc","platform":"linux/amd64"}')
        else:
            self.send_response(404)
            self.end_headers()
    
    def log_message(self, format, *args):
        pass

if __name__ == "__main__":
    import socketserver
    import sys
    
    handlers = {
        8081: OktaHandler,
        8082: GitLabHandler,
        8083: KubernetesHandler,
    }
    
    servers = []
    for port, handler in handlers.items():
        server = socketserver.TCPServer(("127.0.0.1", port), handler)
        servers.append(server)
    
    print("Starting mock servers on ports 8081 (Okta), 8082 (GitLab), 8083 (Kubernetes)")
    try:
        for server in servers:
            server.serve_forever()
    except KeyboardInterrupt:
        for server in servers:
            server.shutdown()
        print("Servers stopped")