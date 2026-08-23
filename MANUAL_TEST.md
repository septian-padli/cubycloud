# CubyCloud Setup Flow — Manual Test Guide

## Prerequisites

- Go 1.22+
- Bun
- macOS/Linux (StatLocal uses Unix syscall.Statfs)

## Test Procedure

### 1. Start Development Environment

```bash
./scripts/dev.sh
```

This will:
- Create `/tmp/cubycloud-dev` with 2 mock storage nodes (ssd1, ssd2)
- Start backend on `http://localhost:8080`
- Start frontend on `http://localhost:5173`

### 2. Test Setup Flow (Happy Path)

1. Open `http://localhost:5173/setup`
2. **Step 1 - Admin:**
   - Username: `admin`
   - Email: `admin@local.dev`
   - Password: `password123` (≥8 chars)
   - Confirm Password: `password123`
   - Click **Selanjutnya**

3. **Step 2 - Storage:**
   - Verify 2 nodes scanned: `/tmp/cubycloud-dev/storage/ssd1`, `ssd2`
   - Both enabled by default
   - Click **Selanjutnya**

4. **Step 3 - Strategy:**
   - Select **Fill First** (default)
   - Click **Selanjutnya**

5. **Step 4 - Cloud (Optional):**
   - Skip (disable cloud)
   - Click **Lewati & Lanjutkan**

6. **Step 5 - Recovery Key:**
   - Click checkbox to acknowledge
   - Click **Selesaikan Setup & Masuk ke CubyCloud**

7. **Success Modal:**
   - Verify 64-char hex recovery key displayed (e.g. `a1b2c3d4...`)
   - Click **Copy** → verify toast "Recovery key copied"
   - Click **Download** → verify `cubycloud-recovery-key.txt` downloaded
   - Check "I have saved my recovery key"
   - Click **Continue to CubyCloud**

8. **Redirect:**
   - Should redirect to `/drive/home`
   - Verify sidebar renders with menu items
   - Verify "Drive Home" page loads

### 3. Test Edge Cases

#### 3.1 Setup Already Completed (409)
1. Refresh `http://localhost:5173/setup`
2. Fill form and submit again
3. **Expected:** Toast error "Setup already completed"

#### 3.2 Validation Errors (400)

**Password < 8 chars:**
1. Clear backend: `rm -rf /tmp/cubycloud-dev/db && ./scripts/dev.sh`
2. Go to `/setup`
3. Step 1: password = `short` (5 chars)
4. **Expected:** Form validation prevents "Selanjutnya" button

**No enabled nodes:**
1. Clear backend: `rm -rf /tmp/cubycloud-dev/db && ./scripts/dev.sh`
2. Go to `/setup`
3. Step 2: disable all nodes
4. **Expected:** "Selanjutnya" button disabled

#### 3.3 Cloud Credentials Invalid

1. Clear backend: `rm -rf /tmp/cubycloud-dev/db && ./scripts/dev.sh`
2. Go to `/setup`
3. Step 4: enable cloud, fill invalid credentials:
   - Provider: R2
   - Endpoint: `https://invalid.example.com`
   - Bucket: `fake`
   - Access Key: `fake`
   - Secret Key: `fake`
4. Submit
5. **Expected:** Toast error "Cloud credentials invalid" or validation modal

### 4. Test Storage Pairing

**Local Storage:**
1. Complete setup successfully
2. Check backend created DB: `ls -lh /tmp/cubycloud-dev/db/cubycloud.db`
3. Query storage_nodes:
   ```bash
   sqlite3 /tmp/cubycloud-dev/db/cubycloud.db "SELECT * FROM storage_nodes;"
   ```
4. **Expected:** 2 rows with type=LOCAL, paths to ssd1/ssd2

**Cloud Storage (S3/R2):**
1. Clear backend: `rm -rf /tmp/cubycloud-dev/db && ./scripts/dev.sh`
2. Go to `/setup`
3. Step 4: enable cloud with **real** R2/S3 credentials
4. Submit
5. Query storage_nodes:
   ```bash
   sqlite3 /tmp/cubycloud-dev/db/cubycloud.db "SELECT type, endpoint, bucket FROM storage_nodes WHERE type='S3';"
   ```
6. **Expected:** 1 row with type=S3, your endpoint/bucket

### 5. Verify JWT Cookie

1. Complete setup
2. Open DevTools → Application → Cookies → `http://localhost:5173`
3. **Expected:** Cookie `cuby_session` (HTTP-only, SameSite=Lax, 7-day expiry)

### 6. Test Recovery Key

1. Complete setup, save recovery key
2. Try bcrypt verification:
   ```bash
   sqlite3 /tmp/cubycloud-dev/db/cubycloud.db "SELECT recovery_key_hash FROM users WHERE username='admin';"
   ```
3. **Expected:** bcrypt hash (60 chars, starts with `$2a$10$`)

## Expected Results Summary

✅ Setup wizard scans real storage nodes from backend  
✅ Submit creates user + storage_nodes + settings in SQLite  
✅ Recovery key: 64-char hex, bcrypt stored, copy/download work  
✅ Success redirects to `/drive/home` with sidebar  
✅ JWT cookie set (HTTP-only)  
✅ Setup guard: 409 on duplicate setup  
✅ Validation: password <8, no nodes, invalid cloud rejected  
✅ Storage pairing: local nodes + cloud nodes inserted correctly  
✅ No console errors  

## Cleanup

```bash
# Stop servers: Ctrl+C in terminal running ./scripts/dev.sh
# Remove test data:
rm -rf /tmp/cubycloud-dev
```

## Troubleshooting

**Backend won't start:**
- Check port 8080 not in use: `lsof -i :8080`
- Check Go version: `go version` (need 1.22+)

**Frontend won't start:**
- Check port 5173 not in use: `lsof -i :5173`
- Check Bun installed: `bun --version`

**Scan returns 0 nodes:**
- Verify dirs exist: `ls -la /tmp/cubycloud-dev/storage/`
- Check backend logs for StatLocal errors

**Cloud validation fails:**
- Verify real credentials (test with `aws s3 ls s3://bucket`)
- Check network access to endpoint
- R2: endpoint format `https://<account_id>.r2.cloudflarestorage.com`
