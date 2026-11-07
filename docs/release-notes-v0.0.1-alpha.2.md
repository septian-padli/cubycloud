# CubyCrate Release Notes: v0.0.1-alpha.2

**Release Target:** `v0.0.1-alpha.2`  
**Channel:** `alpha`  
**Status:** Pre-release draft; publication awaits successful T8 workflow.
**Target Container Image:** `ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2`
*(The image is not yet available on GHCR. Pull instructions apply only after T8 completes successfully.)*

---

## Highlights

CubyCrate `v0.0.1-alpha.2` marks the first official technical release under our new identity: **CubyCrate**.

### 1. CubyCloud → CubyCrate Technical Rename
- Canonical GitHub repository: `septian-padli/cubycrate`
- Canonical Go module: `github.com/septian-padli/cubycrate/apps/backend`
- Canonical container distribution: `ghcr.io/septian-padli/cubycrate`
- Database file: `db/cubycrate.db` (SQLite in WAL mode)
- Default volume name: `cubycrate_data`
- Blob namespace: `cubycrate/{instance_id}/{node_id}/blobs/{blob_id}`
- Product environment variables: `CUBYCRATE_*`
- Session cookie: `cubycrate_token`
- Browser state keys: `cubycrate-theme`, `cubycrate-drive-view`, `cubycrate-language:*`, `cubycrate_setup_draft`

### 2. Clean Break & Fresh Start
- **No in-place upgrade path:** Direct upgrade from `v0.0.1-alpha.1` (historical CubyCloud release) is unsupported.
- **Fresh installation required:** Start with an empty data directory or newly initialized `cubycrate_data` volume.
- **Data safety notice:** Existing historical `cubycloud_data` volumes or databases must be backed up externally before deploying CubyCrate. CubyCrate does not overwrite or read historical CubyCloud volumes.

### 3. Multi-Architecture Publishing Pipeline
- Native container builds for both **`linux/amd64`** and **`linux/arm64`** (Raspberry Pi, ARM VPS, Apple Silicon servers).
- Strict channel tagging policy: prereleases push exact version tag plus channel alias (`:v0.0.1-alpha.2` and `:alpha`), never `:latest`.

---

## Installation via Docker Compose

Create a `compose.yaml` file:

```yaml
services:
  cubycrate:
    image: ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2
    container_name: cubycrate
    restart: unless-stopped
    ports:
      - "8383:8383"
    volumes:
      - cubycrate_data:/app/data
    environment:
      - ENV=production
      - PORT=8383
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://127.0.0.1:8383/health"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3

volumes:
  cubycrate_data:
    name: cubycrate_data
```

Start the application:
```bash
docker compose up -d
```

Open `http://localhost:8383` (or `http://<server-ip>:8383`) in your browser to complete first-run setup.

---

## Known Alpha Limitations

1. **Pre-Alpha / Alpha Maturity:** CubyCrate is in active pre-release development. Do not use as sole backup for mission-critical unbacked data.
2. **Local & S3 Storage:** Supported storage modes are local filesystem and S3-compatible object storage. Storage migrations between nodes are manual.
3. **No Native Sync Client:** Web-based access and TUS resumable uploads only; no desktop filesystem sync daemon.
4. **HTTPS behind Reverse Proxy:** Direct HTTP is intended for LAN or reverse proxy upstream. When exposing over public networks, terminate TLS with an HTTPS reverse proxy (Caddy/Nginx) and set `COOKIE_SECURE=true`.

---

## What's Changed
- Fully rebranded user interface, documentation, and error messages to CubyCrate.
- Switched internal import paths and Go module declaration to `github.com/septian-padli/cubycrate/apps/backend`.
- Updated release workflow with multi-architecture Buildx + QEMU pipeline targeting `linux/amd64,linux/arm64`.
- Standardized healthcheck probe at `GET /health` and internal listen port `8383`.
