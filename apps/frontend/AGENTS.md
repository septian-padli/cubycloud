# CubyCloud — Frontend AI Agent Guidance (SvelteKit & UI)

Patuhi kontrak global di `AGENTS.md` (root) dan dokumen produk berikut **sebelum** menulis kode UI:

- `docs/prd.md` — aturan fitur & quota
- `apps/frontend/DESIGN.md` — design system (wajib)
- `apps/frontend/SITEMAP.md` — route tree & kontrak halaman (wajib)

---

## 1. Tech Stack & Standards

- **Framework:** SvelteKit SPA (`adapter-static`, fallback `index.html` untuk `go:embed`)
- **Runtime:** Svelte 5 **Runes** (`$state`, `$derived`, `$effect`, `$props`). Hindari `writable`/`readable` stores untuk state lokal baru jika bisa diganti Runes.
- **Language:** TypeScript (`typescript-svelte-plugin` di `tsconfig.json` agar IDE resolve `.svelte`)
- **Styling:** Tailwind CSS v4 — token via `@theme inline` di `src/routes/layout.css`
- **Typography:** Montserrat (400/500/600/700)
- **Components:** **shadcn-svelte** (bits-ui) — tambah ke `$lib/components/ui/` via `bunx shadcn-svelte@latest add <name>`
- **Icons:** `@lucide/svelte`, stroke `stroke-2` (bukan arbitrary)
- **Package manager:** Bun (`bun install`, `bun run dev|build|check|lint`)

---

## 2. Design System (Flat Earthy)

- Skema **flat & earthy** (Parchment Cream, Olive Moss, Warm Ochre) — bukan Neo-Notion / hard shadow
- **STRICT zero arbitrary values:** dilarang `p-[14px]`, `w-[320px]`, `text-[#123456]`, `stroke-[1.75]`
- Gunakan token CubyCloud: `bg-app`, `bg-surface`, `text-main`, `text-muted`, `border-subtle`, `bg-primary`, `bg-secondary`, `bg-danger`, dll.
- **Shadow:** **zero shadow** di seluruh UI (`shadow-none`). Overlay/modal/sheet/dropdown cukup `border border-subtle` + surface layering — jangan `shadow-xs` / `shadow-lg`
- **Rounded:** `rounded-full` untuk button, input, switch; `rounded-xl` untuk card/panel/modal; sidebar nav item `rounded-full`
- **Card padding:** Root `py-(--card-spacing)` + slot `px-(--card-spacing)`; token default `--spacing(4)`. List shell edge-to-edge: Root `py-0` + Content `p-0`. Detail: DESIGN §4.3
- **Hover:**
  - Sidebar inactive: `hover:bg-primary/10`; aktif solid **tanpa** hover ekstra
  - Grid card: `bg-surface` → hover/selected `bg-surface/80` (bukan `bg-primary/5`)
  - List row explorer: `hover:bg-black/5` / `dark:hover:bg-white/5`
- Aksesibilitas: kontras WCAG AA, outline fokus 2px, transisi 100–150ms

### Button variants (CubyCloud)

| Variant | Surface | Hover | Border | Contoh |
| ------- | ------- | ----- | ------ | ------ |
| `default` | `bg-primary` + teks putih | `primary/80` | — | Unggah, Simpan, CTA |
| `secondary` | `bg-surface` | `bg-app` | `border-subtle` | Kembali, Pindai ulang, Unduh backup, Kelola sampah |
| `outline` | `bg-app` | `bg-surface` | `border-subtle` | Aksi sekunder di atas `bg-app` |
| `destructive` | `bg-danger` + `text-white` | `danger/90` | — | Hapus permanen, Kosongkan sampah |
| `ghost` | — | konteks | — | Icon ⋮ (lihat di bawah) |

- Base: `rounded-full` — `$lib/components/ui/button/button.svelte`
- **Size standar di `/drive` & `/admin`:** teks = `default` (`h-9`); icon-only = `icon` (`size-9`). Jangan default ke `sm` / `icon-sm` kecuali ada alasan kuat
- Padding horizontal CubyCloud lebih lebar dari shadcn Vega bawaan (lihat DESIGN §5.3)
- Actions ⋮: `variant="ghost" size="icon"` + `bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white`

### Form controls

- **Input:** `rounded-full`, `bg-surface` + `border-subtle`, flat (`shadow-none`)
- **Password fields:** pakai `$lib/components/password-input.svelte` (show/hide) — jangan duplikasi eye toggle
- **Switch:** track nonaktif `bg-app`, aktif `data-[state=checked]:bg-surface`; border `border-primary`; thumb `bg-primary`. Bits-ui: `data-[state=checked]:…` (bukan `data-checked:`); flat (`shadow-none`)
- **Radio / checkbox:** sama — `data-[state=…]`, `shadow-none`

---

## 3. Theme (light / dark)

- State global: `$lib/theme.svelte.ts` (key `localStorage`: `cubycloud-theme`)
- Mode: `light` | `dark` | `system`; class `.dark` pada `<html>`
- Anti-FOUC: script di `src/app.html` sebelum render
- Toggle UI: `$lib/components/theme-toggle.svelte` — reuse di header nav drive, onboarding, error page, dll.
- Init theme di root `+layout.svelte` (`theme.init()` on mount)

---

## 4. Routing & UX (SITEMAP)

- Ikuti route tree di `apps/frontend/SITEMAP.md` — jangan invent route baru tanpa update SITEMAP
- **Lightbox previewer:** URL-driven via query `?preview=file_id`
- **Post-setup / login redirect:** `/drive/home`
- **Error UI:** root `+error.svelte` → `$lib/components/error/error-page.svelte` + `$lib/error-pages.ts`; ilustrasi **wajib** di `$lib/assets/illustration/` (bukan `apps/frontend/assets/`)
- **Popover:** shadcn `$lib/components/ui/popover/` — dipakai stacked bar kategori di profil; overlay `shadow-none` + `border-subtle`
- Sediakan loading state halus untuk perangkat lambat

### Auth (`(auth)` layout)

- Route: `/login`, `/recovery`
- Komponen: `$lib/components/auth/` (`login-form`, `recovery-form`, `auth-split-shell`)
- Mock belum ada API — form submit UI-only; sukses login memanggil `setMockLoggedIn(true)` (`$lib/mocks/auth-session.ts`) lalu redirect `/drive/home` atau `?redirect=`

### Onboarding (`/setup`)

- Route group: `src/routes/(onboarding)/setup/`
- Komponen: `$lib/components/setup/` (wizard + steps); state tunggal di `setup-wizard.svelte`
- Mock: `$lib/mocks/setup.ts` + types `$lib/types/setup.ts`
- 5 langkah (PRD §4.1 / SITEMAP §A): Admin → Storage → Strategy → Cloud opsional → Recovery Key
- Bahasa UI setup: **Indonesia**

### Drive shell (`(drive)` layout)

Layout sekali di `(drive)/+layout.svelte` → `DriveShell`. Struktur:

```text
DriveShell
├── DriveHeaderNav     # full-width, py-4; search kiri; Unggah (variant default) + theme kanan
└── row
    ├── DriveSidebar   # desktop; Sheet drawer mobile; Super Admin block + hrefs
    └── DriveMain
        ├── (fixed) header nav sudah di shell
        └── scroll area → {@render children()} dari +page
            ├── DrivePageHeader?  # opsional per halaman (+ snippet actions)
            └── konten page
```

Komponen kunci (`$lib/components/drive/`):

| Komponen | Peran |
| -------- | ----- |
| `drive-shell.svelte` | Sidebar + main; mobile sheet |
| `drive-header-nav.svelte` | Search, Unggah (`default`), ThemeToggle |
| `drive-sidebar-nav.svelte` | Nav pill + admin links (`/admin/settings\|users\|addons`) |
| `drive-page-header.svelte` | Title/description + optional `actions` snippet |
| `drive-view-toggle.svelte` | List/grid toggle (state global) |
| `drive-file-grid.svelte` / `file-grid-card.svelte` | Grid view berkas |
| `drive-list-row.svelte` | Baris list bersama |
| `explorer/*` | My Storage / Organization explorer + context/actions menu |
| `explorer/share-sheet.svelte` | Drawer Bagikan: tab Pengguna (cari + daftar akses) + tab Tautan |
| `user-profile-view.svelte` | Halaman profil: data akun + kuota + stacked kategori |
| `shared-*` / `starred-*` / `trash-*` / `storage-*` | Halaman shared, starred, trash, storage-stats |

Context & state:

- `setDriveChrome()` / `getDriveChrome()` — `$lib/drive-context.svelte.ts` (user, quota, `updateProfile`, role toggle, upload handler)
- `deriveDriveActiveId()` — `$lib/drive-nav.ts` (pathname → sidebar `activeId`; `/s/*` tidak menandai menu)
- `driveView` — `$lib/drive-view.svelte.ts` (list/grid; init di `(drive)/+layout`)
- Fixture: `$lib/mocks/drive.ts`, types `$lib/types/drive.ts`

Sidebar bawah: blok nama user adalah tautan ke `/drive/profile/{user.id}`; hover `hover:bg-primary/10` + `rounded-full` (sama seperti nav pill).

Scroll: sidebar + header nav **fixed**; page header + content **scroll bersama** di `DriveMain`.

### Profil (`/drive/profile/[userId]`)

- Route unified: sidebar → profil sendiri; admin Users → **Lihat** → profil user tersebut
- Konten: username, nama tampilan, email, peran, kuota efektif + stacked bar kategori (hover/klik → Popover)
- **Edit Profil** hanya untuk diri sendiri (nama tampilan + email); username read-only
- Super Admin: kuota Unlimited, stacked kategori tetap ditampilkan
- Mock: `$lib/mocks/user-profiles.ts`

### Berbagi (user-to-user + tautan)

- Aksi **Bagikan** di explorer (file **dan** folder), starred, dan shared (hanya `Editor`)
- Sheet: `$lib/components/drive/explorer/share-sheet.svelte`
  - Tab **Pengguna:** cari user terdaftar, klik untuk menambah ke “Orang yang memiliki akses”, Viewer/Editor per orang
  - Tab **Tautan:** password opsional + kedaluwarsa; tautan selalu publik (tanpa toggle Restricted di UI sheet)
- Mock grants/links: `$lib/mocks/share-grants.ts`, `$lib/mocks/share-links.ts`

### Share publik (`/s/[token]`)

- Route group `(public)/s/[token]/` — URL tidak berubah meski user login
- Tamu: `$lib/components/share/share-public-shell.svelte` (logo + theme, tanpa sidebar)
- Login mock: `$lib/components/share/share-app-shell.svelte` membungkus `DriveShell` (sidebar + header nav)
- Gate: password (`demo-password` / `demo123`), restricted (`demo-restricted` → `/login?redirect=`), expired, not found
- Token demo: `demo-public`, `demo-folder`, `demo-password`, `demo-restricted`, `demo-expired` (SITEMAP §E)
- `entries` di `+page.ts` wajib untuk `adapter-static`

### Super Admin (`/admin/*` di layout `(drive)`)

- Routes: `/admin/settings`, `/admin/users`, `/admin/addons` (SITEMAP §D)
- Komponen: `$lib/components/admin/`
- Types/mocks: `$lib/types/admin.ts`, `$lib/mocks/admin-*.ts`, `addons.ts`, `job-queue.ts`
- Visibility sidebar: `user.role === 'super_admin'`
- UI mock only — `ponytail: add +layout.server role check when API exists`

### Quota widget (sidebar + `/drive/storage-stats`)

Implementasi UI mengikuti SITEMAP §5 & DESIGN §5.4:

| State       | Perilaku UI                                          |
| ----------- | ---------------------------------------------------- |
| Limited     | `My Storage` + `used / limit GB` + progress bar flat |
| ≥ 80%       | Bar terracotta / warning pada write                  |
| 100%        | Block upload My Storage; toast `USER_QUOTA_EXCEEDED` |
| Unlimited   | Hanya `Used: X GB` (tanpa bar cap)                   |
| Super Admin | Selalu tampilan Unlimited                            |

Salinan error wajib beda untuk kuota user vs pool server penuh.

### Storage node badge

- Badge `[R2]`, `[SSD-n]` **hanya dirender untuk Super Admin** di file explorer
- Regular User: badge **hidden**
- Halaman `/admin/settings`: badge node **selalu visible**

---

## 5. API & Data (pre-backend)

Backend belum tersedia — **jangan** hardcode fetch tersebar di komponen.

- Semua panggilan API melalui layer **`$lib/api/`** (client + types) saat tersedia
- Data sementara dari **`$lib/mocks/`** atau fixture yang mirror kontrak PRD/SITEMAP
  - Drive/admin: `drive.ts`, `file-tree.ts`, `admin-users.ts`, `addons.ts`, …
  - Profil: `user-profiles.ts`
  - Share: `share-grants.ts`, `share-links.ts`
  - Sesi mock: `auth-session.ts` (`cubycloud-logged-in` di sessionStorage)
- Saat `docs/API_CONTRACT.md` ada, selaraskan signature mock → client nyata tanpa ubah komponen

---

## 6. Struktur & Konvensi

```text
apps/frontend/
├── src/routes/
│   ├── +layout.svelte              # theme init + fonts
│   ├── +error.svelte               # error UI global
│   ├── +page.svelte                # / landing
│   ├── (auth)/                     # login, recovery
│   ├── (onboarding)/setup/         # /setup wizard
│   ├── (public)/s/[token]/         # share link guest
│   └── (drive)/                    # drive shell + pages
│       ├── +layout.svelte          # DriveShell + driveView.init
│       ├── drive/
│       │   ├── home/
│       │   ├── my-storage/
│       │   ├── organization/
│       │   ├── shared/
│       │   ├── starred/
│       │   ├── trash/
│       │   ├── storage-stats/
│       │   └── profile/[userId]/
│       └── admin/
│           ├── settings/
│           ├── users/
│           └── addons/
├── src/lib/
│   ├── theme.svelte.ts
│   ├── drive-context.svelte.ts
│   ├── drive-nav.ts
│   ├── drive-view.svelte.ts
│   ├── error-pages.ts
│   ├── assets/                     # favicon, illustration/404.webp, …
│   ├── types/                      # drive.ts, setup.ts, admin.ts
│   ├── mocks/
│   ├── components/
│   │   ├── ui/                     # shadcn (button/card/switch/popover/…, shadow-none)
│   │   ├── drive/
│   │   ├── admin/
│   │   ├── share/
│   │   ├── error/
│   │   ├── auth/
│   │   ├── setup/
│   │   ├── theme-toggle.svelte
│   │   └── password-input.svelte
│   └── api/                        # (akan datang) client API
├── DESIGN.md
└── SITEMAP.md
```

- Komponen reusable: `$lib/components/` (bukan duplikasi di setiap route)
- UI copy drive/setup/auth/admin/error: **Bahasa Indonesia** kecuali landing hero (boleh EN sampai diubah produk)
