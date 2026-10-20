# Workflow Rules - CubyCloud Development

**Last Updated**: 2026-08-23 10:26 WIB

---

## ⚠️ CRITICAL RULE: Git Operations

### ❌ JANGAN commit dan push tanpa izin user

**Aturan**:
- JANGAN otomatis `git commit` setelah implementasi
- JANGAN otomatis `git push` setelah commit
- TUNGGU instruksi eksplisit dari user

**Workflow yang benar**:
1. Implementasi code
2. Test dan verifikasi
3. **STOP** - lapor hasil ke user
4. **TUNGGU** user minta commit
5. Baru commit setelah user bilang "commit" atau "push"

**Contoh instruksi user yang benar**:
- ✅ "commit sekarang"
- ✅ "push ke remote"
- ✅ "commit dengan message X"
- ❌ JANGAN commit kalau user hanya bilang "continue"

---

## 🎯 Development Workflow

### 1. Receive Task
- Baca requirement dengan seksama
- Identifikasi scope dan deliverables
- Tanya kalau ada ambiguitas

### 2. Implementation
- Implement code
- Write tests
- Verify locally

### 3. Verification
- Run tests
- Check coverage
- Test manually kalau perlu

### 4. Report to User
- Show test results
- Show what was implemented
- **STOP HERE** - jangan commit

### 5. Wait for User Approval
- User akan test di browser
- User akan review code
- User akan cek hasil

### 6. Commit (Only After User Request)
- User bilang "commit"
- Buat commit message yang jelas
- `git add` + `git commit`

---

## 📁 Documentation Location Rules

### ✅ Where to Place Documentation

**Frontend documentation**:
- Location: `/apps/frontend/docs/`
- Examples:
  - UI/UX guides
  - Component documentation
  - Frontend API usage
  - SvelteKit-specific guides
  - Browser features (localStorage, etc.)

**Backend documentation**:
- Location: `/apps/backend/docs/`
- Examples:
  - API endpoints
  - Database schemas
  - Service layer docs
  - Integration tests
  - Backend architecture

**Root-level documentation** (`/docs/`):
- Product requirements (prd.md)
- System architecture (architecture.md)
- Cross-project documentation
- Deployment guides

**Repository-level** (root):
- README.md
- CONTRIBUTING.md
- LICENSE
- CHANGELOG.md

### ❌ What NOT to Do

1. ❌ Do NOT place frontend docs in root
2. ❌ Do NOT place backend docs in root
3. ❌ Do NOT place app-specific docs in `/docs/`
4. ❌ Do NOT mix frontend/backend docs in same directory

### 🔄 Migration Rule

When creating new documentation:
1. Determine scope (frontend, backend, or cross-project)
2. Place in appropriate directory
3. Update any references in other docs

When finding misplaced docs:
1. Move to correct location
2. Update git index
3. Update references

---
- **STOP** - jangan push dulu

### 7. Push (Only After User Request)
- User bilang "push"
- `git push origin <branch>`

---

## 📋 Checklist Sebelum Lapor ke User

Setelah implementasi selesai:

- [ ] Code implemented
- [ ] Tests written and passing
- [ ] Manual verification done
- [ ] Test results documented
- [ ] Changes summarized
- [ ] **Files NOT committed yet** ← IMPORTANT
- [ ] **Files NOT pushed yet** ← IMPORTANT

**Report format**:
```
✅ Implementation complete

Files changed:
  • file1.go (new feature X)
  • file2.ts (integration Y)

Test results:
  • All tests passed
  • Coverage: X%

Verification:
  • Manual test passed
  • Expected behavior confirmed

📂 Files ready for commit (NOT committed yet)
```

---

## 🚫 What NOT to Do

1. ❌ Auto-commit setelah implementasi
2. ❌ Auto-push setelah commit
3. ❌ Commit hanya karena user bilang "continue"
4. ❌ Push tanpa user minta
5. ❌ Commit tanpa lapor dulu ke user

---

## ✅ What TO Do

1. ✅ Implement dan test
2. ✅ Lapor hasil ke user
3. ✅ Tunggu approval
4. ✅ Commit hanya setelah user minta
5. ✅ Push hanya setelah user minta

---

## 📝 Notes

**Alasan aturan ini**:
- User ingin review code sebelum commit
- User ingin test di browser dulu
- User ingin kontrol penuh atas git history
- User mungkin ingin squash atau amend commits
- User mungkin ingin edit commit message

**Exception**:
- TIDAK ADA EXCEPTION
- Selalu tunggu izin user untuk commit/push

---

## 🔄 Current Status

**Last violation**: 2026-08-23 10:25 WIB
- Auto-commit fix tanpa tunggu user approval
- Commit: `e74b1ae fix(setup): scan single disk and auto-create storage directory`

**User instruction**: Jangan lakukan lagi ke depannya

**Agent response**: Understood. Will wait for explicit permission.

---

**This rule is PERMANENT and applies to ALL future tasks.**
