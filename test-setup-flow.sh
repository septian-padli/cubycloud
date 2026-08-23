#!/bin/bash
set -e

echo "=========================================="
echo "CubyCloud Setup Flow - Quick Test"
echo "=========================================="

# Kill existing
pkill -9 -f "cmd/server/main.go" 2>/dev/null || true
lsof -ti :8080 | xargs kill -9 2>/dev/null || true
sleep 2

# Setup
TEST_DIR="/tmp/cuby-quick-test"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"/{db,storage/{disk1,disk2},temp/tus}

echo ""
echo "Starting backend..."
cd apps/backend
PORT=8080 DATA_DIR_PATH="$TEST_DIR" JWT_SECRET=quicktest \
  go run cmd/server/main.go > "$TEST_DIR/backend.log" 2>&1 &
PID=$!
sleep 4

if ! curl -sf http://localhost:8080/api/v1/setup/scan-nodes > /dev/null; then
  echo "❌ Backend failed"
  cat "$TEST_DIR/backend.log"
  exit 1
fi

echo "✓ Backend ready (PID: $PID)"
echo ""

echo "Testing setup flow..."
RESULT=$(curl -s -X POST http://localhost:8080/api/v1/setup/complete \
  -H "Content-Type: application/json" \
  -d "{
    \"admin\": {
      \"username\": \"admin\",
      \"email\": \"admin@test.com\",
      \"password\": \"password123\",
      \"recovery_key\": \"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\"
    },
    \"storage_nodes\": [
      {\"path\": \"$TEST_DIR/storage/disk1\", \"capacity_bytes\": 1000000000000, \"enabled\": true}
    ],
    \"placement_strategy\": \"FILL_FIRST\"
  }")

KEY=$(echo "$RESULT" | jq -r '.recovery_key // empty')
if [ -n "$KEY" ] && [ ${#KEY} -eq 64 ]; then
  echo "✅ Setup completed successfully!"
  echo ""
  echo "Recovery Key: $KEY"
  echo "Database: $TEST_DIR/db/cubycloud.db"
  echo ""
  echo "User:"
  sqlite3 "$TEST_DIR/db/cubycloud.db" "SELECT '  ' || username || ' (' || role || ')' FROM users;"
  echo ""
  echo "Storage:"
  sqlite3 "$TEST_DIR/db/cubycloud.db" "SELECT '  ' || type || ' @ ' || path FROM storage_nodes;"
else
  echo "❌ Setup failed"
  echo "$RESULT" | jq .
fi

kill $PID 2>/dev/null
echo ""
echo "Test complete. Run './scripts/dev.sh' for full browser test."
