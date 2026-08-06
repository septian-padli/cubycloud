Langkah yang sangat tepat. Mari kita kembangkan **PRD (Product Requirement Document)** sebagai pijakan utama proyek ini.

Berikut adalah **Outline PRD Lengkap** yang dirancang khusus untuk aplikasi *Self-Hosted Modular Cloud Storage & AI Workspace* ini. Outline ini sudah mencakup seluruh aspek fungsional, arsitektural, hingga aspek bisnis/operasional.

---

# 📋 Outline Product Requirement Document (PRD)

## 1. Document Control & Executive Summary

* **1.1. Title & Metadata:** Nama Proyek, Versi PRD, Status, Author, & Date.
* **1.2. Executive Summary:** Ringkasan singkat visi aplikasi (Cloud storage modular, *single-binary/container deployment*, UI Neo-Notion, integrasi RAG AI).
* **1.3. Problem Statement & Opportunity:** Tantangan cloud storage saat ini (biaya sewa *vendor locking*, privasi data AI, kebingungan mengelola banyak SSD lokal & cloud).
* **1.4. Core Goals & Success Metrics (KPI):**
* Kemudahan instalasi (*Setup Wizard < 3 menit*).
* Efisiensi resource (berjalan lancar di VPS 1 CPU / 1GB RAM).
* Kecepatan transfer (*Zero-overhead* via Direct Presigned URLs).



---

## 2. Target Users & User Personas

* **2.1. Persona 1: Self-Hoster / Homelab Enthusiast** (Ingin menggabungkan drive lokal & cloud R2/S3 tanpa ribet CLI).
* **2.2. Persona 2: Small Team / Tech Startup** (Butuh tempat berbagi dokumen yang aman, murah, hemat penyimpanan, dan terintegrasi AI).
* **2.3. Persona 3: Privacy-Conscious Researcher/Creator** (Membutuhkan AI RAG untuk dokumen pribadi menggunakan LLM Lokal via Ollama).

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