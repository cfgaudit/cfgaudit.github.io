# Copyright the cfgaudit contributors
# SPDX-License-Identifier: Apache-2.0
"""Serve the built site for the checks the way GitHub Pages does (#34).

    serve.py DIR PORT

python3 -m http.server sends every file uncompressed. GitHub Pages compresses
HTML, CSS and SVG with gzip, so Lighthouse measured /progress/ (441 KB of
HTML, 34 KB live) far slower than any visitor sees it. This server gzips
those types when the client asks for it and serves everything else as is.
Standard library only.
"""
import gzip
import http.server
import sys
from functools import partial

COMPRESS = ("text/html", "text/css", "image/svg+xml", "application/xml", "text/xml", "text/plain")


class Handler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        path = self.translate_path(self.path)
        if self.path.endswith("/") or not self.path.rsplit("/", 1)[-1].count("."):
            index = path.rstrip("/") + "/index.html"
            if not self.path.endswith("/"):
                return super().do_GET()  # the parent redirects to the slash form
            path = index
        ctype = self.guess_type(path)
        if "gzip" not in self.headers.get("Accept-Encoding", "") or not ctype.startswith(COMPRESS):
            return super().do_GET()
        try:
            with open(path, "rb") as f:
                body = gzip.compress(f.read(), compresslevel=6)
        except OSError:
            return super().do_GET()
        self.send_response(200)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Encoding", "gzip")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Vary", "Accept-Encoding")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    directory, port = sys.argv[1], int(sys.argv[2])
    http.server.ThreadingHTTPServer(("127.0.0.1", port), partial(Handler, directory=directory)).serve_forever()
