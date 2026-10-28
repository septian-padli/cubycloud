# CubyCloud Release & Deployment Guide

This guide covers versioning policy, installation, upgrade procedures, rollback safety, backup and restore routines, and repository release operations.

---

## 1. Versioning Policy

CubyCloud strictly adheres to [Semantic Versioning 2.0.0](https://semver.org/):

`v<MAJOR>.<MINOR>.<PATCH>[-<CHANNEL>.<NUMBER>]`

### Channels & Release Classification

| Channel | Format Example | Prerelease? | Docker Tags Updated | Stability Target |
| :--- | :--- | :--- | :--- | :--- |
| **Alpha** | `v0.0.1-alpha.1` | Yes | `:v0.0.1-alpha.1`, `:alpha` | Active development, breaking schema changes possible |
| **Beta** | `v0.1.0-beta.1` | Yes | `:v0.1.0-beta.1`, `:beta` | Feature-complete slice, stabilization testing |
| **Release Candidate** | `v1.0.0-rc.1` | Yes | `:v1.0.0-rc.1`, `:rc` | Production candidate validation |
| **Stable** | `v1.0.0` | No | `:v1.0.0`, `:1.0.0`, `:1.0`, `:latest` | General availability |

> **Critical Safety Rule:** Only official stable releases update the `:latest` Docker tag on GitHub Container Registry (GHCR). Routine pushes to `dev` or `main` only trigger quality validation and never produce or publish images.

---

## 2. Quick Start Installation

### Using Docker Compose (Recommended)

1. Download or copy `compose.yaml`:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/septian-padli/cubycloud/main/compose.yaml -o compose.yaml
   ```

2. Pin the specific release tag in your environment (recommended for reproducible setups):
   ```bash
   export CUBYCLOUD_VERSION=v0.0.1-alpha.1
   ```

3. Launch the container:
   ```bash
   docker compose up -d
   ```

4. Open `http://localhost:8383` in your browser to complete initial administrator onboarding.

---

## 3. Upgrading CubyCloud

Because CubyCloud stores its database, master keys, and file blobs inside `/app/data` (backed by the Docker volume `cubycloud_data`), updating the application binary is safe:

1. **Always perform a backup first:**
   ```bash
   docker compose exec cubycloud tar -czf /tmp/cubycloud-backup-$(date +%F).tar.gz /app/data
   docker cp cubycloud:/tmp/cubycloud-backup-$(date +%F).tar.gz ./
   ```

2. **Update the target image tag in your environment or compose.yaml:**
   ```bash
   export CUBYCLOUD_VERSION=v0.0.2-alpha.1
   docker compose pull
   ```

3. **Restart the container with the updated image:**
   ```bash
   docker compose up -d
   ```

4. **Verify container health and logs:**
   ```bash
   docker compose ps
   docker compose logs -f
   ```

---

## 4. Rollback Procedure

If a newly deployed image encounters issues:

1. Stop the active container:
   ```bash
   docker compose down
   ```

2. If database schema was modified or corrupted, restore the volume from your previous backup archive (see section 5).

3. Pin the previous known-good tag:
   ```bash
   export CUBYCLOUD_VERSION=v0.0.1-alpha.1
   docker compose up -d
   ```

4. Check `docker compose ps` to verify the `/health` endpoint reports healthy.

---

## 5. Backup & Restore Procedures

All persistent application data resides in `/app/data`:
- `/app/data/db/cubycloud.db`: SQLite database metadata and job queue
- `/app/data/storage/`: Encrypted/managed disk storage chunks
- `/app/data/master.key` (or environment key): Encryption master key

### Backup Routine

```bash
# 1. Archive the persistent data directory
docker compose exec cubycloud tar -czf /app/data/backup.tar.gz -C /app data
docker cp cubycloud:/app/data/backup.tar.gz ./cubycloud-backup.tar.gz
docker compose exec cubycloud rm /app/data/backup.tar.gz
```

### Restore Routine

```bash
# 1. Stop the application
docker compose down

# 2. Extract backup to the named volume using a temporary Alpine runner
docker run --rm \
  -v cubycloud_data:/target \
  -v "$(pwd)":/backup \
  alpine sh -c "rm -rf /target/* && tar -xzf /backup/cubycloud-backup.tar.gz -C /target"

# 3. Restart CubyCloud
docker compose up -d
```

---

## 6. How Releases Are Published

Releases are fully automated via `.github/workflows/release.yml`.

1. Ensure the `main` branch passes all checks in `.github/workflows/ci.yml`.
2. Push a SemVer Git tag matching the target channel:
   ```bash
   git tag v0.0.1-alpha.1
   git push origin v0.0.1-alpha.1
   ```
3. The workflow executes:
   - Validates the tag against strict SemVer syntax.
   - Runs backend tests, OpenAPI freshness validation, frontend lint, and type check gates.
   - Builds the Docker image and tags it appropriately.
   - Updates `:latest` only if the release is stable.
   - Creates a GitHub Release with auto-generated changelog entries and marks it as Prerelease when appropriate.
