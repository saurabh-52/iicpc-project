#!/usr/bin/env python3
import socket
import threading

HOST = '0.0.0.0'
PORT = 9090

def handle_client(conn, addr):
    try:
        with conn:
            buf = b''
            while True:
                data = conn.recv(1024)
                if not data:
                    break
                buf += data
                # process lines
                while b'\n' in buf:
                    line, buf = buf.split(b'\n', 1)
                    # simple echo or ACK
                    try:
                        conn.sendall(b'OK\n')
                    except Exception:
                        return
    except Exception:
        pass

with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
    s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    s.bind((HOST, PORT))
    s.listen()
    print(f"TCP server listening on {HOST}:{PORT}")
    while True:
        conn, addr = s.accept()
        t = threading.Thread(target=handle_client, args=(conn, addr), daemon=True)
        t.start()
