#!/usr/bin/env python3
"""Serve the Next.js static export the way the deployment serves it.

In production Caddy reverse-proxies every path to PocketBase, which serves the
export and resolves an extensionless path to its `.html` file. `python3 -m
http.server` does not: it sees the `out/muted/` directory that holds the RSC
payloads and answers `/muted` with a directory listing instead of `muted.html`.

That difference is invisible to a chief editor and fatal to a browser test:
every side screen is a real bookmarkable route, so a cold load of `/muted` has
to return the page.

Usage: serve-static.py <port> <directory>
"""

import os
import sys
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer


class ExportHandler(SimpleHTTPRequestHandler):
    def translate_path(self, path):
        resolved = super().translate_path(path)
        # A request for a real file or for the site root is already correct.
        if os.path.isfile(resolved):
            return resolved
        candidate = resolved.rstrip("/") + ".html"
        if os.path.isfile(candidate):
            return candidate
        index = os.path.join(resolved, "index.html")
        if os.path.isfile(index):
            return index
        return resolved

    def log_message(self, *args):
        pass


def main():
    port = int(sys.argv[1])
    directory = sys.argv[2]
    handler = partial(ExportHandler, directory=directory)
    ThreadingHTTPServer(("127.0.0.1", port), handler).serve_forever()


if __name__ == "__main__":
    main()
