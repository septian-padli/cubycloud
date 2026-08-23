#!/bin/bash
# Development runner: backend + frontend

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting CubyCloud Development Environment${NC}"

# Check dependencies
if ! command -v go &> /dev/null; then
    echo -e "${RED}Go not found. Install Go 1.22+${NC}"
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
PORT=8080
DATA_DIR_PATH=/tmp/cubycloud-dev
JWT_SECRET=dev-secret-change-in-production
EOF
fi

# Create data directories
export DATA_DIR_PATH=/tmp/cubycloud-dev
mkdir -p $DATA_DIR_PATH/db
mkdir -p $DATA_DIR_PATH/storage/ssd1
mkdir -p $DATA_DIR_PATH/storage/ssd2
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
echo -e "${GREEN}Starting backend on :8080${NC}"
cd apps/backend
go run cmd/server/main.go &
BACKEND_PID=$!
cd ../..

# Wait for backend
echo -e "${YELLOW}Waiting for backend...${NC}"
for i in {1..30}; do
    if curl -sf http://localhost:8080/api/v1/setup/scan-nodes > /dev/null 2>&1; then
        echo -e "${GREEN}Backend ready${NC}"
        break
    fi
    sleep 1
done

# Start frontend
echo -e "${GREEN}Starting frontend on :5173${NC}"
cd apps/frontend
bun run dev --host &
FRONTEND_PID=$!
cd ../..

echo -e "${GREEN}✓ Backend: http://localhost:8080${NC}"
echo -e "${GREEN}✓ Frontend: http://localhost:5173${NC}"
echo -e "${YELLOW}Press Ctrl+C to stop${NC}"

# Wait for both
wait
