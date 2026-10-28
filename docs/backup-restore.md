# CubyCloud Backup & Disaster Recovery Guide

This runbook defines operational backup, snapshotting, and disaster recovery procedures for CubyCloud self-hosted instances.

---

## 1. Data Anatomy

All application state is isolated inside `DATA_DIR_PATH` (default in container: `/app/data`):

```text
/app/data/
├── db/
│   └── cubycloud.db        # SQLite database metadata, users, sessions, jobs (WAL mode)
├── storage/                # Local physical storage nodes & chunked blobs
│   └── default/
│       └── cubycloud/...
├── secrets/
│   ├── jwt.key             # Session authentication signing key (32 bytes)
│   └── master.key          # AES-256-GCM master key used to encrypt S3 cloud credentials
└── temp/
    └── tus/                # Temporary inflight chunk uploads (transient)
```

> **CRITICAL WARNING:** A database snapshot (`cubycloud.db`) alone is **NOT sufficient** for disaster recovery if S3 nodes or encrypted cloud connections are enabled. Without `/app/data/secrets/master.key`, encrypted storage keys cannot be decrypted upon restart.

---

## 2. Backup Procedures

### Method A: Full Cold Volume Backup (Recommended)

This produces an atomic, complete archive of the database, encryption keys, and all local storage chunks.

```bash
# 1. Stop CubyCloud container to cleanly flush WAL and pending writes
docker compose stop cubycloud

# 2. Create timestamped compressed archive using an ephemeral Alpine runner
BACKUP_FILE="cubycloud-backup-$(date +%Y%m%d-%H%M%S).tar.gz"
docker run --rm \
  -v cubycloud_data:/data:ro \
  -v "$(pwd)":/backup \
  alpine tar -czf "/backup/${BACKUP_FILE}" -C /data .

# 3. Restart CubyCloud container
docker compose start cubycloud

echo "Backup created: ${BACKUP_FILE}"
```

---

### Method B: Hot Database Snapshot (Online)

CubyCloud provides a non-locking SQLite snapshot via `GET /api/v1/admin/settings/backup` or through direct database WAL checkpointing:

```bash
# Execute safe online VACUUM INTO inside container
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
docker compose exec cubycloud mkdir -p /tmp/backups
docker compose exec cubycloud sqlite3 /app/data/db/cubycloud.db "VACUUM INTO '/tmp/backups/db-${TIMESTAMP}.sqlite';"
docker cp cubycloud:/tmp/backups/db-${TIMESTAMP}.sqlite ./cubycloud-db-${TIMESTAMP}.sqlite
docker compose exec cubycloud rm -rf /tmp/backups
```

*Note: Hot DB snapshots must be paired with an archive of `/app/data/secrets/master.key`.*

---

## 3. Disaster Recovery & Restore Procedure

To restore CubyCloud onto a new host or recover from corruption:

### Step 1: Ensure Target Docker Volume Exists
```bash
docker volume create cubycloud_data
```

### Step 2: Unpack Archive Into Volume
```bash
docker run --rm \
  -v cubycloud_data:/data \
  -v "$(pwd)":/backup \
  alpine sh -c "rm -rf /data/* && tar -xzf /backup/cubycloud-backup-*.tar.gz -C /data"
```

### Step 3: Verify File Permissions and Presence
```bash
docker run --rm \
  -v cubycloud_data:/data \
  alpine ls -la /data /data/db /data/secrets
```

### Step 4: Launch Container and Check Health
```bash
docker compose up -d
docker compose ps
docker compose logs --tail=50
```

Verify that `http://localhost:8383/health` returns `{"status":"ok"}`.
