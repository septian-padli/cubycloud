# Stage 1: Build Frontend SPA
FROM oven/bun:1-alpine AS frontend-builder
WORKDIR /app/frontend

COPY apps/frontend/package.json apps/frontend/bun.lock* ./
RUN bun install --frozen-lockfile || bun install

COPY apps/frontend ./
RUN bun run build

# Stage 2: Build Go Backend Binary
FROM golang:alpine AS backend-builder
WORKDIR /app/backend

# Install build dependencies
RUN apk add --no-cache git ca-certificates

COPY apps/backend/go.mod apps/backend/go.sum* ./
RUN go mod download

COPY apps/backend ./
# Sync frontend build artifacts into backend web/dist
COPY --from=frontend-builder /app/frontend/build ./web/dist

# Build statically linked binary (CGO-free)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /cubycloud ./cmd/server/main.go

# Stage 3: Final Minimal Runtime Image (< 150MB target)
FROM alpine:3.21
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/data/db /app/data/storage /app/data/temp/tus

COPY --from=backend-builder /cubycloud /app/cubycloud

ENV PORT=8383
ENV DATA_DIR_PATH=/app/data
ENV ENV=production

VOLUME ["/app/data"]
EXPOSE 8383

ENTRYPOINT ["/app/cubycloud"]
