# SITEMAP.md — CubyCloud Frontend Route Tree & Navigation Guide

## Document Control

| Parameter | Detail |
| --------- | ------ |
| **Project** | CubyCloud v1.0.0 |
| **Audience** | Frontend Developers (SvelteKit), UI/UX Designers, QA Engineers |
| **Design System** | [`DESIGN.md`](./DESIGN.md) — Flat Earthy, Montserrat, Zero Arbitrary Values |
| **Product Spec** | [`docs/prd.md`](../../docs/prd.md) |

---

## 1. Route Tree Overview (`src/routes/`)

CubyCloud memakai **SvelteKit Route Groups** (folder dalam tanda kurung) untuk mengisolasi layout dan route guard tanpa mengubah URL di browser.

```text
src/routes/
├── +layout.svelte                   # Global provider (theme, toast, auth store)
├── +page.svelte                     # / (pre-launch coming soon; post-launch redirect)
│
├── (onboarding)/                    # First-boot guard
│   └── setup/
│       └── +page.svelte             # /setup
│
├── (auth)/                          # Auth surface (PRD: /login + /recovery)
│   ├── login/
│   │   └── +page.svelte             # /login
│   └── recovery/
│       └── +page.svelte             # /recovery
│
├── (drive)/                         # Authenticated + main app chrome (sidebar, header, search)
│   ├── drive/
│   │   ├── home/
│   │   │   └── +page.svelte         # /drive/home (default landing)
│   │   ├── my-storage/
│   │   │   └── +page.svelte         # /drive/my-storage
│   │   ├── organization/
│   │   │   └── +page.svelte         # /drive/organization
│   │   ├── shared/
│   │   │   └── +page.svelte         # /drive/shared
│   │   ├── starred/
│   │   │   └── +page.svelte         # /drive/starred
│   │   ├── trash/
│   │   │   └── +page.svelte         # /drive/trash
│   │   └── storage-stats/
│   │       └── +page.svelte         # /drive/storage-stats
│   │
│   └── admin/                       # Super Admin only (sidebar bottom)
│       ├── settings/
│       │   └── +page.svelte         # /admin/settings
│       ├── users/
│       │   └── +page.svelte         # /admin/users
│       └── addons/
│           └── +page.svelte         # /admin/addons
│
└── (public)/                        # Share link viewer
    └── s/
        └── [token]/
            └── +page.svelte         # /s/:token
```

---

## 2. Sidebar Navigation Structure

Sidebar di layout `(drive)`. Blok Super Admin di bagian bawah, di-_render_ hanya untuk Super Admin.

Di **bawah daftar menu drive** (di atas divider admin), tampilkan **Quota Widget** ringkas untuk semua authenticated user (lihat §5).

| Label Menu | Ikon (Lucide) | Route Target | Visibility |
| ---------- | ------------- | ------------ | ---------- |
| **Beranda** | `Home` | `/drive/home` | All authenticated |
| **My Storage** | `HardDrive` | `/drive/my-storage` | All authenticated |
| **Organization** | `Building2` | `/drive/organization` | All authenticated (badge 🏢) |
| **Dibagikan pada saya** | `Users` | `/drive/shared` | All authenticated |
| **Berbintang** | `Star` | `/drive/starred` | All authenticated |
| **Trash** | `Trash2` | `/drive/trash` | All authenticated |
| **Penyimpanan** | `PieChart` | `/drive/storage-stats` | All authenticated |
| _(Quota Widget)_ | — | — | All authenticated |
| _(divider)_ | — | — | Super Admin only |
| **Admin Settings** | `Settings` | `/admin/settings` | Super Admin only |
| **Users & Quotas** | `UserCog` | `/admin/users` | Super Admin only |
| **Add-ons Manager** | `Blocks` | `/admin/addons` | Super Admin only |

---

## 3. Detailed Route & Page Specification

### A. Onboarding Module

#### `/setup`

- **Path:** `/setup`
- **Access:** Unauthenticated, first-boot only (`is_setup_completed === false`)
- **Post-complete redirect:** `/drive/home`
- **Embedded modals:**
  - _Emergency Key Backup Modal_ — wajib Copy/Download Hex Key 64-karakter sebelum selesai
  - _Device Scan Error Modal_ — jika mount volume/device gagal dipindai
- **Key components & actions:**
  - **Step 1:** Super Admin form (username, email, password; validasi **minimal 8 karakter**)
  - **Step 2:** Scanner disk lokal (`/app/data/storage/*`) + **node capacity cap**; opsional device Linux
  - **Step 3:** Placement strategy (`FILL_FIRST` / `ROUND_ROBIN` / `MOST_FREE_SPACE`)
  - **Step 4 (optional):** Kredensial AWS S3 / Cloudflare R2
  - **Step 5:** Export Emergency Recovery Key
  - **CTA:** `Selesaikan Setup & Masuk ke CubyCloud`

---

### B. Authentication Module

> PRD menyebut permukaan auth. Frontend memecahnya menjadi `/login` dan `/recovery` (bukan satu path `/auth`).

#### `/login`

- **Path:** `/login`
- **Access:** Unauthenticated (redirect ke `/drive/home` jika sesi JWT aktif)
- **Embedded modals / feedback:**
  - _Forgot Password Info Toast_ — pemulihan sandi memakai Emergency Recovery Key (tanpa SMTP di v1.0)
- **Key components & actions:**
  - Form username/email + password
  - CTA `Masuk` — JWT ke HTTP-Only Cookie
  - Link `Pulihkan Akses Admin` → `/recovery`

#### `/recovery`

- **Path:** `/recovery`
- **Access:** Unauthenticated (pemulihan sandi darurat Super Admin)
- **Embedded modals:**
  - _Recovery Success Modal_ — sukses lalu minta login ulang
- **Key components & actions:**
  - Input 64-char Hex Recovery Key + password baru
  - CTA `Reset Password Super Admin`

---

### C. Drive & Workspace Module (`(drive)` layout)

#### `/drive/home` (Beranda — default landing)

- **Path:** `/drive/home`
- **Access:** Authenticated
- **Embedded modals / drawers:**
  - _Quick Upload Drawer_ — unggah ke My Storage (enforce user quota)
  - _File Preview Lightbox_ (`?preview=file_id`)
- **Key components & actions:**
  - Quick Access cards (4–6 berkas terakhir / sering dibuka)
  - Recent activities list (upload, edit, share)
  - Storage summary widget (same data as sidebar quota)

#### `/drive/my-storage` (My Storage)

- **Path:** `/drive/my-storage`
- **URL params:** `?folder=folder_id&preview=file_id`
- **Access:** Authenticated
- **Embedded modals / drawers:**
  - _File Preview Lightbox_ (`?preview=file_id`) — core formats; PDF jika add-on aktif
  - _Conflict Resolution Modal_ — Replace / Keep Both
  - _Share Link Drawer_ — user-to-user, Public/Restricted, password, expiration
  - _New Folder Modal_
  - _Quota Exceeded Toast / Modal_ — `USER_QUOTA_EXCEEDED`
- **Key components & actions:**
  - Category filter chips
  - File explorer list view — Nama, ukuran, tanggal, aksi
  - **Storage Node Badge:** tampil **hanya** jika role = Super Admin
  - Drag-and-drop — TUS + presigned cloud; cek user quota sebelum upload
  - Actions: Download, Rename, Move, Zero-Byte Copy, Star/Unstar, Delete to Trash

#### `/drive/organization` (Organization Drive)

- **Path:** `/drive/organization`
- **URL params:** `?folder=folder_id&preview=file_id`
- **Access:** All registered users — Full Access (Read, Write, Download, Delete)
- **Quota:** **Tidak** dihitung ke user quota; tetap cek kapasitas node fisik
- **Embedded modals / drawers:**
  - _File Preview Lightbox_
  - _Conflict Resolution Modal_
  - _Share Link Drawer_ (internal / tim)
- **Key components & actions:**
  - Header banner + badge `[🏢 Organization]`
  - File explorer list view (komponen sama; node badge admin-only)

#### `/drive/shared` (Dibagikan pada saya)

- **Path:** `/drive/shared`
- **URL params:** `?preview=file_id`
- **Access:** Authenticated
- **Embedded modals / drawers:**
  - _File Preview Lightbox_
  - _Share Details Modal_ — pemilik + `Viewer` / `Editor`
- **Key components & actions:**
  - Shared files table
  - Permission badge — `Viewer` (`secondary` / Ochre), `Editor` (`primary` / Moss)
  - Actions: Download, Preview; Edit/Delete jika `Editor`
  - Quota: owner pays (tidak menambah usage penerima share)

#### `/drive/starred` (Berbintang)

- **Path:** `/drive/starred`
- **URL params:** `?preview=file_id`
- **Access:** Authenticated
- **Embedded modals / drawers:**
  - _File Preview Lightbox_
  - _Share Link Drawer_
- **Key components & actions:**
  - Starred files table (dari `my-storage`, `organization`, `shared`)
  - CTA `Hapus Bintang` (Unstar)

#### `/drive/trash` (Trash Bin)

- **Path:** `/drive/trash`
- **Access:** Authenticated
- **Quota note:** Item di Trash **tetap** dihitung ke user quota sampai purge
- **Embedded modals:**
  - _Missing Parent Folder Modal_ — restore ke `ROOT`
  - _Purge Confirmation Modal_ — aksen `danger` (Rose 600)
- **Key components & actions:**
  - Trashed items + sisa hari sebelum auto-purge 30 hari
  - Actions: `Restore`, `Hapus Permanen`
  - Top CTA danger: `Kosongkan Sampah (Purge Trash)`

#### `/drive/storage-stats` (Penyimpanan / Stats)

- **Path:** `/drive/storage-stats`
- **Access:** Authenticated
- **Embedded modals:**
  - _Cleanup Advisor Modal_
- **Key components & actions:**
  - **Quota detail panel** — `Used / Limit GB` atau `Used: X GB` (Unlimited)
  - Usage chart per kategori (Video, Gambar, Dokumen, Arsip, Trash)
  - Largest files list (top 10; exclude Organization)
  - Shortcut `Kelola Sampah` → `/drive/trash`

---

### D. Super Admin Module

#### `/admin/settings` (Storage Nodes & System Settings)

- **Path:** `/admin/settings`
- **Access:** Super Admin only
- **Embedded modals:**
  - _Detach Node Modal_
  - _Rescan Nodes Modal_
- **Key components & actions:**
  - Storage nodes grid (kapasitas, free space, badge 5% critical) — node badges selalu visible di sini
  - **Default User Quota Policy** — input GB atau toggle Unlimited for new users
  - Worker concurrency (default: **2**)
  - Database backup CTA: `Download DB Backup (.db)`

#### `/admin/users` (Users & Quotas)

- **Path:** `/admin/users`
- **Access:** Super Admin only
- **Embedded modals:**
  - _Create User Modal_
  - _Edit Quota Modal_ — set GB, Unlimited, atau Reset to Default
- **Key components & actions:**
  - User table: name, email, role, effective quota, used GB, actions
  - Super Admin rows: always Unlimited (not editable as limited)
  - Create Regular User (inherits default quota unless overridden)

#### `/admin/addons` (Add-ons Manager)

- **Path:** `/admin/addons`
- **Access:** Super Admin only
- **Embedded modals / drawers:**
  - _Worker Queue Monitor Drawer_ — `PENDING` → `PROCESSING` → `COMPLETED` / `FAILED`
- **Key components & actions:**
  - Add-on cards (toggle ON/OFF):
    1. Image Converter on Download — default **ON**
    2. Compress to ZIP — default **ON**
    3. Download as ZIP — default **ON**
    4. PDF Previewer — default **OFF**
  - CTA: `Lihat Antrean Job (SQLite Queue)`

---

### E. Public Module

#### `/s/[token]` (Share Link & Previewer)

- **Path:** `/s/[token]`
- **URL params:** `?preview=file_id`
- **Access:** Public atau Restricted guest
- **Embedded modals:**
  - _Password Challenge Modal_
  - _File Preview Lightbox_
- **Key components & actions:**
  - Expired / Not Found view
  - Restricted login prompt
  - Guest file explorer / single-file preview
  - Actions: Direct Download, Stream Preview, Download as ZIP (jika add-on aktif)

---

## 4. URL-Driven Preview Specification (`?preview=file_id`)

Lightbox previewer bersifat **URL-driven** via query parameter agar tautan ke satu berkas bisa dibagikan.

### Mekanisme SvelteKit

1. **Trigger lightbox:** Double-click berkas memanggil  
   `goto('?preview=file_id', { keepFocus: true, noScroll: true })`.
2. **Direct shared file link:**  
   - Internal: `https://domain.com/drive/my-storage?preview=abc-123`  
   - Publik: `https://domain.com/s/token?preview=abc-123`
3. **Formats:** Text/Code, Gambar, Video, Audio (core); PDF jika add-on aktif.

### Design notes (DESIGN.md)

- Overlay: `bg-black/60 backdrop-blur-none`
- Container: `rounded-xl bg-surface border border-subtle shadow-lg`

---

## 5. Quota Widget & Enforcement (UI Contract)

### Sidebar widget (all authenticated users)

| State | Display |
| ----- | ------- |
| Limited | Label `My Storage` + `8.2 / 15 GB` + flat progress bar |
| ≥ 80% | Bar `bg-secondary` (Ochre) + toast warning on write attempts |
| 100% | Block My Storage upload; toast `USER_QUOTA_EXCEEDED` |
| Unlimited | `Used: 8.2 GB` only (no bar cap / no limit number) |
| Super Admin | Always Unlimited display |

### Error copy (must differ)

| Code / case | User-facing message |
| ----------- | ------------------- |
| `USER_QUOTA_EXCEEDED` | Kuota My Storage penuh. Hubungi admin. |
| Physical pool full | Penyimpanan server penuh. |

### Visibility rules

| UI element | Regular User | Super Admin |
| ---------- | ------------ | ----------- |
| Storage Node Badge in explorer | Hidden | Visible |
| Node grid in `/admin/settings` | — | Visible |
| Organization files in user quota | Never counted | Never counted |
