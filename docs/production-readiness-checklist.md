# Checklist sebelum CubyCloud go production

Daftar ini adalah task untuk mengubah checkout saat discovery menjadi release yang bisa dipasang dan dioperasikan publik. Semua item belum dicentang; centang hanya setelah acceptance criteria dibuktikan. Prioritas P0 adalah release blocker. P1 perlu selesai sebelum membuka signup/publikasi luas. P2 dapat dikerjakan setelah launch terbatas, dengan owner dan tanggal yang jelas.

### Arti label owner

- **[SAYA]** Saya dapat mengubah repository dan membuktikan hasil dengan build/test/smoke lokal. Tidak mencakup akses/keputusan pada akun, domain, server, atau layanan production milikmu.
- **[KAMU]** Memerlukan keputusan pemilik, credential/aksesmu, operasi infrastruktur production, atau persetujuan legal/bisnis. Saya dapat menyiapkan materi, tetapi tidak dapat membuktikan penyelesaian dari lingkunganmu.
- **[BERSAMA]** Saya menyiapkan perubahan, audit, prosedur, atau skenario uji; kamu menyediakan keputusan/akses dan menjalankan atau menyaksikan validasi yang hanya tersedia di target.

## Temuan discovery yang perlu ditangani dulu

- [x] **[SAYA] [P0: Pulihkan build container yang repeatable.]** `README.md` menyatakan Docker build gagal karena image memakai Go 1.24, tetapi `Dockerfile` yang diperiksa saat discovery menggunakan `FROM golang:alpine` tanpa versi toolchain tetap. Perbarui/pastikan Dockerfile memakai Go >= 1.25.0, hapus fallback dependency install yang tidak reproducible (`bun install` setelah `--frozen-lockfile` gagal), lalu buktikan clean build dan startup image. Jangan anggap dokumentasi lama sebagai bukti kondisi file sekarang.
- [ ] **[BERSAMA] [P0: Putuskan jalur distribusi publik.]** README menyebut produk self-hosted dan intended package berupa satu layanan, namun belum ada cara instalasi production yang telah dibuktikan; container build masih berstatus bermasalah di README. Kamu memilih jalur distribusi; saya dapat menyiapkan artifact dan dokumentasi, lalu kita uji dari artifact bersih sampai aplikasi dapat diakses.
- [x] **[BERSAMA] [P0: Tetapkan lisensi proyek.]** Repository menyediakan GNU AGPL-3.0-or-later di `LICENSE`, dan README menjelaskan lisensi serta kewajiban source untuk modified network versions. Pemilik proyek menyetujui copyright notice yang ditampilkan. Tetap tinjau third-party notices sebelum distribusi.
- [ ] **[BERSAMA] [P0: Sediakan jalur private vulnerability disclosure.]** `SECURITY.md` menyatakan belum ada kontak private khusus atau komitmen response time. Kamu menyediakan/mengaktifkan kanal dan menetapkan komitmen yang sanggup dipenuhi; saya dapat memperbarui SECURITY.md dan memeriksa instruksinya.
- [ ] **[BERSAMA] [P0: Siapkan dan buktikan backup serta restore.]** Data mencakup SQLite dan file di bawah `DATA_DIR_PATH` (`README.md`, `docs/development.md`); README menyuruh operator menjaga backup, tetapi prosedur operasional belum didokumentasikan. Saya dapat menyusun prosedur dan skenario validasi; kamu menyediakan target/akses backup dan menjalankan restore terisolasi. Tetapkan frekuensi, retensi, lokasi terpisah, serta target RPO/RTO bersama.
- [ ] **[BERSAMA] [P0: Putuskan operasi single-instance dan storage.]** Aplikasi memakai SQLite WAL dan satu koneksi DB (`memory://root/memory_summary.md`). Saya dapat mendokumentasikan batasan dan menguji perilaku aplikasi; kamu menetapkan kapasitas, persistent volume, dan topologi deployment, lalu membuktikan restart/deploy tidak menghilangkan data.

## 1. Build, release, dan supply chain

- [x] **[SAYA] [P0: Kunci versi toolchain dan dependencies.]** Pin versi Go minimal 1.25.0, Bun, base image build/runtime, dan semua package installer. Pastikan dependency install tidak fallback ke lockfile yang diabaikan.
- [ ] **[BERSAMA] [P0: Jalankan release build dari checkout bersih.]** Saya dapat menjalankan build dari checkout bersih dan memperbaiki kegagalan repository; kamu menyediakan target runtime production. Pastikan frontend tertanam/disajikan oleh binary, lalu smoke-test endpoint dan halaman di target.
- [ ] **[SAYA] [P0: Tambahkan automated release gate.]** Tambahkan workflow/checks backend tests, frontend check/unit tests, lint, `make docs-check`, dan E2E relevan pada pull request/release. Pastikan kegagalan memblokir publikasi artifact. `docs/testing.md` mencatat `make test` tidak menyertakan Playwright dan lint berjalan terpisah.
- [ ] **[BERSAMA] [P1: Definisikan versi dan artifact release.]** Saya dapat menyiapkan versioning, changelog, checksum, metadata artifact, dan instruksi; kamu menyetujui kebijakan release, target arsitektur, dan menerbitkan artifact pada akun release.
- [ ] **[BERSAMA] [P1: Periksa dependency dan image vulnerabilities.]** Saya dapat menjalankan scanner yang tersedia dan merangkum temuan; kamu menyediakan scanner/credential bila diperlukan dan menyetujui pengecualian risiko serta proses patch.
- [ ] **[SAYA] [P1: Pastikan provenance/reproducibility cukup.]** Simpan build logs dan identitas commit dalam alur release repository, pin dependency, dan uji build agar tidak mengambil versi tidak terkunci.

## 2. Security dan konfigurasi production

- [ ] **[BERSAMA] [P0: Siapkan konfigurasi production per deployment.]** Kamu mengatur `ENV=production`, `JWT_SECRET` random dan kuat, `COOKIE_SECURE=true`, serta `ALLOWED_ORIGINS` origin publik di secret store/environment production. Saya dapat meninjau konfigurasi dan menguji validasi tanpa menerima secret. Kode memvalidasi nilai ini di `apps/backend/internal/config/config.go`.
- [ ] **[KAMU] [P0: Pastikan TLS dan reverse proxy benar.]** Konfigurasikan HTTPS/domain dan reverse proxy pada infrastruktur yang kamu kendalikan. Saya dapat meninjau konfigurasi yang dibagikan tanpa secret; uji cookie `Secure`, login, logout, upload, dan public-share dari jaringan publik.
- [ ] **[BERSAMA] [P0: Lakukan security review untuk auth, setup, share, file, dan admin.]** Saya dapat review kode, memperbaiki temuan, dan menyiapkan skenario uji; kamu menyediakan staging dan menjalankan validasi eksternal/production-like. Tutup temuan critical/high sebelum launch.
- [ ] **[KAMU] [P0: Tetapkan dan simpan secrets dengan aman.]** Kamu membuat, menyimpan, dan merotasi JWT secret serta master key kredensial S3 lewat secret store yang kamu kendalikan. CubyCloud mengamankan kredensial S3 dengan master key 256-bit (auto-generated di `$DATA_DIR_PATH/secrets/master.key` dengan hak akses `0600` atau di-override via `CUBYCLOUD_MASTER_KEY`). Master key ini terpisah dari account recovery key pengguna dan wajib di-backup secara aman agar kredensial node S3 tetap dapat didekripsi saat restore atau migrasi direktori. Jangan kirim nilai secret di chat/repository/log.
- [ ] **[BERSAMA] [P1: Tinjau rate limiting saat multi-instance dan restart.]** `SECURITY.md` menjelaskan rate limiter login dan public-share unlock bersifat in-memory dan reset saat server restart. Kamu memutuskan apakah single-instance cukup; saya dapat menilai/menyiapkan perubahan jika dibutuhkan, lalu kita uji.
- [ ] **[BERSAMA] [P1: Tetapkan hardening host/container.]** Saya dapat menghardening image/repository dan mendokumentasikan opsi; kamu mengatur user layanan, permission volume, network exposure, resource limits, patch OS/image, akses admin minimum, lalu memverifikasi host.
- [ ] **[BERSAMA] [P1: Tetapkan batas dan uji abuse untuk upload.]** Saya dapat meninjau batas/kode dan membuat skenario; kamu menyediakan staging dengan batas disk/storage dan menjalankan uji konkuren, disk penuh, serta kegagalan S3.
- [ ] **[BERSAMA] [P1: Tinjau pengamanan observability.]** Saya dapat mengaudit logging dan memperbaiki kebocoran kode; kamu mengonfigurasi log/telemetry production dan memeriksa data terkirim. Pastikan tidak ada token, kredensial, recovery key, URL share sensitif, atau isi file.

## 3. Data, backup, dan lifecycle

- [ ] **[BERSAMA] [P0: Dokumentasikan direktori persisten dan permission.]** Saya dapat mendokumentasikan struktur dan memperbaiki kode bila diperlukan; kamu menetapkan `DATA_DIR_PATH`, volume, ownership/permission, serta memverifikasi database, storage, dan temporary upload pada host/container.
- [ ] **[BERSAMA] [P0: Uji backup/restore end-to-end.]** Saya dapat menyiapkan langkah dan memvalidasi data pulihan; kamu menjalankan backup dan restore ke host/volume terisolasi. Uji metadata, file lokal namespaced (`cubycloud/{instance_id}/{node_id}/blobs/...`), share, konfigurasi storage, master key (`secrets/master.key` atau `CUBYCLOUD_MASTER_KEY`), unduhan, dan konsistensi SQLite WAL.
- [ ] **[BERSAMA] [P1: Buat prosedur upgrade database dan rollback.]** Saya dapat audit migrasi, menulis prosedur, dan menguji salinan data; kamu menyediakan backup/lingkungan salinan dari release sebelumnya dan menetapkan kebijakan rollback.
- [ ] **[BERSAMA] [P1: Tetapkan retensi serta penghapusan data.]** Saya dapat dokumentasikan perilaku aplikasi dan meninjau purge; kamu menetapkan kebijakan retensi, privasi, backup, dan penghapusan sesuai kewajiban organisasi/pengguna.
- [ ] **[BERSAMA] [P1: Uji storage lokal dan S3 dengan kegagalan nyata.]** Saya dapat menyiapkan/memperbaiki test untuk MinIO dan storage abstractions; kamu menyediakan provider yang akan didukung dan menjalankan uji kredensial, koneksi, kapasitas, objek hilang, serta recovery.

## 4. Reliability dan operasi

- [x] **[BERSAMA] [P0: Tetapkan health check untuk operator.]** Endpoint `GET /api/health` telah ditambahkan untuk memeriksa readiness SQLite database dan storage node fisik (mengembalikan HTTP 200 saat sehat, HTTP 503 saat gagal). Endpoint legacy `GET /health` dipertahankan untuk probe storage. Template reverse proxy production disediakan di `deploy/caddy/Caddyfile` dan `deploy/nginx/cubycloud.conf`, dengan dokumentasi lengkap di `docs/reverse-proxy.md`.
- [ ] **[KAMU] [P0: Jalankan operational smoke test pada deployment target.]** Jalankan di targetmu: buat admin pertama, simpan recovery key aman, login/logout, unggah/unduh, buat folder/share, uji akses user lain, restart, dan pastikan data persisten. Saya dapat mendampingi dan memperbaiki bug.
- [ ] **[BERSAMA] [P1: Definisikan logging, alert, dan on-call.]** Saya dapat menentukan sinyal dan menyiapkan dokumentasi/konfigurasi repo; kamu memilih layanan alert, menetapkan penerima/on-call dan menguji notifikasi untuk crash, disk, backup, DB, storage, dan 5xx.
- [ ] **[BERSAMA] [P1: Uji graceful shutdown dan restart.]** Saya dapat audit/perbaiki shutdown handling dan membuat smoke scenario; kamu menjalankan stop/restart di staging saat request/upload berjalan dan memeriksa volume serta recovery.
- [ ] **[BERSAMA] [P1: Tetapkan kapasitas dan batas dukungan.]** Saya dapat menyusun benchmark dan menjalankan workload pada lingkungan tersedia; kamu menyediakan spesifikasi target/workload representatif dan menyetujui angka kapasitas yang dipublikasikan. README belum memiliki minimum CPU/RAM atau benchmark.
- [ ] **[BERSAMA] [P1: Siapkan runbook incident dan rollback.]** Saya dapat menulis draft runbook; kamu mengisi kontak, akses, pemilik tugas, prosedur platform/komunikasi, lalu menguji rollback/restore di staging.

## 5. Product, legal, dan launch publik

- [ ] **[KAMU] [P0: Tetapkan identitas pemilik dan support.]** Kamu menetapkan maintainer/operator, kanal support, dan komitmen respons; saya dapat menambahkan informasi yang kamu setujui ke dokumentasi.
- [ ] **[BERSAMA] [P0: Publikasikan kebijakan privasi dan batas klaim.]** Saya dapat menyusun draft berdasarkan perilaku produk tanpa klaim tak terbukti; kamu memutuskan kebijakan data/telemetry/retensi serta menyetujui publikasi dan tinjauan legal.
- [ ] **[BERSAMA] [P0: Audit third-party notices dan kompatibilitas.]** Lisensi proyek telah ditetapkan sebagai GNU AGPL-3.0-or-later dan tersedia di `LICENSE`; audit dependency dan bundled assets, konfirmasi kewajiban notice/redistribusi, dan selesaikan tinjauan legal sebelum publikasi. Saya dapat menyiapkan inventaris; kamu menyetujui hasil dan tindakan.
- [ ] **[BERSAMA] [P1: Publikasikan panduan pemasangan yang dapat diikuti.]** Saya dapat menulis dan menguji panduan dari source/artifact; kamu menyediakan domain, TLS, konfigurasi non-secret, serta target bersih untuk uji install independen.
- [ ] **[BERSAMA] [P1: Tulis kebijakan dukungan dan kompatibilitas.]** Saya dapat merangkum kombinasi teruji dan batas teknis; kamu menyetujui target dukungan dan komitmen, bukan kombinasi yang belum diuji.
- [ ] **[KAMU] [P1: Uji onboarding dan recovery dari sudut pandang pengguna baru.]** Jalankan setup/recovery di deployment baru dan simpan recovery key secara aman. Saya dapat memperbaiki bug/instruksi dari hasil yang kamu laporkan.
- [ ] **[BERSAMA] [P1: Persiapkan halaman publik tanpa klaim yang belum terbukti.]** Saya dapat menyunting copy/dokumentasi berdasarkan bukti repository; kamu menyediakan fakta produk dan menyetujui isi publik, termasuk sumber klaim statistik/testimonial/security.
- [ ] **[KAMU] [P2: Rencanakan launch bertahap.]** Kamu menentukan pengguna awal, target keberhasilan, pemantauan, dan kapan memperluas launch; saya dapat menyiapkan template rencana.

## 6. Go/no-go sebelum diumumkan

- [ ] **[BERSAMA]** Semua P0 selesai dan dibuktikan owner, termasuk bukti repository dari saya dan bukti deployment/keputusan darimu.
- [ ] **[BERSAMA]** Release artifact berasal dari commit/tag yang tepat, build bersih, dan smoke test target berhasil; saya membuktikan build/artifact, kamu membuktikan lingkungan target.
- [ ] **[KAMU]** Backup tersedia dan restore terbaru berhasil diuji pada target terisolasi.
- [ ] **[BERSAMA]** Tidak ada temuan security critical/high terbuka tanpa keputusan risiko tertulis darimu; saya membantu menutup temuan kode.
- [ ] **[BERSAMA]** Support/security contact, lisensi, dokumentasi install, upgrade, backup/restore, dan batasan produk dipublikasikan setelah keputusan/kontak darimu dan perubahan dokumentasi saya.
- [ ] **[KAMU]** Operator memantau launch, alert aktif, dan rollback plan siap; saya dapat mendukung respons teknis.

