# Product Requirement Document (PRD)

## CubyCloud — Self-Hosted Modular Cloud Storage (v1.0)

---

## 1. Document Control & Executive Summary

### 1.1 Document Control

| Parameter             | Detail                                                          |
| --------------------- | --------------------------------------------------------------- |
| **Nama Proyek**       | **CubyCloud**                                                   |
| **Penyusun (Author)** | **Septian Padli**                                               |
| **Versi Dokumen**     | `v1.0.0 (Draft)`                                                |
| **Status Dokumen**    | In Review / Draft                                               |
| **Tanggal Pembuatan** | 7 Agustus 2026                                                  |
| **Target Launching**  | **1 Oktober 2026**                                              |
| **Target Audience**   | Core Engineering Team, UI/UX Designer, Open-Source Contributors |

---

### 1.2 Executive Summary

**CubyCloud** adalah aplikasi _self-hosted modular cloud storage_ modern yang dirancang sebagai alternatif berkinerja tinggi untuk **Nextcloud** dan **Google Drive**. CubyCloud hadir untuk menyelesaikan masalah kompleksitas instalasi dan konsumsi sumber daya (_resource hogging_) yang sering dialami oleh pengguna platform _self-hosted_ tradisional.

Dengan pendekatan **Single-Container Architecture** (Go backend dengan embedded SvelteKit frontend), CubyCloud memberikan pengalaman menggunakan Google Drive pribadi tanpa memerlukan konfigurasi CLI (_Command Line Interface_) yang rumit. Mulai dari proses instalasi awal (_onboarding_), pengalokasian penyimpanan disk lokal, hingga integrasi cloud pihak ketiga (S3/R2) dapat dilakukan seluruhnya melalui antarmuka Web GUI berbasis **Flat Earthy Design System** (lihat [`apps/frontend/DESIGN.md`](../apps/frontend/DESIGN.md)).

**Visi jangka panjang** (di luar scope v1.0): CubyCloud akan dilengkapi **AI Workspace & MCP RAG Engine** (bergaya _NotebookLM_) agar folder tertentu dapat dijadikan basis pengetahuan (_knowledge base_) privat via LLM lokal (Ollama) maupun cloud provider. Fitur ini dijadwalkan pada **Fase 2/3 Roadmap**, bukan rilis awal 1 Oktober 2026.

---

### 1.3 Problem Statement & Opportunity

- **Problem (Masalah):**

1. **Kompleksitas & Resource Heavy:** Platform seperti Nextcloud cenderung berat, membutuhkan spesifikasi server yang tinggi, dan sering kali membingungkan pengguna awam karena banyaknya ketergantungan _dependency_ serta pengaturan CLI.
2. **Vendor Lock-in & Privasi:** Layanan cloud komersial (seperti Google Drive) memungut biaya langganan bulanan yang terus meningkat dan memproses data/dokumen pengguna pada server pihak ketiga untuk pelatihan model AI.
3. **Fragmentasi Media Penyimpanan:** Pengguna _homelab_ kesulitan menggabungkan beberapa harddisk/SSD bekas dengan kapasitas berbeda serta bucket cloud hemat biaya (seperti Cloudflare R2) ke dalam satu direktori tunggal yang terpadu.

- **Opportunity (Peluang):**
  CubyCloud mengisi celah pasar dengan menghadirkan cloud storage yang **ringan, estetik, modular, dan fleksibel**, mampu berjalan lancar pada perangkat server kelas bawah (_legacy/budget hardware_) tanpa mengurangi fitur-fitur modern seperti transfer langsung (_presigned direct transfer_) dan penduplikatan file tanpa makan memori (_zero-byte copy_). Fondasi privasi & penyimpanan terpadu di v1.0 juga membuka jalan untuk AI Workspace berbasis RAG pada roadmap berikutnya.

---

### 1.4 Vision & Position Statement

> **"Google Drive versi self-hosted yang ultra-ringan, modern, dan modular. Tanpa keribetan perintah CLI, siap berjalan di home server berdaya rendah."**

---

### 1.5 Success Metrics & Key Performance Indicators (KPIs)

Untuk memastikan CubyCloud memenuhi kriteria performa pada perangkat berspesifikasi minim, berikut adalah target indikator keberhasilan teknis dan pengalaman pengguna (_User Experience_):

#### A. Performa & Resource Efficiency (Target Utama)

| Metric / KPI                     | Target Benchmark                         | Keterangan / Conditions                                            |
| -------------------------------- | ---------------------------------------- | ------------------------------------------------------------------ |
| **Base Hardware Spec**           | Intel Celeron 2-Core (Legacy) + 2 GB RAM | Spesifikasi batas minimum server target                            |
| **Idle RAM Consumption**         | **< 200 MB**                             | Penggunaan memori saat tidak ada aktivitas pengguna                |
| **Active RAM Consumption**       | **< 600 MB**                             | Penggunaan memori puncak saat proses upload/download intensif      |
| **Page Navigation Latency**      | **< 1.0 Detik**                          | Respon waktu pembukaan antar halaman drive pada jaringan LAN lokal |
| **First Contentful Paint (FCP)** | **< 800 ms**                             | Kecepatan pemuatan awal antarmuka frontend SvelteKit               |

#### B. Functional & Deployment KPIs

- **Zero-CLI Onboarding:** 100% proses konfigurasi awal (_Super Admin, Storage Node Discovery, Placement Strategy_) selesai melalui Web Setup Wizard dalam waktu **< 3 menit**.
- **Deployment Footprint:** Didistribusikan dalam bentuk **1 Image Docker** atau **1 Single Executable Binary** tanpa butuh _dependency_ eksternal tambahan.
- **Direct Transfer Efficiency:** 0% beban CPU server terbuang untuk _bouncing data_ saat melakukan upload/download berkas dari penyedia cloud (AWS S3 / Cloudflare R2) berkat penggunaan _Presigned URLs_.

---

## 2. Target Users & User Personas

---

### 2.1 Target User Segmentation

CubyCloud ditujukan untuk ekosistem pengguna _self-hosted_ dan tim skala kecil/menengah yang membutuhkan efisiensi penyimpanan, privasi tinggi, dan antarmuka modern tanpa dibebani oleh _overhead_ teknis.

```text
                               ┌─────────────────────────────────────────┐
                               │            CUBYCLOUD USERS              │
                               └────────────────────┬────────────────────┘
                                                    │
         ┌──────────────────────────────────────────┼──────────────────────────────────────────┐
         ▼                                          ▼                                          ▼
┌─────────────────────────┐              ┌─────────────────────────┐              ┌─────────────────────────┐
│   SysAdmin Minimalis    │              │   Privacy Researcher    │              │    Non-Tech End User    │
│  (Homelab Enthusiast)   │              │   & Content Creator     │              │ (Family / Team Member)  │
└─────────────────────────┘              └─────────────────────────┘              └─────────────────────────┘

```

---

### 2.2 Detailed User Personas

#### Persona A: Homelab Enthusiast & SysAdmin Minimalis ("Budi, 28")

- **Role / Demografi:** IT Support Specialist / Hobbyist Homelab.
- **Lingkungan Hardware:** Home server berbasis PC tua/bekas (Intel Celeron 2-Core, 2 GB RAM, 3 unit SSD/HDD bervariasi).
- **Tingkat Keahlian Teknis:** Menengah ke Atas (Bisa menjalankan `docker run` / `docker-compose`, namun enggan menghabiskan waktu merawat file konfigurasi CLI yang rumit).
- **Goals (Tujuan):**
- Menggabungkan seluruh harddisk bekas dan akun Cloudflare R2 gratisan menjadi satu direktori penyimpanan besar (_Unified Directory_).
- Memiliki cloud storage pribadi yang berjalan ringan di server tua tanpa menghabiskan RAM/CPU.

- **Pain Points (Kekesalan):**
- **Frustrasi pada Nextcloud:** Sangat berat (_resource-hogging_), lambat di server spek Celeron, dan sering _breakdown_ saat proses pembaruan versi via CLI.
- **Frustrasi pada NAS Proprietary (Synology/QNAP):** Biaya _hardware_ resmi terlalu mahal.

- **Solusi CubyCloud:**
- Cukup jalankan 1 container Docker, seluruh konfigurasi disk dan alokasi kuota diatur lewat **Web Setup Wizard**.
- Penggunaan RAM idle **< 200 MB** sehingga server tidak _overheat_ atau _crash_.

---

#### Persona B: Privacy-Conscious Researcher & Creator ("Siti, 24")

- **Role / Demografi:** Mahasiswa Tingkat Akhir / Penulis / Researcher.
- **Lingkungan Hardware:** Laptop kerja harian + Akses ke CubyCloud Server milik tim/pribadi.
- **Tingkat Keahlian Teknis:** Pemula - Menengah (Paham dasar penggunaan web app, tidak mengerti manajemen server).
- **Goals (Tujuan):**
- Menyimpan ratusan dokumen jurnal PDF, skripsi, dan catatan riset di cloud pribadi yang aman dan mudah diakses.
- Menjaga privasi data riset tanpa bergantung pada vendor cloud publik; kelak siap untuk AI Workspace lokal (roadmap).

- **Pain Points (Kekesalan):**
- **Frustrasi pada Google Drive:** Khawatir dokumen privat diproses/di-scrape oleh vendor cloud.
- **Frustrasi pada tool self-hosted berat:** Sulit mendapatkan storage pribadi yang ringan, cepat, dan nyaman untuk arsip riset.

- **Solusi CubyCloud (v1.0):**
- Cloud storage pribadi ringan dengan preview dokumen, share berproteksi, dan Organization Folder untuk kolaborasi tim.
- **Roadmap (Fase 2/3):** Klik kanan folder → **"Set as AI Workspace"** + chat privat via Ollama — bukan bagian rilis v1.0.

---

#### Persona C: Non-Tech End User & Family Member ("Rian, 32")

- **Role / Demografi:** Karyawan Swasta / Anggota Keluarga Pengguna Server.
- **Lingkungan Hardware:** Smartphone & Web Browser Desktop.
- **Tingkat Keahlian Teknis:** Pemula (Hanya familiar dengan antarmuka populer seperti Google Drive, Notion, atau WhatsApp).
- **Goals (Tujuan):**
- Mengunggah, mengunduh, dan berbagi foto/dokumen harian dengan cepat dan tanpa hambatan teknis.
- Melihat pratinjau file (_preview_) langsung di browser tanpa perlu mengunduhnya terlebih dahulu.

- **Pain Points (Kekesalan):**
- Antarmuka aplikasi _self-hosted_ pada umumnya terasa kaku, membingungkan, dan lambat.
- Tautan berbagi (_share link_) yang rumit tanpa fitur keamanan dasar.

- **Solusi CubyCloud:**
- Antarmuka **flat & earthy** yang bersih, responsif, dan nyaman untuk sesi penggunaan panjang — mengikuti token di [`apps/frontend/DESIGN.md`](../apps/frontend/DESIGN.md).
- Fitur berbagi file dengan kata sandi (_Password Protection_) dan masa berlaku (_Expiration Date_) yang instan.

---

### 2.3 User Frustrations & Competitive Pain Point Matrix

| Dimensi Evaluasi        | Google Drive (Public Cloud)                 | Nextcloud (Self-Hosted Traditional)                          | **CubyCloud (Solusi Kita)**                                    |
| ----------------------- | ------------------------------------------- | ------------------------------------------------------------ | -------------------------------------------------------------- |
| **Biaya & Kuota**       | Berlangganan bulanan mahal; kuota terbatas. | Gratis (Self-hosted), namun butuh server mahal agar lancar.  | **Gratis & Hemat:** Optimal di PC tua Celeron 2 GB RAM.        |
| **Penggunaan Resource** | N/A (Cloud Vendor)                          | **Sangat Berat:** RAM idle > 1-2 GB, sering _lag_ di LAN.    | **Ultra-Ringan:** RAM idle **< 200 MB**, respon LAN **< 1s**.  |
| **Kompleksitas Setup**  | Tanpa Setup (Instan).                       | **Sangat Rumit:** Membutuhkan instalasi PHP, DB, Redis, CLI. | **Zero-CLI Wizard:** 1 Command Docker -> Web Wizard GUI.       |
| **Manajemen Storage**   | Terkunci pada kuota vendor.                 | Terbatas pada konfigurasi mount titik awal Linux.            | **Unified Directory:** Gabung SSD 1, SSD 2, & Cloud R2/S3.     |
| **Privasi Data**        | Risiko pemrosesan data oleh vendor cloud.   | Self-hosted, namun setup & resource berat.                   | **Self-hosted ringan:** Data di server milikmu; AI Workspace (RAG/Ollama) di roadmap Fase 2/3. |

---

### 2.4 Critical User Workflows (Core User Journeys)

#### 1. Onboarding Pertama Kali (First-Boot Journey)

1. Admin menjalankan container dengan volume data, contoh: `docker run -p 8080:8080 -v cuby-data:/app/data cubycloud/app:latest`.
2. Admin membuka `http://localhost:8080/setup` di browser.
3. System menampilkan **Setup Wizard**:

- Membuat akun Super Admin.
- **Storage Node Discovery (hybrid):**
  - **Default:** memindai mount/volume di `/app/data/storage/`.
  - **Opsional:** menampilkan device Linux terdeteksi (mis. `/dev/sdb`, `/dev/sdc`) untuk di-_mount_/didaftarkan sebagai node.
- Memilih strategi alokasi penyimpanan (_Fill-First / Round-Robin / Most-Free-Space_).
- (Opsional) Memasukkan API Key Cloudflare R2 / AWS S3.

4. Sistem menyimpan konfigurasi dan langsung mengarahkan Admin ke Beranda `/drive/home`.

#### 2. Workflow Manajemen & Transfer Direct File

1. Pengguna melakukan _drag-and-drop_ berkas video/dokumen berukuran besar ke area browser.
2. Jika destinasi fisik berkas adalah Cloudflare R2, backend Go menerbitkan _Presigned Upload URL_.
3. Browser mengunggah berkas **langsung ke R2** tanpa melewati CPU/RAM server CubyCloud lokal.
4. Tampilan UI memperbarui daftar berkas secara real-time. Label _Storage Node Badge_ (`[R2]`, `[SSD-1]`) **hanya terlihat oleh Super Admin**; Regular User tidak melihat lokasi fisik node.

---

## 3. Product Scope & Boundaries

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                       CUBYCLOUD SCOPE MATRIX (v1.0)                     │
├──────────────────────────────────────┬──────────────────────────────────┤
│           IN-SCOPE (v1.0)            │       OUT-OF-SCOPE (Deferred)    │
├──────────────────────────────────────┼──────────────────────────────────┤
│ • Go Backend + SvelteKit SPA (Embed) │ • AI Workspace & MCP RAG Engine  │
│ • Single Container Deployment        │ • File Versioning & History      │
│ • Zero-CLI Web Setup Wizard          │ • Progressive Web App (PWA)      │
│ • Multi-Node Storage Pooling (VFS)   │ • Desktop & Mobile Native Sync   │
│ • Resumable Upload/Download (TUS)    │ • Real-time Office Editing       │
│ • Duplicate Handling (Numeric Suffix) │ • Email / SMTP Notifications     │
│ • Auto-Purge Trash (30 Days)         │ • Heavy Background Transcoding   │
│ • Preview (Pic/Vid/Aud/Text/PDF)     │ • SaaS Billing & Payment Gateway │
│ • Core Archive (Download as ZIP)     │                                  │
│ • User-to-User & Share Link Engine   │                                  │
│ • Drive Home / Org / Starred / Stats │                                  │
│ • Per-User Storage Quota (My Storage)│                                  │
│ • In-App Toasts & Notification Bar   │                                  │
└──────────────────────────────────────┴──────────────────────────────────┘

```

---

### 3.1 In-Scope Features (Ruang Lingkup v1.0 — Target Launch 1 Oktober 2026)

Seluruh fitur di bawah ini **wajib ada, teruji, dan berfungsi penuh** pada rilis awal versi 1.0.0:

#### 1. Core Architecture & Single-Container Distribution

- **Single Binary / Single Container:** Aplikasi berjalan dalam 1 Image Docker yang menggabungkan Go Backend dan _static build_ **SvelteKit** (di-embed menggunakan `go:embed`).
- **SvelteKit SPA Mode:** Frontend dibangun sebagai Single Page Application (SPA) murni menggunakan `@sveltejs/adapter-static` yang ultra-ringan dan responsif.
- **Zero-CLI Setup Wizard (`/setup`):** Pemasangan awal Super Admin, auto-discovery disk lokal, pemilihan _Placement Strategy_, dan ekspor _Emergency Recovery Key_ seluruhnya berbasis Web GUI.

#### 2. Virtual Storage Engine & Network Resiliency

- **Multi-Node Virtual Storage Pooling:** Menggabungkan beberapa disk lokal (SSD/HDD) dan cloud storage (AWS S3 / Cloudflare R2) ke dalam satu _Unified Virtual Directory_ tanpa RAID/LVM.
- **Placement Strategies:** Algoritma alokasi file otomatis (_Fill-First_, _Round-Robin_, dan _Most-Free-Space_).
- **Direct Presigned Cloud Transfer:** Transfer file ke Cloud (S3/R2) dilakukan _direct_ dari browser untuk menghemat CPU & bandwidth server lokal.
- **Zero-Byte Reference Copy:** Duplikasi file/folder instan tanpa memakan memori fisik tambahan di disk.
- **Resumable Upload & Download Engine:** Pengunggahan dan pengunduhan berkas mendukung mekanisme _pause & resume_ (misal via protokol TUS / _chunked transfer_) agar file besar tidak gagal saat koneksi jaringan terputus atau tidak stabil.
- **Unlimited File Size Support:** Tidak ada batasan buatan (_hard-limit_) untuk ukuran berkas; batas upload murni mengikuti sisa kapasitas fisik storage node.

#### 3. Drive Workspace Surfaces

- **Beranda (`/drive/home`):** Landing default setelah login/setup — quick access, recent activity, ringkasan kuota.
- **My Storage (`/drive/my-storage`):** File manager pribadi (CRUD, filter, preview, share drawer).
- **Organization (`/drive/organization`):** Drive bersama organisasi — Full Access, **tidak** dihitung ke user quota.
- **Shared (`/drive/shared`):** Berkas yang dibagikan ke akun aktif (Viewer / Editor).
- **Starred (`/drive/starred`):** Shortcut berkas/folder yang di-_star_ dari route mana pun.
- **Trash (`/drive/trash`):** Soft-delete + restore + purge (30 hari).
- **Storage Stats (`/drive/storage-stats`):** Chart usage, largest files, widget kuota user.

#### 4. File Operations & Conflict Handling (`/drive/my-storage`)

- **Flat Earthy UI Design System:** Tampilan 100% flat (_no hard shadows_ kecuali modal/toast), _earthy tones_ (Parchment Cream, Olive Moss), tipografi **Montserrat**, dan _list view_ storage-first — spesifikasi lengkap di [`apps/frontend/DESIGN.md`](../apps/frontend/DESIGN.md).
- **Storage Node Badge Visibility:** Badge `[R2]` / `[SSD-n]` **hanya untuk Super Admin**. Regular User tidak melihat lokasi fisik node.
- Konflik nama mengikuti kebijakan same-path di §4.4: upload/duplicate memilih suffix angka otomatis; rename/move menawarkan suffix yang tersedia atau membatalkan; tidak ada overwrite atau merge folder.

- **File Previewer bawaan:** Pratinjau tersedia di My Storage, Organization, Shared, dan tautan publik/restricted sesuai izin akses masing-masing:
- 📄 **Text/Code:** TXT, Markdown (.md), JSON, CSV, Log; tampilkan sebagai teks biasa dengan syntax highlighting PrismJS untuk format kode yang didukung.
- 🖼️ **Gambar:** PNG, JPEG, WebP, GIF, SVG; tampilkan langsung secara native.
- 🎥 **Video:** MP4, WebM; gunakan Vidstack dengan pemutaran media native browser.
- 🎵 **Audio:** MP3, WAV, OGG; perilaku pemutar audio yang sudah ada tetap berlaku.
- 📑 **PDF:** tampilkan menggunakan PDF.js (`pdfjs-dist`); selalu tersedia tanpa aktivasi add-on Admin.

#### 5. Per-User Storage Quota (My Storage Only)

- **Default User Quota:** Super Admin menetapkan kuota default (satuan **GB**) atau **Unlimited** di `/admin/settings`. Berlaku untuk **user baru saja**.
- **Per-User Override:** Di `/admin/users`, Admin dapat mengubah limit satu user (mis. 50 GB) atau menandai **Unlimited**.
- **Scope:** Hanya file dengan `owner_id = user` di luar **Organization Folder**. File Organization **tidak** dihitung ke quota user.
- **Trash:** Ukuran file di Trash **tetap dihitung** sampai purge permanen.
- **Zero-Byte Copy:** Owner sama → tidak menambah usage; copy ke owner berbeda → menambah usage **owner baru**.
- **Super Admin:** Selalu **Unlimited** (tidak kena user quota).
- **Enforcement:** Toast warning pada **≥ 80%**; **block upload** My Storage pada **100%**. Pesan error terpisah dari kegagalan pool fisik.
- **UI User:** Widget kuota di **sidebar** + detail di `/drive/storage-stats`. Jika Unlimited: tampilkan hanya `Used: X GB`.

#### 6. Trash Bin & Lifecycle Management (`/drive/trash`)

- **Soft Delete:** File yang dihapus dipindahkan ke Trash Bin sementara.
- **Automated 30-Day Purge Engine:** Latar belakang sistem (_worker_) akan menghapus file fisik secara permanen jika sudah berada di Trash Bin lebih dari 30 hari.
- **Manual Purge & Restore:** Pengguna memiliki opsi untuk mengembalikan file (_restore_) ke folder asal atau mengosongkan sampah (_Purge Trash_) secara manual kapan saja.

#### 7. Sharing & Access Control (`/drive/shared` & `/s/:token`)

- **User-to-User Internal Sharing:** Pengguna dapat membagikan file/folder ke pengguna lain yang terdaftar dalam sistem (mirip Google Drive share permission).
- **Share Link Engine:** Menerbitkan tautan berbagi dengan mode **Public** atau **Restricted**.
- **Link Security Controls:** Pengaturan proteksi kata sandi (_Password Protection_) dan batas tanggal kedaluwarsa (_Expiration Date_).
- **Access Scope Settings:** Opsi pembatasan tautan:
- _Restricted:_ Hanya bisa dibuka oleh pengguna yang memiliki akun & sudah login.
- _Public:_ Dapat dibuka/diunduh oleh siapa saja di internet yang memiliki link/password.
- **Quota on Share (Owner Pays):** Ukuran file tetap dihitung ke quota **owner** asli; berbagi ke user lain tidak menduplikasi pemakaian quota.

#### 8. Core File Operations & Compression (Download as ZIP)

Fitur Download as ZIP adalah fitur core file manager yang tetap aktif dan terpasang secara bawaan tanpa memerlukan modul Add-on Manager:

- 📥 **Download as ZIP:** Mengunduh beberapa file atau folder sekaligus yang dipaketkan secara langsung sebagai satu berkas `.zip`.
- 🗜️ **Compress to ZIP:** Fitur untuk mengarsip beberapa berkas atau folder terpilih menjadi file `.zip` di dalam storage.

#### 9. Notifications & Feedback

- **In-App Toast & Notification Center:** Notifikasi aktivitas (seperti upload selesai, proses kompresi selesai, atau peringatan kuota user / node hampir penuh) ditampilkan via _Toast popup_ melayang dan panel notifikasi bawaan aplikasi.

---

### 3.2 Out-of-Scope (Sengaja Ditunda ke Fase Berikutnya / Roadmap)

Fitur-fitur berikut **secara eksplisit TIDAK dimasukkan** ke dalam rilis v1.0 untuk menjaga performa ringan dan menjamin target rilis 1 Oktober 2026:

- 🚫 **AI Workspace & MCP RAG Engine:** Tidak ada pemrosesan vector database, indeksasi dokumen RAG, maupun integrasi Ollama/LLM di v1.0 (ditunda ke Fase 2/3 Roadmap).
- 🚫 **Grid View (File Explorer):** v1.0 hanya **List View**; toggle tampilan grid ditunda ke fase berikutnya.
- 🚫 **File Versioning & History:** Tidak ada pencatatan riwayat versi file lama (v1, v2). Konflik nama tidak menimpa item lama; lihat kebijakan suffix di §4.4.
- 🚫 **Progressive Web App (PWA):** Tidak ada dukungan instalasi PWA atau _offline caching asset_ di v1.0.
- 🚫 **Desktop & Mobile Native Sync Client:** Tidak ada aplikasi _background sync_ (seperti Google Drive for Desktop).
- 🚫 **Real-time Collaborative Office Editing:** Tidak ada integrasi OnlyOffice/Collabora untuk edit dokumen bersama secara live.
- 🚫 **Email / SMTP Notification Integration:** Tidak ada pengiriman email otomatis untuk verifikasi, lupa password, atau notifikasi share link.
- 🚫 **Heavy Video Transcoding Engine:** Tidak ada konversi video otomatis via FFmpeg di server.
- 🚫 **Public SaaS Registration & Payment Gateway:** Tidak ada sistem pendaftaran publik bebas atau integrasi pembayaran berbayar.

---

### 3.3 Technical Constraints & Performance Assumptions (Batasan Teknis)

1. **Hardware Ceiling Target (Strict Target):**

- **Base Hardware Spec:** Intel Celeron 2-Core (Legacy), 2 GB RAM, 10 GB Storage OS.
- **RAM Footprint:** Idle RAM **< 200 MB**, Active Transfer Peak RAM **< 600 MB**.
- **LAN Response Time:** Respon perpindahan halaman drive **< 1.0 detik** di jaringan lokal.

2. **Framework & Architecture Constraints:**

- **Frontend:** SvelteKit (SPA Static Mode via `@sveltejs/adapter-static`), Svelte 5 Runes, Tailwind CSS v4, shadcn-svelte; build tooling **Bun** (`apps/frontend/AGENTS.md`).
- **UI/UX:** Wajib mengikuti [`apps/frontend/DESIGN.md`](../apps/frontend/DESIGN.md) — token warna via CSS variables, **dilarang** _arbitrary Tailwind values_.
- **Backend:** Go (Golang) Single Executable Binary dengan asset frontend di-embed via `go:embed`.
- **Database & Queue:** SQLite (WAL Mode) sebagai database utama sekaligus _Job Queue_ asinkron tanpa Redis/RabbitMQ.

3. **Network Constraints:**

- Pengunggahan berkas lokal menggunakan _chunked HTTP/TUS protocol_ untuk menjamin ketahanan terhadap koneksi yang terputus-putus (_network resiliency_).

---

## 4. Detailed Functional Requirements

### 4.1 Module 1: Onboarding & Initial Setup Wizard (`/setup`)

Modul ini bertanggung jawab menginsialisasi sistem saat pertama kali dijalankan (_first-boot_). Pengguna tidak perlu menyentuh terminal/CLI untuk konfigurasi awal.

```text
[First Boot Detected] ──► [/setup]
    ├── Step 1: Super Admin Account Setup (Min 8 Characters)
    ├── Step 2: Storage Node Discovery (Volume Path + Optional Devices)
    ├── Step 3: Placement Strategy Selection (Fill-First / Round-Robin / Most-Free)
    ├── Step 4: Optional Cloud Node Setup (S3 / Cloudflare R2 API Credentials)
    └── Step 5: Generate and export Recovery Key (`XXXXX-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX`)

```

#### Functional Specifications:

1. **Setup State Detection:**

- Backend Go memeriksa status `is_setup_completed` pada SQLite.
- Jika `false`, seluruh _route_ aplikasi selain `/setup` dan permukaan auth (`/login`, `/recovery`) akan di-redirect secara otomatis ke `/setup`.

2. **Step 1 — Super Admin Creation:**

- Form pembuatan akun Super Admin pertama (`username`, `email`, `password`).
- Validasi password: **Minimal 8 karakter**.

3. **Step 2 — Storage Node Discovery (Hybrid):**

- **Default (volume path):** Sistem memindai direktori `/app/data/storage/` untuk menemukan mount/volume yang terhubung (model Docker volume yang direkomendasikan).
- **Opsional (device discovery):** GUI dapat menampilkan device Linux terdeteksi (mis. `/dev/sdb`, `/dev/sdc`) agar Admin dapat mendaftarkan/mengaitkannya sebagai storage node.
- GUI menampilkan daftar node lokal beserta kapasitas total dan _free space_.
- Admin mencentang node mana saja yang ingin diaktifkan dan menetapkan **batas kapasitas node** (_node capacity cap_ — beda dari per-user My Storage quota).

4. **Step 3 — Placement Strategy Configuration:**

- Admin memilih strategi alokasi berkas default:
- **`FILL_FIRST`:** Mengisi disk pertama sampai penuh sebelum beralih ke disk berikutnya.
- **`ROUND_ROBIN`:** Membagi alokasi file secara bergiliran ke seluruh disk aktif.
- **`MOST_FREE_SPACE`:** Selalu memilih disk dengan sisa memori fisik terbesar.

5. **Step 4 — Optional Cloud Integration:**

- Input kredensial API untuk Cloudflare R2 atau AWS S3 (`Endpoint`, `Bucket Name`, `Access Key`, `Secret Key`).

6. **Step 5 — Emergency Recovery Key Export:**

- Browser generates a cryptographically secure random Recovery Key with 30 uppercase alphanumeric characters in six groups of five: `XXXXX-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX` (35 characters including separators).
- The key is shown for the user to copy or download. Setup stores the SHA-256 hash of its canonical 30-character value (without separators), not the plaintext key or a bcrypt hash.
- The Recovery Key can be used once to reset the Super Admin password. Successful reset consumes the key atomically and returns a newly generated key in `new_recovery_key`; the previous key is no longer usable. The user must save the replacement key.
- After setup, `is_setup_completed` is set to `true` and the system redirects to `/drive/home`.

---

### 4.2 Module 2: Virtual Storage Engine & Admin Storage Settings (`/admin/settings`)

Modul backend yang mengabstraksi lokasi penyimpanan fisik menjadi satu direktori virtual utuh (_Unified Directory_).

#### Functional Specifications:

1. **User-Side Abstraction (Transparent Storage):**

- Pengguna _end-user_ **tidak akan melihat dan tidak perlu tahu** lokasi fisik disk mana yang menyimpan file mereka.
- Badge node (`[R2]`, `[SSD-n]`) **hanya ditampilkan kepada Super Admin**.
- File pribadi ditampilkan di `/drive/my-storage`; Organization Drive di `/drive/organization`.

2. **System Health & 5% Safety Margin Rule:**

- Backend memantau sisa memori fisik setiap disk lokal secara berkala menggunakan `unix.Statfs`.
- **Rule Disk Kritis:** Jika sisa kapasitas fisik sebuah disk **< 5%**, disk tersebut otomatis ditandai `READ_ONLY`.
- **Auto-Failover Upload:** Jika pengguna mengunggah file baru saat disk utama dalam kondisi `READ_ONLY`, sistem secara transparan (tanpa _error_ ke pengguna) mengalihkan proses pengunggahan ke disk lokal/cloud lain yang masih memiliki kapasitas aman.
- **Admin Warning Notification:** Peringatan _"Storage Node [Nama Node] Hampir Habis (< 5%)"_ dikirimkan secara internal dan hanya ditampilkan kepada Admin melalui In-App Notification Center & Toast.
- **All Nodes Full:** Jika tidak ada node aman tersisa, upload gagal dengan pesan **"Penyimpanan server penuh"** (bukan pesan user-quota).

3. **Default User Quota Policy:**

- Di `/admin/settings`, Admin mengatur **Default User Quota** (GB) atau toggle **Unlimited for new users**.
- Perubahan default **hanya berlaku untuk user yang dibuat setelahnya** (bukan retroaktif ke user lama).
- Override per user dikelola di `/admin/users` (lihat §4.9).

4. **Rescan Disk Feature (Tanpa Entry Setup Wizard):**

- Di halaman `/admin/settings`, terdapat tombol **"Rescan Storage Nodes"**.
- Ketika diklik, backend Go memindai ulang `/app/data/storage/` (default) dan secara opsional menampilkan device Linux baru yang terdeteksi, tanpa perlu masuk kembali ke Setup Wizard.

5. **Graceful Detach Engine:**

- Admin dapat menekan tombol **"Detach Node"** pada disk tertentu.
- Sistem mengunci disk menjadi `READ_ONLY` dan memindahkan (_auto-migrate_) seluruh file fisik di dalamnya ke disk lain secara bertahap di latar belakang via SQLite Job Queue.

6. **Audit Log Retention Policy:**

- Di `/admin/settings`, Admin mengatur retensi log audit (`audit_log_retention_days`).
- **Default:** 90 hari. Opsi UI: **7, 14, 30, 60, 90** hari.
- Pembersihan otomatis via job `AUDIT_CLEANUP_JOB` (lihat [`docs/architecture.md`](./architecture.md) §6.2).

---

### 4.3 Module 3: Drive Home (`/drive/home`)

Landing default untuk pengguna terautentikasi setelah login atau selesai setup.

#### Functional Specifications:

1. **Quick Access Cards:** 5 berkas/folder terakhir dibuka oleh akun aktif (tersimpan per akun lintas perangkat).
2. **Recent Activities List:** 5 berkas/folder berbeda dengan aksi upload atau ubah terbaru (termasuk rename dan pindah folder) di My Storage dan Organization yang dilakukan oleh akun aktif; deduplikasi satu entri per berkas/folder.
3. **Storage Summary Widget:** Ringkasan kuota My Storage (used / limit GB, atau `Used: X GB` jika Unlimited).
4. **Quick Upload Drawer:** Unggah instan ke My Storage (tetap enforce user quota + node capacity).

---

### 4.4 Module 4: File Manager & Explorer (`/drive/my-storage`)

Antarmuka file pribadi berbasis **SvelteKit SPA** dengan **Flat Earthy Design System** (Montserrat, `bg-app` / `bg-surface`). **v1.0:** explorer hanya **List View** (tanpa toggle Grid View).

```text
+-------------------------------------------------------------------------------+
| Top Bar: [Search Input... ] [Filter: All v]  [ + New / Upload ]                    |
+-------------------------------------------------------------------------------+
| Breadcrumb: My Storage > Work Projects > Documents                            |
+-------------------------------------------------------------------------------+
| [📁 Personal Folder A]  [📁 Personal Folder B]                                |
|                                                                               |
| [📄 Document (1).pdf]     [🖼️ Photo.jpg]           [🎥 Video.mp4]             |
+-------------------------------------------------------------------------------+

```

#### Functional Specifications:

1. **File Operations:**

- Upload (file picker, drag-and-drop, and folder upload with nested contents), Direct Download, Rename, Move, Duplicate (_Zero-Byte Reference Copy_), Star/Unstar, Delete to Trash.
- Upload / duplicate / move ke My Storage **dicek user quota** sebelum diterima (lihat §4.10).
- Operasi folder berlaku rekursif terhadap seluruh isi folder. Penghapusan folder meminta konfirmasi bahwa seluruh isinya ikut dipindahkan ke Trash.
- Folder upload diproses parsial: item yang berhasil tetap tersimpan jika item lain gagal, dan UI menampilkan item yang gagal.

2. **Same-Path Name Conflicts:**

- Nama hanya konflik jika berada pada direktori (path) yang sama; item dengan nama sama di direktori berbeda diperbolehkan.
- Upload dan duplicate otomatis memilih nama tersedia berikutnya dengan suffix angka, misalnya `Laporan (1).pdf`, `Laporan (2).pdf`. Saat folder di-upload/duplicate, suffix diterapkan pada folder root yang konflik; isi folder mempertahankan nama asal kecuali ada konflik sibling pada path tujuan.
- Rename dan move tidak menimpa item. Jika nama bertabrakan di path tujuan, UI menawarkan nama tersedia berikutnya dengan suffix angka atau membatalkan aksi. Memindahkan folder tidak menggabungkan folder dengan nama sama.

3. **Upload Queue & Progress:**

- Saat file/folder diunggah, UI menampilkan notifikasi upload mengambang di kanan atas dengan progress bar. Chevron membuka daftar item yang sedang antre dan diunggah.
- Notifikasi progres tetap terbuka sampai ditutup pengguna melalui tombol close atau digeser ke kanan. Setelah antrean selesai, UI menampilkan Sonner toast biasa di kanan bawah.

4. **Search & Quick Category Filters:**


- **Fuzzy Text Search:** Pencarian instan berdasarkan nama file/folder.
- **Category Filter Chips:** Filter cepat berdasarkan tipe file:
- 🖼️ _Gambar_ (PNG, JPG, WebP, GIF, SVG)
- 🎥 _Video_ (MP4, WebM)
- 🎵 _Audio_ (MP3, WAV, OGG)
- 📄 _Dokumen_ (PDF, TXT, MD, CSV, DOCX)
- 🗜️ _Arsip_ (ZIP, RAR, 7Z)

5. **File Previewer bawaan:**

- _Double-click_ pada file membuka _lightbox modal_ untuk Gambar native, Video MP4/WebM dengan Vidstack, Audio MP3/WAV/OGG dengan perilaku yang sudah ada, Teks/Code/Markdown sebagai teks biasa dengan syntax highlighting PrismJS untuk kode yang didukung, dan PDF dengan PDF.js (`pdfjs-dist`).
- Pratinjau berlaku di My Storage, Organization, Shared, dan tautan publik/restricted sesuai izin berkas; PDF selalu tersedia tanpa add-on atau toggle Admin.
- Endpoint unduh yang sama menyediakan pratinjau hanya saat `?preview=1`: respons menggunakan `Content-Disposition: inline` dengan MIME yang diizinkan untuk preview dan mendukung HTTP Range bila tersedia pada storage. Tanpa parameter tersebut, perilaku unduh sebagai attachment tetap berlaku; otorisasi dan proteksi password share link tidak berubah.

6. **Network Resiliency & Resumable Transfer:**

- Menggunakan _chunked upload_ untuk disk lokal dan _multipart presigned transfer_ untuk S3/R2. Jika jaringan pengguna terputus saat upload/download, indikator _Progress Bar_ akan memposting status _Paused_ dan otomatis melanjutkan (_resume_) saat koneksi terhubung kembali.

---

### 4.5 Module 5: Trash Bin & Lifecycle Engine (`/drive/trash`)

Tempat penampungan sementara berkas yang dihapus pengguna.

#### Functional Specifications:

1. **Soft Delete Action:**

- Menghapus file/folder memindahkan status rekaman ke Trash Bin (`is_trashed = true`). File fisik di disk belum dihapus.
- Ukuran file di Trash **tetap dihitung** ke user quota sampai purge permanen.

2. **Automated 30-Day Purge Worker:**

- _Worker process_ di latar belakang Go memeriksa `trashed_at_timestamp` setiap hari pukul 00:00.
- File yang berada di Trash Bin **> 30 hari** akan dihapus secara permanen dari database dan disk fisik secara otomatis (usage quota berkurang).

3. **Restore Logic with Missing Parent Folder Handling:**

- Pengguna dapat menekan tombol **"Restore"** pada file di Trash Bin.
- **Handling Folder Induk Terhapus:** Jika folder asli tempat file tersebut berasal sudah dihapus secara permanen, sistem akan menampilkan **Modal Konfirmasi**:

  > _"Folder induk asli file ini telah dihapus. File akan dikembalikan ke lokasi utama (Root / My Storage). Lanjutkan?"_

- Jika disetujui, file di-restore dengan `parent_id = ROOT`.

---

### 4.6 Module 6: Starred Items (`/drive/starred`)

Shortcut berkas/folder yang ditandai bintang dari My Storage, Organization, atau Shared.

#### Functional Specifications:

1. **Star / Unstar:** Aksi tersedia di explorer; status disimpan per user.
2. **Starred Table:** Daftar agregat dari semua sumber; CTA cepat **"Hapus Bintang"**.
3. **Preview & Share:** Mendukung `?preview=file_id` dan Share Link Drawer sesuai izin asli berkas.

---

### 4.7 Module 7: Storage Stats (`/drive/storage-stats`)

Halaman statistik penyimpanan untuk pengguna terautentikasi.

#### Functional Specifications:

1. **Quota Widget:** `Used / Limit GB` (atau hanya `Used: X GB` jika Unlimited) — sinkron dengan widget sidebar.
2. **Usage Chart:** Breakdown per kategori (Video, Gambar, Dokumen, Arsip, Trash).
3. **Largest Files List:** 10 berkas terbesar milik user (My Storage + Trash; exclude Organization).
4. **Cleanup Advisor Modal:** Saran file besar jarang dibuka untuk dipindah ke Trash.
5. **Shortcut:** `Kelola Sampah` → `/drive/trash`.

---

### 4.8 Module 8: Access Control, Sharing, & Organization Drive

Modul untuk mengelola hak akses antar-pengguna internal serta direktori bersama organisasi.

#### Functional Specifications:

1. **User-to-User Internal Sharing (`/drive/shared`):**

- Pengguna dapat membagikan file/folder ke pengguna terdaftar lain dengan menetapkan tingkat hak akses:
- **`Viewer`:** Hanya memiliki izin melihat dan mengunduh (_Read Only_).
- **`Editor`:** Memiliki izin melihat, mengunduh, menambahkan file baru, mengedit, dan menghapus file di dalam folder tersebut (_Read, Write, Delete_).
- **Quota:** Owner pays — pemakaian tetap dihitung ke owner asli.

2. **Organization Drive (`/drive/organization`):**

- **Fixed shared tree v1.0:** Organization Drive adalah pohon virtual **tetap** yang diakses semua user terdaftar lewat sidebar **Organization** (`/drive/organization`) dengan badge 🏢. **Tidak ada** UI v1.0 untuk Super Admin "menandai" folder My Storage sebagai Organization Folder.
- **Universal Full Access:** Read, Write, Download, Rename, **Delete** tanpa pengaturan per-user.
- **Quota:** File Organization **tidak** dihitung ke user quota mana pun; tetap memakai kapasitas fisik pool (node 5% / failover tetap berlaku).
- Pindah file Organization → My Storage: cek user quota sebelum move; jika tidak muat → tolak.

---

### 4.9 Module 9: User Management & Quota Overrides (`/admin/users`)

Halaman Super Admin untuk mengelola akun dan override kuota My Storage.

#### Functional Specifications:

1. **User Table:** Username, email, role, quota efektif (Default / Override GB / Unlimited), used GB, status.
2. **Create User:** Membuat Regular User baru; kuota awal mengikuti **Default User Quota** di settings (kecuali di-_override_ saat create).
3. **Edit Quota Modal:** Set angka GB atau toggle **Unlimited**; opsi **Reset to Default**.
4. **Super Admin rows:** Selalu menampilkan Unlimited (tidak dapat dipaksa limit user-quota).

---

### 4.10 Module 10: User Storage Quota Engine

Enforcement kuota logis per user untuk My Storage (lapisan terpisah dari node capacity).

#### Functional Specifications:

1. **What Counts Toward Usage:**

- Semua file dengan `owner_id = user` yang **bukan** bagian Organization tree (termasuk file di Trash sampai purge).
- **Zero-byte copy:** owner sama → usage tidak bertambah; copy ke owner berbeda → usage bertambah pada **owner baru** sebesar ukuran logical file.

2. **Effective Limit Resolution:**

```text
if user.role == SuperAdmin → Unlimited
else if user.storage_quota_override is set → override (GB | Unlimited)
else → system.default_user_quota (GB | Unlimited)
```

3. **Enforcement on My Storage Writes:**

- Upload / import / move-into-personal / copy-to-other-owner:
  - Jika usage ≥ 80% limit → Toast warning (masih boleh lanjut selama di bawah 100%).
  - Jika usage + ukuran baru melebihi limit → **reject** dengan kode `USER_QUOTA_EXCEEDED` dan pesan _"Kuota My Storage penuh. Hubungi admin."_
- Organization writes: **skip** user-quota check (tetap cek node capacity).

4. **Physical Pool Fallback (Existing):**

- Node di bawah 5% → `READ_ONLY` + auto-failover; semua node penuh → `"Penyimpanan server penuh"`.
- Pesan error **wajib berbeda** dari `USER_QUOTA_EXCEEDED`.

---

### 4.11 Module 11: Share Link Engine (`/s/:token`)

Modul untuk membagikan file/folder melalui tautan, dengan mode **Public** atau **Restricted**.

#### Functional Specifications:

1. **Share Link Generation:**

- Menerbitkan URL unik dengan token acak (contoh: `https://cubycloud.domain/s/aB1xY9z`).

2. **Security Controls:**

- **Password Protection (Opsional):** Kunci link dengan kata sandi (di-hash menggunakan bcrypt). Pengunjung di-redirect ke halaman _Password Challenge_ sebelum dapat melihat isi file.
- **Expiration Date (Opsional):** Menetapkan tanggal kedaluwarsa tautan. Tautan otomatis hangus (404 / Expired) setelah melewati tanggal tersebut.
- **Access Level Setting:**
- _Restricted:_ Hanya bisa dibuka jika pengunjung sudah login ke akun CubyCloud.
- _Public:_ Dapat dibuka langsung oleh siapapun di internet.

3. **Unlimited Downloads:**

- Tidak ada batasan jumlah unduhan untuk tautan berbagi.

---

### 4.12 Module 12: Core Archive & Worker Queue

Fitur pengarsipan dan kompresi (termasuk Download as ZIP) merupakan kapabilitas inti (_core file manager_) yang selalu aktif dan terpasang secara bawaan. Route `/admin/addons` (Add-on Manager) ditiadakan.

#### Functional Specifications:

1. **Core Archive & Download as ZIP:**

- 📥 **Download as ZIP:** Mengunduh beberapa file/folder sekaligus yang di-stream secara langsung sebagai berkas `.zip` (fitur core bawaan).
- 🗜️ **Compress to ZIP:** Membuat arsip `.zip` dari beberapa file/folder terpilih di dalam storage.
2. **Configurable Worker Concurrency Limit:**

- Di halaman `/admin/settings`, Admin dapat mengatur jumlah maksimal proses konversi/kompresi berat yang boleh berjalan bersamaan (_Concurrent Worker Limit_).
- **Default Value:** **2 Proses** (disesuaikan untuk spesifikasi server Celeron 2GB RAM).
- Jika antrean melebihi limit, proses baru akan berstatus `PENDING` di dalam SQLite Transactional Job Queue dan berjalan otomatis begitu antrean sebelumnya selesai.

3. **Error Handling & Fault Tolerance:**

- Jika proses konversi/kompresi gagal (misal karena disk penuh atau file corrupt):
- Worker membatalkan tugas secara aman (_rollback_).
- Peringatan dikirimkan via **Toast Error Popup** ("_Gagal mengompresi file: Storage penuh_").
- Pesan error dicatat di **In-App Notification Center** pengguna dan **Internal System Audit Log** milik Admin.

---

### Summary of Data Entity & Access Rules Matrix

| Fitur / Modul | Role Pengguna | Hak Akses Utama |
| ------------- | ------------- | --------------- |
| **Setup Wizard (`/setup`)** | Unauthenticated (First-Boot Only) | Membuat Admin, Pindai Disk, Ekspor Recovery Key |
| **Auth (`/login`, `/recovery`)** | Unauthenticated | Login JWT; reset Super Admin via Recovery Key |
| **Beranda (`/drive/home`)** | Regular User & Super Admin | Quick access, activity, quota summary |
| **My Storage (`/drive/my-storage`)** | Regular User & Super Admin | CRUD pribadi; node badge **hidden** untuk Regular User |
| **Organization (`/drive/organization`)** | All Registered Users | Full Access; **no user quota** |
| **Starred / Shared / Trash / Stats** | Authenticated | Sesuai izin sumber; stats = own usage |
| **User-to-User Share** | Target User | `Viewer` atau `Editor` (owner pays quota) |
| **Share Link (`/s/:token`)** | Public / Restricted Guest | Viewer + Password & Expiration |
| **Admin Settings (`/admin/settings`)** | Super Admin Only | Nodes, workers, **default user quota**, DB backup |
| **Admin Users (`/admin/users`)** | Super Admin Only | Create users, **per-user quota override** |

---

## 5. Non-Functional Requirements (NFRs)

### 5.1 Performance & Resource Efficiency

Modul backend Go dan frontend SvelteKit SPA dirancang dengan efisiensi tinggi agar dapat berjalan secara optimal pada _hardware_ berspesifikasi terbatas (_legacy hardware_).

| Parameter / Kriteria          | Target Threshold / Constraint | Keterangan & Lingkungan Pengujian                                                    |
| ----------------------------- | ----------------------------- | ------------------------------------------------------------------------------------ |
| **API Latency (LAN)**         | **< 50 ms**                   | Waktu respon API Go Backend untuk fetching metadata/list berkas di jaringan lokal    |
| **Page Navigation (LAN)**     | **< 1.0 Detik**               | Respon perpindahan antar halaman utama (`/drive/my-storage`, `/drive/shared`, dll.)  |
| **CPU Idle Usage**            | **< 10% Utilization**         | Penggunaan CPU server saat tidak ada aktivitas transaksi data (Intel Celeron 2-Core) |
| **Memory Footprint (Idle)**   | **< 200 MB RAM**              | Konsumsi RAM total kontainer Docker saat sistem berstatus idle                       |
| **Memory Footprint (Active)** | **< 600 MB RAM**              | Batas konsumsi RAM puncak saat proses transfer berkas atau kompresi ZIP              |
| **Direct Transfer Overhead**  | **~0% CPU & Bandwidth**       | Transfer ke cloud storage (S3/R2) via Presigned URL tanpa melewati Go Server         |

---

### 5.2 Security & Session Management

Keamanan difokuskan pada perlindungan data kredensial, kontrol akses API, dan pencegahan serangan siber tanpa membebani performa I/O server.

1. **JWT-Based Session Management:**

- Autentikasi berbasis **JSON Web Token (JWT)** yang ditandatangani secara kriptografis menggunakan algoritma `HS256`.
- Token disimpan pada **HTTP-Only Cookie** dengan konfigurasi `SameSite=Lax` dan atribut `Secure` (saat berjalan di HTTPS).
- _Token Expiration:_ Akses token berlaku selama **24 jam**; pengguna login ulang setelah kedaluwarsa. **Tidak ada refresh token terpisah** di v1.0 (menjaga implementasi ringan).

2. **Password Security:**

- Seluruh kata sandi pengguna (Super Admin & Regular Users) serta kata sandi proteksi tautan berbagi wajib di-hash menggunakan **bcrypt** (dengan _work factor_ / cost = 10).

3. **Storage Security (No Application-Level Encryption at Rest):**

- Berkas fisik yang tersimpan di disk lokal (SSD/HDD) **tidak dienkripsi di tingkat aplikasi** (_Plain-FileSystem Storage_) untuk menghindari pengikatan beban CPU (_CPU throttling_) akibat proses enkripsi/dekripsi _real-time_ pada prosesor Intel Celeron.
- Keamanan data berkas fisik sepenuhnya mengandalkan batas hak akses sistem operasi (_POSIX File Permissions_) dan isolasi volume kontainer Docker.

4. **Emergency Access Recovery:**

- The current emergency recovery credential is the 30-character uppercase alphanumeric key generated in six groups of five during setup. Its canonical separator-free value is stored as a SHA-256 hash. Reset is single-use and rotates the key; see §4.1.6.
- Migration-only compatibility: the backend accepts a legacy 64-character hexadecimal key only for accounts that still have a bcrypt recovery hash. Successful legacy recovery rotates the account to the current 6×5 format with SHA-256 storage. The legacy format is not generated or accepted by the UI.

---

### 5.3 Data Integrity & Fault Tolerance

Menjamin konsistensi database dan file fisik bahkan ketika terjadi kegagalan sistem atau penghentian daya secara mendadak (_unclean shutdown_ / mati listrik).

1. **SQLite Write-Ahead Logging (WAL) Mode:**

- Database SQLite dikonfigurasi dalam mode **WAL Mode (`PRAGMA journal_mode=WAL;`)** dengan `synchronous=NORMAL` untuk menjamin ketahanan transaksi data (_ACID Compliance_) serta mencegah korupsi database saat listrik padam mendadak.

2. **Orphan File Sweeper (Startup Garbage Collection):**

- Setiap kali aplikasi _booting_ (kontainer dinyalakan), _background process_ **Orphan File Sweeper** akan melakukan audit pencocokan antara berkas fisik di disk dan rekaman di tabel `physical_blobs`.
- Berkas parsial/sampah sisa pengunggahan yang terputus di tengah jalan akibat mati listrik akan otomatis dibersihkan dari disk untuk menghemat kapasitas penyimpanan.

3. **Transactional Job Queue Recovery:**

- Antrean pekerjaan (_Job Queue_) untuk kompresi ZIP dan konversi gambar yang terputus di tengah jalan akan ditandai `FAILED` saat _booting_, dan sistem mengizinkan pengguna melakukan _retry_ pekerjaan secara aman.

---

### 5.4 Maintainability, Portability, & Backup

Kemudahan pemeliharaan sistem, portabilitas kontainer, serta backup data bagi Admin _homelab_.

1. **Docker Container Footprint:**

- **Target Docker Image Size:** **< 150 MB** secara keseluruhan.
- Dipublikasikan menggunakan teknik _Multi-Stage Docker Build_ berbasis image minimalist (**Alpine Linux** / **Scratch**) yang berisi biner Go terkompilasi statis beserta asset SvelteKit SPA bawaan.

2. **One-Click Database Backup:**

- Pada halaman `/admin/settings`, tersedia tombol **"Download DB Backup"**.
- Admin dapat mendownload berkas snapshot SQLite (`cubycloud.db`) secara langsung dari Web GUI untuk keperluan _backup & disaster recovery_ berkala tanpa perlu akses terminal.

3. **Zero External Runtime Dependencies:**

- Sistem tidak memerlukan instalasi dependensi runtime eksternal seperti Node.js, Python, PHP, Redis, MinIO, atau PostgreSQL. Seluruh fungsi inti berjalan murni di dalam 1 kontainer Go + SQLite.

---

### 5.5 Browser & Platform Compatibility Matrix

Antarmuka **SvelteKit SPA** dirancang dengan standar web modern agar dapat diakses dengan lancar dan responsif dari berbagai _device_ dan sistem operasi.

```text
                               ┌─────────────────────────────────────────┐
                               │     CUBYCLOUD CROSS-PLATFORM SUPPORT    │
                               └────────────────────┬────────────────────┘
                                                    │
         ┌──────────────────────────────────────────┼──────────────────────────────────────────┐
         ▼                                          ▼                                          ▼
┌─────────────────────────┐              ┌─────────────────────────┐              ┌─────────────────────────┐
│     Desktop Browsers    │              │      Apple Ecosystem    │              │     Mobile & Tablets    │
│ (Chrome, Firefox, Edge) │              │  (macOS, iOS, iPadOS)   │              │   (Android Web Browsers)│
└─────────────────────────┘              └─────────────────────────┘              └─────────────────────────┘

```

#### Support Matrix (2 Tahun Terakhir):

| OS / Platform               | Browser Target                                                   | Status Kompatibilitas               |
| --------------------------- | ---------------------------------------------------------------- | ----------------------------------- |
| **Windows / Linux / macOS** | Google Chrome, Mozilla Firefox, Microsoft Edge (Versi 2024–2026) | **Full Support** (Optimal Layout)   |
| **Apple macOS**             | Apple Safari (macOS 14+ / Safari 17+)                            | **Full Support**                    |
| **Apple iOS & iPadOS**      | Safari Mobile, Chrome iOS (WebKit Engine 2024–2026)              | **Full Support** (Touch Responsive) |
| **Android**                 | Chrome Mobile, Firefox Mobile (2024–2026)                        | **Full Support** (Touch Responsive) |

- **Web Standards Compliance:** Menggunakan HTML5 standar, ES2022+ JavaScript, CSS Grid/Flexbox, serta Fetch API standar tanpa _vendor-specific polyfill_ yang memberatkan bundle size.

---

## 6. System Dependencies & Technical Stack

Dokumen ini mengunci seluruh _technology stack_, dependensi pustaka (_libraries_), dan _tooling_ pengembangan untuk CubyCloud v1.0. Pemilihan _stack_ didasarkan pada prinsip utama: **Performa tinggi, jejak memori ultra-ringan (Idle RAM < 200 MB), serta nol dependensi eksternal (_Zero External Dependencies_)**.

---

### 6.1 Backend Stack (Go Core Engine)

Backend dikembangkan menggunakan **Go (Golang)** dengan gaya arsitektur _Clean Architecture / Domain-Driven Design (DDD)_ yang dikompilasi menjadi _Single Binary Executable_.

| Komponen / Layer           | Teknologi / Pustaka           | Versi Target | Alasan Pemilihan & Fungsi                                                                                                               |
| -------------------------- | ----------------------------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------------- |
| **Programming Language**   | **Go (Golang)**               | `v1.22+`     | Performa _native_, konkurensi Goroutine matang, alokasi RAM sangat kecil, dan kemampuan `go:embed`.                                     |
| **Web Framework**          | **Echo** (`labstack/echo/v4`) | `v4.12+`     | Minimalis, alokasi memori sangat hemat, kompatibel dengan standard `net/http`, dan memiliki native `StaticFS` untuk `go:embed`.         |
| **Database Engine**        | **SQLite** (WAL Mode)         | `v3.45+`     | Embedded RDBMS tanpa _overhead_ service terpisah. Mode WAL (_Write-Ahead Logging_) menjamin performa I/O tinggi & ACID _compliance_.    |
| **ORM & Data Access**      | **Bun ORM** (`github.com/uptrace/bun`) | `v1.2+` | SQL-first di atas `database/sql` (bukan codegen). Struct Go + tag `bun`; skema kanonik tetap file SQL di `migrations/`. **Bukan** Bun runtime JavaScript (`bun.sh`) yang dipakai frontend. |
| **SQLite Driver**          | **`glebarez/go-sqlite`**      | `v1.2+`      | Driver SQLite _Pure Go_ (CGO-Free). Memungkinkan _cross-compilation_ tanpa GCC dan menjaga ukuran Docker Image tetap kecil.             |
| **Object Storage SDK**     | **AWS SDK for Go v2**         | `v1.30+`     | Penerbitan _Presigned Upload/Download URLs_ untuk Cloudflare R2 dan AWS S3 secara aman & hemat memori.                                  |
| **Authentication & Token** | **`golang-jwt/jwt/v5`**       | `v5.2+`      | Penanganan token JWT (_JSON Web Token_) untuk _stateless session management_.                                                           |
| **Password Hashing**       | **`golang.org/x/crypto`**     | `latest`     | Pengolahan kata sandi menggunakan **bcrypt** (_cost factor_ = 10).                                                                      |

---

### 6.2 Frontend Stack (SvelteKit User Interface)

Frontend dikembangkan sebagai **Single Page Application (SPA)** yang di-build menjadi aset statis (HTML/JS/CSS) dan di-_embed_ langsung ke dalam biner Go.

| Komponen / Layer       | Teknologi / Pustaka                 | Versi Target | Alasan Pemilihan & Fungsi                                                                                                          |
| ---------------------- | ----------------------------------- | ------------ | ---------------------------------------------------------------------------------------------------------------------------------- |
| **Frontend Framework** | **SvelteKit**                       | `v2.0+`      | Framework reaktif, _bundle size_ JS sangat kecil; state lokal via **Svelte 5 Runes** (`$state`, `$derived`, `$effect`).                               |
| **Build Adapter**      | **`@sveltejs/adapter-static`**      | `v3.0+`      | Mengompilasi aplikasi SvelteKit menjadi SPA statis murni tanpa membutuhkan Node.js runtime di server produksi.                     |
| **Styling Engine**     | **Tailwind CSS**                    | `v4.x`       | Utility-first CSS v4 dengan token `@theme inline` di `layout.css`; tema **Flat Earthy** sesuai [`DESIGN.md`](../apps/frontend/DESIGN.md). |
| **Typography**         | **Montserrat** (Google Fonts)       | —            | Satu-satunya _font family_ UI; skala hierarki didefinisikan di DESIGN.md §3.                                                              |
| **UI Components**      | **shadcn-svelte**                   | `latest`     | Komponen _headless & accessible_ berbasis `bits-ui`; di-_theme_ dengan token CubyCloud (primary moss, danger rose).                        |
| **Icons Library**      | **Lucide Svelte** (`lucide-svelte`) | `v0.400+`    | Set ikon SVG geometris yang ringan dan konsisten.                                                                                  |
| **Resumable Upload**   | **TUS Client / Fetch Chunking**     | `latest`     | Menangani _chunked upload_ untuk menjamin unggahan file besar dapat di-_resume_ saat koneksi jaringan terputus.                    |
| **Package Manager**    | **Bun** (`bun.sh`)                  | `1.x`        | Runtime/PM JavaScript untuk frontend (`bun install`, `bun run build`). Bukan Bun ORM Go. |

---

### 6.3 Database, Queue, & Background Processing Architecture

Untuk menjaga konsumsi sumber daya tetap hemat, CubyCloud **tidak menggunakan Redis, RabbitMQ, atau Celery**.

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        CUBYCLOUD BACKEND PROCESS                       │
│                                                                        │
│  ┌───────────────────┐    SQL Insert    ┌───────────────────────────┐  │
│  │ Echo Web Handlers │ ───────────────► │ SQLite Transactional Job  │  │
│  └───────────────────┘                  │ Queue Table (`jobs`)      │  │
│                                         └─────────────┬─────────────┘  │
│                                                       │ Poll / Claim   │
│                                                       ▼                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Go Worker Pool (Goroutines Limit: Configurable Default 2 Worker) │  │
│  │                                                                  │  │
│  │  [ Worker 1: Compress ZIP ]     [ Worker 2: Convert Image ]      │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────┘

```

1. **SQLite Transactional Queue:**

- Pekerjaan latar belakang (_ZIP compression_, _Image format conversion_, _Orphan file sweeping_) dicatat ke dalam tabel SQLite `jobs`.
- _Job Status Lifecycle:_ `PENDING` ──► `PROCESSING` ──► `COMPLETED` / `FAILED`.

2. **Goroutine Worker Pool:**

- Backend Go menjalankan _Worker Pool_ internal yang memantau tabel `jobs` secara asinkron.
- _Concurrency Constraint:_ Jumlah worker aktif dibatasi oleh konfigurasi Admin (Default: **2 Worker**) untuk mencegah _CPU Throttling_ pada prosesor Intel Celeron.

---

### 6.4 Build Automation & Development Tooling

Tooling yang digunakan untuk mempermudah proses alur kerja pengembangan (_developer experience_) dan otomatisasi _release build_.

| Tool                                   | Penggunaan & Fungsi                                                                                                                                                    |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **`Makefile`**                         | Alat otomatisasi _build pipeline_: kompilasi frontend SvelteKit, penyalinan asset statis, kompilasi biner Go, hingga pengemasan Docker Image. |
| **`Air`** (`cosmtrek/air`)             | Tool _live-reload_ untuk Go Backend selama fase pengembangan lokal. Otomatis mengompilasi ulang biner Go saat ada perubahan kode `.go`.                                |
| **Docker Multi-Stage Build**           | Pengemasan kontainer produksi:                                                                                                                                         |

- _Stage 1 (Bun Build):_ Compile SvelteKit SPA (`bun run build` di `apps/frontend/`).
- _Stage 2 (Go Build):_ Compile biner Go + Embed static assets.
- _Stage 3 (Final Image):_ Base image **Alpine Linux** (`alpine:latest`) menghasilkan final image **< 150 MB**. |

---

### 6.5 System Dependencies & Runtime Summary Matrix

```text
+-----------------------------------------------------------------------+
|                       CUBYCLOUD RUNTIME STACK                         |
+-----------------------------------------------------------------------+
|  [Frontend SPA]   SvelteKit + Tailwind CSS v4 + Montserrat + shadcn-svelte + Bits UI  |
|         │                                                             |
|         ▼ (Embedded via go:embed at compile time)                     |
|  [Backend App]    Go v1.22+ Engine + Echo Router + Bun ORM (uptrace/bun) |
|         │                                                             |
|         ├──────────────► Local File System (/app/data/storage/nodes)  |
|         ├──────────────► Embedded SQLite WAL (glebarez/go-sqlite)     |
|         └──────────────► AWS SDK v2 (Cloudflare R2 / AWS S3 Presigned)|
+-----------------------------------------------------------------------+
|  [Deployment]     1 Single Docker Container (Target Size < 150 MB)    |
+-----------------------------------------------------------------------+

```
