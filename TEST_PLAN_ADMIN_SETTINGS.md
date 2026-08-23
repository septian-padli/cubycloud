# Test Plan: Admin Settings Complete Integration

## Scope
Membangun dan menguji **semua fitur** di halaman `/admin/settings` hingga siap produksi dengan test coverage tinggi.

## Gap Analysis

### Setup Features (existing)
1. ✅ Storage nodes scan & setup
2. ✅ Cloud storage (S3) configuration
3. ✅ Placement strategy selection
4. ✅ Admin account creation
5. ✅ Capacity limits per node

### Admin Settings Features (current state)
1. ✅ System settings (quota, retention, concurrency)
2. ✅ Storage nodes list & rescan
3. ⚠️ Detach node (implemented but not fully tested)
4. ❌ Add new storage node (missing)
5. ❌ Edit storage node capacity (missing)
6. ❌ Cloud storage management (missing)
7. ❌ Placement strategy update (not exposed in UI)

## Features to Build

### 1. Add Storage Node
**Backend:**
- POST `/api/v1/admin/storage-nodes` - Add local disk node
- POST `/api/v1/admin/storage-nodes/cloud` - Add S3 node
- Validation: path exists, not duplicate, capacity > 0

**Frontend:**
- Dialog with tabs: Local / Cloud (S3)
- Local: path input, capacity limit, enabled checkbox
- Cloud: endpoint, bucket, region, access/secret key, capacity
- Real-time path validation
- Success toast + refresh list

### 2. Edit Storage Node
**Backend:**
- PUT `/api/v1/admin/storage-nodes/:id` - Update capacity, enabled status
- Validation: capacity >= used space

**Frontend:**
- Edit dialog with current values
- Capacity slider/input
- Enable/disable toggle
- Show current usage vs new capacity
- Warning if reducing below usage

### 3. Cloud Storage Management
**Backend:**
- GET `/api/v1/admin/storage-nodes/:id/cloud-config` - Get S3 credentials (masked)
- PUT `/api/v1/admin/storage-nodes/:id/cloud-config` - Update S3 credentials
- DELETE `/api/v1/admin/storage-nodes/:id/cloud-config` - Remove cloud node

**Frontend:**
- Cloud nodes show endpoint/bucket
- Edit button for S3 credentials
- Test connection button
- Migration status (if data exists)

### 4. Placement Strategy UI
**Backend:**
- Already in settings, just expose in UI

**Frontend:**
- Radio group: FILL_FIRST / BALANCE / PERFORMANCE
- Info tooltip explaining each strategy
- Show in System Settings section

## Test Coverage Requirements

### Backend Unit Tests
**Target: >80% coverage**

1. Admin Service Tests (`admin/service_test.go`)
   - [ ] GetSettings returns default values
   - [ ] UpdateSettings validates quota >= 1GB
   - [ ] UpdateSettings validates retention in {7,14,30,60,90}
   - [ ] UpdateSettings validates concurrency 1-10
   - [ ] GetStorageNodes returns correct disk stats
   - [ ] DisableNode marks node as disabled
   - [ ] RescanNodes updates capacity from disk
   - [ ] AddStorageNode validates path
   - [ ] AddStorageNode rejects duplicate path
   - [ ] UpdateStorageNode validates capacity >= usage
   - [ ] DeleteStorageNode soft-deletes node

2. Admin Repository Tests (`admin/repository_test.go`)
   - [ ] All CRUD operations
   - [ ] Transaction rollback on error
   - [ ] Concurrent updates handled correctly

3. Middleware Tests
   - [ ] JWT auth rejects invalid token
   - [ ] JWT auth rejects expired token
   - [ ] Super Admin guard rejects regular user
   - [ ] Super Admin guard allows SUPER_ADMIN

### Backend Integration Tests
**Target: 100% endpoint coverage**

Script: `test-admin-settings-full.sh`

**Settings:**
- [ ] GET /admin/settings returns defaults
- [ ] PUT /admin/settings updates all fields
- [ ] PUT /admin/settings rejects invalid quota
- [ ] PUT /admin/settings rejects invalid retention
- [ ] PUT /admin/settings rejects invalid concurrency

**Storage Nodes:**
- [ ] GET /admin/storage-nodes returns nodes from setup
- [ ] POST /admin/storage-nodes adds local node
- [ ] POST /admin/storage-nodes/cloud adds S3 node
- [ ] PUT /admin/storage-nodes/:id updates capacity
- [ ] PUT /admin/storage-nodes/:id/disable disables node
- [ ] POST /admin/storage-nodes/rescan updates disk stats
- [ ] DELETE /admin/storage-nodes/:id soft-deletes node

**Auth:**
- [ ] All endpoints require JWT token
- [ ] All endpoints require SUPER_ADMIN role
- [ ] Invalid token returns 401
- [ ] Regular user returns 403

**Backup:**
- [ ] GET /admin/backup downloads SQLite file
- [ ] Backup file is valid SQLite database
- [ ] Backup contains all tables

### Frontend Tests
**Target: >70% coverage**

Using Vitest + Testing Library

1. Admin API Client Tests (`lib/api/admin.test.ts`)
   - [ ] getSettings handles 200
   - [ ] getSettings handles 403
   - [ ] updateSettings sends correct payload
   - [ ] addStorageNode validates input
   - [ ] downloadBackup triggers download

2. Component Tests
   - [ ] AdminSystemSettings renders current values
   - [ ] AdminSystemSettings validates before save
   - [ ] AdminSystemSettings shows error toast
   - [ ] AdminStorageManagement lists nodes
   - [ ] AdminStorageManagement add dialog works
   - [ ] AdminStorageManagement edit dialog works
   - [ ] AdminStorageManagement detach confirmation

3. E2E Test (`admin-settings.spec.ts` with Playwright)
   - [ ] Login as super admin
   - [ ] Navigate to /admin/settings
   - [ ] Change quota and save
   - [ ] Add new local node
   - [ ] Edit node capacity
   - [ ] Detach node with confirmation
   - [ ] Rescan nodes
   - [ ] Download backup

## Validation Rules (must match setup)

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

## Setup Alignment Checklist

- [ ] Storage nodes created in setup appear in admin
- [ ] Capacity limits from setup editable in admin
- [ ] Cloud storage from setup manageable in admin
- [ ] Placement strategy from setup changeable in admin
- [ ] Default quota from setup matches system settings
- [ ] All setup validations apply in admin

## Acceptance Criteria

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

## Timeline Estimate
- Backend: 2-3 hours (endpoints + tests)
- Frontend: 2-3 hours (UI + integration)
- Testing: 1-2 hours (E2E + validation)
- **Total: 5-8 hours**
