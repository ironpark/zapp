"""Accepts PUT uploads into a directory, for testing the zapp action.

Usage: put-server.py PORT DIRECTORY TOKEN
"""
import http.server
import os
import sys
import urllib.parse

port, directory, token = int(sys.argv[1]), sys.argv[2], sys.argv[3]


class Handler(http.server.BaseHTTPRequestHandler):
    def do_PUT(self):
        if self.headers.get("Authorization") != "Bearer " + token:
            self.send_response(401)
            self.end_headers()
            return
        data = self.rfile.read(int(self.headers["Content-Length"]))
        name = os.path.basename(urllib.parse.unquote(self.path.split("?")[0]))
        with open(os.path.join(directory, name), "wb") as f:
            f.write(data)
        self.send_response(201)
        self.end_headers()


os.makedirs(directory, exist_ok=True)
http.server.HTTPServer(("127.0.0.1", port), Handler).serve_forever()
