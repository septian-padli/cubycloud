# CubyCloud — Frontend AI Agent Guidance (SvelteKit & UI)

Patuhi kontrak global di `AGENTS.md` (root) dan dokumen produk berikut **sebelum** menulis kode UI:

- `docs/prd.md` — aturan fitur & quota
- `apps/frontend/DESIGN.md` — design system (wajib)
- `apps/frontend/SITEMAP.md` — route tree & kontrak halaman (wajib)

---

## 1. Tech Stack & Standards

- **Framework:** SvelteKit SPA (`adapter-static`, fallback `index.html` untuk `go:embed`)
- **Runtime:** Svelte 5 **Runes** (`$state`, `$derived`, `$effect`, `$props`). Hindari `writable`/`readable` stores untuk state lokal baru jika bisa diganti Runes.
- **Language:** TypeScript
- **Styling:** Tailwind CSS v4 — token via `@theme inline` di `src/routes/layout.css`
- **Typography:** Montserrat (400/500/600/700)
- **Components:** **shadcn-svelte** — tambah komponen ke `$lib/components/ui/` via `bunx shadcn-svelte@latest add <name>`
- **Package manager:** Bun (`bun install`, `bun run dev|build|check|lint`)

---

## 2. Design System (Flat Earthy)

- Skema **flat & earthy** (Parchment Cream, Olive Moss, Warm Ochre) — bukan Neo-Notion / hard shadow
- **STRICT zero arbitrary values:** dilarang `p-[14px]`, `w-[320px]`, `text-[#123456]`, `stroke-[1.75]`
- Gunakan token CubyCloud: `bg-app`, `bg-surface`, `text-main`, `text-muted`, `border-subtle`, `bg-primary`, `bg-secondary`, dll.
- Shadow hanya untuk elemen mengambang (modal, toast, dropdown)
- Aksesibilitas: kontras WCAG AA, outline fokus 2px, transisi 100–150ms

---

## 3. Routing & UX (SITEMAP)

- Ikuti route tree di `apps/frontend/SITEMAP.md` — jangan invent route baru tanpa update SITEMAP
- **Lightbox previewer:** URL-driven via query `?preview=file_id`
- **Post-setup / login redirect:** `/drive/home`
- Sediakan loading state halus untuk perangkat lambat

### Quota widget (sidebar + `/drive/storage-stats`)

Implementasi UI mengikuti SITEMAP §5 & DESIGN §5.4:

| State       | Perilaku UI                                          |
| ----------- | ---------------------------------------------------- |
| Limited     | `My Storage` + `used / limit GB` + progress bar flat |
| ≥ 80%       | Bar ochre (`bg-secondary`) + peringatan pada write   |
| 100%        | Block upload My Storage; toast `USER_QUOTA_EXCEEDED` |
| Unlimited   | Hanya `Used: X GB` (tanpa bar cap)                   |
| Super Admin | Selalu tampilan Unlimited                            |

Salinan error wajib berbeda untuk kuota user vs pool server penuh.

### Storage node badge

- Badge `[R2]`, `[SSD-n]` **hanya dirender untuk Super Admin** di file explorer
- Regular User: badge **hidden**
- Halaman `/admin/settings`: badge node **selalu visible**

---

## 4. API & Data (pre-backend)

Backend belum tersedia — **jangan** hardcode fetch tersebar di komponen.

- Semua panggilan API melalui layer **`$lib/api/`** (client + types)
- Data sementara dari **`$lib/mocks/`** atau fixture yang mirror kontrak PRD/SITEMAP
- Saat `docs/API_CONTRACT.md` ada, selaraskan signature mock → client nyata tanpa ubah komponen

---

## 5. Struktur & Konvensi

```text
apps/frontend/
├── src/routes/          # SvelteKit routes (lihat SITEMAP)
├── src/lib/
│   ├── components/ui/   # shadcn-svelte
│   ├── api/             # API client (mock → real)
│   └── mocks/           # Fixture sementara
├── DESIGN.md
└── SITEMAP.md
```

- Komponen reusable: `$lib/components/` (bukan duplikasi di setiap route)
- Ikon: `lucide-svelte`, stroke `stroke-2` (bukan arbitrary)
