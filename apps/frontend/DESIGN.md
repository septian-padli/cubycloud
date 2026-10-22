# DESIGN.md — CubyCloud UI/UX Specification & Design System

## 1. Executive Visual Summary & Design Philosophy

CubyCloud mengadopsi bahasa desain **100% Flat Design** dengan ritme tata letak dan _spacing_ yang terinspirasi dari kebersihan antarmuka modern **WhatsApp**, dipadukan dengan skema warna **Earthy Tones** yang hangat serta tipografi geometris **Montserrat**.

### Core Philosophy

1. **Clean & Flat Surface:** Tanpa bayangan (_zero shadow_), tanpa efek kaca (_glassmorphism_), dan tanpa garis batas yang agresif. Pemisahan antarelemen mengandalkan kontras latar belakang (_surface layering_) dan garis pemisah super-tipis (_subtle dividers_).
2. **Low Eye Strain (Earthy Neutrality):** Menggantikan dominasi warna biru-putih steril khas _cloud storage_ konvensional dengan warna krem alami (_Parchment Cream_) dan hijau hutan pekat (_Obsidian Forest_). Sempurna untuk sesi penggunaan berdurasi panjang.
3. **Storage-First Scanning Efficiency:** Struktur daftar berkas diorientasikan pada model **List View** (satu-satunya tampilan explorer di v1.0) berbaris horizontal dengan _padding_ dan proporsi yang teratur. Antarmuka disesuaikan khusus untuk manajemen penyimpanan berkas—tanpa elemen visual bergaya perpesanan (_chat bubbles_ atau _notification dots_) yang tidak relevan.
4. **Strict Utility Styling (Tailwind CSS v4):** Seluruh implementasi komponen UI di dalam SvelteKit **wajib** menggunakan token standar Tailwind CSS v4. Penggunaan _arbitrary values_ (seperti `w-[13px]`, `p-[14px]`, `text-[15px]`) **dilarang keras**.

---

## 2. Design Tokens & Color Architecture

Skema warna dirancang agar identitas _Earthy Tones_ konsisten di mode terang (_Light Mode_) dan tetap nyaman di mode gelap (_Dark Mode_), dengan pemisahan tegas antara warna aksen terakota dan warna peringatan destruktif (_danger_).

### 2.1. Color Palette Mapping

| Token Functional   | Light Mode (Hex / Name)      | Dark Mode (Hex / Name)        | Peran & Penggunaan Utama di Dalam UI                          |
| ------------------ | ---------------------------- | ----------------------------- | ------------------------------------------------------------- |
| `bg-app`           | `#F0EEE3` (Parchment Cream)  | `#0F130E` (Obsidian Forest)   | Latar belakang utama seluruh aplikasi                         |
| `surface-card`     | `#FFFFFF` (Pure White)       | `#182017` (Deep Moss Surface) | Latar belakang kartu, _sidebar_, tabel berkas, modal          |
| `primary`          | `#606C38` (Olive Moss)       | `#99AB5E` (Bright Sage)       | Tombol CTA utama, ikon aktif, penanda status sukses           |
| `primary-accent`   | `#283618` (Deep Forest)      | `#ADCB8A` (Minty Sage)        | Heading utama, teks penting, ikon folder bawaan               |
| `secondary`        | `#DDA15E` (Warm Ochre)       | `#E1AB70` (Soft Amber)        | Badge penyimpanan lokal (`[SSD-1]`), indikator proses         |
| `secondary-accent` | `#BC6C25` (Burnt Terracotta) | `#D78032` (Warm Terracotta)   | Badge _cloud storage_ (`[R2]`, `[S3]`), aksen sorotan penting |
| `danger`           | `#EC003F` (Rose 600)         | `#FF3366` (Bright Rose)       | Aksen destruktif, Purge Trash, konfirmasi hapus permanen      |
| `text-main`        | `#283618` (Deep Forest)      | `#F0EEE3` (Parchment Cream)   | Teks utama deskriptif dan nama berkas                         |
| `text-muted`       | `#5A6B46` (Muted Olive)      | `#9CA88F` (Muted Sage)        | Metadata (ukuran berkas, tanggal, info kuota)                 |
| `border-subtle`    | `rgba(0,0,0,0.06)`           | `rgba(255,255,255,0.08)`      | Garis pemisah antarbaris berkas & panel                       |

---

### 2.2. Tailwind CSS v4 Configuration (`src/routes/layout.css`)

Pada Tailwind CSS v4, seluruh konfigurasi token warna, tipografi, dan _custom properties_ dikelola melalui satu berkas CSS utama menggunakan direktif `@import "tailwindcss";` dan blok `@theme`.

```css
@import 'tailwindcss';

@layer base {
	:root {
		--bg-app: #f0eee3;
		--surface-card: #ffffff;
		--primary: #606c38;
		--primary-accent: #283618;
		--secondary: #dda15e;
		--secondary-accent: #bc6c25;
		--danger: #ec003f;
		--text-main: #283618;
		--text-muted: #5a6b46;
		--border-subtle: rgba(0, 0, 0, 0.06);
	}

	.dark {
		--bg-app: #0f130e;
		--surface-card: #182017;
		--primary: #99ab5e;
		--primary-accent: #adcb8a;
		--secondary: #e1ab70;
		--secondary-accent: #d78032;
		--danger: #ff3366;
		--text-main: #f0eee3;
		--text-muted: #9ca88f;
		--border-subtle: rgba(255, 255, 255, 0.08);
	}
}

@theme inline {
	--font-sans: 'Montserrat', sans-serif;

	--color-app: var(--bg-app);
	--color-surface: var(--surface-card);
	--color-primary: var(--primary);
	--color-primary-accent: var(--primary-accent);
	--color-secondary: var(--secondary);
	--color-secondary-accent: var(--secondary-accent);
	--color-danger: var(--danger);
	--color-main: var(--text-main);
	--color-muted: var(--text-muted);
	--color-subtle: var(--border-subtle);
}
```

---

## 3. Typography System (Montserrat Scale)

CubyCloud menggunakan Google Font **Montserrat** sebagai satu-satunya _font family_. Karena karakter Montserrat cenderung proporsional dan lebar, rasio _line-height_ dan ukuran _font_ diatur ketat agar antarmuka tetap padat secara horizontal tanpa mengorbankan keterbacaan.

| Hierarchy            | Tailwind Class                       | Size / Line-Height | Weight           | Penggunaan Utama                              |
| -------------------- | ------------------------------------ | ------------------ | ---------------- | --------------------------------------------- |
| **Page Title**       | `text-2xl font-bold leading-tight`   | 24px / 28px        | `700 (Bold)`     | Judul halaman (`/drive/my-storage`, `/setup`) |
| **Section Heading**  | `text-lg font-semibold leading-snug` | 18px / 24px        | `600 (SemiBold)` | Judul blok modul, dialog konfirmasi           |
| **Item Title**       | `text-sm font-medium leading-normal` | 14px / 20px        | `500 (Medium)`   | Nama berkas, nama folder, item navigasi       |
| **Body Text**        | `text-sm font-normal leading-normal` | 14px / 20px        | `400 (Regular)`  | Deskripsi fitur, pesan _toast_, _error logs_  |
| **Metadata / Badge** | `text-xs font-medium leading-none`   | 12px / 16px        | `500 (Medium)`   | Ukuran berkas, tanggal unggah, _badge_ node   |

---

## 4. Layout & Grid Principles (100% Flat Surface)

### 4.1. Visual Depth via Layering (Zero Shadow)

- Konten utama ditempatkan pada lapisan kontainer putih/gelap (`bg-surface`) di atas latar belakang halaman (`bg-app`).
- Pemisahan elemen secara horizontal maupun vertikal **hanya** menggunakan garis tipis `border-subtle`.
- **Dilarang** memakai bayangan Tailwind (`shadow-xs`, `shadow-sm`, `shadow-lg`, dll.) — termasuk modal, sheet drawer, toast, dropdown, card, input, button, checkbox, dan switch. Gunakan `shadow-none` + `border border-subtle` bila perlu kontras overlay.

### 4.2. Rounded Corner Scale

- `rounded-full`: Input form, tombol (_button_), _filter chips_, item sidebar navigasi, switch track.
- `rounded-xl` (12px): Kontainer modal, _card_ indikator penyimpanan, dan panel ringkas.
- `rounded-full`: Avatar pengguna dan _pill badge storage node_ (`[R2]`, `[SSD]`, `[S3]`).

### 4.3. Card Padding (satu token, semua sisi)

Padding luar card **simetris** lewat `--card-spacing` (default `--spacing(4)` = 16px):

| Slot                                 | Padding                                                                    |
| ------------------------------------ | -------------------------------------------------------------------------- |
| `Card.Root`                          | `py-(--card-spacing)` + `gap-(--card-spacing)` antar Header/Content/Footer |
| `Card.Header` / `Content` / `Footer` | `px-(--card-spacing)` saja (tanpa `last:pb` / `pt` asymetris)              |

**Pengecualian:** shell list/tabel edge-to-edge → override Root `py-0` + Content `p-0` (contoh: daftar berkas terbesar, quota detail flat).

**Dilarang:** mencampur override `p-3` / `p-4` pada Content dengan token `--card-spacing` yang berbeda — pilih satu model.

### 4.4. List vs Grid (Drive)

- Mode tampilan global: `$lib/drive-view.svelte.ts` (`localStorage` key `cubycloud-drive-view`) + `DriveViewToggle`.
- **List:** baris explorer / `DriveListRow` — lihat §5.1.
- **Grid:** `FileGridCard` — area ikon full-width (`aspect-video`), default `bg-surface`, hover/selected `bg-surface/80` (tanpa tint `bg-primary/5`).

---

## 5. Core Component Design Library

### 5.1. File Explorer Row (List View Default)

Baris berkas disajikan dengan proporsi dan _spacing_ horizontal yang konsisten untuk mempercepat pemindaian metadata oleh pengguna. Efek _hover_ berupa perubahan warna latar belakang yang mulus tanpa bayangan.

```svelte
<div
	class="flex cursor-pointer items-center justify-between border-b border-subtle px-4 py-3 transition-colors hover:bg-black/5 dark:hover:bg-white/5"
>
	<div class="flex items-center gap-3">
		<div class="rounded-lg bg-primary/10 p-2 text-primary">
			<FolderIcon class="h-5 w-5 stroke-2" />
		</div>
		<div>
			<p class="text-sm font-medium text-main">Laporan_Arsitektur_v1.pdf</p>
			<p class="text-xs text-muted">2.4 MB • Diubah 2 jam lalu</p>
		</div>
	</div>

	<div class="flex items-center gap-2">
		<!-- Storage node badge: Super Admin only -->
		<span class="rounded-full bg-secondary-accent px-2.5 py-1 text-xs font-medium text-white">
			R2 Cloud
		</span>
		<!-- Actions ⋮: Button ghost size=icon; bg-surface, hover bg-primary text-white -->
		<button
			class="inline-flex size-9 items-center justify-center rounded-full bg-surface text-muted hover:bg-primary hover:text-white"
			aria-label="Aksi berkas"
		>
			<MoreVerticalIcon class="h-4 w-4 stroke-2" />
		</button>
	</div>
</div>
```

Aksi baris/grid: **DropdownMenu** (klik ⋮) dan **ContextMenu** (klik kanan) — mirror item yang sama.

---

### 5.2. Storage Node Badges

Badge disajikan dalam format _pill_ (`rounded-full`) dengan font mini (`text-xs font-medium`) untuk memberikan indikator visual lokasi fisik penyimpanan secara cepat dan estetik.

**Visibility:** Badge node (`[R2]`, `[SSD-n]`) **hanya dirender untuk Super Admin** di explorer. Regular User tidak melihat lokasi fisik. Halaman `/admin/settings` selalu menampilkan badge node.

- **Cloud Storage Badge (`[R2]`, `[S3]`):**
  - `bg-secondary-accent text-white dark:text-app` (Burnt Terracotta / Warm Terracotta)

- **Local Disk Storage Badge (`[SSD-1]`, `[HDD-2]`):**
  - `bg-secondary text-main dark:text-app` (Warm Ochre / Soft Amber)

- **Organization Folder Badge (`[🏢 Organization]`):**
  - `bg-primary/15 text-primary font-medium` (Aksen Moss/Sage transparan) — terlihat semua role di `/drive/organization`

---

### 5.3. Buttons (shadcn-svelte + CubyCloud)

Implementasi: `$lib/components/ui/button/button.svelte` (`rounded-full`, `shadow-none`).

#### Variants

| Variant       | Surface                    | Hover           | Border          | Contoh                                             |
| ------------- | -------------------------- | --------------- | --------------- | -------------------------------------------------- |
| `default`     | `bg-primary` + teks putih  | `bg-primary/80` | —               | Unggah, Simpan, CTA utama                          |
| `secondary`   | `bg-surface`               | `bg-app`        | `border-subtle` | Kembali, Pindai ulang, Unduh backup, Kelola sampah |
| `outline`     | `bg-app`                   | `bg-surface`    | `border-subtle` | Aksi sekunder di atas `bg-app`                     |
| `destructive` | `bg-danger` + `text-white` | `bg-danger/90`  | —               | Hapus permanen, Kosongkan sampah                   |
| `ghost`       | transparan                 | sesuai konteks  | —               | Icon ⋮ (lihat di bawah)                            |
| `link`        | —                          | underline       | —               | Tautan teks                                        |

#### Size (tinggi = skala shadcn Vega; padding horizontal CubyCloud lebih lebar)

| Size                              | Tinggi                          | Catatan                                                    |
| --------------------------------- | ------------------------------- | ---------------------------------------------------------- |
| `default`                         | `h-9`                           | **Standar** di `/drive` & `/admin` (teks) — `px-3.5`       |
| `sm` / `lg` / `xs`                | `h-8` / `h-10` / `h-6`          | Hanya jika eksplisit dibutuhkan                            |
| `icon`                            | `size-9`                        | **Standar** icon-only (⋮, menu) — sejajar tinggi `default` |
| `icon-sm` / `icon-lg` / `icon-xs` | `size-8` / `size-10` / `size-6` | Opsional                                                   |

#### Actions ⋮ (ghost icon)

```text
variant="ghost" size="icon"
class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
```

#### Primary / Danger (referensi visual)

```svelte
<!-- Primary CTA -->
<button class="rounded-full bg-primary text-white hover:bg-primary/80 … …">Unggah Berkas</button>

<!-- Danger -->
<button class="rounded-full bg-danger text-white hover:bg-danger/90 … …">Hapus Permanen</button>
```

---

### 5.3.1. Switch

Track: nonaktif `bg-app`, aktif `data-[state=checked]:bg-surface`; border `border-primary`; thumb `bg-primary`. Flat (`shadow-none`). Style state dengan `data-[state=checked]:…` (bits-ui).

---

### 5.4. My Storage Quota Widget (Sidebar + Stats)

Widget kuota tampil di **sidebar** (semua authenticated user) dan detail di `/drive/storage-stats`. Satuan UI: **GB**.

#### Limited quota

```svelte
<div class="border-t border-subtle px-4 py-3">
	<p class="text-xs font-medium text-muted">My Storage</p>
	<p class="mt-1 text-sm font-medium text-main">8.2 / 15 GB</p>
	<div class="mt-2 h-1.5 rounded-full bg-black/10 dark:bg-white/10">
		<!-- width bound dynamically from used/limit ratio (not a Tailwind arbitrary class) -->
		<div class="h-1.5 w-7/12 rounded-full bg-primary"></div>
	</div>
</div>
```

- **≥ 80% used:** isi bar memakai `bg-secondary-accent` / indikator terracotta; toast warning saat aksi tulis My Storage.
- **100%:** block upload; toast error dengan salinan _"Kuota My Storage penuh. Hubungi admin."_ (`USER_QUOTA_EXCEEDED`).

#### Unlimited / Super Admin

```svelte
<div class="border-t border-subtle px-4 py-3">
	<p class="text-xs font-medium text-muted">My Storage</p>
	<p class="mt-1 text-sm font-medium text-main">Used: 8.2 GB</p>
</div>
```

Tidak menampilkan angka limit, persen, atau progress bar dengan cap.

#### Permission badge (Shared)

- `Viewer`: `bg-secondary text-main` (Ochre)
- `Editor`: `bg-primary/15 text-primary` (Moss)

---

### 5.5. Error pages (`+error.svelte`)

- Komponen: `$lib/components/error/error-page.svelte` + copy/mapping `$lib/error-pages.ts`
- Ilustrasi: `$lib/assets/illustration/` (import via `$lib`; status 401/403/404/500/503 — asset boleh diganti per status nanti)
- Layout: `bg-app`, ilustrasi ~`md:w-1/2` (desktop), CTA **Kembali ke beranda** → `/` (`Button` default)

---

## 6. Micro-Interactions & State Guidelines

1. **Hover & Active States:**

- Dilarang menggunakan animasi pergerakan posisi (_translate_) atau perubahan bayangan (_shadow jump_) pada daftar berkas.
- Efek interaktif **hanya** transisi warna latar (`transition-colors`), tanpa shadow.
- **Sidebar nav:** inactive `hover:bg-primary/10`; item **aktif** (`bg-primary` solid) **tanpa** hover tambahan.
- **File grid card:** default `bg-surface`; hover & selected `bg-surface/80` (bukan tint `bg-primary/5`).
- **Baris list / aktivitas:** netral `hover:bg-black/5` (Light) / `hover:bg-white/5` (Dark), atau tint ringan beranda jika masih memakai `hover:bg-primary/5` pada list aktivitas — jangan campur dengan selected state grid.
- **File explorer row** (list view berkas): netral `hover:bg-black/5` / `hover:bg-white/5` — lihat §5.1.

2. **Resumable TUS Transfer Progress:**

- Bilah progres pengunggahan menggunakan format _flat bar_ setinggi `h-1.5 rounded-full bg-black/10 dark:bg-white/10` dengan indikator pengisi `bg-primary`.
- Jika status transfer _Paused_ akibat kendala jaringan, warna indikator bar otomatis berubah menjadi `bg-secondary` (Warm Ochre).

3. **Modal Dialogs & Lightbox Previewer:**

- Overlay modal menggunakan `bg-black/60 backdrop-blur-none` untuk menjaga performa rendering tetap ringan pada perangkat bertaraf prosesor Celeron lama.
- Container modal dihiasi dengan sudut `rounded-xl`, latar `bg-surface`, garis `border border-subtle`, tanpa bayangan (`shadow-none`).

4. **Offline-backed file rows:**

- Keep the file row and its metadata visible when `storage_available` is `false`; disable actions that require access to that unavailable file.
- Use a shadcn-svelte Popover that can open on hover or click. Its localized message says the storage node is offline and must not reveal a physical node identifier or name.
- Do not disable folders or prevent navigation solely because a storage node is unavailable.

5. **English and Indonesian UI:**

- Support both English and Indonesian, selecting Indonesian when the browser language begins with `id` and English otherwise. All user-facing copy, including labels, titles, placeholders, errors, statuses, confirmations, notifications, and accessible names, follows the active locale.
- A user-selected language is stored only in browser `localStorage`, scoped by authenticated account ID. Do not persist it server-side.


- The global language selector pairs a Globe icon with the shadcn-svelte `Select`; its visible options are the short locale codes `id` and `en`, and its accessible name follows the active language.

---

## 7. Strict Tailwind CSS Rules (Zero Arbitrary Values)

Untuk menjaga konsistensi visual di seluruh ekosistem SvelteKit SPA CubyCloud, pengembang **wajib** mematuhi aturan berikut:

### ❌ Dilarang (Arbitrary Values):

```html
<div class="gap-[7px] rounded-[10px] bg-[#606C38] p-[14px] text-[13px]">...</div>
```

### ✅ Wajib (Tailwind Standard Tokens):

```html
<div class="gap-2 rounded-lg bg-primary p-3.5 text-xs">...</div>
```

### Panduan Linter & Review Kode

- Segala _pull request_ yang mengandung pola regex `\[(?:px|rem|em|#|rgb|hsl)\]` pada atribut class berkas `.svelte` atau `.html` harus ditolak saat proses _Code Review_.
- Jika ada dimensi atau token yang belum didukung oleh utilitas dasar, variabel tersebut **wajib** didaftarkan secara resmi di dalam `src/routes/layout.css` di bawah blok `@theme inline` terlebih dahulu.
