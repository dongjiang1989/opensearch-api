#!/usr/bin/env python3
"""Mock embedding server that simulates OpenAI-compatible /v1/embeddings API.

Returns deterministic dummy vectors (all 0.01) for any input text.
Supports both OpenAI format and batch requests.
"""
import http.server
import json
import hashlib

PORT = 11435
VECTOR_DIM = 1536


def deterministic_vector(text: str, dim: int) -> list[float]:
    """Generate a deterministic vector from input text."""
    seed = int(hashlib.md5(text.encode()).hexdigest(), 16)
    rng = seed
    vec = []
    for _ in range(dim):
        rng = (rng * 1103515245 + 12345) & 0x7FFFFFFF
        vec.append(round((rng / 0x7FFFFFFF) * 2 - 1, 6))
    return vec


class EmbeddingHandler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        req = json.loads(body)

        inputs = req.get("input", "")
        model = req.get("model", "mock-embedding")

        if isinstance(inputs, str):
            inputs = [inputs]

        data = []
        for i, text in enumerate(inputs):
            vec = deterministic_vector(text, VECTOR_DIM)
            data.append({
                "object": "embedding",
                "index": i,
                "embedding": vec,
            })

        resp = {
            "object": "list",
            "data": data,
            "model": model,
            "usage": {
                "prompt_tokens": sum(len(t.split()) for t in inputs),
                "total_tokens": sum(len(t.split()) for t in inputs),
            },
        }

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(resp).encode())

    def log_message(self, format, *args):
        pass  # silence logs


if __name__ == "__main__":
    server = http.server.HTTPServer(("0.0.0.0", PORT), EmbeddingHandler)
    print(f"Mock embedding server running on port {PORT}")
    server.serve_forever()
