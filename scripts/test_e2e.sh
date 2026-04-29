#!/bin/bash
set -euo pipefail

BASE_URL="http://localhost:18080/api/v1"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
TMPF=$(mktemp)
trap 'rm -f "$TMPF"' EXIT

pass() { echo -e "  ${GREEN}PASS${NC} $1"; }
fail() { echo -e "  ${RED}FAIL${NC} $1"; exit 1; }
section() { echo -e "\n${YELLOW}=== $1 ===${NC}"; }

# curl wrapper: writes response body to TMPF, returns HTTP code
curl_json() {
  CODE=$(curl -s -w '%{http_code}' -o "$TMPF" "$@")
  BODY=$(cat "$TMPF")
}

# ---- Step 1: Create test files ----
section "1. Creating test files"

echo "This is a test document about machine learning and artificial intelligence. Natural language processing is a key area of AI." > /tmp/test_document.txt

python3 -c "
import struct, zlib
pdf = b'%PDF-1.4\n'
pdf += b'1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n'
pdf += b'2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n'
content = b'BT /F1 12 Tf 100 700 Td (Machine Learning Test Document) Tj ET\nBT /F1 10 Tf 100 680 Td (This document is about machine learning, deep learning, and neural networks.) Tj ET\nBT /F1 10 Tf 100 660 Td (Computer vision and natural language processing are important subfields.) Tj ET'
stream = zlib.compress(content)
pdf += b'3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> >>\nendobj\n'
pdf += b'4 0 obj\n<< /Length %d >>\nstream\n' % len(stream)
pdf += stream
pdf += b'\nendstream\nendobj\n'
pdf += b'xref\n0 5\n'
pdf += b'trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n0\n%%EOF\n'
with open('/tmp/test_document.pdf', 'wb') as f:
    f.write(pdf)
print('PDF created')
"

python3 -c "
import zlib, struct
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
with open('/tmp/test_image.png', 'wb') as f:
    f.write(png_sig + ihdr + idat + iend)
print('PNG created')
"

echo "  Text: $(wc -c < /tmp/test_document.txt) bytes"
echo "  PDF:  $(wc -c < /tmp/test_document.pdf) bytes"
echo "  PNG:  $(wc -c < /tmp/test_image.png) bytes"

# ---- Step 2: Tenant Management ----
section "2. Tenant Management"

echo "Creating tenant-A..."
curl_json -X POST "$BASE_URL/admin/tenants" \
  -H "Content-Type: application/json" \
  -d '{"id":"tenant-a","name":"Test Tenant A","description":"Tenant for testing"}'
[ "$CODE" = "201" ] && pass "Tenant-A created (HTTP $CODE)" || fail "Tenant-A creation failed (HTTP $CODE): $BODY"

echo "Creating tenant-B..."
curl_json -X POST "$BASE_URL/admin/tenants" \
  -H "Content-Type: application/json" \
  -d '{"id":"tenant-b","name":"Test Tenant B","description":"Another tenant for isolation test"}'
[ "$CODE" = "201" ] && pass "Tenant-B created (HTTP $CODE)" || fail "Tenant-B creation failed (HTTP $CODE): $BODY"

echo "Testing duplicate tenant creation..."
curl_json -X POST "$BASE_URL/admin/tenants" \
  -H "Content-Type: application/json" \
  -d '{"id":"tenant-a","name":"Duplicate"}'
[ "$CODE" = "409" ] && pass "Duplicate tenant rejected (HTTP $CODE)" || fail "Expected 409, got HTTP $CODE"

echo "Listing tenants..."
curl_json "$BASE_URL/admin/tenants?page=1&size=20"
[ "$CODE" = "200" ] && pass "Tenant list retrieved (HTTP $CODE)" || fail "Tenant list failed (HTTP $CODE)"

echo "Getting tenant-A details..."
curl_json "$BASE_URL/admin/tenants/tenant-a"
[ "$CODE" = "200" ] && pass "Tenant-A details retrieved (HTTP $CODE)" || fail "Tenant-A details failed (HTTP $CODE)"

echo "Updating tenant-A..."
curl_json -X PUT "$BASE_URL/admin/tenants/tenant-a" \
  -H "Content-Type: application/json" \
  -d '{"id":"tenant-a","name":"Updated Tenant A","description":"Updated description"}'
[ "$CODE" = "200" ] && pass "Tenant-A updated (HTTP $CODE)" || fail "Tenant-A update failed (HTTP $CODE)"

# ---- Step 3: Generate JWT Tokens ----
section "3. Generate JWT Tokens"

echo "Generating token for tenant-A..."
curl_json -X POST "$BASE_URL/token" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"tenant-a","role":"admin"}'
TOKEN_A=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null)
[ -n "$TOKEN_A" ] && pass "Token-A generated (${#TOKEN_A} chars)" || fail "Token-A generation failed"

echo "Generating token for tenant-B..."
curl_json -X POST "$BASE_URL/token" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"tenant-b","role":"user"}'
TOKEN_B=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null)
[ -n "$TOKEN_B" ] && pass "Token-B generated (${#TOKEN_B} chars)" || fail "Token-B generation failed"

echo "Testing invalid token..."
curl_json "$BASE_URL/files" \
  -H "Authorization: Bearer invalid-token" \
  -H "X-Tenant-ID: tenant-a"
[ "$CODE" = "401" ] && pass "Invalid token rejected (HTTP $CODE)" || fail "Expected 401, got HTTP $CODE"

# ---- Step 4: Upload Files (Tenant-A) ----
section "4. Upload Files (Tenant-A)"

AUTH_A="Authorization: Bearer $TOKEN_A"
AUTH_B="Authorization: Bearer $TOKEN_B"
TENANT_A="X-Tenant-ID: tenant-a"
TENANT_B="X-Tenant-ID: tenant-b"

# Upload helper
upload() {
  curl_json -X POST "$BASE_URL/files" \
    -H "$1" -H "$2" \
    -F "file=$3" \
    -F "description=$4" \
    -F "tags[]=${5:-test}"
}

echo "Uploading test_document.txt..."
curl_json -X POST "$BASE_URL/files" \
  -H "$AUTH_A" -H "$TENANT_A" \
  -F "file=@/tmp/test_document.txt" \
  -F "description=A text document about machine learning" \
  -F "tags[]=ml" \
  -F "tags[]=ai"
[ "$CODE" = "200" ] && pass "Text file uploaded (HTTP $CODE)" || fail "Text upload failed (HTTP $CODE): $BODY"

echo "Uploading test_document.pdf..."
curl_json -X POST "$BASE_URL/files" \
  -H "$AUTH_A" -H "$TENANT_A" \
  -F "file=@/tmp/test_document.pdf" \
  -F "description=A PDF about machine learning and neural networks" \
  -F "tags[]=pdf" \
  -F "tags[]=ml" \
  -F "tags[]=deep-learning"
[ "$CODE" = "200" ] && pass "PDF uploaded (HTTP $CODE)" || fail "PDF upload failed (HTTP $CODE): $BODY"

echo "Uploading test_image.png..."
curl_json -X POST "$BASE_URL/files" \
  -H "$AUTH_A" -H "$TENANT_A" \
  -F "file=@/tmp/test_image.png" \
  -F "description=A red test image for computer vision testing" \
  -F "tags[]=image" \
  -F "tags[]=test"
[ "$CODE" = "200" ] && pass "Image uploaded (HTTP $CODE)" || fail "Image upload failed (HTTP $CODE): $BODY"

echo "Waiting 3s for OpenSearch indexing..."
sleep 3

# ---- Step 5: File Operations (Tenant-A) ----
section "5. File Operations (Tenant-A)"

echo "Listing files for tenant-A..."
curl_json "$BASE_URL/files?page=1&size=20" -H "$AUTH_A" -H "$TENANT_A"
[ "$CODE" = "200" ] && pass "Files listed for tenant-A (HTTP $CODE)" || fail "File list failed (HTTP $CODE)"
FILE_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  Total files: $FILE_COUNT"
[ "$FILE_COUNT" -gt 0 ] && pass "Files exist in tenant-A" || fail "No files found for tenant-A"

FILE_ID=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',[{}])[0].get('id',''))" 2>/dev/null || echo "")
if [ -n "$FILE_ID" ]; then
  echo "Getting metadata for file: $FILE_ID..."
  curl_json "$BASE_URL/files/$FILE_ID/metadata" -H "$AUTH_A" -H "$TENANT_A"
  [ "$CODE" = "200" ] && pass "File metadata retrieved (HTTP $CODE)" || fail "Metadata failed (HTTP $CODE)"
fi

# ---- Step 6: Text Search (Tenant-A) ----
section "6. Text Search (Tenant-A)"

echo "Searching for 'machine learning'..."
curl_json "$BASE_URL/search?q=machine+learning" -H "$AUTH_A" -H "$TENANT_A"
[ "$CODE" = "200" ] && pass "GET search succeeded (HTTP $CODE)" || fail "GET search failed (HTTP $CODE)"
SCOUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  Results: $SCOUNT"
[ "$SCOUNT" -gt 0 ] && pass "Found matching files" || echo "  WARN: No results (indexing may be delayed)"

echo "Searching for 'neural' (POST)..."
curl_json -X POST "$BASE_URL/search" \
  -H "$AUTH_A" -H "$TENANT_A" \
  -H "Content-Type: application/json" \
  -d '{"query":"neural","size":10}'
[ "$CODE" = "200" ] && pass "POST search succeeded (HTTP $CODE)" || fail "POST search failed (HTTP $CODE)"

echo "Searching with filter file_type=pdf..."
curl_json "$BASE_URL/search?q=learning&file_type=pdf" -H "$AUTH_A" -H "$TENANT_A"
[ "$CODE" = "200" ] && pass "Filtered search succeeded (HTTP $CODE)" || fail "Filtered search failed (HTTP $CODE)"

# ---- Step 7: Tenant Isolation ----
section "7. Tenant Isolation Test"

echo "Searching tenant-A files from tenant-B (should be empty)..."
curl_json "$BASE_URL/search?q=machine+learning" -H "$AUTH_B" -H "$TENANT_B"
[ "$CODE" = "200" ] && pass "Tenant-B search succeeded (HTTP $CODE)" || fail "Tenant-B search failed"
B_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "?")
echo "  Results in tenant-B: $B_COUNT"
[ "$B_COUNT" = "0" ] && pass "Tenant isolation: tenant-B sees 0 files" || echo "  WARN: tenant-B found $B_COUNT files"

echo "Uploading unique file to tenant-B..."
echo "SECRET TENANT B DOCUMENT - only visible to tenant B" > /tmp/tenant_b_secret.txt
curl_json -X POST "$BASE_URL/files" \
  -H "$AUTH_B" -H "$TENANT_B" \
  -F "file=@/tmp/tenant_b_secret.txt" \
  -F "description=Secret document only for tenant B" \
  -F "tags[]=secret"
[ "$CODE" = "200" ] && pass "File uploaded to tenant-B (HTTP $CODE)" || fail "Upload to tenant-B failed"

sleep 2

echo "Searching for 'SECRET' in tenant-A (should find 0)..."
curl_json "$BASE_URL/search?q=SECRET" -H "$AUTH_A" -H "$TENANT_A"
A_SECRET=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "?")
echo "  Tenant-A found $A_SECRET results"
[ "$A_SECRET" = "0" ] && pass "Isolation: tenant-A cannot see tenant-B data" || fail "Tenant isolation broken!"

echo "Searching for 'SECRET' in tenant-B (should find it)..."
curl_json "$BASE_URL/search?q=SECRET" -H "$AUTH_B" -H "$TENANT_B"
B_SECRET=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "?")
echo "  Tenant-B found $B_SECRET results"
[ "$B_SECRET" -gt 0 ] && pass "Tenant-B can see own data" || echo "  WARN: indexing delayed"

echo "Testing no-token request..."
curl_json "$BASE_URL/files" -H "$TENANT_A"
[ "$CODE" = "401" ] && pass "No-token request rejected (HTTP $CODE)" || fail "Expected 401, got HTTP $CODE"

echo "Testing expired token..."
EXPIRED_TOKEN=$(python3 -c "
import jwt, time
token = jwt.encode({
    'tenant_id': 'tenant-a',
    'role': 'admin',
    'exp': time.time() - 3600,
    'iat': time.time() - 7200,
    'iss': 'opensearch-file-api',
}, 'dev-secret-key-change-in-production', algorithm='HS256')
print(token)
" 2>/dev/null || echo "")
if [ -n "$EXPIRED_TOKEN" ]; then
  curl_json "$BASE_URL/files" \
    -H "Authorization: Bearer $EXPIRED_TOKEN" -H "$TENANT_A"
  [ "$CODE" = "401" ] && pass "Expired token rejected (HTTP $CODE)" || fail "Expected 401, got HTTP $CODE"
fi

# ---- Step 8: KNN Vector Search ----
section "8. Vector Search (KNN & Hybrid)"

echo "Testing KNN search (sending dummy 1536-dim vector)..."
python3 -c "
import json
vector = [0.01] * 1536
body = json.dumps({'vector': vector, 'field': 'content_vector', 'k': 10})
print(body)
" > /tmp/knn_body.json
curl_json -X POST "$BASE_URL/search/knn" \
  -H "$AUTH_A" -H "$TENANT_A" \
  -H "Content-Type: application/json" \
  -d @/tmp/knn_body.json
# KNN may return 0 if files have no vectors (no embedding service running)
[ "$CODE" = "200" ] && pass "KNN search succeeded (HTTP $CODE)" || fail "KNN search failed (HTTP $CODE)"
KNN_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  KNN results: $KNN_COUNT"

echo "Testing hybrid search (text + dummy vector)..."
python3 -c "
import json
vector = [0.01] * 1536
body = json.dumps({'query': 'machine learning', 'vector': vector, 'k': 10})
print(body)
" > /tmp/hybrid_body.json
curl_json -X POST "$BASE_URL/search/hybrid" \
  -H "$AUTH_A" -H "$TENANT_A" \
  -H "Content-Type: application/json" \
  -d @/tmp/hybrid_body.json
[ "$CODE" = "200" ] && pass "Hybrid search succeeded (HTTP $CODE)" || fail "Hybrid search failed (HTTP $CODE)"

echo "Testing image vector search (dummy 512-dim vector)..."
python3 -c "
import json
vector = [0.01] * 512
body = json.dumps({'vector': vector, 'field': 'image_vector', 'k': 10})
print(body)
" > /tmp/knn_img_body.json
curl_json -X POST "$BASE_URL/search/knn" \
  -H "$AUTH_A" -H "$TENANT_A" \
  -H "Content-Type: application/json" \
  -d @/tmp/knn_img_body.json
[ "$CODE" = "200" ] && pass "Image KNN search succeeded (HTTP $CODE)" || fail "Image KNN failed (HTTP $CODE)"

# ---- Step 9: Aggregate & Count ----
section "9. Aggregate & Count"

echo "Aggregating by file_type..."
curl_json -X POST "$BASE_URL/search/aggregate" \
  -H "$AUTH_A" -H "$TENANT_A" \
  -H "Content-Type: application/json" \
  -d '{"field":"file_type"}'
[ "$CODE" = "200" ] && pass "Aggregate succeeded (HTTP $CODE)" || fail "Aggregate failed (HTTP $CODE)"
echo "  $BODY" | python3 -m json.tool 2>/dev/null || echo "  $BODY"

echo "Counting files in tenant-A..."
curl_json "$BASE_URL/search/count" -H "$AUTH_A" -H "$TENANT_A"
[ "$CODE" = "200" ] && pass "Count succeeded (HTTP $CODE)" || fail "Count failed (HTTP $CODE)"
echo "  $BODY" | python3 -m json.tool 2>/dev/null || echo "  $BODY"

# ---- Step 10: Health Check ----
section "10. Health Check"

curl_json "http://localhost:18080/health"
[ "$CODE" = "200" ] && pass "Health check passed (HTTP $CODE)" || fail "Health check failed (HTTP $CODE)"

curl_json "http://localhost:18080/ping"
[ "$CODE" = "200" ] && pass "Ping check passed (HTTP $CODE)" || fail "Ping check failed (HTTP $CODE)"

# ---- Summary ----
section "All tests completed successfully"
echo "Tenant-A token: ${TOKEN_A:0:30}..."
echo "Tenant-B token: ${TOKEN_B:0:30}..."
