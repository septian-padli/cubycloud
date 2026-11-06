# CubyCrate Release & Deployment Guide

This guide covers versioning policy, installation, upgrade procedures, rollback safety, backup and restore routines, and repository release operations.

---

## 1. Versioning Policy
CubyCrate strictly adheres to [Semantic Versioning 2.0.0](https://semver.org/):

`v<MAJOR>.<MINOR>.<PATCH>[-<CHANNEL>.<NUMBER>]`

### Channels & Release Classification

| Channel | Format Example | Prerelease? | Docker Tags Updated | Stability Target |
| :--- | :--- | :--- | :--- | :--- |
| **Alpha** | `v0.0.1-alpha.1` | Yes | `:v0.0.1-alpha.1`, `:alpha` | Active development, breaking schema changes possible |
| **Beta** | `v0.1.0-beta.1` | Yes | `:v0.1.0-beta.1`, `:beta` | Feature-complete slice, stabilization testing |
| **Release Candidate** | `v1.0.0-rc.1` | Yes | `:v1.0.0-rc.1`, `:rc` | Production candidate validation |
| **Stable** | `v1.0.0` | No | `:v1.0.0`, `:1.0.0`, `:1.0`, `:latest` | General availability |

> **Critical Safety Rule:** Only official stable releases update the `:latest` Docker tag on GitHub Container Registry (GHCR). Routine pushes to `dev` or `main` only trigger quality validation and never produce or publish images. Note: The existing published container path remains `ghcr.io/septian-padli/cubycloud`; future packages may target `ghcr.io/septian-padli/cubycrate` once published.

---

## 2. Quick Start Installation

### Using Docker Compose (Recommended)

1. Download or copy `compose.yaml`:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/septian-padli/cubycloud/main/compose.yaml -o compose.yaml
   ```

2. Pin the specific release tag in your environment (recommended for reproducible setups):
   ```bash
   export CUBYCRATE_VERSION=v0.0.1-alpha.1
   ```

3. Launch the container:
   ```bash
   docker compose up -d
   ```

4. Open `http://localhost:8383` in your browser to complete initial administrator onboarding.

---

## 3. Upgrading CubyCrate

> **Clean Break Notice:** Upgrading directly from v0.0.1-alpha.1 to post-rename releases is not supported as an automatic in-place migration due to changes in database filenames, secrets paths, and volume labels.

Because CubyCrate stores its database, master keys, and file blobs inside `/app/data` (backed by the Docker volume `cubycrate_data`), updating between compatible versions is straightforward:

1. **Always perform a full backup first (admin DB download is NOT a full backup):**
   ```bash
   docker compose exec cubycrate tar -czf /tmp/cubycrate-backup-$(date +%F).tar.gz /app/data
   docker cp cubycrate:/tmp/cubycrate-backup-$(date +%F).tar.gz ./
   ```
2. **Update the target image tag in your environment or compose.yaml:**
   ```bash
   export CUBYCRATE_VERSION=v0.0.2-alpha.1
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
   export CUBYCRATE_VERSION=v0.0.1-alpha.1
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
docker compose exec cubycrate tar -czf /app/data/backup.tar.gz -C /app data
docker cp cubycrate:/app/data/backup.tar.gz ./cubycrate-backup.tar.gz
docker compose exec cubycrate rm /app/data/backup.tar.gz

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
   git tag v0.0.1-alpha.1
   git push origin v0.0.1-alpha.1
   ```
3. The workflow executes:
   - Validates the tag against strict SemVer syntax.
   - Runs backend tests, OpenAPI freshness validation, frontend lint, and type check gates.
   - Builds the Docker image and tags it appropriately.
   - Updates `:latest` only if the release is stable.
   - Creates a GitHub Release with auto-generated changelog entries and marks it as Prerelease when appropriate.
