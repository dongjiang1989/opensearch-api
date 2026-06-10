#!/usr/bin/env python3
"""Mock DashScope chat completions server (OpenAI-compatible).

Simulates the Qwen3.7-Plus document parsing API for E2E testing.
Supports:
- POST /api/v1/files — file upload (returns mock file_id)
- DELETE /api/v1/files/<id> — file deletion
- POST /v1/chat/completions — chat completions with file/image references
Returns deterministic extracted text based on content type.
"""
import http.server
import json
import base64
import uuid
import re

PORT = 11436

# In-memory file store: file_id → content_type
uploaded_files = {}

# Map content type patterns to mock extracted text
MOCK_RESPONSES = {
    "image/jpeg": "MOCK_IMAGE_OCR_RESULT: This is text extracted from a JPEG image. The image contains the words MOCK_JPEG_CONTENT.",
    "image/png": "MOCK_IMAGE_OCR_RESULT: This is text extracted from a PNG image. The image contains the words MOCK_PNG_CONTENT.",
    "image/gif": "MOCK_IMAGE_OCR_RESULT: This is text extracted from a GIF image. MOCK_GIF_CONTENT.",
    "image/webp": "MOCK_IMAGE_OCR_RESULT: This is text extracted from a WebP image. MOCK_WEBP_CONTENT.",
    "image/tiff": "MOCK_IMAGE_OCR_RESULT: This is text extracted from a TIFF image. MOCK_TIFF_CONTENT.",
    "image/bmp": "MOCK_IMAGE_OCR_RESULT: This is text extracted from a BMP image. MOCK_BMP_CONTENT.",
    "application/pdf": "MOCK_PDF_EXTRACTED: This is content extracted from a PDF document. The PDF contains paragraphs about MOCK_PDF_TOPIC including machine learning and neural networks.",
    "application/msword": "MOCK_DOC_EXTRACTED: This is content extracted from a Word document. The document discusses MOCK_DOCX_TOPIC with tables and formatting.",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": "MOCK_DOCX_EXTRACTED: This is content extracted from a Word DOCX document. The document covers MOCK_DOCX_SUBJECT with structured sections.",
    "application/vnd.ms-excel": "MOCK_XLS_EXTRACTED: This is content extracted from an Excel spreadsheet. Sheet1 contains MOCK_XLS_DATA with columns and rows.",
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": "MOCK_XLSX_EXTRACTED: This is content extracted from an Excel XLSX spreadsheet. The workbook contains MOCK_XLSX_DATA with formulas and values.",
    "application/vnd.ms-powerpoint": "MOCK_PPT_EXTRACTED: This is content extracted from a PowerPoint presentation. Slide 1 contains MOCK_PPT_CONTENT with bullet points.",
    "application/vnd.openxmlformats-officedocument.presentationml.presentation": "MOCK_PPTX_EXTRACTED: This is content extracted from a PowerPoint PPTX presentation. MOCK_PPTX_SLIDES with multiple slides.",
    "application/rtf": "MOCK_RTF_EXTRACTED: This is content extracted from an RTF document. MOCK_RTF_CONTENT with rich text formatting.",
    "application/epub+zip": "MOCK_EPUB_EXTRACTED: This is content extracted from an EPUB ebook. Chapter 1: MOCK_EPUB_CONTENT with narrative text.",
}

DEFAULT_RESPONSE = "MOCK_DEFAULT_EXTRACTED: This is extracted text content from the document."

request_log = []


def detect_content_type(content_items):
    """Detect content type from chat message content items."""
    for item in content_items:
        if isinstance(item, dict):
            # Check image_url type
            if item.get("type") == "image_url":
                url = item.get("image_url", {}).get("url", "")
                for ct in MOCK_RESPONSES:
                    if ct.startswith("image/") and ct in url:
                        return ct
                return "image/png"  # default image type

            # Check file type — file is now a string (file_id)
            if item.get("type") == "file":
                file_ref = item.get("file", "")
                if isinstance(file_ref, str):
                    # Look up content type from uploaded files store
                    if file_ref in uploaded_files:
                        return uploaded_files[file_ref]
                elif isinstance(file_ref, dict):
                    # Legacy format: {"url": "data:..."}
                    url = file_ref.get("url", "")
                    for ct in MOCK_RESPONSES:
                        if ct in url:
                            return ct

    return None


class DocParseHandler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        # File upload endpoint
        if self.path == "/api/v1/files":
            self._handle_file_upload()
            return

        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)

        try:
            req = json.loads(body)
        except json.JSONDecodeError:
            self.send_response(400)
            self.end_headers()
            self.wfile.write(b'{"error": "invalid JSON"}')
            return

        model = req.get("model", "qwen3.7-plus")
        messages = req.get("messages", [])

        # Extract content from the first message
        content_items = []
        if messages:
            content = messages[0].get("content", [])
            if isinstance(content, list):
                content_items = content
            elif isinstance(content, str):
                content_items = [{"type": "text", "text": content}]

        # Detect content type and generate appropriate response
        detected_type = detect_content_type(content_items)
        response_text = MOCK_RESPONSES.get(detected_type, DEFAULT_RESPONSE)

        # Log the request for verification
        request_log.append({
            "path": self.path,
            "model": model,
            "detected_type": detected_type,
            "content_items_count": len(content_items),
        })

        # Build OpenAI-compatible response
        resp = {
            "id": "chatcmpl-mock-001",
            "object": "chat.completion",
            "created": 1700000000,
            "model": model,
            "choices": [
                {
                    "index": 0,
                    "message": {
                        "role": "assistant",
                        "content": response_text,
                    },
                    "finish_reason": "stop",
                }
            ],
            "usage": {
                "prompt_tokens": 100,
                "completion_tokens": 50,
                "total_tokens": 150,
            },
        }

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(resp).encode())

    def do_DELETE(self):
        """Handle file deletion: DELETE /api/v1/files/<file_id>"""
        if self.path.startswith("/api/v1/files/"):
            file_id = self.path.split("/")[-1]
            removed = uploaded_files.pop(file_id, None)
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"id": file_id, "deleted": removed is not None}).encode())
            return
        self.send_response(404)
        self.end_headers()

    def _handle_file_upload(self):
        """Handle multipart file upload: POST /api/v1/files"""
        content_type = self.headers.get("Content-Type", "")
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)

        # Extract boundary from Content-Type
        filename = ""
        if "multipart/form-data" in content_type:
            for part in content_type.split(";"):
                part = part.strip()
                if part.startswith("boundary="):
                    boundary = part.split("=", 1)[1].strip().encode()
                    break
            else:
                boundary = b"----WebKitFormBoundary"

            # Extract filename from Content-Disposition header in body
            body_str = body.decode("utf-8", errors="replace")
            for line in body_str.split("\r\n"):
                if "filename=" in line:
                    m = re.search(r'filename="([^"]*)"', line)
                    if m:
                        filename = m.group(1)
                        break

        # Generate mock file ID and store content type
        file_id = "file-mock-" + uuid.uuid4().hex[:12]

        # Guess content type from filename extension
        ct = "application/octet-stream"
        ext_map = {
            ".pdf": "application/pdf",
            ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
            ".doc": "application/msword",
            ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
            ".xls": "application/vnd.ms-excel",
            ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
            ".ppt": "application/vnd.ms-powerpoint",
            ".rtf": "application/rtf",
            ".epub": "application/epub+zip",
        }
        for ext, mime in ext_map.items():
            if filename.lower().endswith(ext):
                ct = mime
                break

        uploaded_files[file_id] = ct

        request_log.append({
            "path": self.path,
            "action": "file_upload",
            "file_id": file_id,
            "filename": filename,
            "content_type": ct,
        })

        resp = {
            "id": file_id,
            "object": "file",
            "bytes": content_length,
            "created_at": 1700000000,
            "filename": filename,
            "purpose": "file-extract",
        }
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(resp).encode())

    def do_GET(self):
        """Health check endpoint."""
        if self.path == "/health":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"status": "ok", "requests": len(request_log)}).encode())
            return

        if self.path == "/requests":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(request_log).encode())
            return

        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        pass  # silence logs


if __name__ == "__main__":
    server = http.server.HTTPServer(("0.0.0.0", PORT), DocParseHandler)
    print(f"Mock doc parse server running on port {PORT}")
    server.serve_forever()
