# CubyCloud Testing Guide

## Overview

Testing strategy untuk CubyCloud mencakup:
- Setup flow testing
- Admin settings feature testing
- Manual testing checklist
- Integration testing scripts

---

## 1. Setup Flow Testing

### Prerequisites
- Go 1.22+
- Bun
- macOS/Linux (StatLocal uses Unix syscall.Statfs)

### Start Development Environment

```bash
./scripts/dev.sh
```

Creates `/tmp/cubycloud-dev` with 2 mock storage nodes (ssd1, ssd2), starts backend (8080) + frontend (5173).

### Happy Path Test Procedure

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
   - Verify the Recovery Key displays as six groups of five uppercase alphanumeric characters: `XXXXX-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX` (30 characters, 35 including separators)
   - Click **Copy** → verify the copied value preserves the grouped format
   - Click **Download** → verify `cubycloud-recovery-key.txt` downloaded
   - Check "I have saved my recovery key"
   - Click **Continue to CubyCloud**

8. **Redirect:**
   - Should redirect to `/drive/home`
   - Verify sidebar renders with menu items

### Edge Cases

**Setup Already Completed (409):**
- Refresh `/setup` after completion
- Expected: toast error "Setup already completed"

**Password Validation:**
- Clear backend: `rm -rf /tmp/cubycloud-dev/db && ./scripts/dev.sh`
- Try password < 8 chars
- Expected: form validation prevents submit

**No Enabled Nodes:**
- Disable all nodes in Step 2
- Expected: "Selanjutnya" button disabled

**Cloud Credentials Invalid:**
- Enable cloud with fake credentials
- Expected: toast error "Cloud credentials invalid"

### Storage Pairing Verification

```bash
# Check DB created
ls -lh /tmp/cubycloud-dev/db/cubycloud.db

# Query storage nodes
sqlite3 /tmp/cubycloud-dev/db/cubycloud.db "SELECT * FROM storage_nodes;"
```

Expected: 2 rows with type=LOCAL, paths to ssd1/ssd2

### JWT Cookie Verification

DevTools → Application → Cookies → `http://localhost:5173`
Expected: Cookie `cuby_session` (HTTP-only, SameSite=Lax, 7-day expiry)

### Recovery Key Verification

1. Complete setup and capture the displayed 6×5 Recovery Key; verify its shape is exactly 30 uppercase alphanumeric characters grouped with five hyphens.
2. Verify the saved `recovery_key_hash` is SHA-256 of the canonical 30-character key (remove hyphens; compare hash rather than expecting bcrypt).
3. Open `/recovery`, enter the current key and a new password, and submit. Verify success returns `new_recovery_key` in the current 6×5 format.
4. Verify the newly returned key works for a subsequent reset and the original key is rejected as consumed.
5. UI accepts only the current 6×5 format.
6. Migration-only backend compatibility case: a 64-character hexadecimal key works only when the account has a legacy bcrypt recovery hash; after successful reset, verify the replacement uses 6×5 with SHA-256 and the old key is unusable. This is not the current UI or product format.

Passwords themselves remain bcrypt-hashed; that is separate from Recovery Key storage.

---

## 2. Admin Settings Testing

### Test Coverage Requirements

**Backend Unit Tests:** Target >80% coverage
- Admin service tests (`admin/service_test.go`)
- Admin repository tests (`admin/repository_test.go`)
- Middleware tests (JWT auth, Super Admin guard)

**Backend Integration Tests:** 100% endpoint coverage
- Run script: `scripts/test/admin-settings-full.sh`
- Covers: settings CRUD, storage nodes CRUD, auth/authz, backup

**Frontend Tests:** Target >70% coverage
- API client tests (`lib/api/admin.test.ts`)
- Component tests (Vitest + Testing Library)
- E2E tests (Playwright)

### Manual Testing Checklist

#### Phase 1: Setup & Authentication
- [ ] Navigate to `/setup`
- [ ] Create super admin account (admin@test.local / Test1234!)
- [ ] Save recovery key
- [ ] Verify redirect to `/drive` after setup
- [ ] Verify JWT cookie `cuby_token` set in DevTools

#### Phase 2: Navigate to Admin Settings
- [ ] Click user menu (top-right)
- [ ] Click "Pengaturan Admin" / Admin Settings
- [ ] Verify redirect to `/admin/settings`
- [ ] Verify page loads without errors
- [ ] Verify 3 sections visible:
  - Sistem & Kuota
  - Penyimpanan & Node
  - Cadangan & Pemulihan

#### Phase 3: System Settings (Sistem & Kuota)
- [ ] Change default quota from 10 GB to 20 GB
- [ ] Change audit retention from 90 days to 30 days
- [ ] Change worker concurrency from 4 to 6
- [ ] Click "Simpan Perubahan" / Save Changes
- [ ] Verify success toast notification
- [ ] Reload page
- [ ] Verify settings persisted

#### Phase 4: Placement Strategy
- [ ] Find "Strategi Penempatan File" dropdown
- [ ] Verify default value: "Seimbang (balanced)"
- [ ] Change to "Isi Penuh Dulu (fill-first)"
- [ ] Verify auto-save (no explicit button)
- [ ] Verify success toast
- [ ] Reload page
- [ ] Verify strategy persisted

#### Phase 5: Storage Nodes - Add Local Node
- [ ] Scroll to "Penyimpanan & Node" section
- [ ] Verify default node visible (usually `/app/data/storage/default`)
- [ ] Click "Tambah Node" button
- [ ] Dialog opens with 2 tabs: "Disk Lokal" and "Cloud (S3)"
- [ ] Fill form: Path `/tmp/test-storage-node-1`, Capacity 50 GB, Enabled checked
- [ ] Click "Tambah Node Lokal"
- [ ] Verify validation error: "path does not exist"
- [ ] Create directory: `mkdir -p /tmp/test-storage-node-1`
- [ ] Try again with same path
- [ ] Verify success toast
- [ ] Verify new node appears in grid

#### Phase 6: Storage Nodes - Add Cloud Node (S3)
- [ ] Click "Tambah Node" button again
- [ ] Click "Cloud (S3)" tab
- [ ] Fill form: Name "Test S3 Bucket", endpoint, bucket, region, keys, capacity 100 GB
- [ ] Click "Tambah Node Cloud"
- [ ] Verify success toast
- [ ] Verify new cloud node in grid

#### Phase 7: Storage Nodes - Edit Node
- [ ] Find the local node created in Phase 5
- [ ] Click "Edit" button
- [ ] Change capacity from 50 GB to 75 GB
- [ ] Toggle "Status Node" switch to disabled
- [ ] Click "Simpan Perubahan"
- [ ] Verify success toast
- [ ] Verify node updated in grid (capacity 75 GB, disabled badge)

#### Phase 8: Storage Nodes - Disable Node
- [ ] Find a enabled node
- [ ] Click "Nonaktifkan" / Disable button
- [ ] Confirmation dialog opens
- [ ] Click "Ya, Nonaktifkan"
- [ ] Verify success toast
- [ ] Verify node shows "Dinonaktifkan" badge

#### Phase 9: Storage Nodes - Rescan
- [ ] Create new directory: `mkdir -p /tmp/test-storage-node-2`
- [ ] Click "Rescan" button
- [ ] Rescan dialog opens
- [ ] Click "Rescan Sekarang"
- [ ] Verify progress/loading state
- [ ] Verify success toast
- [ ] Verify new node `/tmp/test-storage-node-2` appears (auto-discovered)

#### Phase 10: Backup & Recovery (Cadangan & Pemulihan)
- [ ] Scroll to "Cadangan & Pemulihan" section
- [ ] Click "Unduh Backup Database" button
- [ ] Verify file download starts
- [ ] Verify filename: `cubycloud-backup-YYYY-MM-DD.db`
- [ ] Verify file size > 0 bytes
- [ ] Verify file is valid SQLite: `file cubycloud-backup-*.db`

#### Phase 11: Error Handling
- [ ] Try adding node with invalid capacity (0 GB) → validation error toast
- [ ] Try adding local node with duplicate path → error toast
- [ ] Try navigating to `/admin/settings` without auth (logout first) → redirect or error

#### Phase 12: Responsive UI
- [ ] Resize browser to mobile width (375px)
- [ ] Verify all sections stack vertically
- [ ] Verify dialogs responsive
- [ ] Verify no horizontal scroll

#### Phase 13: Accessibility
- [ ] Navigate using Tab key
- [ ] Verify focus indicators visible
- [ ] Verify dialogs trap focus
- [ ] Verify Escape key closes dialogs
- [ ] Verify form labels associated with inputs

#### Phase 14: Data Persistence
- [ ] Make several changes (quota, nodes, placement strategy)
- [ ] Restart backend server
- [ ] Reload frontend
- [ ] Verify all changes persisted

#### Phase 15: Cross-Feature Integration
- [ ] Add files via `/drive` (upload test files)
- [ ] Verify files distributed across enabled nodes
- [ ] Disable all nodes except one
- [ ] Try uploading more files
- [ ] Verify files only go to enabled node
- [ ] Re-enable nodes
- [ ] Verify placement strategy affects new uploads

### Success Criteria
- All checklist items pass
- No console errors in browser DevTools
- No panics or errors in backend logs
- All API responses return valid JSON
- Toast notifications display correctly
- Data persists across restarts
- UI remains responsive under load

---

## 3. Validation Rules

### Storage Node Capacity
- Local: 1GB - disk total capacity
- S3: 1GB - unlimited (admin-defined)
- Cannot reduce below current usage

### Quota Settings
- Unlimited: -1 in DB
- Limited: >= 1GB

### Retention
- Exactly: 7, 14, 30, 60, or 90 days
- Default: 90 days

### Concurrency
- Range: 1-10 workers
- Default: 2 workers

### Placement Strategy
- FILL_FIRST: Fill nodes sequentially
- BALANCE: Distribute evenly
- PERFORMANCE: Prefer fastest nodes (future)

---

## 4. Integration Test Scripts

Located in `scripts/test/`:
- `admin-complete.sh` - Complete admin feature test
- `admin-full-integration.sh` - Full integration test suite
- `admin-settings-full.sh` - Admin settings endpoint test
- `admin-settings.sh` - Basic admin settings test
- `recovery-key-new-flow.sh` - 6×5 format, SHA-256 storage, reset rotation and input normalization

Run example:
```bash
cd scripts/test
./admin-settings-full.sh
```

### My Storage Browser E2E

Run from `apps/frontend/`:

```bash
bunx playwright install chromium
bun run test:e2e
```

The Playwright suite starts the real Go backend and SvelteKit frontend with an isolated temporary database/storage directory. It exercises the My Storage UI against the backend API; it does not substitute mocked API responses. The temporary data is removed after the run.

The E2E suite also verifies that the upload queue sits below the top navigation and reports completion only after all queued uploads succeed.

The backend `mystorage` package tests verify oversized upload rejection against user quota and physical-capacity limits, including bounded writes and cleanup of partial files.


---

## 5. Cleanup & Troubleshooting

### Reset Development Data

```bash
# Full reset (recommended)
rm -rf /tmp/cubycloud-dev

# Database only (keep uploaded files)
rm /tmp/cubycloud-dev/db/cubycloud.db*
```

### Troubleshooting

**Backend won't start:**
- Check port 8080: `lsof -i :8080`
- Check Go version: `go version` (need 1.22+)

**Frontend won't start:**
- Check port 5173: `lsof -i :5173`
- Check Bun: `bun --version`

**Scan returns 0 nodes:**
- Verify dirs exist: `ls -la /tmp/cubycloud-dev/storage/`
- Check backend logs for StatLocal errors

**Cloud validation fails:**
- Verify real credentials (test with `aws s3 ls s3://bucket`)
- Check network access to endpoint
- R2 endpoint format: `https://<account_id>.r2.cloudflarestorage.com`

---

## 6. Test Plan: Admin Settings Complete Integration

### Scope
Build and test all features on `/admin/settings` page until production-ready with high test coverage.

### Gap Analysis

**Setup Features (existing):**
1. ✅ Storage nodes scan & setup
2. ✅ Cloud storage (S3) configuration
3. ✅ Placement strategy selection
4. ✅ Admin account creation
5. ✅ Capacity limits per node

**Admin Settings Features (current state):**
1. ✅ System settings (quota, retention, concurrency)
2. ✅ Storage nodes list & rescan
3. ⚠️ Detach node (implemented but not fully tested)
4. ❌ Add new storage node (missing)
5. ❌ Edit storage node capacity (missing)
6. ❌ Cloud storage management (missing)
7. ❌ Placement strategy update (not exposed in UI)

### Features to Build

**1. Add Storage Node**
- Backend: POST `/api/v1/admin/storage-nodes`, POST `/api/v1/admin/storage-nodes/cloud`
- Validation: path exists, not duplicate, capacity > 0
- Frontend: Dialog with tabs (Local/Cloud), real-time validation, success toast

**2. Edit Storage Node**
- Backend: PUT `/api/v1/admin/storage-nodes/:id`
- Validation: capacity >= used space
- Frontend: Edit dialog, capacity slider, enable/disable toggle, usage warning

**3. Cloud Storage Management**
- Backend: GET/PUT/DELETE `/api/v1/admin/storage-nodes/:id/cloud-config`
- Frontend: Masked credentials view, test connection button, migration status

**4. Placement Strategy UI**
- Backend: Already in settings
- Frontend: Radio group (FILL_FIRST/BALANCE/PERFORMANCE), info tooltip

### Acceptance Criteria

**Backend:**
- [ ] All endpoints implemented and documented
- [ ] Unit tests >80% coverage
- [ ] Integration tests pass (all scenarios)
- [ ] No compilation errors
- [ ] No linting errors

**Frontend:**
- [ ] All features UI implemented
- [ ] Real API integration (no mocks)
- [ ] Error handling with Indonesian messages
- [ ] Loading states for async operations
- [ ] Success/error toasts
- [ ] No TypeScript errors
- [ ] No console errors in dev

**Integration:**
- [ ] Setup → Admin data flow works
- [ ] All CRUD operations tested
- [ ] Auth/authz working correctly
- [ ] Database migrations applied
- [ ] No data loss scenarios

### Timeline Estimate
- Backend: 2-3 hours (endpoints + tests)
- Frontend: 2-3 hours (UI + integration)
- Testing: 1-2 hours (E2E + validation)
- **Total: 5-8 hours**
