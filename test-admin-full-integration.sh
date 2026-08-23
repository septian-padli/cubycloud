#!/usr/bin/env bash
set -euo pipefail

API="http://localhost:8080/api/v1"
ADMIN_TOKEN=""

export DATA_DIR_PATH=/tmp/cubycloud-test-admin-full
export PORT=8080
export JWT_SECRET=test-secret-admin-full

# Cleanup function
cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    pkill -f "cmd/server" || true
    rm -rf $DATA_DIR_PATH
}
trap cleanup EXIT

# API call helper
api_call() {
    local method=$1
    local endpoint=$2
    local data=${3:-}
    
    if [ -z "$data" ]; then
        curl -s -X $method "$API$endpoint" \
            -H "Cookie: cuby_token=$ADMIN_TOKEN"
    else
        curl -s -X $method "$API$endpoint" \
            -H "Content-Type: application/json" \
            -H "Cookie: cuby_token=$ADMIN_TOKEN" \
            -d "$data"
    fi
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🧪 Admin Full Integration Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "1️⃣  Setup: Creating test database..."
rm -rf $DATA_DIR_PATH
mkdir -p $DATA_DIR_PATH/db $DATA_DIR_PATH/storage/default $DATA_DIR_PATH/storage/secondary
echo ""

echo "2️⃣  Starting backend server..."
cd apps/backend
go build -o /tmp/cubycloud-test-server ./cmd/server
/tmp/cubycloud-test-server &
SERVER_PID=$!
cd ../..
sleep 2
echo ""

echo "3️⃣  Running setup to create super_admin user..."
SETUP_RESP=$(curl -s -X POST "$API/setup/complete" \
    -H "Content-Type: application/json" \
    -d '{
        "admin": {
            "username": "admin",
            "email": "admin@test.local",
            "password": "Admin123!",
            "recovery_key": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
        },
        "storage_nodes": [{
            "path": "'"$DATA_DIR_PATH"'/storage/default",
            "capacity_bytes": 107374182400,
            "enabled": true
        }],
        "cloud": {"enabled": false},
        "placement_strategy": "FILL_FIRST"
    }')

ADMIN_TOKEN=$(echo "$SETUP_RESP" | jq -r '.token // empty')
if [ -z "$ADMIN_TOKEN" ]; then
    echo "❌ Failed to get admin token"
    echo "$SETUP_RESP" | jq .
    exit 1
fi
echo "✅ Admin token obtained"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 Test 1: GET /admin/settings"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
SETTINGS=$(api_call GET /admin/settings)
echo "$SETTINGS" | jq .

DEFAULT_QUOTA=$(echo "$SETTINGS" | jq -r '.default_quota.mode')
DEFAULT_GB=$(echo "$SETTINGS" | jq -r '.default_quota.gb')
if [ "$DEFAULT_QUOTA" != "limited" ] || [ "$DEFAULT_GB" != "100" ]; then
    echo "❌ Expected default_quota: limited 100GB, got: mode=$DEFAULT_QUOTA, gb=$DEFAULT_GB"
    exit 1
fi
echo "✅ GET settings OK"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📦 Test 2: GET /admin/storage-nodes (initial)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
NODES=$(api_call GET /admin/storage-nodes)
echo "$NODES" | jq .

NODES_COUNT=$(echo "$NODES" | jq '.nodes | length')
if [ "$NODES_COUNT" -lt 1 ]; then
    echo "❌ Expected at least 1 node from setup"
    exit 1
fi
echo "✅ GET storage nodes OK ($NODES_COUNT nodes)"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "➕ Test 3: POST /admin/storage-nodes (add local node)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
ADD_NODE_RESP=$(api_call POST /admin/storage-nodes '{
    "path": "'"$DATA_DIR_PATH"'/storage/secondary",
    "capacity_cap_gb": 50,
    "enabled": true
}')
echo "$ADD_NODE_RESP" | jq .

NEW_NODE_ID=$(echo "$ADD_NODE_RESP" | jq -r '.id // empty')
if [ -z "$NEW_NODE_ID" ]; then
    echo "❌ Failed to create node"
    exit 1
fi
echo "✅ Add local node OK (ID: $NEW_NODE_ID)"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "❌ Test 4: POST /admin/storage-nodes (duplicate path)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
DUP_RESP=$(api_call POST /admin/storage-nodes '{
    "path": "'"$DATA_DIR_PATH"'/storage/secondary",
    "capacity_cap_gb": 50,
    "enabled": true
}')
DUP_ERROR=$(echo "$DUP_RESP" | jq -r '.error // empty')
if [[ ! "$DUP_ERROR" =~ "already exists" ]]; then
    echo "❌ Expected duplicate path error"
    exit 1
fi
echo "✅ Duplicate validation works"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✏️  Test 5: PUT /admin/storage-nodes/:id (update capacity)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
UPDATE_RESP=$(api_call PUT /admin/storage-nodes/$NEW_NODE_ID '{
    "capacity_cap_gb": 75
}')
echo "$UPDATE_RESP" | jq .

UPDATE_MSG=$(echo "$UPDATE_RESP" | jq -r '.message // empty')
if [[ ! "$UPDATE_MSG" =~ "updated successfully" ]]; then
    echo "❌ Expected success message"
    exit 1
fi
echo "✅ Update node capacity OK"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "❌ Test 6: PUT /admin/storage-nodes/:id (invalid capacity)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
INVALID_RESP=$(api_call PUT /admin/storage-nodes/$NEW_NODE_ID '{
    "capacity_cap_gb": 0
}')
INVALID_ERROR=$(echo "$INVALID_RESP" | jq -r '.error // empty')
if [[ ! "$INVALID_ERROR" =~ "at least 1 GB" ]]; then
    echo "❌ Expected capacity validation error"
    exit 1
fi
echo "✅ Capacity validation works"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔌 Test 7: PUT /admin/storage-nodes/:id/disable"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
DISABLE_RESP=$(api_call PUT /admin/storage-nodes/$NEW_NODE_ID/disable)
echo "$DISABLE_RESP" | jq .
echo "✅ Disable node OK"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🗑️  Test 8: DELETE /admin/storage-nodes/:id"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
DELETE_RESP=$(api_call DELETE /admin/storage-nodes/$NEW_NODE_ID)
echo "$DELETE_RESP" | jq .

DELETE_MSG=$(echo "$DELETE_RESP" | jq -r '.message // empty')
if [[ ! "$DELETE_MSG" =~ "deleted successfully" ]]; then
    echo "❌ Expected success message"
    exit 1
fi
echo "✅ Delete node OK"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔄 Test 9: POST /admin/storage-nodes/rescan"
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
echo "📝 Test 10: PUT /admin/settings"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
UPDATE_SETTINGS_RESP=$(api_call PUT /admin/settings '{
    "default_quota": {"mode": "limited", "gb": 200},
    "audit_retention_days": 60,
    "worker_concurrency": 5
}')
echo "$UPDATE_SETTINGS_RESP" | jq .

NEW_QUOTA=$(echo "$UPDATE_SETTINGS_RESP" | jq -r '.default_quota.gb')
if [ "$NEW_QUOTA" != "200" ]; then
    echo "❌ Settings not updated"
    exit 1
fi
echo "✅ PUT settings OK"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "💾 Test 11: GET /admin/backup"
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
echo "✅ ALL TESTS PASSED (11/11)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
