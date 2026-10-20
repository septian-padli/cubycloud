#!/usr/bin/env bash
set -euo pipefail

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🧪 Admin Settings Integration Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

API="http://localhost:8080/api/v1"
ADMIN_TOKEN=""

api_call() {
    local method=$1
    local endpoint=$2
    local data=${3:-}
    
    if [ -n "$data" ]; then
        curl -s -X "$method" \
            -H "Content-Type: application/json" \
            -H "Cookie: cuby_token=$ADMIN_TOKEN" \
            -d "$data" \
            "$API$endpoint"
    else
        curl -s -X "$method" \
            -H "Cookie: cuby_token=$ADMIN_TOKEN" \
            "$API$endpoint"
    fi
}

echo "1️⃣  Setup: Creating test database..."
rm -rf /tmp/cubycloud-test
mkdir -p /tmp/cubycloud-test/db /tmp/cubycloud-test/storage/default

export DATA_DIR_PATH=/tmp/cubycloud-test
export PORT=8080
export JWT_SECRET=test-secret-key-for-admin-testing

echo ""
echo "2️⃣  Starting backend server..."
cd "$(dirname "$0")/apps/backend"
go build -o /tmp/cubycloud-server ./cmd/server
/tmp/cubycloud-server &
SERVER_PID=$!

cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    kill $SERVER_PID 2>/dev/null || true
    rm -rf /tmp/cubycloud-test
    rm -f /tmp/cubycloud-server
}
trap cleanup EXIT

sleep 3

echo ""
echo "3️⃣  Running setup to create super_admin user..."
SETUP_RESP=$(curl -si -X POST "$API/setup/complete" \
    -H "Content-Type: application/json" \
    -d '{
        "admin": {
            "username": "admin",
            "email": "admin@test.local",
            "password": "Admin123!",
            "recovery_key": "ABCDE-FGHJK-LMNPQ-RSTUV-WXYZ2-34567"
        },
        "storage_nodes": [{
            "path": "/tmp/cubycloud-test/storage/default",
            "capacity_bytes": 107374182400,
            "enabled": true
        }],
        "cloud": {
            "enabled": false
        },
        "placement_strategy": "FILL_FIRST"
    }')

# Extract token from Set-Cookie header
ADMIN_TOKEN=$(echo "$SETUP_RESP" | grep -i 'set-cookie: cuby_token=' | sed 's/.*token=\([^;]*\).*/\1/')

if [ -z "$ADMIN_TOKEN" ]; then
    echo "❌ Failed to get admin token from setup"
    echo "Response:"
    echo "$SETUP_RESP"
    exit 1
fi

echo "✅ Admin token obtained"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 Test 1: GET /admin/settings"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
SETTINGS=$(api_call GET /admin/settings)
echo "$SETTINGS" | jq .

# Validate structure (setup defaults to limited 100GB)
DEFAULT_QUOTA=$(echo "$SETTINGS" | jq -r '.default_quota.mode')
DEFAULT_GB=$(echo "$SETTINGS" | jq -r '.default_quota.gb')
if [ "$DEFAULT_QUOTA" != "limited" ] || [ "$DEFAULT_GB" != "100" ]; then
    echo "❌ Expected default_quota: limited 100GB, got: mode=$DEFAULT_QUOTA, gb=$DEFAULT_GB"
    exit 1
fi
echo "✅ GET settings OK"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📝 Test 2: PUT /admin/settings"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
UPDATE_RESP=$(api_call PUT /admin/settings '{
    "default_quota": {"mode": "limited", "gb": 50},
    "audit_retention_days": 30,
    "worker_concurrency": 4
}')
echo "$UPDATE_RESP" | jq .

UPDATED_MODE=$(echo "$UPDATE_RESP" | jq -r '.default_quota.mode')
UPDATED_GB=$(echo "$UPDATE_RESP" | jq -r '.default_quota.gb')
if [ "$UPDATED_MODE" != "limited" ] || [ "$UPDATED_GB" != "50" ]; then
    echo "❌ Quota update failed"
    exit 1
fi
echo "✅ PUT settings OK"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📦 Test 3: GET /admin/storage-nodes"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
NODES=$(api_call GET /admin/storage-nodes)
echo "$NODES" | jq .

NODE_COUNT=$(echo "$NODES" | jq '.nodes | length')
if [ "$NODE_COUNT" -lt 1 ]; then
    echo "❌ Expected at least 1 node"
    exit 1
fi

FIRST_NODE_ID=$(echo "$NODES" | jq -r '.nodes[0].id')
echo "✅ GET storage nodes OK ($NODE_COUNT nodes)"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔌 Test 4: PUT /admin/storage-nodes/:id/disable"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
DISABLE_RESP=$(api_call PUT "/admin/storage-nodes/$FIRST_NODE_ID/disable")
echo "$DISABLE_RESP" | jq .

NODES_AFTER=$(api_call GET /admin/storage-nodes)
DISABLED_STATUS=$(echo "$NODES_AFTER" | jq -r ".nodes[] | select(.id == \"$FIRST_NODE_ID\") | .enabled")
if [ "$DISABLED_STATUS" != "false" ]; then
    echo "❌ Node not disabled"
    exit 1
fi
echo "✅ Disable node OK"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔄 Test 5: POST /admin/storage-nodes/rescan"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
RESCAN_RESP=$(api_call POST /admin/storage-nodes/rescan)
echo "$RESCAN_RESP" | jq .

RESCAN_COUNT=$(echo "$RESCAN_RESP" | jq '.nodes | length')
if [ "$RESCAN_COUNT" -lt 1 ]; then
    echo "❌ Rescan returned no nodes"
    exit 1
fi
echo "✅ Rescan OK ($RESCAN_COUNT nodes)"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "❌ Test 6: Validation - quota < 1GB"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
INVALID=$(api_call PUT /admin/settings '{"default_quota": {"mode": "limited", "gb": 0}}')
ERROR_MSG=$(echo "$INVALID" | jq -r '.error // empty')
if [[ ! "$ERROR_MSG" =~ "at least 1 GB" ]]; then
    echo "❌ Expected validation error"
    exit 1
fi
echo "✅ Validation works"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "💾 Test 7: GET /admin/backup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
curl -s -o /tmp/cubycloud-backup-test.db \
    -H "Cookie: cuby_token=$ADMIN_TOKEN" \
    "$API/admin/backup"

if [ ! -f /tmp/cubycloud-backup-test.db ]; then
    echo "❌ Backup not downloaded"
    exit 1
fi

BACKUP_SIZE=$(stat -f%z /tmp/cubycloud-backup-test.db 2>/dev/null || stat -c%s /tmp/cubycloud-backup-test.db)
if [ "$BACKUP_SIZE" -lt 1000 ]; then
    echo "❌ Backup too small"
    exit 1
fi
echo "✅ Backup OK ($BACKUP_SIZE bytes)"
rm -f /tmp/cubycloud-backup-test.db
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ ALL TESTS PASSED"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
