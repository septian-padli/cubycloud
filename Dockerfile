# Stage 1: Build Frontend SPA
FROM --platform=$BUILDPLATFORM oven/bun:1.3.14-alpine AS frontend-builder
WORKDIR /app/frontend

COPY apps/frontend/package.json apps/frontend/bun.lock ./
RUN bun install --frozen-lockfile

COPY apps/frontend ./
RUN bun run build

# Stage 2: Build Go Backend Binary
FROM --platform=$BUILDPLATFORM golang:1.25.6-alpine AS backend-builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app/backend

# Install build dependencies
RUN apk add --no-cache git ca-certificates

COPY apps/backend/go.mod apps/backend/go.sum* ./
RUN go mod download

COPY apps/backend ./
# Clean any host dist artifacts and sync fresh frontend build into backend web/dist
RUN rm -rf ./web/dist
COPY --from=frontend-builder /app/frontend/build ./web/dist
# Build statically linked binary (CGO-free)
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /cubycloud ./cmd/server/main.go

# Stage 3: Final Minimal Runtime Image (< 150MB target)
FROM alpine:3.21.3
LABEL org.opencontainers.image.title="CubyCloud"
LABEL org.opencontainers.image.description="Self-hosted cloud storage orchestrator"
LABEL org.opencontainers.image.source="https://github.com/septian-padli/cubycloud"
LABEL org.opencontainers.image.licenses="AGPL-3.0"

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget \
    && mkdir -p /app/data/db /app/data/storage /app/data/temp/tus

COPY --from=backend-builder /cubycloud /app/cubycloud

ENV PORT=8383
ENV DATA_DIR_PATH=/app/data
ENV ENV=production

VOLUME ["/app/data"]
EXPOSE 8383

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:${PORT:-8383}/health || exit 1

ENTRYPOINT ["/app/cubycloud"]
