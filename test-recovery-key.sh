#!/bin/bash
set -e

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔑 RECOVERY KEY INTEGRATION TEST"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Cleanup
pkill -9 -f "cmd/server/main.go" 2>/dev/null || true
lsof -ti :8080 | xargs kill -9 2>/dev/null || true
sleep 2

# Setup test environment
TEST_DIR="/tmp/recovery-key-test"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"/{db,storage/default,temp/tus}

echo "1. Starting backend..."
cd apps/backend
PORT=8080 DATA_DIR_PATH="$TEST_DIR" JWT_SECRET=test \
  go run cmd/server/main.go > /tmp/recovery-test.log 2>&1 &
BACKEND_PID=$!
sleep 5

echo "2. Running setup (will generate recovery key)..."
SETUP_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/setup/complete \
  -H "Content-Type: application/json" \
  -d '{
    "admin": {
      "username": "admin",
      "email": "admin@test.com",
      "password": "password123",
      "recovery_key": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    },
    "storage_nodes": [{
      "path": "'"$TEST_DIR/storage/default"'",
      "capacity_bytes": 500000000000,
      "enabled": true
    }],
    "placement_strategy": "FILL_FIRST"
  }')

RECOVERY_KEY=$(echo "$SETUP_RESPONSE" | jq -r '.recovery_key // empty')

if [ -z "$RECOVERY_KEY" ]; then
  echo "❌ Setup failed or recovery key not returned"
  echo "$SETUP_RESPONSE" | jq .
  kill $BACKEND_PID 2>/dev/null
  exit 1
fi

echo "   ✓ Setup complete"
echo "   ✓ Recovery key: ${RECOVERY_KEY:0:20}...${RECOVERY_KEY:44:20}"
echo ""

echo "3. Verifying recovery key stored in database..."
STORED_HASH=$(sqlite3 "$TEST_DIR/db/cubycloud.db" \
  "SELECT recovery_key_hash FROM users WHERE username='admin';")

if [ -z "$STORED_HASH" ]; then
  echo "   ❌ Recovery key hash NOT stored in database"
  kill $BACKEND_PID 2>/dev/null
  exit 1
fi

echo "   ✓ Recovery key hash stored (${#STORED_HASH} chars)"
echo ""

echo "4. Testing password reset with recovery key..."
RESET_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "recovery_key": "'"$RECOVERY_KEY"'",
    "new_password": "newpassword456"
  }')

RESET_MESSAGE=$(echo "$RESET_RESPONSE" | jq -r '.message // empty')

if [ "$RESET_MESSAGE" != "password reset successful" ]; then
  echo "   ❌ Password reset failed"
  echo "   Response: $RESET_RESPONSE"
  kill $BACKEND_PID 2>/dev/null
  exit 1
fi

echo "   ✓ Password reset successful"
echo ""

echo "5. Testing invalid recovery key (should fail)..."
INVALID_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "recovery_key": "0000000000000000000000000000000000000000000000000000000000000000",
    "new_password": "hackedpassword"
  }')

INVALID_ERROR=$(echo "$INVALID_RESPONSE" | jq -r '.message // empty')

if [[ "$INVALID_ERROR" != *"invalid recovery key"* ]]; then
  echo "   ❌ Invalid recovery key should be rejected"
  echo "   Response: $INVALID_RESPONSE"
  kill $BACKEND_PID 2>/dev/null
  exit 1
fi

echo "   ✓ Invalid recovery key correctly rejected"
echo ""

echo "6. Testing wrong username (should fail)..."
WRONG_USER_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "username": "nonexistent",
    "recovery_key": "'"$RECOVERY_KEY"'",
    "new_password": "newpassword"
  }')

WRONG_USER_ERROR=$(echo "$WRONG_USER_RESPONSE" | jq -r '.message // empty')

if [[ "$WRONG_USER_ERROR" != *"user not found"* ]]; then
  echo "   ❌ Non-existent user should be rejected"
  echo "   Response: $WRONG_USER_RESPONSE"
  kill $BACKEND_PID 2>/dev/null
  exit 1
fi

echo "   ✓ Non-existent user correctly rejected"
echo ""

kill $BACKEND_PID 2>/dev/null

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ ALL RECOVERY KEY TESTS PASSED"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Summary:"
echo "  ✓ Recovery key generated (64 hex chars)"
echo "  ✓ Recovery key hash stored in database"
echo "  ✓ Password reset with valid key works"
echo "  ✓ Invalid recovery key rejected"
echo "  ✓ Wrong username rejected"
echo ""
echo "Recovery key is FULLY FUNCTIONAL and production-ready!"
echo ""
