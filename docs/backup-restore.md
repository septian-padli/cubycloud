# CubyCrate Backup & Disaster Recovery Guide

This runbook defines operational backup, snapshotting, and disaster recovery procedures for CubyCrate self-hosted instances.

---

## 1. Data Anatomy

All application state is isolated inside `DATA_DIR_PATH` (default in container: `/app/data`):

```text
/app/data/
├── db/
│   └── cubycrate.db        # SQLite database metadata, users, sessions, jobs (WAL mode)
├── storage/                # Local physical storage nodes & chunked blobs
│   └── default/
│       └── cubycrate/...
├── secrets/
│   ├── jwt.key             # Session authentication signing key (32 bytes)
│   └── master.key          # AES-256-GCM master key used to encrypt S3 cloud credentials
└── temp/
    └── tus/                # Temporary inflight chunk uploads (transient)
```

> **CRITICAL WARNING:** A database snapshot (`cubycrate.db`, including the admin UI database download at `GET /api/v1/admin/backup`) alone is **NOT a full backup** and is **NOT sufficient** for disaster recovery. It contains only SQLite metadata, omitting physical storage blobs and the master key (`/app/data/secrets/master.key`). Without the master key, encrypted S3/cloud credentials cannot be decrypted upon restart. Always back up the full `/app/data` volume (`cubycrate_data`) for disaster recovery.

### Scope & External Storage Limitations

A full backup of `/app/data` protects:
- SQLite database and schemas
- Embedded secrets (`jwt.key`, `master.key`)
- Default local storage chunks under `/app/data/storage/default`

It **does NOT** protect:
- **External secondary disks:** Any host directory mounted into `/app/data/storage/<node>` from external drives. Those mounts must be backed up via host filesystem snapshots or external archive jobs.
- **Remote S3/MinIO buckets:** Object storage blobs reside on external servers and require their own backup or replication policies (e.g. AWS S3 versioning, MinIO site replication).
- **External Master Keys:** If `CUBYCRATE_MASTER_KEY` or `CUBYCRATE_MASTER_KEY_FILE` is passed via environment variables pointing outside the container volume, that key material must be backed up independently.

## 2. Backup Procedures

### Method A: Full Cold Volume Backup (Recommended for Disaster Recovery)

This produces an atomic, consistent archive of the database (including SQLite WAL/SHM files), encryption keys, and local storage chunks.

A trap is used so that the CubyCrate container is safely restarted even if the archive creation or validation step fails:

```bash
set -Eeuo pipefail

# 1. Prepare backup destination outside the application volume.
BACKUP_DIR="${HOME}/cubycrate-backups"
mkdir -p "${BACKUP_DIR}"
TEMP_DIR="$(mktemp -d "${BACKUP_DIR}/.backup-XXXXXX")"
BACKUP_FILE="${BACKUP_DIR}/cubycrate-backup-$(date +%Y%m%d-%H%M%S)-$(basename "${TEMP_DIR}").tar.gz"
CONTENTS_FILE="${TEMP_DIR}/contents.txt"

# 2. Stop gracefully; after a successful stop, always try restart and clean up.
docker compose -f compose.yaml stop cubycrate
trap 'docker compose -f compose.yaml start cubycrate; rm -f "${CONTENTS_FILE}"; rmdir "${TEMP_DIR}"' EXIT

# 3. Create compressed archive using an ephemeral Alpine runner (read-only volume mount)
docker run --rm \
  -v "${CUBYCRATE_VOLUME_NAME:-cubycrate_data}:/data:ro" \
  -v "${BACKUP_DIR}:/backup" \
  alpine tar -czf "/backup/$(basename "${BACKUP_FILE}")" -C /data .

# Verify compressed archive integrity and expected database/storage contents.
# Review listing for generated secrets; separately back up externally supplied secrets.
tar -tzf "${BACKUP_FILE}" > "${CONTENTS_FILE}"
grep -Eq '^(\./)?db/cubycrate\.db$' "${CONTENTS_FILE}"
grep -Eq '^(\./)?storage/' "${CONTENTS_FILE}"
grep -Eq '^(\./)?secrets/' "${CONTENTS_FILE}" || true
echo "Cold full-volume backup created and verified: ${BACKUP_FILE}"
```
The helper runs as root by default and preserves file modes. The runtime image also runs as root; do not restore through a non-root helper that cannot preserve access to mode-`0600` secret files.
---

### Method B: Database-Only Snapshot (Metadata Only)

> **Note:** A database snapshot is useful for schema inspection or metadata audits, but is **NOT a full backup**. It does NOT contain file blobs or encryption keys.

#### 1. Admin UI / API Download
Authenticated administrators can stream a SQLite VACUUM snapshot via HTTP:
```bash
curl -f -H "Cookie: cubycrate_token=<session-token>" \
  http://127.0.0.1:8383/api/v1/admin/backup -o cubycrate-metadata.sqlite
```



## 3. Disaster Recovery & Restore Procedures

> **Safety Rule:** Never destructively wipe an existing production volume (`rm -rf` or `docker compose down -v` on production risks unrecoverable data loss). Always restore into a **brand-new, isolated volume** or directory to inspect and verify before switching traffic.
>
> **Staging & S3 Warning:** Do not connect staging, local, or test restore containers to production S3/MinIO buckets or production master keys! If a test container loads production credentials, automated deletion, test uploads, or file cleanups will modify or delete files in the shared production S3 bucket.
>
> **Clean Break Notice:** Direct upgrade or restore from historical `v0.0.1-alpha.1` CubyCloud data is unsupported. CubyCrate `v0.0.1-alpha.2` is a clean technical release.

Follow these steps to restore onto a new server or into a disposable isolated volume. Never connect a test restore with production S3 credentials or production external mounts.

#### Step 1: Create a Brand-New Target Volume
Choose a unique name and stop if it already exists. Do not remove or reuse an existing volume:
```bash
set -Eeuo pipefail
RESTORE_VOLUME="cubycrate_data_restore_$(date +%Y%m%d-%H%M%S)"
if docker volume inspect "${RESTORE_VOLUME}" >/dev/null 2>&1; then
  echo "Refusing to reuse existing volume: ${RESTORE_VOLUME}" >&2
  exit 1
fi
docker volume create "${RESTORE_VOLUME}"
```

#### Step 2: Unpack the Verified Archive
```bash
BACKUP_FILE="${HOME}/cubycrate-backups/<verified-backup-file>.tar.gz"
tar -tzf "${BACKUP_FILE}" >/dev/null
docker run --rm \
  -v "${RESTORE_VOLUME}:/target" \
  -v "$(dirname "${BACKUP_FILE}"):/backup:ro" \
  alpine tar -xzf "/backup/$(basename "${BACKUP_FILE}")" -C /target
```

#### Step 3: Verify Database, Secrets, and Storage
```bash
docker run --rm \
  -v "${RESTORE_VOLUME}:/target:ro" \
  alpine sh -c 'test -s /target/db/cubycrate.db && test -d /target/secrets && test -d /target/storage && ls -la /target/db /target/secrets /target/storage'
```
Verify generated `jwt.key` and `master.key` files are present when those secrets live inside the volume. If externally supplied through `JWT_SECRET`, `CUBYCRATE_MASTER_KEY`, or `CUBYCRATE_MASTER_KEY_FILE`, restore the exact original values/files before startup; never replace them with newly generated keys.

#### Step 4: Test in an Isolated Container
Use an existing compatible image. The example requires local image `cubycrate:local`; do not provide production credentials or mount production external storage:
```bash
docker run -d --name cubycrate-restore-test \
  --network none \
  -v "${RESTORE_VOLUME}:/app/data" \
  -e ENV=production -e PORT=8383 \
  -e CUBYCRATE_STORAGE_CLEANUP_DRY_RUN=true \
  cubycrate:local

# Query health inside the isolated container; it has no external network access.
docker exec cubycrate-restore-test wget -qO- http://127.0.0.1:8383/health
docker exec cubycrate-restore-test wget -qO- http://127.0.0.1:8383/api/health
```
Both probes return HTTP 200 and `{"status":"ok"}` if SQLite and configured local storage are available. If restored settings depend on S3 or external local storage, health may fail while offline. Do not connect this test to production services.

After testing, stop and remove only the specifically named disposable container. Keep the restored volume unless an operator separately verifies it is disposable:
```bash
docker stop cubycrate-restore-test
docker rm cubycrate-restore-test
```

#### Step 5: Production Restore
Only after isolated verification, stop the production service. Point `CUBYCRATE_VOLUME_NAME` at a newly verified restore volume, start the app, then verify `/health`, login, and known files. Do not overwrite any existing volume.

### Rollback

Rollback to historical CubyCloud `v0.0.1-alpha.1` is unsupported. `v0.0.1-alpha.2` is the first CubyCrate technical release, so no earlier CubyCrate image is available. Future rollback requires a compatible prior CubyCrate image and a consistent pre-upgrade snapshot of database, local/external storage, and matching secrets.
