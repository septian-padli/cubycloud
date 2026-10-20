#!/bin/bash
# Development runner: backend + frontend

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting CubyCloud Development Environment${NC}"

if ! command -v air &> /dev/null; then
    echo -e "${RED}Air not found. Install Air to run the development backend.${NC}"
    exit 1
fi

if ! command -v bun &> /dev/null; then
    echo -e "${RED}Bun not found. Install Bun${NC}"
    exit 1
fi

# Setup backend env if missing
if [ ! -f apps/backend/.env ]; then
    echo -e "${YELLOW}Creating backend .env${NC}"
    cat > apps/backend/.env <<EOF
PORT=8383
DATA_DIR_PATH=/tmp/cubycloud-dev
JWT_SECRET=dev-secret-change-in-production
EOF
fi

# Create data directories
export DATA_DIR_PATH=/tmp/cubycloud-dev
mkdir -p $DATA_DIR_PATH/db
mkdir -p $DATA_DIR_PATH/storage/default
mkdir -p $DATA_DIR_PATH/temp/tus

echo -e "${GREEN}Data directory: $DATA_DIR_PATH${NC}"

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Shutting down...${NC}"
    kill $(jobs -p) 2>/dev/null || true
    exit 0
}
trap cleanup INT TERM

# Start backend
DEV_BACKEND_PORT=8383
export PORT="$DEV_BACKEND_PORT"
echo -e "${GREEN}Starting backend on :${DEV_BACKEND_PORT}${NC}"
cd apps/backend
# Use air for hot reload instead of go run
air &
BACKEND_PID=$!
cd ../..

# Wait for the current My Storage route, not just the setup endpoint.
echo -e "${YELLOW}Waiting for backend My Storage API...${NC}"
BACKEND_STATUS=000
for i in {1..30}; do
    BACKEND_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${DEV_BACKEND_PORT}/api/v1/drive/my-storage" || true)
    if [ "$BACKEND_STATUS" = "401" ]; then
        echo -e "${GREEN}Backend ready${NC}"
        break
    fi
    sleep 1
done
if [ "$BACKEND_STATUS" != "401" ]; then
    echo -e "${RED}Backend My Storage API unavailable (expected 401, got ${BACKEND_STATUS}).${NC}"
    kill "$BACKEND_PID" 2>/dev/null || true
    wait "$BACKEND_PID" 2>/dev/null || true
    exit 1
fi

# Start frontend with the same backend port used above.

# Start frontend
echo -e "${GREEN}Starting frontend on :5188${NC}"
cd apps/frontend
VITE_API_PROXY_TARGET="http://localhost:${DEV_BACKEND_PORT}" bun run dev --port 5188 --host &
FRONTEND_PID=$!
cd ../..

echo -e "${GREEN}✓ Backend: http://localhost:8383${NC}"
echo -e "${GREEN}✓ Frontend: http://localhost:5188${NC}"
echo -e "${YELLOW}Press Ctrl+C to stop${NC}"

# Monitor processes; if either exits unexpectedly, clean up
while kill -0 "$BACKEND_PID" 2>/dev/null && kill -0 "$FRONTEND_PID" 2>/dev/null; do
    sleep 1
done

echo -e "\n${RED}Salah satu service (backend atau frontend) terhenti.${NC}"
cleanup
