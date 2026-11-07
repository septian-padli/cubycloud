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

> **CRITICAL WARNING:** A database snapshot (`cubycrate.db`, including the admin UI database download) alone is **NOT a full backup** and is **NOT sufficient** for disaster recovery. It contains only SQLite metadata, omitting physical storage blobs and the master key (`/app/data/secrets/master.key`). Without `/app/data/secrets/master.key`, encrypted S3/cloud credentials cannot be decrypted upon restart. Always back up the full `/app/data` tree.

---

## 2. Backup Procedures

### Method A: Full Cold Volume Backup (Recommended)

This produces an atomic, complete archive of the database, encryption keys, and all local storage chunks.

```bash
# 1. Stop CubyCrate container to cleanly flush WAL and pending writes
docker compose stop cubycrate

# 2. Create timestamped compressed archive using an ephemeral Alpine runner
BACKUP_FILE="cubycrate-backup-$(date +%Y%m%d-%H%M%S).tar.gz"
docker run --rm \
  -v cubycrate_data:/data:ro \
  -v "$(pwd)":/backup \
  alpine tar -czf "/backup/${BACKUP_FILE}" -C /data .

# 3. Restart CubyCrate container
docker compose start cubycrate

echo "Backup created: ${BACKUP_FILE}"
```

---

### Method B: Hot Database Snapshot (Online)

CubyCrate provides a non-locking SQLite snapshot via `GET /api/v1/admin/settings/backup` or through direct database WAL checkpointing:

```bash
# Execute safe online VACUUM INTO inside container
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
docker compose exec cubycrate mkdir -p /tmp/backups
docker compose exec cubycrate sqlite3 /app/data/db/cubycrate.db "VACUUM INTO '/tmp/backups/db-${TIMESTAMP}.sqlite';"
docker cp cubycrate:/tmp/backups/db-${TIMESTAMP}.sqlite ./cubycrate-db-${TIMESTAMP}.sqlite
docker compose exec cubycrate rm -rf /tmp/backups
```

*Note: Admin UI DB download and hot DB snapshots are NOT full backups; they only capture SQLite records and must be paired with `/app/data/secrets/master.key` and `/app/data/storage/`.*

---

## 3. Disaster Recovery & Restore Procedure

> **Safety Rule:** When testing restore procedures or validating backup integrity, restore only into a disposable isolated copy (such as a temporary volume on a staging or test machine) rather than overwriting live production state.
>
> **Clean Break Notice:** CubyCrate releases following the technical rename do not support automatic in-place database upgrades from v0.0.1-alpha.1.

To restore CubyCrate onto a new host or isolated test environment:

### Step 1: Ensure Target Docker Volume Exists
```bash
docker volume create cubycrate_data
```

### Step 2: Unpack Archive Into Volume
```bash
docker run --rm \
  -v cubycrate_data:/data \
  -v "$(pwd)":/backup \
  alpine sh -c "rm -rf /data/* && tar -xzf /backup/cubycrate-backup-*.tar.gz -C /data"
```

### Step 3: Verify File Permissions and Presence
```bash
docker run --rm \
  -v cubycrate_data:/data \
  alpine ls -la /data /data/db /data/secrets
```

### Step 4: Launch Container and Check Health
```bash
docker compose up -d
docker compose ps
docker compose logs --tail=50
```

Verify that `http://localhost:8383/health` returns `{"status":"ok"}`.
