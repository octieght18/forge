#!/usr/bin/env python3
"""Hold only native IPv4 loopback port 8081 until the parent releases stdin."""
import socket
import sys

with socket.socket() as listener:
    listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    listener.bind(("127.0.0.1", 8081))
    listener.listen(1)
    print("READY", flush=True)
    # EOF also releases the socket if the measurement process exits unexpectedly.
    sys.stdin.readline()
