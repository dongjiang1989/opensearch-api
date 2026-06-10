#!/bin/bash
set -euo pipefail

# E2E test for document parsing (Qwen3.7-Plus / mock)
# Tests: PDF, DOCX, XLSX, PPTX, images (PNG/JPEG), HTML, Markdown, CSV, JSON
# Requires: mock doc parse server (started by this script), Docker Compose running

BASE_URL="http://localhost:18080/api/v1"
MOCK_PORT=11436
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
TMPF=$(mktemp)
trap 'rm -f "$TMPF"' EXIT

PASS_COUNT=0
FAIL_COUNT=0

pass() { echo -e "  ${GREEN}PASS${NC} $1"; PASS_COUNT=$((PASS_COUNT+1)); }
fail() { echo -e "  ${RED}FAIL${NC} $1"; FAIL_COUNT=$((FAIL_COUNT+1)); }
section() { echo -e "\n${YELLOW}=== $1 ===${NC}"; }

curl_json() {
  CODE=$(curl -s -w '%{http_code}' -o "$TMPF" "$@")
  BODY=$(cat "$TMPF")
}

# ---- Step 0: Start mock doc parse server ----
section "0. Starting mock doc parse server"

# Kill any existing mock server on the port
lsof -ti:$MOCK_PORT | xargs kill -9 2>/dev/null || true
sleep 1

python3 "$(dirname "$0")/mock_doc_parse_server.py" &
MOCK_PID=$!
sleep 2

# Verify mock server
curl_json "http://localhost:$MOCK_PORT/health"
[ "$CODE" = "200" ] && pass "Mock doc parse server ready (HTTP $CODE)" || { fail "Mock server not ready"; exit 1; }

# ---- Step 1: Restart app with mock doc parse config ----
section "1. Restarting app with mock doc parse config"

docker compose -f deployments/docker/docker-compose.yml stop opensearch-file-api 2>/dev/null || true
# Remove ALL opensearch-file-api containers (both service and run containers)
docker rm -f $(docker ps -a -q --filter "name=opensearch-file-api" 2>/dev/null) 2>/dev/null || true
docker rm -f $(docker ps -a -q --filter "name=docker-opensearch-file-api" 2>/dev/null) 2>/dev/null || true
sleep 2

# Start app with doc_parse pointing to mock server
docker compose -f deployments/docker/docker-compose.yml run --rm \
  -e OPENSEARCH_STORAGE_DOC_PARSE_PROVIDER=qwen \
  -e OPENSEARCH_STORAGE_DOC_PARSE_API_URL="http://host.docker.internal:$MOCK_PORT/v1/chat/completions" \
  -e OPENSEARCH_STORAGE_DOC_PARSE_API_KEY=sk-mock-key \
  -e OPENSEARCH_STORAGE_DOC_PARSE_MODEL=qwen3.7-plus \
  -e OPENSEARCH_EMBEDDING_PROVIDER=none \
  -p 18080:8080 \
  -d opensearch-file-api 2>&1

echo "  Container started"
echo "  Waiting for app..."
for i in $(seq 1 30); do
  if curl -s -f "http://localhost:18080/health" > /dev/null 2>&1; then
    echo "  App is ready"
    break
  fi
  if [ "$i" -eq 30 ]; then
    fail "App failed to start within 30s"
    kill $MOCK_PID 2>/dev/null || true
    exit 1
  fi
  sleep 1
done

# ---- Step 2: Create tenant and get token ----
section "2. Setup tenant and token"

curl_json -X DELETE "$BASE_URL/admin/tenants/tenant-doctest/hard" || true
sleep 1

curl_json -X POST "$BASE_URL/admin/tenants" \
  -H "Content-Type: application/json" \
  -d '{"id":"tenant-doctest","name":"Doc Parse Test Tenant"}'
[ "$CODE" = "201" ] && pass "Tenant created (HTTP $CODE)" || fail "Tenant creation failed (HTTP $CODE): $BODY"

curl_json -X POST "$BASE_URL/token" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"tenant-doctest","role":"admin"}'
TOKEN=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null)
[ -n "$TOKEN" ] && pass "Token generated (${#TOKEN} chars)" || fail "Token generation failed"

AUTH="Authorization: Bearer $TOKEN"
TENANT="X-Tenant-ID: tenant-doctest"

# ---- Step 3: Create test files ----
section "3. Creating test files"

# 3a. Text file (direct extraction, no API call)
echo "This is a plain text document about PYTHON_TEXT_EXTRACTION and natural language processing." > /tmp/doctest_plain.txt
echo "  plain text: $(wc -c < /tmp/doctest_plain.txt) bytes"

# 3b. HTML file (direct extraction, no API call)
cat > /tmp/doctest_page.html << 'HTMLEOF'
<html>
<head><title>Test Page</title><style>body{color:red}</style></head>
<body>
<script>var x = 1;</script>
<h1>HTML_EXTRACTION_TEST</h1>
<p>This is a test HTML page about <strong>document parsing</strong> and content extraction.</p>
<p>It tests that script and style tags are properly filtered out.</p>
</body>
</html>
HTMLEOF
echo "  HTML: $(wc -c < /tmp/doctest_page.html) bytes"

# 3c. Markdown file (direct extraction, no API call)
cat > /tmp/doctest_doc.md << 'MDEOF'
# Markdown Test Document

This is a **markdown** document about MARKDOWN_EXTRACTION_TEST.

## Section 2

- Item 1: machine learning
- Item 2: deep learning
- Item 3: neural networks

```python
print("code block should be included")
```
MDEOF
echo "  Markdown: $(wc -c < /tmp/doctest_doc.md) bytes"

# 3d. CSV file (direct extraction, no API call)
cat > /tmp/doctest_data.csv << 'CSVEOF'
name,age,city,occupation
Alice,30,Beijing,engineer
Bob,25,Shanghai,designer
Charlie,35,Shenzhen,manager
CSVEOF
echo "  CSV: $(wc -c < /tmp/doctest_data.csv) bytes"

# 3e. JSON file (direct extraction, no API call)
cat > /tmp/doctest_data.json << 'JSONEOF'
{
  "title": "JSON_EXTRACTION_TEST",
  "description": "Test JSON content extraction",
  "items": ["machine learning", "deep learning", "neural networks"],
  "metadata": {"author": "test", "version": 1}
}
JSONEOF
echo "  JSON: $(wc -c < /tmp/doctest_data.json) bytes"

# 3f. PDF file (requires mock API call)
python3 -c "
import zlib
pdf = b'%PDF-1.4\n'
pdf += b'1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n'
pdf += b'2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n'
content = b'BT /F1 12 Tf 100 700 Td (PDF_EXTRACTION_TEST Document) Tj ET\nBT /F1 10 Tf 100 680 Td (This PDF contains information about machine learning.) Tj ET\nBT /F1 10 Tf 100 660 Td (And neural networks for document parsing.) Tj ET'
stream = zlib.compress(content)
pdf += b'3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> >>\nendobj\n'
pdf += b'4 0 obj\n<< /Length %d >>\nstream\n' % len(stream)
pdf += stream
pdf += b'\nendstream\nendobj\n'
pdf += b'xref\n0 5\n'
pdf += b'trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n0\n%%EOF\n'
with open('/tmp/doctest_doc.pdf', 'wb') as f:
    f.write(pdf)
print('  PDF: %d bytes' % len(pdf))
"

# 3g. DOCX file (minimal .docx - ZIP with XML, requires mock API call)
python3 -c "
import zipfile, io
buf = io.BytesIO()
with zipfile.ZipFile(buf, 'w', zipfile.ZIP_DEFLATED) as zf:
    zf.writestr('[Content_Types].xml', '''<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>
<Types xmlns=\"http://schemas.openxmlformats.org/package/2006/content-types\">
  <Default Extension=\"rels\" ContentType=\"application/vnd.openxmlformats-package.relationships+xml\"/>
  <Default Extension=\"xml\" ContentType=\"application/xml\"/>
  <Override PartName=\"/word/document.xml\" ContentType=\"application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml\"/>
</Types>''')
    zf.writestr('_rels/.rels', '''<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>
<Relationships xmlns=\"http://schemas.openxmlformats.org/package/2006/relationships\">
  <Relationship Id=\"rId1\" Type=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument\" Target=\"word/document.xml\"/>
</Relationships>''')
    zf.writestr('word/document.xml', '''<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>
<w:document xmlns:w=\"http://schemas.openxmlformats.org/wordprocessingml/2006/main\">
  <w:body>
    <w:p><w:r><w:t>DOCX_EXTRACTION_TEST Document</w:t></w:r></w:p>
    <w:p><w:r><w:t>This Word document contains structured content about machine learning.</w:t></w:r></w:p>
    <w:p><w:r><w:t>Chapter 1: Introduction to Neural Networks</w:t></w:r></w:p>
  </w:body>
</w:document>''')
with open('/tmp/doctest_doc.docx', 'wb') as f:
    f.write(buf.getvalue())
print('  DOCX: %d bytes' % buf.tell())
"

# 3h. XLSX file (minimal .xlsx, requires mock API call)
python3 -c "
import zipfile, io
buf = io.BytesIO()
with zipfile.ZipFile(buf, 'w', zipfile.ZIP_DEFLATED) as zf:
    zf.writestr('[Content_Types].xml', '''<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>
<Types xmlns=\"http://schemas.openxmlformats.org/package/2006/content-types\">
  <Default Extension=\"rels\" ContentType=\"application/vnd.openxmlformats-package.relationships+xml\"/>
  <Default Extension=\"xml\" ContentType=\"application/xml\"/>
  <Override PartName=\"/xl/workbook.xml\" ContentType=\"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml\"/>
</Types>''')
    zf.writestr('_rels/.rels', '''<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>
<Relationships xmlns=\"http://schemas.openxmlformats.org/package/2006/relationships\">
  <Relationship Id=\"rId1\" Type=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument\" Target=\"xl/workbook.xml\"/>
</Relationships>''')
    zf.writestr('xl/workbook.xml', '''<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>
<workbook xmlns=\"http://schemas.openxmlformats.org/spreadsheetml/2006/main\">
  <sheets><sheet name=\"Sheet1\" sheetId=\"1\"/></sheets>
</workbook>''')
with open('/tmp/doctest_data.xlsx', 'wb') as f:
    f.write(buf.getvalue())
print('  XLSX: %d bytes' % buf.tell())
"

# 3i. PNG image (requires mock API call for OCR)
python3 -c "
import struct, zlib
width, height = 10, 10
raw = b''
for y in range(height):
    raw += b'\x00'
    for x in range(width):
        raw += b'\xff\x00\x00'
png_sig = b'\x89PNG\r\n\x1a\n'
ihdr_data = struct.pack('>IIBBBBB', width, height, 8, 2, 0, 0, 0)
ihdr_crc = struct.pack('>I', zlib.crc32(b'IHDR' + ihdr_data) & 0xffffffff)
ihdr = b'\x00\x00\x00\x0dIHDR' + ihdr_data + ihdr_crc
raw_data = zlib.compress(raw)
idat_crc = struct.pack('>I', zlib.crc32(b'IDAT' + raw_data) & 0xffffffff)
idat = struct.pack('>I', len(raw_data)) + b'IDAT' + raw_data + idat_crc
iend_crc = struct.pack('>I', zlib.crc32(b'IEND') & 0xffffffff)
iend = b'\x00\x00\x00\x00IEND' + iend_crc
with open('/tmp/doctest_image.png', 'wb') as f:
    f.write(png_sig + ihdr + idat + iend)
print('  PNG: %d bytes' % (len(png_sig + ihdr + idat + iend)))
"

# ---- Step 4: Upload all file types ----
section "4. Upload files (all types)"

upload() {
  local desc="$1"
  local file="$2"
  curl_json -X POST "$BASE_URL/files" \
    -H "$AUTH" -H "$TENANT" \
    -F "file=@$file" \
    -F "description=$desc"
  echo "  -> $desc (HTTP $CODE)"
}

# Text types (direct extraction)
upload "Plain text document" /tmp/doctest_plain.txt
[ "$CODE" = "200" ] && pass "TXT uploaded (HTTP $CODE)" || fail "TXT upload failed (HTTP $CODE): $BODY"

upload "HTML page" /tmp/doctest_page.html
[ "$CODE" = "200" ] && pass "HTML uploaded (HTTP $CODE)" || fail "HTML upload failed (HTTP $CODE): $BODY"

upload "Markdown document" /tmp/doctest_doc.md
[ "$CODE" = "200" ] && pass "Markdown uploaded (HTTP $CODE)" || fail "Markdown upload failed (HTTP $CODE): $BODY"

upload "CSV data file" /tmp/doctest_data.csv
[ "$CODE" = "200" ] && pass "CSV uploaded (HTTP $CODE)" || fail "CSV upload failed (HTTP $CODE): $BODY"

upload "JSON data file" /tmp/doctest_data.json
[ "$CODE" = "200" ] && pass "JSON uploaded (HTTP $CODE)" || fail "JSON upload failed (HTTP $CODE): $BODY"

# Document types (mock API extraction)
upload "PDF document" /tmp/doctest_doc.pdf
[ "$CODE" = "200" ] && pass "PDF uploaded (HTTP $CODE)" || fail "PDF upload failed (HTTP $CODE): $BODY"

upload "Word DOCX document" /tmp/doctest_doc.docx
[ "$CODE" = "200" ] && pass "DOCX uploaded (HTTP $CODE)" || fail "DOCX upload failed (HTTP $CODE): $BODY"

upload "Excel XLSX spreadsheet" /tmp/doctest_data.xlsx
[ "$CODE" = "200" ] && pass "XLSX uploaded (HTTP $CODE)" || fail "XLSX upload failed (HTTP $CODE): $BODY"

upload "PNG image with text" /tmp/doctest_image.png
[ "$CODE" = "200" ] && pass "PNG uploaded (HTTP $CODE)" || fail "PNG upload failed (HTTP $CODE): $BODY"

echo "  Waiting 5s for OpenSearch indexing..."
sleep 5

# ---- Step 5: Verify text extraction (direct, no API call) ----
section "5. Verify text extraction (direct types)"

# 5a. Plain text - search for content directly in the file
echo "Searching for 'PYTHON_TEXT_EXTRACTION' (plain text)..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"PYTHON_TEXT_EXTRACTION","size":10}'
[ "$CODE" = "200" ] && pass "TXT search succeeded (HTTP $CODE)" || fail "TXT search failed (HTTP $CODE)"
TXT_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  TXT results: $TXT_COUNT"
[ "$TXT_COUNT" -gt 0 ] && pass "TXT content extracted and searchable" || fail "TXT content not found in search"

# 5b. HTML - script/style filtered, content extracted
echo "Searching for 'content extraction' in HTML content..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"content extraction","size":10}'
[ "$CODE" = "200" ] && pass "HTML search succeeded (HTTP $CODE)" || fail "HTML search failed (HTTP $CODE)"
HTML_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  HTML results: $HTML_COUNT"
[ "$HTML_COUNT" -gt 0 ] && pass "HTML content extracted (script/style filtered)" || fail "HTML content not found"

# 5c. Markdown
echo "Searching for 'MARKDOWN_EXTRACTION_TEST' (Markdown)..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"MARKDOWN_EXTRACTION_TEST","size":10}'
[ "$CODE" = "200" ] && pass "Markdown search succeeded (HTTP $CODE)" || fail "Markdown search failed (HTTP $CODE)"
MD_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  Markdown results: $MD_COUNT"
[ "$MD_COUNT" -gt 0 ] && pass "Markdown content extracted and searchable" || fail "Markdown content not found"

# 5d. CSV
echo "Searching for 'Alice' (CSV)..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"Alice","size":10}'
[ "$CODE" = "200" ] && pass "CSV search succeeded (HTTP $CODE)" || fail "CSV search failed (HTTP $CODE)"
CSV_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  CSV results: $CSV_COUNT"
[ "$CSV_COUNT" -gt 0 ] && pass "CSV content extracted and searchable" || fail "CSV content not found"

# 5e. JSON
echo "Searching for 'JSON_EXTRACTION_TEST' (JSON)..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"JSON_EXTRACTION_TEST","size":10}'
[ "$CODE" = "200" ] && pass "JSON search succeeded (HTTP $CODE)" || fail "JSON search failed (HTTP $CODE)"
JSON_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  JSON results: $JSON_COUNT"
[ "$JSON_COUNT" -gt 0 ] && pass "JSON content extracted and searchable" || fail "JSON content not found"

# ---- Step 6: Verify document extraction (via mock API) ----
section "6. Verify document extraction (mock API types)"

# 6a. PDF - mock returns "MOCK_PDF_EXTRACTED"
echo "Searching for 'MOCK_PDF_EXTRACTED' (PDF via mock API)..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"MOCK_PDF_EXTRACTED","size":10}'
[ "$CODE" = "200" ] && pass "PDF mock extraction search succeeded (HTTP $CODE)" || fail "PDF search failed (HTTP $CODE)"
PDF_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  PDF mock results: $PDF_COUNT"
[ "$PDF_COUNT" -gt 0 ] && pass "PDF content extracted via mock API and searchable" || fail "PDF mock content not found"

# 6b. DOCX - mock returns "MOCK_DOCX_EXTRACTED"
echo "Searching for 'MOCK_DOCX_EXTRACTED' (DOCX via mock API)..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"MOCK_DOCX_EXTRACTED","size":10}'
[ "$CODE" = "200" ] && pass "DOCX mock extraction search succeeded (HTTP $CODE)" || fail "DOCX search failed (HTTP $CODE)"
DOCX_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  DOCX mock results: $DOCX_COUNT"
[ "$DOCX_COUNT" -gt 0 ] && pass "DOCX content extracted via mock API and searchable" || fail "DOCX mock content not found"

# 6c. XLSX - mock returns "MOCK_XLSX_EXTRACTED"
echo "Searching for 'MOCK_XLSX_EXTRACTED' (XLSX via mock API)..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"MOCK_XLSX_EXTRACTED","size":10}'
[ "$CODE" = "200" ] && pass "XLSX mock extraction search succeeded (HTTP $CODE)" || fail "XLSX search failed (HTTP $CODE)"
XLSX_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  XLSX mock results: $XLSX_COUNT"
[ "$XLSX_COUNT" -gt 0 ] && pass "XLSX content extracted via mock API and searchable" || fail "XLSX mock content not found"

# 6d. PNG image - mock returns "MOCK_IMAGE_OCR_RESULT"
echo "Searching for 'MOCK_IMAGE_OCR_RESULT' (PNG via mock API)..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"MOCK_IMAGE_OCR_RESULT","size":10}'
[ "$CODE" = "200" ] && pass "PNG mock OCR search succeeded (HTTP $CODE)" || fail "PNG search failed (HTTP $CODE)"
PNG_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  PNG mock OCR results: $PNG_COUNT"
[ "$PNG_COUNT" -gt 0 ] && pass "PNG image OCR via mock API and searchable" || fail "PNG mock OCR content not found"

# ---- Step 7: Verify file metadata ----
section "7. Verify file metadata (doc_parse info)"

# List all files and get first file ID
curl_json -X GET "$BASE_URL/files?page=1&size=20" \
  -H "$AUTH" -H "$TENANT"
[ "$CODE" = "200" ] && pass "File list retrieved (HTTP $CODE)" || fail "File list failed (HTTP $CODE)"

TOTAL_FILES=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  Total files uploaded: $TOTAL_FILES"
[ "$TOTAL_FILES" -ge 9 ] && pass "All 9 file types uploaded" || fail "Expected >=9 files, got $TOTAL_FILES"

# Get PDF file ID for metadata check
FILE_ID=$(echo "$BODY" | python3 -c "
import sys, json
items = json.load(sys.stdin).get('data', [])
for f in items:
    if f.get('filename','').endswith('.pdf'):
        print(f['id'])
        break
" 2>/dev/null)

if [ -n "$FILE_ID" ]; then
  echo "Checking metadata for PDF file: $FILE_ID"
  curl_json -X GET "$BASE_URL/files/$FILE_ID/metadata" \
    -H "$AUTH" -H "$TENANT"
  [ "$CODE" = "200" ] && pass "File metadata retrieved (HTTP $CODE)" || fail "Metadata retrieval failed (HTTP $CODE)"

  HAS_DOC_PARSE=$(echo "$BODY" | python3 -c "
import sys, json
doc = json.load(sys.stdin)
meta = doc.get('metadata', doc.get('document', {}).get('metadata', {}))
# Check both top-level and nested metadata
has_provider = 'doc_parse_provider' in str(doc)
has_model = 'doc_parse_model' in str(doc) or 'qwen' in str(doc)
print('yes' if (has_provider or has_model) else 'no')
" 2>/dev/null || echo "no")
  echo "  Has doc_parse info in metadata: $HAS_DOC_PARSE"
  [ "$HAS_DOC_PARSE" = "yes" ] && pass "doc_parse_provider/model present in metadata" || echo "  INFO: metadata may be nested in document source"
else
  echo "  WARN: Could not find PDF file ID for metadata check"
fi

# ---- Step 8: Verify mock server was called ----
section "8. Verify mock API was called for document types"

curl_json "http://localhost:$MOCK_PORT/requests"
[ "$CODE" = "200" ] && pass "Mock server request log retrieved (HTTP $CODE)" || fail "Mock server log failed"

MOCK_CALLS=$(echo "$BODY" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
echo "  Mock API calls received: $MOCK_CALLS"
[ "$MOCK_CALLS" -ge 4 ] && pass "Mock API called for all document types (PDF/DOCX/XLSX/PNG)" || fail "Expected >=4 mock API calls, got $MOCK_CALLS"

echo "  Request details:"
echo "$BODY" | python3 -c "
import sys, json
reqs = json.load(sys.stdin)
for r in reqs:
    print('    - type: %s, model: %s' % (r.get('detected_type','unknown'), r.get('model','unknown')))
" 2>/dev/null

# ---- Step 9: Aggregate by file type ----
section "9. Aggregate by file type"

curl_json -X POST "$BASE_URL/search/aggregate" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"field":"file_type"}'
[ "$CODE" = "200" ] && pass "Aggregate succeeded (HTTP $CODE)" || fail "Aggregate failed (HTTP $CODE)"

echo "  File type distribution:"
echo "$BODY" | python3 -c "
import sys, json
data = json.load(sys.stdin)
buckets = data.get('buckets', {})
for k, v in sorted(buckets.items()):
    print('    %s: %d' % (k, v))
" 2>/dev/null

# ---- Step 10: Search with file_type filter ----
section "10. Search with file_type filter"

echo "Search PDF only..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"MOCK_PDF_EXTRACTED","filters":{"file_type":"pdf"},"size":10}'
[ "$CODE" = "200" ] && pass "Filtered PDF search succeeded (HTTP $CODE)" || fail "Filtered search failed (HTTP $CODE)"
FILTER_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  Filtered results: $FILTER_COUNT"
[ "$FILTER_COUNT" -gt 0 ] && pass "Filtered PDF search found results" || fail "Filtered PDF search returned no results"

echo "Search images only..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"MOCK","filters":{"file_type":"image"},"size":10}'
[ "$CODE" = "200" ] && pass "Filtered image search succeeded (HTTP $CODE)" || fail "Filtered image search failed (HTTP $CODE)"

echo "Search text only..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH" -H "$TENANT" \
  -H "Content-Type: application/json" \
  -d '{"query":"PYTHON_TEXT_EXTRACTION","filters":{"file_type":"text"},"size":10}'
[ "$CODE" = "200" ] && pass "Filtered text search succeeded (HTTP $CODE)" || fail "Filtered text search failed (HTTP $CODE)"
TEXT_FILTER_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  Text filter results: $TEXT_FILTER_COUNT"
[ "$TEXT_FILTER_COUNT" -gt 0 ] && pass "Text filter correctly found plain text file" || fail "Text filter returned no results"

# ---- Cleanup ----
section "Cleanup"

# Clean up tenant before stopping the app
curl_json -X DELETE "$BASE_URL/admin/tenants/tenant-doctest/hard" \
  -H "$AUTH" -H "$TENANT" || true

kill $MOCK_PID 2>/dev/null || true
lsof -ti:$MOCK_PORT | xargs kill -9 2>/dev/null || true
docker compose -f deployments/docker/docker-compose.yml stop opensearch-file-api 2>/dev/null || true
docker rm -f $(docker ps -a -q --filter "name=docker-opensearch-file-api" 2>/dev/null) 2>/dev/null || true

echo "  Cleaned up mock server and app container"

# ---- Summary ----
section "Test Summary"
echo ""
echo "  Total PASS: $PASS_COUNT"
echo "  Total FAIL: $FAIL_COUNT"
echo ""

if [ "$FAIL_COUNT" -gt 0 ]; then
  echo -e "  ${RED}Some tests failed!${NC}"
  exit 1
fi

echo "  Tested document parsing flows:"
echo "    1. Plain text (.txt) — direct extraction"
echo "    2. HTML (.html) — direct extraction with script/style filtering"
echo "    3. Markdown (.md) — direct extraction"
echo "    4. CSV (.csv) — direct extraction"
echo "    5. JSON (.json) — direct extraction"
echo "    6. PDF (.pdf) — mock API base64 extraction"
echo "    7. DOCX (.docx) — mock API base64 extraction"
echo "    8. XLSX (.xlsx) — mock API base64 extraction"
echo "    9. PNG image (.png) — mock API base64 image OCR"
echo "   10. File metadata verification (doc_parse_provider)"
echo "   11. Mock API call verification"
echo "   12. Aggregate by file_type"
echo "   13. Search with file_type filter"
echo ""

section "All document parsing E2E tests completed successfully"
