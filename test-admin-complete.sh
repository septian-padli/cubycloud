#!/bin/bash
set -e

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🧪 Admin Complete Integration Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Cleanup function
cleanup() {
  echo ""
  echo "🧹 Cleaning up..."
  if [ -n "$SERVER_PID" ]; then
    kill $SERVER_PID 2>/dev/null || true
  fi
  rm -f /tmp/cuby_test_complete.db
  rm -rf /tmp/cuby_test_storage_complete
}
trap cleanup EXIT

# 1. Setup test database
echo "1️⃣  Setup: Creating test database..."
rm -f /tmp/cuby_test_complete.db
mkdir -p /tmp/cuby_test_storage_complete/default

# 2. Start server
echo ""
echo "2️⃣  Starting backend server..."
cd apps/backend
JWT_SECRET="test-secret-key-32-bytes-long!!" \
DATA_DIR="/tmp/cuby_test_storage_complete" \
DB_PATH="/tmp/cuby_test_complete.db" \
SERVER_PORT="8765" \
go run cmd/server/main.go &
SERVER_PID=$!
cd ../..

sleep 3

# 3. Setup: Create super admin
echo ""
echo "3️⃣  Setup: Creating super admin..."
SETUP_RESP=$(curl -s -c /tmp/cuby_cookies.txt -X POST http://localhost:8765/api/v1/setup \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.local","password":"Test1234!","recovery_key":"test-recovery-key-1234567890"}')

if echo "$SETUP_RESP" | grep -q "error"; then
  echo "❌ Setup failed: $SETUP_RESP"
  exit 1
fi
echo "✅ Super admin created, JWT cookie set"

# 4. Test: GET /admin/settings (initial)
echo ""
echo "4️⃣  Test: GET /admin/settings..."
SETTINGS=$(curl -s -b /tmp/cuby_cookies.txt http://localhost:8765/api/v1/admin/settings)
if echo "$SETTINGS" | grep -q "default_quota"; then
  echo "✅ Settings retrieved"
else
  echo "❌ Failed: $SETTINGS"
  exit 1
fi

# 5. Test: PUT /admin/settings (update quota)
echo ""
echo "5️⃣  Test: PUT /admin/settings..."
UPDATE=$(curl -s -b /tmp/cuby_cookies.txt -X PUT http://localhost:8765/api/v1/admin/settings \
  -H "Content-Type: application/json" \
  -d '{"default_quota":{"mode":"limited","gb":20},"audit_retention_days":30,"worker_concurrency":5}')
if echo "$UPDATE" | grep -q "default_quota"; then
  echo "✅ Settings updated"
else
  echo "❌ Failed: $UPDATE"
  exit 1
fi

# 6. Test: GET /admin/storage-nodes
echo ""
echo "6️⃣  Test: GET /admin/storage-nodes..."
NODES=$(curl -s -b /tmp/cuby_cookies.txt http://localhost:8765/api/v1/admin/storage-nodes)
if echo "$NODES" | grep -q "nodes"; then
  echo "✅ Storage nodes retrieved"
  DEFAULT_NODE_ID=$(echo "$NODES" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  echo "   Default node ID: $DEFAULT_NODE_ID"
else
  echo "❌ Failed: $NODES"
  exit 1
fi

# 7. Test: POST /admin/storage-nodes (add local)
echo ""
echo "7️⃣  Test: POST /admin/storage-nodes (add local)..."
mkdir -p /tmp/cuby_test_storage_complete/disk2
ADD_LOCAL=$(curl -s -b /tmp/cuby_cookies.txt -X POST http://localhost:8765/api/v1/admin/storage-nodes \
  -H "Content-Type: application/json" \
  -d '{"path":"/tmp/cuby_test_storage_complete/disk2","capacity_cap_gb":50,"enabled":true}')
if echo "$ADD_LOCAL" | grep -q '"id"'; then
  LOCAL_NODE_ID=$(echo "$ADD_LOCAL" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  echo "✅ Local node added: $LOCAL_NODE_ID"
else
  echo "❌ Failed: $ADD_LOCAL"
  exit 1
fi

# 8. Test: POST /admin/storage-nodes/cloud (add S3)
echo ""
echo "8️⃣  Test: POST /admin/storage-nodes/cloud (add S3)..."
ADD_CLOUD=$(curl -s -b /tmp/cuby_cookies.txt -X POST http://localhost:8765/api/v1/admin/storage-nodes/cloud \
  -H "Content-Type: application/json" \
  -d '{"name":"Test S3","endpoint":"https://s3.example.com","bucket":"test-bucket","region":"us-east-1","access_key":"AKIATEST","secret_key":"secrettest","capacity_cap_gb":100,"enabled":false}')
if echo "$ADD_CLOUD" | grep -q '"id"'; then
  CLOUD_NODE_ID=$(echo "$ADD_CLOUD" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  echo "✅ Cloud node added: $CLOUD_NODE_ID"
else
  echo "❌ Failed: $ADD_CLOUD"
  exit 1
fi

# 9. Test: PUT /admin/storage-nodes/:id (edit)
echo ""
echo "9️⃣  Test: PUT /admin/storage-nodes/$LOCAL_NODE_ID (edit)..."
EDIT=$(curl -s -b /tmp/cuby_cookies.txt -X PUT http://localhost:8765/api/v1/admin/storage-nodes/$LOCAL_NODE_ID \
  -H "Content-Type: application/json" \
  -d '{"capacity_cap_gb":75,"enabled":true}')
if echo "$EDIT" | grep -q "message"; then
  echo "✅ Node edited"
else
  echo "❌ Failed: $EDIT"
  exit 1
fi

# 10. Test: GET /admin/storage-nodes/:id/disable
echo ""
echo "🔟 Test: GET /admin/storage-nodes/$CLOUD_NODE_ID/disable..."
DISABLE=$(curl -s -b /tmp/cuby_cookies.txt http://localhost:8765/api/v1/admin/storage-nodes/$CLOUD_NODE_ID/disable)
if echo "$DISABLE" | grep -q "message"; then
  echo "✅ Node disabled"
else
  echo "❌ Failed: $DISABLE"
  exit 1
fi

# 11. Test: DELETE /admin/storage-nodes/:id
echo ""
echo "1️⃣1️⃣  Test: DELETE /admin/storage-nodes/$CLOUD_NODE_ID..."
DELETE=$(curl -s -b /tmp/cuby_cookies.txt -X DELETE http://localhost:8765/api/v1/admin/storage-nodes/$CLOUD_NODE_ID)
if echo "$DELETE" | grep -q "message"; then
  echo "✅ Node deleted (soft)"
else
  echo "❌ Failed: $DELETE"
  exit 1
fi

# 12. Test: GET /admin/storage-nodes/rescan
echo ""
echo "1️⃣2️⃣  Test: GET /admin/storage-nodes/rescan..."
RESCAN=$(curl -s -b /tmp/cuby_cookies.txt http://localhost:8765/api/v1/admin/storage-nodes/rescan)
if echo "$RESCAN" | grep -q "nodes"; then
  echo "✅ Rescan completed"
else
  echo "❌ Failed: $RESCAN"
  exit 1
fi

# 13. Test: GET /admin/backup
echo ""
echo "1️⃣3️⃣  Test: GET /admin/backup..."
curl -s -b /tmp/cuby_cookies.txt http://localhost:8765/api/v1/admin/backup -o /tmp/backup_test.db
if [ -f /tmp/backup_test.db ] && [ -s /tmp/backup_test.db ]; then
  echo "✅ Backup downloaded ($(stat -f%z /tmp/backup_test.db 2>/dev/null || stat -c%s /tmp/backup_test.db) bytes)"
  rm -f /tmp/backup_test.db
else
  echo "❌ Backup failed"
  exit 1
fi

# 14. Test: Validation - path does not exist
echo ""
echo "1️⃣4️⃣  Test: Validation - nonexistent path..."
INVALID=$(curl -s -b /tmp/cuby_cookies.txt -X POST http://localhost:8765/api/v1/admin/storage-nodes \
  -H "Content-Type: application/json" \
  -d '{"path":"/nonexistent/path","capacity_cap_gb":10,"enabled":true}')
if echo "$INVALID" | grep -q "error"; then
  echo "✅ Validation correctly rejected nonexistent path"
else
  echo "❌ Should have rejected: $INVALID"
  exit 1
fi

# 15. Test: Validation - duplicate path
echo ""
echo "1️⃣5️⃣  Test: Validation - duplicate path..."
DUP=$(curl -s -b /tmp/cuby_cookies.txt -X POST http://localhost:8765/api/v1/admin/storage-nodes \
  -H "Content-Type: application/json" \
  -d '{"path":"/tmp/cuby_test_storage_complete/disk2","capacity_cap_gb":10,"enabled":true}')
if echo "$DUP" | grep -q "error"; then
  echo "✅ Validation correctly rejected duplicate path"
else
  echo "❌ Should have rejected: $DUP"
  exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ ALL 15 TESTS PASSED"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
