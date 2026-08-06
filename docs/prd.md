# Product Requirement Document (PRD)

## CubyCloud — Self-Hosted Modular Cloud Storage & AI Workspace

---

## 1. Document Control & Executive Summary

### 1.1 Document Control

| Parameter | Detail |
| --- | --- |
| **Nama Proyek** | **CubyCloud** |
| **Penyusun (Author)** | **Septian Padli** |
| **Versi Dokumen** | `v1.0.0 (Draft)` |
| **Status Dokumen** | In Review / Draft |
| **Tanggal Pembuatan** | 7 Agustus 2026 |
| **Target Launching** | **1 Oktober 2026** |
| **Target Audience** | Core Engineering Team, UI/UX Designer, Open-Source Contributors |

---

### 1.2 Executive Summary

**CubyCloud** adalah aplikasi *self-hosted modular cloud storage* modern yang dirancang sebagai alternatif berkinerja tinggi untuk **Nextcloud** dan **Google Drive**. CubyCloud hadir untuk menyelesaikan masalah kompleksitas instalasi dan konsumsi sumber daya (*resource hogging*) yang sering dialami oleh pengguna platform *self-hosted* tradisional.

Dengan pendekatan **Single-Container Architecture** (Go backend dengan embedded SvelteKit frontend), CubyCloud memberikan pengalaman menggunakan Google Drive pribadi tanpa memerlukan konfigurasi CLI (*Command Line Interface*) yang rumit. Mulai dari proses instalasi awal (*onboarding*), pengalokasian penyimpanan disk lokal, hingga integrasi cloud pihak ketiga (S3/R2) dapat dilakukan seluruhnya melalui antarmuka Web GUI berbasis **Neo-Notion**.

Selain sebagai penyimpanan berkas, CubyCloud dilengkapi dengan **AI Workspace & MCP RAG Engine** bawaan (bergaya *NotebookLM*). Fitur ini memungkinkan pengguna menjadikan folder tertentu sebagai basis pengetahuan (*knowledge base*) AI yang dapat diajak berdiskusi secara privat menggunakan model LLM lokal (via Ollama) maupun cloud provider, tanpa mengorbankan privasi data.

---

### 1.3 Problem Statement & Opportunity

* **Problem (Masalah):**
1. **Kompleksitas & Resource Heavy:** Platform seperti Nextcloud cenderung berat, membutuhkan spesifikasi server yang tinggi, dan sering kali membingungkan pengguna awam karena banyaknya ketergantungan *dependency* serta pengaturan CLI.
2. **Vendor Lock-in & Privasi:** Layanan cloud komersial (seperti Google Drive) memungut biaya langganan bulanan yang terus meningkat dan memproses data/dokumen pengguna pada server pihak ketiga untuk pelatihan model AI.
3. **Fragmentasi Media Penyimpanan:** Pengguna *homelab* kesulitan menggabungkan beberapa harddisk/SSD bekas dengan kapasitas berbeda serta bucket cloud hemat biaya (seperti Cloudflare R2) ke dalam satu direktori tunggal yang terpadu.


* **Opportunity (Peluang):**
CubyCloud mengisi celah pasar dengan menghadirkan cloud storage yang **ringan, estetik, modular, dan fleksibel**, mampu berjalan lancar pada perangkat server kelas bawah (*legacy/budget hardware*) tanpa mengurangi fitur-fitur modern seperti transfer langsung (*presigned direct transfer*), penduplikatan file tanpa makan memori (*zero-byte copy*), dan kecerdasan buatan berbasis RAG.

---

### 1.4 Vision & Position Statement

> **"Google Drive versi self-hosted yang ultra-ringan, modern, dan modular. Tanpa keribetan perintah CLI, siap berjalan di home server berdaya rendah."**

---

### 1.5 Success Metrics & Key Performance Indicators (KPIs)

Untuk memastikan CubyCloud memenuhi kriteria performa pada perangkat berspesifikasi minim, berikut adalah target indikator keberhasilan teknis dan pengalaman pengguna (*User Experience*):

#### A. Performa & Resource Efficiency (Target Utama)

| Metric / KPI | Target Benchmark | Keterangan / Conditions |
| --- | --- | --- |
| **Base Hardware Spec** | Intel Celeron 2-Core (Legacy) + 2 GB RAM | Spesifikasi batas minimum server target |
| **Idle RAM Consumption** | **< 200 MB** | Penggunaan memori saat tidak ada aktivitas pengguna |
| **Active RAM Consumption** | **< 600 MB** | Penggunaan memori puncak saat proses upload/download intensif |
| **Page Navigation Latency** | **< 1.0 Detik** | Respon waktu pembukaan antar halaman drive pada jaringan LAN lokal |
| **First Contentful Paint (FCP)** | **< 800 ms** | Kecepatan pemuatan awal antarmuka frontend SvelteKit |

#### B. Functional & Deployment KPIs

* **Zero-CLI Onboarding:** 100% proses konfigurasi awal (*Super Admin, Storage Node Discovery, Placement Strategy*) selesai melalui Web Setup Wizard dalam waktu **< 3 menit**.
* **Deployment Footprint:** Didistribusikan dalam bentuk **1 Image Docker** atau **1 Single Executable Binary** tanpa butuh *dependency* eksternal tambahan.
* **Direct Transfer Efficiency:** 0% beban CPU server terbuang untuk *bouncing data* saat melakukan upload/download berkas dari penyedia cloud (AWS S3 / Cloudflare R2) berkat penggunaan *Presigned URLs*.


---

## 2. Target Users & User Personas

---

### 2.1 Target User Segmentation

CubyCloud ditujukan untuk ekosistem pengguna *self-hosted* dan tim skala kecil/menengah yang membutuhkan efisiensi penyimpanan, privasi tinggi, dan antarmuka modern tanpa dibebani oleh *overhead* teknis.

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

* **Role / Demografi:** IT Support Specialist / Hobbyist Homelab.
* **Lingkungan Hardware:** Home server berbasis PC tua/bekas (Intel Celeron 2-Core, 2 GB RAM, 3 unit SSD/HDD bervariasi).
* **Tingkat Keahlian Teknis:** Menengah ke Atas (Bisa menjalankan `docker run` / `docker-compose`, namun enggan menghabiskan waktu merawat file konfigurasi CLI yang rumit).
* **Goals (Tujuan):**
* Menggabungkan seluruh harddisk bekas dan akun Cloudflare R2 gratisan menjadi satu direktori penyimpanan besar (*Unified Directory*).
* Memiliki cloud storage pribadi yang berjalan ringan di server tua tanpa menghabiskan RAM/CPU.


* **Pain Points (Kekesalan):**
* **Frustrasi pada Nextcloud:** Sangat berat (*resource-hogging*), lambat di server spek Celeron, dan sering *breakdown* saat proses pembaruan versi via CLI.
* **Frustrasi pada NAS Proprietary (Synology/QNAP):** Biaya *hardware* resmi terlalu mahal.


* **Solusi CubyCloud:**
* Cukup jalankan 1 container Docker, seluruh konfigurasi disk dan alokasi kuota diatur lewat **Web Setup Wizard**.
* Penggunaan RAM idle **< 200 MB** sehingga server tidak *overheat* atau *crash*.



---

#### Persona B: Privacy-Conscious Researcher & Creator ("Siti, 24")

* **Role / Demografi:** Mahasiswa Tingkat Akhir / Penulis / Researcher.
* **Lingkungan Hardware:** Laptop kerja harian + Akses ke CubyCloud Server milik tim/pribadi.
* **Tingkat Keahlian Teknis:** Pemula - Menengah (Paham dasar penggunaan web app, tidak mengerti manajemen server).
* **Goals (Tujuan):**
* Memiliki basis pengetahuan (*knowledge base*) pribadi untuk ratusan dokumen jurnal PDF, skripsi, dan catatan riset.
* Menggunakan kecerdasan buatan (AI) untuk merangkum dan bertanya jawab tentang isi dokumen tanpa khawatir data risetnya digunakan untuk melatih LLM publik.


* **Pain Points (Kekesalan):**
* **Frustrasi pada Google Drive / ChatGPT Cloud:** Khawatir dokumen privat diuji/di-scrape oleh vendor cloud.
* **Frustrasi pada Aplikasi AI Terpisah:** Malas mengunggah ulang file satu per satu ke platform AI eksternal setiap kali ingin melakukan riset.


* **Solusi CubyCloud:**
* Klik kanan pada folder dokumen -> **"Set as AI Workspace"**.
* Berdiskusi langsung dengan dokumen melalui panel chat berbasis **Ollama (Llama 3 Local)** yang 100% terisolasi secara privat.



---

#### Persona C: Non-Tech End User & Family Member ("Rian, 32")

* **Role / Demografi:** Karyawan Swasta / Anggota Keluarga Pengguna Server.
* **Lingkungan Hardware:** Smartphone & Web Browser Desktop.
* **Tingkat Keahlian Teknis:** Pemula (Hanya familiar dengan antarmuka populer seperti Google Drive, Notion, atau WhatsApp).
* **Goals (Tujuan):**
* Mengunggah, mengunduh, dan berbagi foto/dokumen harian dengan cepat dan tanpa hambatan teknis.
* Melihat pratinjau file (*preview*) langsung di browser tanpa perlu mengunduhnya terlebih dahulu.


* **Pain Points (Kekesalan):**
* Antarmuka aplikasi *self-hosted* pada umumnya terasa kaku, membingungkan, dan lambat.
* Tautan berbagi (*share link*) yang rumit tanpa fitur keamanan dasar.


* **Solusi CubyCloud:**
* Antarmuka **Neo-Notion** yang bersih, responsif, dan familiar.
* Fitur berbagi file dengan kata sandi (*Password Protection*) dan masa berlaku (*Expiration Date*) yang instan.



---

### 2.3 User Frustrations & Competitive Pain Point Matrix

| Dimensi Evaluasi | Google Drive (Public Cloud) | Nextcloud (Self-Hosted Traditional) | **CubyCloud (Solusi Kita)** |
| --- | --- | --- | --- |
| **Biaya & Kuota** | Berlangganan bulanan mahal; kuota terbatas. | Gratis (Self-hosted), namun butuh server mahal agar lancar. | **Gratis & Hemat:** Optimal di PC tua Celeron 2 GB RAM. |
| **Penggunaan Resource** | N/A (Cloud Vendor) | **Sangat Berat:** RAM idle > 1-2 GB, sering *lag* di LAN. | **Ultra-Ringan:** RAM idle **< 200 MB**, respon LAN **< 1s**. |
| **Kompleksitas Setup** | Tanpa Setup (Instan). | **Sangat Rumit:** Membutuhkan instalasi PHP, DB, Redis, CLI. | **Zero-CLI Wizard:** 1 Command Docker -> Web Wizard GUI. |
| **Manajemen Storage** | Terkunci pada kuota vendor. | Terbatas pada konfigurasi mount titik awal Linux. | **Unified Directory:** Gabung SSD 1, SSD 2, & Cloud R2/S3. |
| **Privasi Data & AI** | Risiko pemrosesan data oleh vendor AI. | Butuh plugin eksternal yang rumit diatur. | **Built-in Private RAG:** MCP Host terintegrasi dengan Ollama. |

---

### 2.4 Critical User Workflows (Core User Journeys)

#### 1. Onboarding Pertama Kali (First-Boot Journey)

1. Admin menjalankan `docker run -p 8080:8080 cubycloud/app:latest`.
2. Admin membuka `http://localhost:8080/setup` di browser.
3. System menampilkan **Setup Wizard**:
* Membuat akun Super Admin.
* Menampilkan deteksi otomatis SSD lokal (`/dev/sdb`, `/dev/sdc`).
* Memilih strategi alokasi penyimpanan (*Fill-First / Round-Robin*).
* (Opsional) Memasukkan API Key Cloudflare R2 / AWS S3.


4. Sistem menyimpan konfigurasi dan langsung mengarahkan Admin ke dashboard `/drive/my-storage`.

#### 2. Workflow Manajemen & Transfer Direct File

1. Pengguna melakukan *drag-and-drop* berkas video/dokumen berukuran besar ke area browser.
2. Jika destinasi fisik berkas adalah Cloudflare R2, backend Go menerbitkan *Presigned Upload URL*.
3. Browser mengunggah berkas **langsung ke R2** tanpa melewati CPU/RAM server CubyCloud lokal.
4. Tampilan UI memperbarui daftar berkas secara real-time dengan label *Storage Node Badge* (`[R2]`).
---

## 3. Product Scope & Boundaries

* **3.1. In-Scope (Fitur Utama):**
* Single Container Deployment (Go + Embedded Svelte).
* Onboarding Wizard via Web GUI.
* Unified Directory & Virtual File System.
* Multi-Node Storage Manager & Placement Strategies (*Fill-First*, *Round-Robin*, *Most-Free-Space*).
* Direct Presigned URL Upload/Download (S3/R2).
* Zero-Byte Reference Copy Engine.
* Add-ons Store Engine (Dynamic UI Hooks & Task Queue).
* AI Workspace & MCP RAG Protocol (Ollama/Cloud LLM Support).
* Access Control & Link Sharing (Password & Expiration Date).


* **3.2. Out-of-Scope (Sengaja Tidak Dibuat pada v1.0):**
* Desktop Native Sync Client (seperti Dropbox Sync) -> *Fokus Web Client dulu*.
* Collaborative Real-Time Document Editing (seperti Google Docs) -> *Bisa via Add-on di masa depan*.



---

## 4. Detailed Functional Requirements (Feature Requirements)

* **4.1. Module 1: Onboarding & Initial Setup Wizard (`/setup`)**
* Super Admin Account Creation.
* Automatic Disk Discovery & Storage Quota Allocation.
* Placement Strategy Selection.
* Emergency Recovery Key Export.


* **4.2. Module 2: Unified File Manager (`/drive/my-storage`)**
* Virtual Directory Abstraction.
* Operations: Upload, Download, Rename, Move, Zero-Byte Copy, Delete.
* Fast File Previewer (PDF, Image, Video, Code, Text).
* Drag-and-Drop Overlay & Multi-Select Batch Actions.


* **4.3. Module 3: Storage Node & Infrastructure Manager (`/admin/settings`)**
* Real-time Capacity Monitoring (Aggregate & Per-Node).
* Node Attachment & Detachment.
* Graceful Detach & Background File Migration Engine.


* **4.4. Module 4: Sharing & Access Control (`/drive/shared` & `/s/:token`)**
* Public Shared Links Generation.
* Password Protection & Expiration Date Enforcement.
* User Management & Role-Based Access Control (RBAC).


* **4.5. Module 5: Add-ons System (`/admin/addons`)**
* Add-on Lifecycle Management (Install, Enable, Disable, Uninstall).
* Dynamic UI Hooks Registration (Injecting context menu items).
* Isolated Task Worker Queue.


* **4.6. Module 6: AI Workspace & MCP Integration (`/drive/ai-chat`)**
* Folder-to-Workspace Conversion.
* Vector Indexing Pipeline (Text Extraction & Chunking).
* Model Context Protocol (MCP) Integration (Ollama / External Provider).
* Conversational RAG Interface with Document Citations.



---

## 5. Non-Functional Requirements (NFRs)

* **5.1. Performance & Speed:**
* UI First Contentful Paint (FCP) < 1 detik.
* Latensi API Backend < 50ms (di luar waktu transfer file fisik).


* **5.2. Resource Efficiency & Footprint:**
* RAM idle < 100 MB.
* Penggunaan CPU minimal saat tidak ada proses AI/Konversi.


* **5.3. Security & Compliance:**
* Password Hashing menggunakan Argon2 / bcrypt.
* Direct-to-Cloud Uploads menggunakan temporary signed credentials.
* Token-based authentication (JWT / Encrypted HTTP-only Cookies).


* **5.4. Data Integrity & Reliability:**
* Atomic DB Operations untuk mencegah *dangling/orphaned physical files*.
* Recovery mechanism jika proses migrasi disk terputus (*Graceful Detach Fault Tolerance*).



---

## 6. User Experience (UX) & Design Guidelines

* **6.1. Visual Style:** Neo-Notion / Light Brutalism (Hard borders, high contrast, clean typography).
* **6.2. Key Interactions:** Drag-and-Drop, Context Menu (Klik-Kanan), Keyboard Shortcuts (`⌘+K`, `Esc`).
* **6.3. Responsive States:** Desktop First (3-Column Layout) dengan penyesuaian collapsible panel untuk layar tablet/mobile.

---

## 7. System Dependencies & Technical Stack

* **7.1. Frontend:** Svelte / SvelteKit + Tailwind CSS + Lucide Icons.
* **7.2. Backend Core:** Go (Golang) + Embedded Static Assets.
* **7.3. Storage Drivers:** Local POSIX Driver, AWS S3 SDK, Cloudflare R2 SDK.
* **7.4. Data Persistence:** SQLite (Default for Single Container) / PostgreSQL (Opsional via Docker Dev).

---

## 8. Release Criteria & Test Strategy

* **8.1. Alpha Criteria:** Setup Wizard, Single Node Storage, Basic File Manager, Auth.
* **8.2. Beta Criteria:** Multi-Node Placement Strategy, Zero-Byte Copy, Presigned S3/R2 Transfer.
* **8.3. GA (v1.0) Criteria:** Add-ons Store Engine, AI Workspace MCP Integration, Graceful Node Detach.

---

Bagian mana dari PRD ini yang ingin kita **detailkan dan tulis isinya terlebih dahulu**? (Misalnya: langsung tancap gas isi **Section 1 & 3**, atau langsung ke **Section 4: Functional Requirements**?).