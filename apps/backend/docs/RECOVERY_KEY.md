# Recovery Key Implementation - Complete Guide

**Last Updated**: 2026-08-23 17:50 WIB

---

## ✅ Implementation Complete

Recovery key sekarang **100% functional** dan teruji:

1. ✅ Backend generates 64-char hex recovery key
2. ✅ Stored as bcrypt hash in database
3. ✅ Frontend shows recovery key in success dialog
4. ✅ Copy and download buttons work
5. ✅ Password reset endpoint functional
6. ✅ All security validations working

---

## 🎯 User Flow

### 1. Complete Setup (Step 1-5)
```
Step 1: Create admin account
Step 2: Select storage nodes
Step 3: Configure cloud (optional)
Step 4: Choose placement strategy
Step 5: Final confirmation
```

### 2. Success Dialog Appears
After clicking "Selesaikan Setup & Masuk ke CubyCloud":
- ✅ Dialog shows recovery key (64 hex characters)
- ✅ Copy button → copies to clipboard
- ✅ Download button → saves as `cubycloud-recovery-key.txt`
- ✅ Must acknowledge before continuing

### 3. Recovery Key Storage
User MUST save the key because:
- Only shown ONCE in success dialog
- Cannot be retrieved later from UI
- Stored as bcrypt hash (irreversible)
- Required for password recovery

---

## 🔑 Password Reset Flow

### Endpoint
```http
POST /api/v1/auth/reset-password
Content-Type: application/json

{
  "username": "admin",
  "recovery_key": "ec3b9019f87db838c73a0be479fc51e19ef19d6bef87340206f3e43f0fe919cd",
  "new_password": "newpassword456"
}
```

### Response (Success)
```json
{
  "message": "password reset successful"
}
```

### Response (Invalid Key)
```json
{
  "message": "invalid recovery key"
}
```

### Response (User Not Found)
```json
{
  "message": "user not found"
}
```

---

## 🧪 Testing

### Automated Test
```bash
./test-recovery-key.sh
```

**Test coverage**:
- ✅ Recovery key generation (64 hex)
- ✅ Bcrypt hash storage
- ✅ Password reset with valid key
- ✅ Invalid key rejection (401)
- ✅ Wrong username rejection (404)

### Manual Browser Test
```bash
./scripts/dev.sh
# Open: http://localhost:5173/setup
```

**Steps**:
1. Complete all 5 setup steps
2. Click "Selesaikan Setup & Masuk ke CubyCloud"
3. ✅ Success dialog should appear
4. ✅ Recovery key displayed (64 chars)
5. ✅ Click "Salin" → key copied to clipboard
6. ✅ Click "Unduh" → downloads `.txt` file
7. ✅ Check acknowledgment checkbox
8. ✅ Click "Lanjut ke Beranda"

---

## 🐛 Troubleshooting

### Issue: "Setup already completed" error

**Cause**: Database already has `is_setup_completed = true`

**Solution 1**: Reset database (development)
```bash
# Backup and remove existing database
mv apps/backend/data/db/cubycloud.db apps/backend/data/db/cubycloud.db.backup
# Restart dev server
./scripts/dev.sh
```

**Solution 2**: Check database status
```bash
sqlite3 apps/backend/data/db/cubycloud.db \
  "SELECT key, value FROM settings WHERE key='is_setup_completed';"
```

If returns `is_setup_completed|true`, setup cannot be run again (by design).

### Issue: Success dialog tidak muncul

**Debug steps**:
1. Open browser console (F12)
2. Go to Network tab
3. Complete setup
4. Check `/api/v1/setup/complete` response
5. Should return:
   ```json
   {
     "recovery_key": "64-char-hex-string",
     "redirect": "/drive/home"
   }
   ```

If response is 409 (Conflict), database already setup.

### Issue: Recovery key tidak bisa di-copy

**Cause**: Browser clipboard API requires HTTPS or localhost

**Solution**: 
- Development: Already on localhost (should work)
- Production: Must use HTTPS

### Issue: Download button tidak bekerja

**Debug**:
1. Open browser console
2. Click "Unduh"
3. Check for JavaScript errors
4. Verify download triggered (check Downloads folder)

---

## 📊 Database Schema

### users table
```sql
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  recovery_key_hash TEXT,  -- bcrypt hash of recovery key
  role TEXT NOT NULL,
  quota_bytes INTEGER NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
```

**Security**:
- `password_hash`: bcrypt with cost=10
- `recovery_key_hash`: bcrypt with cost=10
- Both hashes use same algorithm for consistency

---

## 🔒 Security Considerations

### What's Stored
- ✅ Recovery key hash (bcrypt, irreversible)
- ✅ Password hash (bcrypt, irreversible)

### What's NOT Stored
- ❌ Plaintext recovery key
- ❌ Plaintext password

### Key Generation
```go
// 32 bytes random → 64 hex chars
recoveryKeyBytes := make([]byte, 32)
rand.Read(recoveryKeyBytes)
recoveryKey := hex.EncodeToString(recoveryKeyBytes)
// Result: "ec3b9019f87db838c73a0be479fc51e19ef19d6bef87340206f3e43f0fe919cd"
```

### Verification
```go
// Compare user input with stored hash
err := bcrypt.CompareHashAndPassword(
  []byte(storedHash), 
  []byte(userInputKey)
)
if err != nil {
  return "invalid recovery key"
}
```

---

## 📁 Implementation Files

### Backend
```
apps/backend/internal/modules/auth/
├── handler.go       # POST /api/v1/auth/reset-password
├── service.go       # Password reset logic
├── repository.go    # Database operations
└── types.go         # Request/response types
```

### Frontend
```
apps/frontend/src/lib/components/setup/
├── setup-wizard.svelte              # Main wizard logic
├── setup-success-dialog.svelte      # Recovery key display
└── steps/step-recovery.svelte       # Step 5 confirmation
```

### Tests
```
test-recovery-key.sh                 # Integration test script
```

---

## 🎯 Next Steps (Optional Improvements)

1. **Frontend password reset UI**
   - Create `/reset-password` page
   - Form: username + recovery key + new password
   - Call `POST /api/v1/auth/reset-password`

2. **Email recovery key** (future)
   - Send recovery key via email after setup
   - Requires SMTP configuration
   - Still show in dialog (primary method)

3. **Multiple recovery methods**
   - Recovery key (current)
   - Backup codes (generate 10 one-time codes)
   - Security questions (optional)

4. **Audit log**
   - Log password reset attempts
   - Track recovery key usage
   - Alert on multiple failed attempts

---

## ✅ Production Readiness Checklist

- [x] Recovery key generated securely (crypto/rand)
- [x] Stored as bcrypt hash (cost=10)
- [x] Only shown once in success dialog
- [x] Copy and download functionality working
- [x] Password reset endpoint functional
- [x] Invalid key rejected (401)
- [x] Wrong username rejected (404)
- [x] Manual validation (no external validator dependency)
- [x] SQLite compatible (time.Now() instead of NOW())
- [x] Integration tests passing
- [x] Error handling complete
- [x] User acknowledgment required

**Status**: ✅ Production-ready

---

## 📞 Support

If recovery key is lost:
1. **Development**: Reset database and re-run setup
2. **Production**: Manual database intervention required (SQL UPDATE users SET password_hash = ... WHERE username = 'admin')

**Important**: There is NO way to recover the recovery key itself if lost. This is by design for security.

---

**Implementation verified**: 2026-08-23 17:50 WIB  
**Test status**: All tests passing ✅
