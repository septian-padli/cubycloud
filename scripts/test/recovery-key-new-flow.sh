#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"


PORT="${PORT:-8394}"
if lsof -nP -iTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "Port $PORT is already in use; set PORT to an available port and retry." >&2
  exit 1
fi

TEST_DIR=$(mktemp -d "${TMPDIR:-/tmp}/cubycloud-recovery-flow.XXXXXX")
BACKEND_PID=
cleanup() {
  if [[ -n "$BACKEND_PID" ]]; then
    kill "$BACKEND_PID" 2>/dev/null || true
    wait "$BACKEND_PID" 2>/dev/null || true
  fi
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT

DATA_DIR="$TEST_DIR/data"
mkdir -p "$DATA_DIR/db" "$DATA_DIR/temp/tus"
cd "$REPO_ROOT/apps/backend"
go build -o "$TEST_DIR/server" ./cmd/server
PORT="$PORT" DATA_DIR_PATH="$DATA_DIR" JWT_SECRET=test \
  "$TEST_DIR/server" >"$TEST_DIR/server.log" 2>&1 &
BACKEND_PID=$!
BASE_URL="http://127.0.0.1:$PORT"

ready=false
for _ in {1..40}; do
  if curl -fsS "$BASE_URL/api/v1/setup/status" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 0.25
done
if [[ "$ready" != true ]]; then
  cat "$TEST_DIR/server.log" >&2
  echo "Backend did not become ready on port $PORT." >&2
  exit 1
fi

CLIENT_KEY="ABCDE-FGHJK-LMNPQ-RSTUV-WXYZ2-34567"
if [[ ! "$CLIENT_KEY" =~ ^[A-Z0-9]{5}(-[A-Z0-9]{5}){5}$ ]]; then
  echo "Test recovery key is not in the 6×5 format." >&2
  exit 1
fi

echo "1. Completing setup with a client-generated 6×5 recovery key..."
SETUP_RESPONSE=$(curl -fsS -X POST "$BASE_URL/api/v1/setup/complete" \
  -H "Content-Type: application/json" \
  -d '{
    "admin": {
      "username": "admin",
      "email": "admin@test.com",
      "password": "password123",
      "recovery_key": "'"$CLIENT_KEY"'"
    },
    "storage_nodes": [{
      "path": "'"$DATA_DIR/storage/default"'",
      "capacity_bytes": 500000000000,
      "enabled": true
    }],
    "placement_strategy": "FILL_FIRST"
  }')

BACKEND_KEY=$(jq -r '.recovery_key // empty' <<<"$SETUP_RESPONSE")
if [[ "$BACKEND_KEY" != "$CLIENT_KEY" ]]; then
  echo "Setup did not return the submitted 6×5 key: $SETUP_RESPONSE" >&2
  exit 1
fi

echo "2. Verifying the stored SHA-256 hash..."
STORED_HASH=$(sqlite3 "$DATA_DIR/db/cubycloud.db" \
  "SELECT recovery_key_hash FROM users WHERE username='admin';")
EXPECTED_HASH=$(printf '%s' "${CLIENT_KEY//-/}" | shasum -a 256 | cut -d ' ' -f 1)
if [[ "$STORED_HASH" != "$EXPECTED_HASH" ]]; then
  echo "Stored recovery hash does not match canonical SHA-256." >&2
  exit 1
fi

echo "3. Resetting the password and checking key rotation..."
RESET_RESPONSE=$(curl -fsS -X POST "$BASE_URL/api/v1/auth/reset-password" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "recovery_key": "'"$CLIENT_KEY"'",
    "new_password": "newpassword456"
  }')
RESET_MESSAGE=$(jq -r '.message // empty' <<<"$RESET_RESPONSE")
NEXT_KEY=$(jq -r '.new_recovery_key // empty' <<<"$RESET_RESPONSE")
if [[ "$RESET_MESSAGE" != "password reset successful" || ! "$NEXT_KEY" =~ ^[A-Z0-9]{5}(-[A-Z0-9]{5}){5}$ ]]; then
  echo "Password reset did not return a new 6×5 recovery key: $RESET_RESPONSE" >&2
  exit 1
fi

REUSED_STATUS=$(curl -sS -o "$TEST_DIR/reused-key.json" -w '%{http_code}' \
  -X POST "$BASE_URL/api/v1/auth/reset-password" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "recovery_key": "'"$CLIENT_KEY"'",
    "new_password": "hackedpassword"
  }')
if [[ "$REUSED_STATUS" != 401 ]]; then
  echo "Consumed recovery key was accepted again (HTTP $REUSED_STATUS)." >&2
  exit 1
fi

NORMALIZED_KEY=$(tr '[:upper:]' '[:lower:]' <<<"${NEXT_KEY//-/}")
NEXT_RESET=$(curl -fsS -X POST "$BASE_URL/api/v1/auth/reset-password" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "recovery_key": "'"$NORMALIZED_KEY"'",
    "new_password": "password7890"
  }')
if [[ "$(jq -r '.message // empty' <<<"$NEXT_RESET")" != "password reset successful" ]]; then
  echo "Ungrouped, lowercase replacement key was rejected: $NEXT_RESET" >&2
  exit 1
fi

echo "Recovery Key setup, SHA-256 storage, single-use rotation, and normalization passed."
