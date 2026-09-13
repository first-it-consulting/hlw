#!/usr/bin/env python3
"""Minimal OpenAI-compatible /v1/models endpoint for the demo recording.

Keeps the recording reproducible and independent of whatever models happen to
be pulled on the machine doing the recording.

    python3 demo/models-server.py &
    vhs demo/hlw.tape        # or: make -C demo gif
"""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

MODELS = [
    ("qwen3-coder-30b", 262144),
    ("deepseek-v3.2", 163840),
    ("llama3.3-70b", 131072),
    ("devstral-small", 131072),
    ("gpt-oss-120b", 131072),
    ("mistral-large", 131072),
    ("gemma3-27b", 131072),
    ("phi-4", 16384),
]


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        body = json.dumps({
            "object": "list",
            "data": [
                {"id": name, "object": "model", "owned_by": "demo", "max_model_len": ctx}
                for name, ctx in MODELS
            ],
        }).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    HTTPServer(("127.0.0.1", 8765), Handler).serve_forever()
