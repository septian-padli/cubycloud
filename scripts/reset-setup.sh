#!/bin/bash
set -e

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔄 RESET SETUP (Development Only)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

DB_PATH="apps/backend/data/db/cubycloud.db"

# 1. Kill processes
echo "1. Stopping dev servers..."
lsof -ti :8080 | xargs kill -9 2>/dev/null || true
lsof -ti :5173 | xargs kill -9 2>/dev/null || true
pkill -9 -f 'cmd/server/main.go' 2>/dev/null || true
pkill -9 -f 'vite' 2>/dev/null || true
sleep 1

# 2. Clean dev & test databases
echo "2. Cleaning dev databases..."
rm -rf /tmp/cubycloud-dev /tmp/recovery-key-test /tmp/recovery-key-new-flow /tmp/cuby-quick-test
if [ -f "$DB_PATH" ]; then
  rm -f "$DB_PATH" "$DB_PATH-shm" "$DB_PATH-wal"
  echo "   ✓ Local database cleared: $DB_PATH"
else
  echo "   ✓ Database already clean"
fi

# 3. Clear localStorage (reminder)
echo ""
echo "2. Database reset complete"
echo ""
echo "3. Next steps:"
echo "   a. Start dev server: ./scripts/dev.sh"
echo "   b. Hard refresh browser (Cmd+Shift+R / Ctrl+Shift+R)"
echo "   c. Browser localStorage auto-cleared on load"
echo ""
echo "✅ Ready for fresh setup!"
echo ""
