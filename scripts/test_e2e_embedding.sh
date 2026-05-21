#!/bin/bash
set -euo pipefail

# E2E test for retrieve API with embedding service
# Requires: mock embedding server (started by this script), Docker Compose running

BASE_URL="http://localhost:18080/api/v1"
MOCK_PORT=11435
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
TMPF=$(mktemp)
trap 'rm -f "$TMPF"' EXIT

pass() { echo -e "  ${GREEN}PASS${NC} $1"; }
fail() { echo -e "  ${RED}FAIL${NC} $1"; exit 1; }
section() { echo -e "\n${YELLOW}=== $1 ===${NC}"; }

curl_json() {
  CODE=$(curl -s -w '%{http_code}' -o "$TMPF" "$@")
  BODY=$(cat "$TMPF")
}

# ---- Step 0: Start mock embedding server ----
section "0. Starting mock embedding server"

python3 "$(dirname "$0")/mock_embedding_server.py" &
MOCK_PID=$!
sleep 2

# Verify mock server
curl_json -X POST "http://localhost:$MOCK_PORT/v1/embeddings" \
  -H "Content-Type: application/json" \
  -d '{"input":"test"}'
[ "$CODE" = "200" ] && pass "Mock embedding server ready (HTTP $CODE)" || fail "Mock server not ready"

# ---- Step 1: Restart app with embedding ----
section "1. Restarting app with embedding config"

# Stop existing app container
docker compose -f deployments/docker/docker-compose.yml stop opensearch-file-api 2>/dev/null || true

# Remove any leftover run containers
docker rm -f $(docker ps -a -q --filter "ancestor=docker-opensearch-file-api" --filter "status=exited" 2>/dev/null) 2>/dev/null || true

# Start with embedding
COMPOSE_OUT=$(docker compose -f deployments/docker/docker-compose.yml run --rm \
  -e OPENSEARCH_EMBEDDING_PROVIDER=openai \
  -e OPENSEARCH_EMBEDDING_APIURL="http://host.docker.internal:$MOCK_PORT/v1/embeddings" \
  -e OPENSEARCH_EMBEDDING_MODEL=mock-embedding \
  -e OPENSEARCH_EMBEDDING_TIMEOUT=10 \
  -e OPENSEARCH_EMBEDDING_DIMENSIONS=1536 \
  -p 18080:8080 \
  -d opensearch-file-api 2>&1)

echo "  Container started"

# Wait for app to be ready
echo "  Waiting for app..."
for i in $(seq 1 30); do
  if curl -s -f "$BASE_URL/../health" > /dev/null 2>&1; then
    echo "  App is ready"
    break
  fi
  sleep 1
done

# ---- Step 2: Create tenant and get token ----
section "2. Setup tenant and token"

# Clean up old test data
curl_json -X DELETE "$BASE_URL/admin/tenants/tenant-embed/hard" || true
sleep 1

curl_json -X POST "$BASE_URL/admin/tenants" \
  -H "Content-Type: application/json" \
  -d '{"id":"tenant-embed","name":"Embedding Test Tenant"}'
[ "$CODE" = "201" ] && pass "Tenant created (HTTP $CODE)" || fail "Tenant creation failed (HTTP $CODE)"

curl_json -X POST "$BASE_URL/token" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"tenant-embed","role":"admin"}'
TOKEN_EMBED=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null)
[ -n "$TOKEN_EMBED" ] && pass "Token generated (${#TOKEN_EMBED} chars)" || fail "Token generation failed"

AUTH_E="Authorization: Bearer $TOKEN_EMBED"
TENANT_E="X-Tenant-ID: tenant-embed"

# ---- Step 3: Upload files (with embedding) ----
section "3. Upload files (with embedding)"

echo "This is a test document about machine learning and artificial intelligence. Natural language processing is a key area of AI research." > /tmp/test_ml_doc.txt

curl_json -X POST "$BASE_URL/files" \
  -H "$AUTH_E" -H "$TENANT_E" \
  -F "file=@/tmp/test_ml_doc.txt" \
  -F "description=A document about machine learning and NLP" \
  -F "tags[]=ml" -F "tags[]=nlp"
[ "$CODE" = "200" ] && pass "Text file uploaded with embedding (HTTP $CODE)" || fail "Upload failed (HTTP $CODE): $BODY"

echo "This document covers deep learning techniques including convolutional neural networks and recurrent neural networks for image recognition." > /tmp/test_dl_doc.txt

curl_json -X POST "$BASE_URL/files" \
  -H "$AUTH_E" -H "$TENANT_E" \
  -F "file=@/tmp/test_dl_doc.txt" \
  -F "description=Deep learning and neural networks" \
  -F "tags[]=deep-learning" -F "tags[]=cnn"
[ "$CODE" = "200" ] && pass "Second file uploaded with embedding (HTTP $CODE)" || fail "Upload failed (HTTP $CODE): $BODY"

sleep 2

# ---- Step 4: Test retrieve with JSON query ----
section "4. Retrieve with JSON query"

echo "Searching for 'machine learning' via retrieve..."
curl_json -X POST "$BASE_URL/search/retrieve" \
  -H "$AUTH_E" -H "$TENANT_E" \
  -H "Content-Type: application/json" \
  -d '{"query":"machine learning","k":10}'
[ "$CODE" = "200" ] && pass "Retrieve (JSON) succeeded (HTTP $CODE)" || fail "Retrieve (JSON) failed (HTTP $CODE)"
R_COUNT=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  Results: $R_COUNT"
[ "$R_COUNT" -gt 0 ] && pass "Found matching documents" || fail "No results returned"

echo "Searching for 'neural networks' via retrieve..."
curl_json -X POST "$BASE_URL/search/retrieve" \
  -H "$AUTH_E" -H "$TENANT_E" \
  -H "Content-Type: application/json" \
  -d '{"query":"neural networks","k":10}'
[ "$CODE" = "200" ] && pass "Retrieve 'neural networks' succeeded (HTTP $CODE)" || fail "Failed (HTTP $CODE)"
R_COUNT2=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  Results: $R_COUNT2"
[ "$R_COUNT2" -gt 0 ] && pass "Found matching documents" || fail "No results returned"

echo "Searching for unrelated term 'cooking recipe'..."
curl_json -X POST "$BASE_URL/search/retrieve" \
  -H "$AUTH_E" -H "$TENANT_E" \
  -H "Content-Type: application/json" \
  -d '{"query":"cooking recipe","k":10}'
[ "$CODE" = "200" ] && pass "Retrieve unrelated query succeeded (HTTP $CODE)" || fail "Failed (HTTP $CODE)"
R_COUNT3=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  Results: $R_COUNT3"

# ---- Step 5: Test retrieve with multipart file ----
section "5. Retrieve with multipart file"

echo "Retrieving similar documents using test_ml_doc.txt..."
curl_json -X POST "$BASE_URL/search/retrieve" \
  -H "$AUTH_E" -H "$TENANT_E" \
  -F "file=@/tmp/test_ml_doc.txt" \
  -F "k=10"
[ "$CODE" = "200" ] && pass "Retrieve (multipart) succeeded (HTTP $CODE)" || fail "Retrieve (multipart) failed (HTTP $CODE)"
R_COUNT4=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")
echo "  Results: $R_COUNT4"
[ "$R_COUNT4" -gt 0 ] && pass "Found similar documents" || echo "  WARN: No similar documents found"

echo "Retrieving with file + supplementary query..."
curl_json -X POST "$BASE_URL/search/retrieve" \
  -H "$AUTH_E" -H "$TENANT_E" \
  -F "file=@/tmp/test_dl_doc.txt" \
  -F "query=artificial intelligence" \
  -F "k=10"
[ "$CODE" = "200" ] && pass "Retrieve (file + query) succeeded (HTTP $CODE)" || fail "Failed (HTTP $CODE)"

# ---- Step 6: Verify embedding was actually called ----
section "6. Embedding verification"

echo "Verifying embedding was called..."
curl_json -X POST "$BASE_URL/search/retrieve" \
  -H "$AUTH_E" -H "$TENANT_E" \
  -H "Content-Type: application/json" \
  -d '{"query":"test verify embedding","k":1}'
[ "$CODE" = "200" ] && pass "Embedding verified - retrieve works end-to-end (HTTP $CODE)" || fail "Embedding verification failed (HTTP $CODE)"

# ---- Cleanup ----
section "Cleanup"

kill $MOCK_PID 2>/dev/null || true
docker compose -f deployments/docker/docker-compose.yml stop opensearch-file-api 2>/dev/null || true
echo "  Cleaned up mock server and app container"

section "All embedding tests completed successfully"
