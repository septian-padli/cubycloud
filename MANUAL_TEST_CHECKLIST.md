# Manual Testing Checklist - Admin Settings

## Pre-requisites
- [ ] Backend compiled: `cd apps/backend && go build ./cmd/server`
- [ ] Frontend compiled: `cd apps/frontend && bun run build`
- [ ] Run backend: `cd apps/backend && go run cmd/server/main.go`
- [ ] Access: http://localhost:5173

## Phase 1: Setup & Authentication
- [ ] Navigate to `/setup`
- [ ] Create super admin account
  - Email: admin@test.local
  - Password: Test1234!
  - Recovery key: (save it)
- [ ] Verify redirect to `/drive` after setup
- [ ] Verify JWT cookie `cuby_token` set (check DevTools → Application → Cookies)

## Phase 2: Navigate to Admin Settings
- [ ] Click user menu (top-right)
- [ ] Click "Pengaturan Admin" / Admin Settings
- [ ] Verify redirect to `/admin/settings`
- [ ] Verify page loads without errors
- [ ] Verify 3 sections visible:
  - Sistem & Kuota
  - Penyimpanan & Node
  - Cadangan & Pemulihan

## Phase 3: System Settings (Sistem & Kuota)
- [ ] Change default quota from 10 GB to 20 GB
- [ ] Change audit retention from 90 days to 30 days
- [ ] Change worker concurrency from 4 to 6
- [ ] Click "Simpan Perubahan" / Save Changes
- [ ] Verify success toast notification
- [ ] Reload page
- [ ] Verify settings persisted

## Phase 4: Placement Strategy
- [ ] Find "Strategi Penempatan File" dropdown
- [ ] Verify default value: "Seimbang (balanced)"
- [ ] Change to "Isi Penuh Dulu (fill-first)"
- [ ] Verify auto-save (no explicit button)
- [ ] Verify success toast
- [ ] Reload page
- [ ] Verify strategy persisted

## Phase 5: Storage Nodes - Add Local Node
- [ ] Scroll to "Penyimpanan & Node" section
- [ ] Verify default node visible (usually `/app/data/storage/default`)
- [ ] Click "Tambah Node" button
- [ ] Dialog opens with 2 tabs: "Disk Lokal" and "Cloud (S3)"
- [ ] Verify "Disk Lokal" tab active
- [ ] Fill form:
  - Path: `/tmp/test-storage-node-1`
  - Capacity: 50 GB
  - Enabled: checked
- [ ] Click "Tambah Node Lokal"
- [ ] Verify validation error: "path does not exist"
- [ ] Create directory: `mkdir -p /tmp/test-storage-node-1`
- [ ] Try again with same path
- [ ] Verify success toast
- [ ] Verify new node appears in grid
- [ ] Verify node details: name, path, capacity, enabled status

## Phase 6: Storage Nodes - Add Cloud Node (S3)
- [ ] Click "Tambah Node" button again
- [ ] Click "Cloud (S3)" tab
- [ ] Fill form:
  - Name: "Test S3 Bucket"
  - Endpoint: `https://s3.amazonaws.com`
  - Bucket: `cuby-test-bucket`
  - Region: `us-east-1`
  - Access Key: `AKIATEST1234567890`
  - Secret Key: `test-secret-key-1234567890`
  - Capacity: 100 GB
  - Enabled: unchecked (disabled by default for testing)
- [ ] Click "Tambah Node Cloud"
- [ ] Verify success toast
- [ ] Verify new cloud node in grid
- [ ] Verify node details: name, type S3, capacity, disabled badge

## Phase 7: Storage Nodes - Edit Node
- [ ] Find the local node created in Phase 5
- [ ] Click "Edit" button
- [ ] Edit dialog opens
- [ ] Verify current values pre-filled
- [ ] Change capacity from 50 GB to 75 GB
- [ ] Toggle "Status Node" switch to disabled
- [ ] Click "Simpan Perubahan"
- [ ] Verify success toast
- [ ] Verify node updated in grid (capacity 75 GB, disabled badge)

## Phase 8: Storage Nodes - Disable Node
- [ ] Find a enabled node
- [ ] Click "Nonaktifkan" / Disable button
- [ ] Confirmation dialog opens
- [ ] Click "Ya, Nonaktifkan"
- [ ] Verify success toast
- [ ] Verify node shows "Dinonaktifkan" badge
- [ ] Verify "Nonaktifkan" button no longer visible for that node

## Phase 9: Storage Nodes - Rescan
- [ ] Create a new directory: `mkdir -p /tmp/test-storage-node-2`
- [ ] Click "Rescan" button
- [ ] Rescan dialog opens
- [ ] Click "Rescan Sekarang"
- [ ] Verify progress/loading state
- [ ] Verify success toast
- [ ] Verify new node `/tmp/test-storage-node-2` appears in grid (auto-discovered)

## Phase 10: Backup & Recovery (Cadangan & Pemulihan)
- [ ] Scroll to "Cadangan & Pemulihan" section
- [ ] Verify backup section visible
- [ ] Click "Unduh Backup Database" button
- [ ] Verify file download starts
- [ ] Verify filename: `cubycloud-backup-YYYY-MM-DD.db`
- [ ] Verify file size > 0 bytes (check Downloads folder)
- [ ] Verify file is a valid SQLite database: `file cubycloud-backup-*.db`

## Phase 11: Error Handling
- [ ] Try adding node with invalid capacity (0 GB)
- [ ] Verify validation error toast
- [ ] Try adding local node with duplicate path
- [ ] Verify error toast: "node with path X already exists"
- [ ] Try navigating to `/admin/settings` without auth (logout first)
- [ ] Verify redirect to `/setup` or error response

## Phase 12: Responsive UI
- [ ] Resize browser to mobile width (375px)
- [ ] Verify all sections stack vertically
- [ ] Verify dialogs are responsive
- [ ] Verify forms remain usable
- [ ] Verify no horizontal scroll
- [ ] Verify touch targets adequate (>44px)

## Phase 13: Accessibility
- [ ] Navigate using Tab key
- [ ] Verify focus indicators visible
- [ ] Verify dialogs trap focus
- [ ] Verify Escape key closes dialogs
- [ ] Verify form labels associated with inputs
- [ ] Verify error messages announced (check with screen reader if available)

## Phase 14: Data Persistence
- [ ] Make several changes (quota, nodes, placement strategy)
- [ ] Restart backend server
- [ ] Reload frontend
- [ ] Verify all changes persisted
- [ ] Verify nodes still visible
- [ ] Verify settings unchanged

## Phase 15: Cross-Feature Integration
- [ ] Add files via `/drive` (upload some test files)
- [ ] Verify files distributed across enabled nodes
- [ ] Disable all nodes except one
- [ ] Try uploading more files
- [ ] Verify files only go to the enabled node
- [ ] Re-enable nodes
- [ ] Verify placement strategy affects new uploads

## Success Criteria
- All checklist items pass
- No console errors in browser DevTools
- No panics or errors in backend logs
- All API responses return valid JSON
- Toast notifications display correctly
- Data persists across restarts
- UI remains responsive under load

## Notes
- Record any bugs or unexpected behavior
- Note performance issues (slow loads, laggy UI)
- Capture screenshots of issues
- Check backend logs for warnings/errors

