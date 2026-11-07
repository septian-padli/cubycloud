# CubyCrate Release & Deployment Guide

This guide covers versioning policy, installation, upgrade procedures, rollback safety, backup and restore routines, and repository release operations.

---

## 1. Versioning Policy
CubyCrate strictly adheres to [Semantic Versioning 2.0.0](https://semver.org/):

`v<MAJOR>.<MINOR>.<PATCH>[-<CHANNEL>.<NUMBER>]`

### Channels & Release Classification

| Channel | Format Example | Prerelease? | Docker Tags Updated | Stability Target |
| :--- | :--- | :--- | :--- | :--- |
| **Alpha (Historical)** | `v0.0.1-alpha.1` | Yes | `:v0.0.1-alpha.1`, `:alpha` | Historical CubyCloud alpha; direct upgrade to CubyCrate is unsupported |
| **Alpha (Current)** | `v0.0.1-alpha.2` | Yes | `:v0.0.1-alpha.2`, `:alpha` | First CubyCrate technical release; Clean Break fresh start |
| **Beta** | `v0.1.0-beta.1` | Yes | `:v0.1.0-beta.1`, `:beta` | Feature-complete slice, stabilization testing |
| **Release Candidate** | `v1.0.0-rc.1` | Yes | `:v1.0.0-rc.1`, `:rc` | Production candidate validation |
| **Stable** | `v1.0.0` | No | `:v1.0.0`, `:1.0.0`, `:1.0`, `:latest` | General availability; only stable releases update `:latest` |

> **Critical Safety Rule:** Only official stable releases update the `:latest` Docker tag on GHCR. Routine pushes to `dev` or `main` trigger validation only. The official image `ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2` is staged for T8 and **is not yet published to GHCR**.

---

## 2. Quick Start Installation

### Option A: Using Pre-built GHCR Image (Quick Start)

> **Image Availability:** The official container image `ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2` is staged for T8 and **is not yet published to GHCR**. Run these commands only after T8 succeeds. They require Docker, Docker Compose, and downloaded `compose.yaml`; no repository clone or source compilation is needed.

1. Download `compose.yaml`:
   ```bash
   curl -fsSLO https://raw.githubusercontent.com/septian-padli/cubycrate/main/compose.yaml
   export CUBYCRATE_IMAGE=ghcr.io/septian-padli/cubycrate
   export CUBYCRATE_VERSION=v0.0.1-alpha.2
   docker pull ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2
   docker compose pull
   docker compose up -d
   curl -f http://127.0.0.1:8383/health
   ```
   The probe returns `{"status":"ok"}` with HTTP 200. Open `http://localhost:8383/setup` or `http://<server-ip>:8383/setup` to configure the first administrator. This path needs only Docker, Compose, and `compose.yaml`; it does not clone or compile source.

Compose uses host port 8383, internal port 8383, and named volume `cubycrate_data` by default. Set `CUBYCRATE_PORT` to override the host port and `CUBYCRATE_VOLUME_NAME` to choose a different named volume.

### Option B: Local Source Build (Dev / Pre-Release)

Until the GHCR image is published or when developing locally, clone the full repository because `docker build` needs the complete build context, not only `compose.yaml`:

1. Clone, build, start Compose, and check health:
   ```bash
   git clone https://github.com/septian-padli/cubycrate.git
   cd cubycrate
   docker build -t cubycrate:local .
   docker compose up -d
   curl -f http://127.0.0.1:8383/health
   ```

2. Open `http://localhost:8383/setup` to finish configuration.

---

## 3. Upgrading CubyCrate

> **Clean Break Notice:** Direct in-place upgrade from historical `v0.0.1-alpha.1` CubyCloud release is unsupported. CubyCrate does not read or migrate old CubyCloud database schemas, volumes, blobs, secrets, or cookies. Always start CubyCrate with an empty volume or fresh data directory.

Compatible CubyCrate releases store persistent data in `/app/data` (`cubycrate_data` volume). To upgrade between compatible CubyCrate releases:

1. **Before upgrades:** Follow the cold full-volume backup procedure in section 5 and verify the archive before changing images. Keep a copy outside the application volume. Do not use `docker compose down -v`.

2. Update image:
   - For GHCR releases (after publication): set `CUBYCRATE_IMAGE=ghcr.io/septian-padli/cubycrate` and `CUBYCRATE_VERSION=<target-tag>`, then run `docker compose pull`.
   - For source development, run `git pull` from the repository root and `docker build -t cubycrate:local .`.

3. Restart with the selected image using `docker compose up -d`.

4. Verify status and health:
   ```bash
   docker compose ps
   docker compose logs --tail=100
   curl -f http://127.0.0.1:8383/health
   ```
   A healthy response is HTTP 200 with `{"status":"ok"}`. The health endpoint checks the database and configured storage nodes.

---

## 4. Rollback Procedure

If a newly deployed CubyCrate release encounters issues:

Rollback to historical CubyCloud `v0.0.1-alpha.1` is unsupported. Since `v0.0.1-alpha.2` is the first CubyCrate technical release, no earlier CubyCrate image is available. Future rollback is possible only with a compatible previous CubyCrate image and a consistent pre-upgrade snapshot of database, local/external storage, and matching secrets.

For future compatible releases, stop the app, restore the pre-upgrade snapshot into a new isolated volume, verify it, then start the compatible image with `CUBYCRATE_VOLUME_NAME` pointing at that volume. Never overwrite the active volume or use a snapshot created after the incompatible migration.

## 5. Backup & Restore Procedures

> **Important:** The admin UI database download (`GET /api/v1/admin/backup`) is **NOT a full backup**; it downloads only SQLite metadata, omitting the encryption master key and physical file blobs.

Persistent application state inside the container (`/app/data`):
- `/app/data/db/cubycrate.db`: SQLite database tables, metadata, and background jobs (WAL mode; including `cubycrate.db-wal` and `cubycrate.db-shm` if active)
- `/app/data/storage/`: Managed local disk storage nodes and file chunks
- `/app/data/secrets/master.key`: AES-256-GCM master key used to encrypt cloud storage credentials (unless externalized via `CUBYCRATE_MASTER_KEY` / `CUBYCRATE_MASTER_KEY_FILE`)
- `/app/data/secrets/jwt.key`: Session authentication signing key

External secondary storage mounts (e.g. host disks bind-mounted to `/app/data/storage/<node>`) and remote S3 buckets live outside this volume and must be backed up separately.

### Safe Cold Backup Routine

Stop container to cleanly flush SQLite WAL buffers. Use a trap to ensure container restarts even if archiving fails:

```bash
set -Eeuo pipefail
BACKUP_DIR="${HOME}/cubycrate-backups"
mkdir -p "${BACKUP_DIR}"
TEMP_DIR="$(mktemp -d "${BACKUP_DIR}/.backup-XXXXXX")"
BACKUP_FILE="${BACKUP_DIR}/cubycrate-backup-$(date +%Y%m%d-%H%M%S)-$(basename "${TEMP_DIR}").tar.gz"
CONTENTS_FILE="${TEMP_DIR}/contents.txt"

# Stop gracefully; after a successful stop, always try restart and clean up.
docker compose -f compose.yaml stop cubycrate
trap 'docker compose -f compose.yaml start cubycrate; rm -f "${CONTENTS_FILE}"; rmdir "${TEMP_DIR}"' EXIT

# The stopped volume is mounted read-only; WAL/SHM files are included if present.
docker run --rm \
  -v "${CUBYCRATE_VOLUME_NAME:-cubycrate_data}:/data:ro" \
  -v "${BACKUP_DIR}:/backup" \
  alpine tar -czf "/backup/$(basename "${BACKUP_FILE}")" -C /data .

# Validate archive and expected database/storage contents. Review listing for
# generated secrets; separately back up externally supplied secret values/files.
tar -tzf "${BACKUP_FILE}" > "${BACKUP_FILE}.contents"
grep -Eq '^(\./)?db/cubycrate\.db$' "${BACKUP_FILE}.contents"
grep -Eq '^(\./)?storage/' "${BACKUP_FILE}.contents"
grep -Eq '^(\./)?secrets/' "${BACKUP_FILE}.contents" || true
echo "Cold full-volume backup verified: ${BACKUP_FILE}"
```

The archive is written under `~/cubycrate-backups`, outside the Docker volume. Protect it as sensitive data because it contains database records, generated keys, and local files. Never archive only the active database file.



For a comprehensive runbook including hot SQLite snapshots vs cold backups, see [docs/backup-restore.md](backup-restore.md).
## 6. How Releases Are Published

Releases are fully automated via `.github/workflows/release.yml`.

1. Ensure the `main` branch passes all checks in `.github/workflows/ci.yml`.
2. Push a SemVer Git tag matching the target channel:
   ```bash
   git tag v0.0.1-alpha.2
   git push origin v0.0.1-alpha.2
   ```
3. The workflow executes:
   - Validates the tag against strict SemVer syntax.
   - Runs backend tests, OpenAPI freshness validation, frontend lint, and type check gates.
   - Builds the Docker image and tags it appropriately.
   - Updates `:latest` only if the release is stable.
   - Creates a GitHub Release with auto-generated changelog entries and marks it as Prerelease when appropriate.
