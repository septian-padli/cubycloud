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

- `rounded-full`: Input form, tombol (_button_), _filter chips_, item sidebar navigasi.
- `rounded-xl` (12px): Kontainer modal, _card_ indikator penyimpanan, dan panel ringkas.
- `rounded-full`: Avatar pengguna dan _pill badge storage node_ (`[R2]`, `[SSD]`, `[S3]`).

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
		<button class="rounded-lg p-1.5 text-muted hover:text-main">
			<MoreVerticalIcon class="h-4 w-4 stroke-2" />
		</button>
	</div>
</div>
```

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

### 5.3. Primary CTA Button & Danger Button

#### Primary Button (Upload / Action)

```svelte
<button
	class="inline-flex items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary/90 focus:ring-2 focus:ring-primary/50 focus:outline-none"
>
	<PlusIcon class="h-4 w-4 stroke-2" />
	<span>Unggah Berkas</span>
</button>
```

#### Danger Button (Delete / Purge Trash)

```svelte
<button
	class="inline-flex items-center justify-center gap-2 rounded-lg bg-danger px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-danger/90 focus:ring-2 focus:ring-danger/50 focus:outline-none"
>
	<TrashIcon class="h-4 w-4 stroke-2" />
	<span>Hapus Permanen</span>
</button>
```

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

- **≥ 80% used:** isi bar memakai `bg-secondary` (Warm Ochre); toast warning saat aksi tulis My Storage.
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

## 6. Micro-Interactions & State Guidelines

1. **Hover & Active States:**

- Dilarang menggunakan animasi pergerakan posisi (_translate_) atau perubahan bayangan (_shadow jump_) pada daftar berkas.
- Efek interaktif **hanya** transisi warna latar (`transition-colors`), tanpa shadow.
- **Drive chrome & beranda** (sidebar nav, card Akses Cepat, baris Aktivitas Terbaru): tint primary — `hover:bg-primary/10` untuk item pill/nav kecil; card Akses Cepat `bg-primary/5` dengan hover `bg-primary/10`; baris Aktivitas Terbaru `hover:bg-primary/5`. Item sidebar **aktif** (`bg-primary` solid) **tanpa** hover tambahan.
- **File explorer row** (list view berkas): netral `hover:bg-black/5` (Light) / `hover:bg-white/5` (Dark) — lihat §5.1.

2. **Resumable TUS Transfer Progress:**

- Bilah progres pengunggahan menggunakan format _flat bar_ setinggi `h-1.5 rounded-full bg-black/10 dark:bg-white/10` dengan indikator pengisi `bg-primary`.
- Jika status transfer _Paused_ akibat kendala jaringan, warna indikator bar otomatis berubah menjadi `bg-secondary` (Warm Ochre).

3. **Modal Dialogs & Lightbox Previewer:**

- Overlay modal menggunakan `bg-black/60 backdrop-blur-none` untuk menjaga performa rendering tetap ringan pada perangkat bertaraf prosesor Celeron lama.
- Container modal dihiasi dengan sudut `rounded-xl`, latar `bg-surface`, garis `border border-subtle`, tanpa bayangan (`shadow-none`).

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
