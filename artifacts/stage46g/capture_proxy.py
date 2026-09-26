#!/usr/bin/env python3
# Stage 46g wire capture proxy: LDAP (and optionally KDC) traffic
# Listens on 127.0.0.1:<listen>, forwards to <target>, hex-dumps both directions.
import socket, threading, sys, time

LISTEN = int(sys.argv[1]) if len(sys.argv) > 1 else 3890
TARGET = (sys.argv[2] if len(sys.argv) > 2 else "172.18.0.2", int(sys.argv[3]) if len(sys.argv) > 3 else 389)
LOG = open(f"/tmp/capture-{LISTEN}.log", "w", buffering=1)
lock = threading.Lock()

def dump(tag, data):
    with lock:
        ts = time.time()
        LOG.write(f"[{ts:.3f}] {tag} {len(data)} bytes\n")
        for i in range(0, len(data), 16):
            chunk = data[i:i+16]
            hexs = " ".join(f"{b:02x}" for b in chunk)
            asc = "".join(chr(b) if 32 <= b < 127 else "." for b in chunk)
            LOG.write(f"  {i:04x}  {hexs:<47}  {asc}\n")

def pipe(src, dst, tag):
    try:
        while True:
            data = src.recv(65536)
            if not data:
                dump(tag, b"")
                LOG.write(f"[{ts()}] {tag} EOF\n")
                break
            dump(tag, data)
            dst.sendall(data)
    except Exception as e:
        LOG.write(f"[{ts()}] {tag} ERROR {e}\n")
    try:
        dst.shutdown(socket.SHUT_WR)
    except Exception:
        pass

def ts():
    return time.time()

def handle(client):
    try:
        upstream = socket.create_connection(TARGET, timeout=10)
    except Exception as e:
        LOG.write(f"[{ts()}] UPSTREAM CONNECT ERROR {e}\n")
        client.close()
        return
    t1 = threading.Thread(target=pipe, args=(client, upstream, "C->S"), daemon=True)
    t2 = threading.Thread(target=pipe, args=(upstream, client, "S->C"), daemon=True)
    t1.start(); t2.start()
    t1.join(); t2.join()
    client.close(); upstream.close()

srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
srv.bind(("0.0.0.0", LISTEN))
srv.listen(5)
LOG.write(f"proxy listening on {LISTEN} -> {TARGET}\n")
while True:
    c, _ = srv.accept()
    threading.Thread(target=handle, args=(c,), daemon=True).start()
