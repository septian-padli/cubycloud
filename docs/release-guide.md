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

### Using Docker Compose (Recommended)

1. Download or copy `compose.yaml`:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/septian-padli/cubycrate/main/compose.yaml -o compose.yaml
   ```

2. Build the image locally until the CubyCrate GHCR package is published after T7:
   ```bash
   docker build -t cubycrate:local .
   ```
3. Launch the container:
   ```bash
   docker compose up -d
   ```

4. Open `http://localhost:8383` in your browser to complete initial administrator onboarding.

---

## 3. Upgrading CubyCrate

> **Clean Break:** Direct in-place upgrade from the historical `v0.0.1-alpha.1` CubyCloud release is not supported. Start with an empty CubyCrate data directory or volume. CubyCrate does not read or migrate the old database, volume, blobs, secrets, cookies, or browser persistence.

Compatible CubyCrate releases store the database, master key, and file blobs under `/app/data` (the `cubycrate_data` volume). To upgrade between compatible CubyCrate releases:

1. **Always perform a full backup first (admin DB download is NOT a full backup):**
   ```bash
   docker compose exec cubycrate tar -czf /tmp/cubycrate-backup-$(date +%F).tar.gz /app/data
   docker cp cubycrate:/tmp/cubycrate-backup-$(date +%F).tar.gz ./
   ```

2. Use a compatible CubyCrate image tag after the registry package is published in T7. Until then, rebuild the updated source locally with `docker build -t cubycrate:local .`.

3. Restart the container:
   ```bash
   CUBYCRATE_VERSION=local docker compose up -d
   ```

4. Verify container health and logs:
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

3. Pin a previous **compatible CubyCrate** image tag available on your configured registry; the historical `v0.0.1-alpha.1` CubyCloud image is not a supported rollback target.
   ```bash
   export CUBYCRATE_VERSION=<previous-compatible-cubycrate-tag>
   docker compose up -d
   ```

4. Check `docker compose ps` to verify the `/health` endpoint reports healthy.

---

## 5. Backup & Restore Procedures

> **Important:** The admin UI database download (`GET /api/v1/admin/settings/backup`) is **NOT a full backup**; it downloads only SQLite metadata, omitting the encryption master key and physical file blobs.

All persistent application data resides in `/app/data`:
- `/app/data/db/cubycrate.db`: SQLite database metadata and job queue
- `/app/data/storage/`: Managed disk storage chunks
- `/app/data/secrets/master.key`: AES-256-GCM master key used to encrypt S3 cloud credentials (or provided via `CUBYCRATE_MASTER_KEY` / `CUBYCRATE_MASTER_KEY_FILE`)
- `/app/data/secrets/jwt.key`: Session authentication signing key
### Backup Routine

```bash
# 1. Archive the persistent data directory
docker compose exec cubycrate tar -czf /tmp/cubycrate-backup.tar.gz -C /app/data .
docker cp cubycrate:/tmp/cubycrate-backup.tar.gz ./cubycrate-backup.tar.gz
docker compose exec cubycrate rm /tmp/cubycrate-backup.tar.gz
```

### Restore Routine

> **Disaster Recovery Rule:** Always test restores on a **disposable isolated copy** (e.g. in a staging environment or temporary volume) before performing any production recovery.

```bash
# 1. Stop the application
docker compose down

# 2. Extract backup to the named volume using a temporary Alpine runner
docker run --rm \
  -v cubycrate_data:/target \
  -v "$(pwd)":/backup \
  alpine sh -c "rm -rf /target/* && tar -xzf /backup/cubycrate-backup.tar.gz -C /target"

# 3. Restart CubyCrate
docker compose up -d
```
---

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
