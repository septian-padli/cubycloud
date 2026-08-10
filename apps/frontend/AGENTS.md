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
- Gunakan token CubyCloud: `bg-app`, `bg-surface`, `text-main`, `text-muted`, `border-subtle`, `bg-primary`, `bg-secondary`, dll.
- **Shadow:** **zero shadow** di seluruh UI (`shadow-none`). Overlay/modal/sheet/dropdown cukup `border border-subtle` + surface layering — jangan `shadow-xs` / `shadow-lg`
- **Rounded:** `rounded-full` untuk button & input (base shadcn); `rounded-xl` untuk card/panel/modal; sidebar nav item `rounded-full`
- **Hover (drive):** `hover:bg-primary/10` nav pill sidebar (inactive); card Akses Cepat `bg-primary/5` + hover `bg-primary/10`; baris Aktivitas Terbaru `hover:bg-primary/5`; item sidebar aktif tanpa hover
- Aksesibilitas: kontras WCAG AA, outline fokus 2px, transisi 100–150ms

### Button variants (CubyCloud)

| Variant | Surface | Hover | Border | Contoh |
| ------- | ------- | ----- | ------ | ------ |
| `default` | `bg-primary` | `primary/80` | — | Unggah (header nav), Lanjut / Selesaikan Setup |
| `secondary` | `bg-surface` | `bg-app` | `border-subtle` | Kembali, Lewati |
| `outline` | `bg-app` | `bg-surface` | `border-subtle` | Uji koneksi |

Base button: `rounded-full` (lihat `$lib/components/ui/button/button.svelte`).

### Form controls

- **Input:** `rounded-full`, `bg-surface` + `border-subtle`, flat (`shadow-none`)
- **Password fields:** pakai `$lib/components/password-input.svelte` (show/hide) — jangan duplikasi eye toggle
- **Switch / radio / checkbox:** bits-ui memakai `data-state="checked"|"unchecked"` — style dengan `data-[state=checked]:…` (bukan `data-checked:`); flat (`shadow-none`)

---

## 3. Theme (light / dark)

- State global: `$lib/theme.svelte.ts` (key `localStorage`: `cubycloud-theme`)
- Mode: `light` | `dark` | `system`; class `.dark` pada `<html>`
- Anti-FOUC: script di `src/app.html` sebelum render
- Toggle UI: `$lib/components/theme-toggle.svelte` — reuse di header nav drive, onboarding, dll.
- Init theme di root `+layout.svelte` (`theme.init()` on mount)

---

## 4. Routing & UX (SITEMAP)

- Ikuti route tree di `apps/frontend/SITEMAP.md` — jangan invent route baru tanpa update SITEMAP
- **Lightbox previewer:** URL-driven via query `?preview=file_id`
- **Post-setup / login redirect:** `/drive/home`
- Sediakan loading state halus untuk perangkat lambat

### Auth (`(auth)` layout)

- Route: `/login`, `/recovery`
- Komponen: `$lib/components/auth/` (`login-form`, `recovery-form`, `auth-split-shell`)
- Mock belum ada API — form submit UI-only

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
    ├── DriveSidebar   # desktop; Sheet drawer mobile
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
| `drive-sidebar-nav.svelte` | Nav pill: aktif `bg-primary text-primary-foreground` (no hover); inactive `hover:bg-primary/10` |
| `drive-page-header.svelte` | Title/description + optional `actions` snippet |
| `quick-access-card.svelte` | Variant file kinds; icon `bg-primary/10`; card `bg-primary/5`, hover `bg-primary/10` |
| `home-quick-access.svelte` | Grid Akses Cepat di `/drive/home` |
| `home-recent-activities.svelte` | List aktivitas; baris `hover:bg-primary/5` |

Context & mock:

- `setDriveChrome()` / `getDriveChrome()` — `$lib/drive-context.svelte.ts` (user, quota, role toggle, upload handler)
- `deriveDriveActiveId()` — `$lib/drive-nav.ts` (pathname → sidebar `activeId`)
- Fixture: `$lib/mocks/drive.ts`, types `$lib/types/drive.ts`

Scroll: sidebar + header nav **fixed**; page header + content **scroll bersama** di `DriveMain`.

### Quota widget (sidebar + `/drive/storage-stats`)

Implementasi UI mengikuti SITEMAP §5 & DESIGN §5.4:

| State       | Perilaku UI                                          |
| ----------- | ---------------------------------------------------- |
| Limited     | `My Storage` + `used / limit GB` + progress bar flat |
| ≥ 80%       | Bar ochre (`bg-secondary`) + peringatan pada write   |
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
- Saat `docs/API_CONTRACT.md` ada, selaraskan signature mock → client nyata tanpa ubah komponen

---

## 6. Struktur & Konvensi

```text
apps/frontend/
├── src/routes/
│   ├── +layout.svelte              # theme init + fonts
│   ├── +page.svelte                # / landing
│   ├── (auth)/                     # login, recovery
│   ├── (onboarding)/setup/       # /setup wizard
│   └── (drive)/                    # drive shell + pages
│       ├── +layout.svelte          # DriveShell
│       └── drive/home/             # /drive/home
├── src/lib/
│   ├── theme.svelte.ts
│   ├── drive-context.svelte.ts
│   ├── drive-nav.ts
│   ├── types/                      # drive.ts, setup.ts, …
│   ├── mocks/
│   ├── components/
│   │   ├── ui/                     # shadcn-svelte (button/input rounded-full, shadow-none)
│   │   ├── drive/                  # shell, sidebar, quick-access-card, …
│   │   ├── auth/
│   │   ├── setup/
│   │   ├── theme-toggle.svelte
│   │   └── password-input.svelte
│   └── api/                        # (akan datang) client API
├── DESIGN.md
└── SITEMAP.md
```

- Komponen reusable: `$lib/components/` (bukan duplikasi di setiap route)
- UI copy drive/setup/auth: **Bahasa Indonesia** kecuali landing hero (boleh EN sampai diubah produk)
