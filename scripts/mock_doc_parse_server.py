#!/usr/bin/env python3
"""Mock DashScope chat completions server (OpenAI-compatible).

Simulates the Qwen3.7-Plus document parsing API for E2E testing.
Returns deterministic extracted text based on content type detected
from the base64 data URL in the request.
"""
import http.server
import json
import base64

PORT = 11436

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

            # Check file type
            if item.get("type") == "file":
                url = item.get("file", {}).get("url", "")
                for ct in MOCK_RESPONSES:
                    if ct in url:
                        return ct
                # Try to detect from data URL
                if "application/pdf" in url:
                    return "application/pdf"
                if "wordprocessingml" in url or "msword" in url:
                    return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
                if "spreadsheetml" in url or "ms-excel" in url:
                    return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                if "presentationml" in url or "ms-powerpoint" in url:
                    return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
                if "epub" in url:
                    return "application/epub+zip"
                if "rtf" in url:
                    return "application/rtf"

    return None


class DocParseHandler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
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
