# SITEMAP.md — CubyCloud Frontend Route Tree & Navigation Guide

## Document Control

| Parameter         | Detail                                                                      |
| ----------------- | --------------------------------------------------------------------------- |
| **Project**       | CubyCloud v1.0.0                                                            |
| **Audience**      | Frontend Developers (SvelteKit), UI/UX Designers, QA Engineers              |
| **Design System** | [`DESIGN.md`](./DESIGN.md) — Flat Earthy, Montserrat, Zero Arbitrary Values |
| **Product Spec**  | [`docs/prd.md`](../../docs/prd.md)                                          |

---

## Application-wide Health, Availability & Language Contract

- `GET /health` is an unauthenticated service endpoint, not a SvelteKit page. It returns HTTP `200` only when every currently configured storage node is accessible and HTTP `503` if any one is inaccessible; all registered nodes are probed. Its public response does not expose storage names, paths, or credentials.
- My Storage and Organization file-list items include the JSON boolean `storage_available` for files. Offline-backed files remain listed with their metadata; they are not treated as not found. Regular users must not receive physical node identifiers or names.
- The UI supports English and Indonesian. Browser language beginning with `id` defaults to Indonesian; all other browser languages default to English. A user's explicit choice is stored only in browser `localStorage`, keyed by authenticated account ID, so accounts in the same browser may use different languages. It is never persisted server-side. User-facing page and component copy, errors, notifications, and accessible names follow the active language; established product/technical terms and user-provided content remain unchanged where translation is inappropriate.
- The global language control uses a Globe icon and shadcn-svelte Select, displaying the short locale codes `id` and `en`.

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
├── (drive)/                         # Authenticated + main app chrome (sidebar, global search, upload)
│   ├── drive/
│   │   ├── home/
│   │   │   └── +page.svelte         # /drive/home (default landing)
│   │   ├── search/
│   │   │   └── +page.svelte         # /drive/search?q=...&page=...
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
│   │   ├── storage-stats/
│   │   │   └── +page.svelte         # /drive/storage-stats
│   │   └── profile/
│   │       └── [userId]/
│   │           ├── +page.svelte     # /drive/profile/:userId
│   │           └── +page.ts         # Authenticated profile loader
│   │
│   └── admin/                       # Super Admin only (sidebar bottom)
│       ├── storage/
│       │   └── +page.svelte         # /admin/storage
│       ├── settings/
│       │   └── +page.svelte         # /admin/settings
│       ├── users/
│       │   └── +page.svelte         # /admin/users
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

| Label Menu              | Ikon (Lucide) | Route Target           | Visibility                   |
| ----------------------- | ------------- | ---------------------- | ---------------------------- |
| **Beranda**             | `Home`        | `/drive/home`          | All authenticated            |
| **My Storage**          | `HardDrive`   | `/drive/my-storage`    | All authenticated            |
| **Organization**        | `Building2`   | `/drive/organization`  | All authenticated (badge 🏢) |
| **Dibagikan pada saya** | `Users`       | `/drive/shared`        | All authenticated            |
| **Berbintang**          | `Star`        | `/drive/starred`       | All authenticated            |
| **Trash**               | `Trash2`      | `/drive/trash`         | All authenticated            |
| **Penyimpanan**         | `PieChart`    | `/drive/storage-stats` | All authenticated            |
| _(Quota Widget)_        | —             | —                      | All authenticated            |
| _(divider)_             | —             | —                      | Super Admin only             |
| **Penyimpanan**         | `HardDrive`   | `/admin/storage`       | Super Admin only             |
| **Admin Settings**      | `Settings`    | `/admin/settings`      | Super Admin only             |
| **Users & Quotas**      | `UserCog`     | `/admin/users`         | Super Admin only             |

---

## 3. Detailed Route & Page Specification

### A. Onboarding Module

#### `/setup`

- **Path:** `/setup`
- **Access:** Unauthenticated, first-boot only (`is_setup_completed === false`)
- **Post-complete redirect:** `/drive/home`
- **Embedded modals:**
  - _Recovery Key Backup Modal_ — show the current key in six groups of five uppercase alphanumeric characters (`XXXXX-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX`); offer Copy/Download and require acknowledgement before continuing
  - _Device Scan Error Modal_ — jika mount volume/device gagal dipindai
- **Key components & actions:**
  - **Step 1:** Super Admin form (username, email, password; validasi **minimal 8 karakter**)
  - **Step 2:** Scanner disk lokal (`/app/data/storage/*`) + **node capacity cap**; opsional device Linux
  - **Step 3:** Placement strategy (`FILL_FIRST` / `ROUND_ROBIN` / `MOST_FREE_SPACE`)
  - **Step 4 (optional):** Kredensial Cloudflare R2 / AWS S3 / S3 Compatible (endpoint custom)
  - **Step 5:** Generate and export the Recovery Key (30 characters, six groups of five)
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
  - Input current 6×5 Recovery Key + new password
  - UI accepts the current key format only; legacy 64-character hexadecimal keys are not supported by the UI
  - On successful reset, show and require saving the replacement key returned as `new_recovery_key`; the supplied key is single-use
  - CTA `Reset Password Super Admin`

---

### C. Drive & Workspace Module (`(drive)` layout)

**Global authenticated header:** Pencarian nama file/folder menampilkan pratinjau dan membuka hasil penuh di `/drive/search`. Tombol `Unggah` selalu ada; tujuannya akar My Storage kecuali pengguna sedang membuka folder Shared With Me dengan izin efektif `Editor`, saat file masuk ke folder itu. Daftar Shared atau izin `Viewer` menampilkan penolakan tanpa fallback ke My Storage.

#### `/drive/home` (Beranda — default landing)

- **Path:** `/drive/home`
- **Access:** Authenticated
- **Embedded modals / drawers:**
  - _Quick Upload Drawer_ — unggah ke My Storage (enforce user quota)
  - _File Preview Lightbox_ (`?preview=file_id`)
- **Key components & actions:**
  - Quick Access cards (5 berkas/folder terakhir dibuka oleh akun aktif, urut terbaru ke terlama, tersimpan per akun)
  - Recent activities list (5 berkas/folder berbeda dengan aksi upload atau ubah terbaru oleh akun aktif, termasuk rename dan pindah folder di My Storage dan Organization; deduplikasi per berkas/folder)
  - Storage summary widget (same data as sidebar quota)

#### `/drive/search` (Pencarian Berkas)

- **Path:** `/drive/search`
- **URL params:** `?q=query&page=number`
- **Access:** Authenticated
- **Key components & actions:**
  - Halaman hasil penuh dengan paginasi; header menyediakan pratinjau hasil dan tautan ke halaman ini
  - Cari nama file/folder yang dapat diakses di My Storage milik pengguna, Organization, dan subtree Shared yang diberikan kepadanya; file di Trash dan isi file tidak dicari
  - Tampilkan sumber, lokasi, tipe, ukuran, dan waktu perubahan; lokasi Shared tidak mengungkap ancestor privat di atas akar share
  - Memilih folder membuka folder pada sumbernya; memilih file membuka pratinjau pada folder induknya

#### `/drive/my-storage` (My Storage)

- **Path:** `/drive/my-storage`
- **URL params:** `?folder=folder_id&preview=file_id`
- **Access:** Authenticated
- **Embedded modals / drawers:**
  - _File Preview Lightbox_ (`?preview=file_id`) — gambar native, video Vidstack, teks/kode PrismJS, PDF.js, dan audio yang sudah ada
  - _Name Conflict Dialog_ — upload/duplicate use the next available numeric suffix automatically; rename/move offer that name or cancel, without overwriting or merging folders
  - _Folder Delete Confirmation_ — confirm that deleting a folder also sends all of its contents to Trash
  - _Upload Queue Notification_ — floating top-right, below the top navigation; progress is weighted by file size and reaches 100% only after server success; expandable queued/uploading items remain until closed or swiped right, then a regular completion toast appears at bottom-right
  - _Share Sheet_ — existing product behavior; unchanged by this file-management scope
  - _New Folder Modal_
  - _Quota Exceeded Toast / Modal_ — `USER_QUOTA_EXCEEDED`
- **Key components & actions:**
  - Category filter chips
  - File explorer list view — Nama, ukuran, tanggal, aksi; file items include `storage_available`
    - Jika `storage_available` bernilai `false`, item dan metadata tetap terlihat. Nonaktifkan aksi berkas yang memerlukan storage tersebut dan tampilkan shadcn-svelte Popover saat hover atau klik dengan pesan bahasa UI yang menyatakan node offline tanpa mengungkap identitas node. Folder tetap dapat dijelajahi dan tidak dinonaktifkan hanya karena node tidak tersedia.
    - Storage Node Badge hanya untuk Super Admin; regular users tidak menerima identifier atau nama physical node.
  - Upload file/folder via picker or drag-and-drop — process each file independently, preserve folder hierarchy, show failed items, and check user quota
  - Download file/folder; Rename; Move; recursive Duplicate (zero-byte references); Delete to Trash recursively
  - Preview gambar, video, teks/kode, PDF, dan audio menggunakan akses file sesuai izin; star dan share behavior tetap berlaku

#### `/drive/organization` (Organization Drive)

- **Path:** `/drive/organization`
- **URL params:** `?folder=folder_id&preview=file_id`
- **Access:** All registered users — Full Access (Read, Write, Download, Delete)
- **Model v1.0:** Pohon Organization **fixed** (bukan folder My Storage yang di-"mark" admin); semua user CRUD di route ini
- **Quota:** **Tidak** dihitung ke user quota; tetap cek kapasitas node fisik
- **Embedded modals / drawers:**
  - _File Preview Lightbox_ — format dan perilaku sama dengan My Storage
  - _Conflict Resolution Modal_
- **Key components & actions:**
  - File explorer list view (komponen sama; file items include `storage_available`; node badge admin-only; aksi Bagikan disembunyikan karena sudah dapat diakses bersama secara internal)
    - File offline tetap menampilkan metadata; nonaktifkan aksi yang memerlukan storage, lalu tampilkan shadcn-svelte Popover pada hover atau klik dengan pesan lokal bahwa node offline tanpa menampilkan identitas node. Folder tetap navigable.

#### `/drive/shared` (Dibagikan pada saya)

- **Path:** `/drive/shared`
- **URL params:** `?folder=folder_id&preview=file_id`
- **Access:** Authenticated
- **Embedded modals / drawers:**
  - _File Preview Lightbox_ — format dan perilaku sama dengan My Storage, termasuk untuk penerima `Viewer`
  - _Share Details Modal_ — pemilik + `Viewer` / `Editor`
  - _Share Sheet_ — hanya jika izin `Editor`
- **Key components & actions:**
  - Shared files table
  - Permission badge — `Viewer` (`secondary` / Ochre), `Editor` (`primary` / Moss)
  - Actions: Download, Preview; Edit/Delete/**Bagikan** jika `Editor`
  - Upload dari header dalam folder bersama hanya tersedia untuk izin efektif `Editor`; Viewer dan daftar akar menampilkan dialog penolakan tanpa fallback
  - Quota: owner pays (tidak menambah usage penerima share)

#### `/drive/starred` (Berbintang)

- **Path:** `/drive/starred`
- **URL params:** `?preview=file_id`
- **Access:** Authenticated
- **Embedded modals / drawers:**
  - _File Preview Lightbox_ (`?preview=file_id`)
  - _Share Sheet_
- **Key components & actions:**
  - Starred files table (dari `my-storage`, `organization`, `shared`)
  - Double-click berkas membuka _File Preview Lightbox_, menu aksi menyediakan "Lihat Pratinjau"
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

#### `/drive/profile/[userId]` (Profil Akun)

- **Path:** `/drive/profile/:userId`
- **Access:** Authenticated; pemilik dapat mengedit profilnya sendiri. Super Admin dapat membaca profil lain secara read-only; pengguna reguler tidak dapat membaca atau mengubah profil pengguna lain.
- **Key components & actions:**
  - Tampilkan nama tampilan, email, username, role, kuota efektif, dan penggunaan aktual; username, role, serta kuota tidak dapat diedit dari halaman profil
  - Edit nama tampilan dan email; perubahan email memerlukan kata sandi saat ini dan menolak format tidak valid atau duplikat tanpa membedakan kapitalisasi
  - Ubah kata sandi dengan kata sandi saat ini, validasi panjang minimum, dan konfirmasi; keberhasilan mencabut semua sesi dan meminta login kembali

---

### D. Super Admin Module

#### `/admin/storage` (Penyimpanan Sistem)

- **Path:** `/admin/storage`
- **Access:** Super Admin only
- **Key components & actions:**
  - **Detail Kuota & Pemakaian Sistem:** Ringkasan kapasitas akumulasi seluruh storage node yang terdaftar (`SUM capacity_bytes`) dan total pemakaian berkas pengguna (`SUM size_bytes` dedup `disk_key`).
    - Progress bar visual persentase pemakaian sistem (`(total_used / total_capacity) * 100%`).
    - Ringkasan statistik: Total Kapasitas Node (jumlah node), Total Pemakaian Berkas, Sisa Kapasitas Bebas.
  - **Pengguna dengan Konsumsi Terbesar:** Ranking pengguna dengan konsumsi penyimpanan terbesar ke terkecil:
    - Informasi pengguna: avatar, username, email, role badge (`Super Admin` / `User`), badge kuota (`Unlimited` / `GB`).
    - Ukuran pemakaian storage (formatted byte & GB) dan persentase terhadap total pemakaian berkas sistem.
    - Infinite scroll / pagination (`limit=20`, `offset`, `has_more`, `next_offset`) menggunakan Intersection Observer.
    - Shortcut CTA menuju `/admin/users` untuk mengelola kuota pengguna.
    - State: loading skeleton, empty state ("Belum ada data pemakaian pengguna"), dan error toast.
  - Action Segarkan: Memperbarui data statistik penyimpanan secara instan.

#### `/admin/settings` (Storage Nodes & System Settings)

- **Path:** `/admin/settings`
- **Access:** Super Admin only
- **Embedded modals:**
  - _Detach Node Modal_
  - _Rescan Nodes Modal_
- **Key components & actions:**
  - Storage nodes grid menampilkan kapasitas, pemakaian fisik per node, free space, dan badge 5% critical; pemakaian tidak berasal dari host filesystem.
  - Nama node dapat diedit; nama harus terisi dan unik tanpa membedakan kapitalisasi.
  - **Default User Quota Policy** — input GB atau toggle Unlimited for new users
  - **Audit Log Retention** — dropdown 7 / 14 / 30 / 60 / 90 hari (default: **90**)
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

> Catatan: Modul Add-ons Manager (`/admin/addons`) telah ditiadakan sepenuhnya. Fitur **Download as ZIP** dan pengarsipan berkas kini menjadi fitur core file manager yang selalu aktif dan terpasang secara bawaan tanpa memerlukan modul add-on.

---

### E. Public Module

#### `/s/[token]` (Share Link & Previewer)

- **Path:** `/s/[token]`
- **URL params:** `?preview=file_id`
- **Access:** Public atau Restricted guest
- **Embedded modals:**
  - _Password Challenge_ (full-page card)
  - _Restricted login prompt_ → `/login?redirect=/s/{token}`
  - _File Preview Lightbox_ — format yang sama, termasuk PDF tanpa add-on; akses tetap tunduk pada password/restricted link
- **Key components & actions:**
  - Expired / Not Found view (copy berbeda)
  - Guest file explorer (folder) / single-file card
  - Actions: Direct Download, Stream Preview, Download as ZIP (fitur core terpasang)
- **Demo tokens (UI mock):** `demo-public`, `demo-folder`, `demo-password` (sandi `demo123`), `demo-restricted`, `demo-expired`

---

## 4. URL-Driven Preview Specification (`?preview=file_id`)

Lightbox previewer bersifat **URL-driven** via query parameter agar tautan ke satu berkas bisa dibagikan.

### Mekanisme SvelteKit

1. **Trigger lightbox:** Double-click berkas memanggil  
   `goto('?preview=file_id', { keepFocus: true, noScroll: true })`.
2. **Direct shared file link:**
   - Internal: `https://domain.com/drive/my-storage?preview=abc-123`
   - Publik: `https://domain.com/s/token?preview=abc-123`
3. **Formats:** Gambar native (PNG, JPEG, WebP, GIF, SVG), Video MP4/WebM (Vidstack dengan media native), Text/Code/Markdown (teks biasa, syntax highlighting PrismJS untuk kode), Audio MP3/WAV/OGG (perilaku yang sudah ada), dan PDF (PDF.js melalui `pdfjs-dist`, selalu tersedia tanpa add-on Admin). Berlaku pada My Storage, Organization, Shared, dan `/s/[token]` sesuai izin masing-masing.
4. **Sumber berkas:** Gunakan endpoint unduh yang sudah ada dengan opt-in `?preview=1` untuk respons inline dan MIME preview yang diizinkan; gunakan HTTP Range bila endpoint/storage mendukung. Tanpa `preview=1`, unduhan tetap attachment. Parameter `?preview=file_id` pada URL halaman tetap hanya mengendalikan lightbox; akses file tetap mengikuti otorisasi route dan password share link.

### Design notes (DESIGN.md)

- Overlay: `bg-black/60 backdrop-blur-none`
- Container: `rounded-xl bg-surface border border-subtle shadow-none`

---

## 5. Quota Widget & Enforcement (UI Contract)

### Sidebar widget (all authenticated users)

| State       | Display                                                      |
| ----------- | ------------------------------------------------------------ |
| Limited     | Label `My Storage` + `8.2 / 15 GB` + flat progress bar       |
| ≥ 80%       | Bar `bg-secondary` (Ochre) + toast warning on write attempts |
| 100%        | Block My Storage upload; toast `USER_QUOTA_EXCEEDED`         |
| Unlimited   | `Used: 8.2 GB` only (no bar cap / no limit number)           |
| Super Admin | Always Unlimited display                                     |

### Error copy (must differ)

| Code / case           | User-facing message                    |
| --------------------- | -------------------------------------- |
| `USER_QUOTA_EXCEEDED` | Kuota My Storage penuh. Hubungi admin. |
| Physical pool full    | Penyimpanan server penuh.              |

### Visibility rules

| UI element                       | Regular User  | Super Admin   |
| -------------------------------- | ------------- | ------------- |
| Storage Node Badge in explorer   | Hidden        | Visible       |
| Node grid in `/admin/settings`   | —             | Visible       |
| Organization files in user quota | Never counted | Never counted |
