# Laporan Tinjauan Keamanan Kode CubyCloud

Dokumen ini memuat hasil security code review terhadap lima area inti backend CubyCloud: autentikasi (auth), inisialisasi sistem (setup), pembagian file (share), manajemen file dan penyimpanan (file), serta administrasi (admin).

Tanggal audit: 2026-10-26  
Status temuan: 0 Critical, 0 High yang memblokir fungsi, 2 Medium, 2 Low.

---

## 1. Lingkup dan Metodologi

Pemeriksaan dilakukan pada lapisan backend Go (Echo v4) dan skema SQLite:

1. **Autentikasi (Auth):** `apps/backend/internal/modules/auth/`, `apps/backend/internal/middleware/jwt_auth.go`, `apps/backend/internal/middleware/rate_limiter.go`.
2. **Setup Awal (Setup):** `apps/backend/internal/modules/setup/`, `apps/backend/internal/middleware/setup_guard.go`.
3. **Pembagian File (Share):** `apps/backend/internal/modules/share/` (manajemen grant per-user dan public share link).
4. **Penyimpanan dan Operasi File (File):** `apps/backend/internal/modules/mystorage/`, `apps/backend/internal/storage/`.
5. **Administrasi (Admin):** `apps/backend/internal/modules/admin/`, `apps/backend/internal/middleware/super_admin_guard.go`, `apps/backend/internal/middleware/audit_log.go`.

---

## 2. Ringkasan Pengamanan yang Sudah Terbukti Baik

Kode basis telah menerapkan sejumlah kontrol keamanan fundamental:

- **Bcrypt Password Hashing:** Password disimpan menggunakan bcrypt (cost 10). Password dibatasi maksimal 72 byte sesuai batas algoritma bcrypt (`apps/backend/internal/modules/auth/service.go:23, 177`).
- **Token Version Invalidation:** Perubahan password (`ChangePassword`), reset via recovery key (`ResetPassword`), maupun reset password oleh admin (`ResetUserPassword`) menaikkan nilai `token_version` di database. Middleware `JWTAuth` memvalidasi `token_version` terhadap database pada setiap request (`apps/backend/internal/middleware/jwt_auth.go:72-99`), sehingga token lama otomatis gugur.
- **Perlindungan Path Traversal File:** Nama file pada disk fisik tidak menggunakan nama input dari user. File disimpan menggunakan `disk_key` internal berupa ULID atau konten hash SHA-256 (`apps/backend/internal/storage/impl.go:26-40`). Header unduhan `Content-Disposition` menggunakan nama file yang disanitasi.
- **Validasi ID Unggahan TUS:** Parameter ID unggahan sementara pada protokol TUS divalidasi ketat oleh fungsi `tusValidID` (`apps/backend/internal/modules/mystorage/tus.go:171-181`), hanya mengizinkan karakter alfanumerik dan tanda hubung, mencegah manipulasi path traversal di direktori temp.
- **Atomic Recovery Key Consumption:** Konsumsi recovery key pada `ResetPassword` dijalankan dalam transaksi atomik dengan filter `oldHash` (`apps/backend/internal/modules/auth/service.go:258-275`), mencegah race condition pada reset bersamaan.
- **Cookie Keamanan:** Cookie sesi `cuby_token` dikonfigurasi `HttpOnly: true`, `SameSite: Lax`, dan mewajibkan `Secure: true` pada mode production (`apps/backend/internal/config/config.go:66-72`).

---

## 3. Temuan Keamanan dan Rekomendasi

### Temuan 1 (Tingkat: Medium)
**Derivasi Kunci Enkripsi Kredensial S3 Menggunakan Nilai DATA_DIR_PATH Statis**

- **Lokasi Kode:**
  - `apps/backend/internal/storage/crypto.go:21-23`
  - `apps/backend/internal/modules/setup/service.go:181`
  - `apps/backend/internal/modules/admin/service.go:503`
- **Analisis Bukti:**
  Fungsi `EncryptCredential` dan `DecryptCredential` menggunakan PBKDF2 dengan salt konstan `"cubycloud-salt"` dan key material yang diambil dari `s.dataDirPath`:
  ```go
  keyMaterial := []byte(s.dataDirPath)
  accessKeyEnc, err := storage.EncryptCredential([]byte(req.Cloud.AccessKey), keyMaterial)
  ```
  Pada deployment container standar, nilai `DATA_DIR_PATH` bernilai tetap (`/app/data`). Jika pihak tidak berwenang berhasil memperoleh salinan file database SQLite (`cubycloud.db`), mereka dapat mendekripsi access key dan secret key S3 tanpa perlu mengetahui secret lain, karena algoritma dan input kunci bersifat deterministik.
- **Rekomendasi:**
  Gunakan kunci turunan dari `JWT_SECRET` atau sediakan environment variable terpisah (misalnya `STORAGE_MASTER_KEY`).
  *Catatan Operasional:* Perubahan kunci enkripsi memerlukan migrasi data kredensial node yang sudah tersimpan di database sebelum dapat diterapkan di production.

---

### Temuan 2 (Tingkat: Medium)
**Rate Limiter Berbasis Memori Mereset Saat Server Restart**

- **Lokasi Kode:**
  - `apps/backend/internal/middleware/rate_limiter.go:21-95`
  - `apps/backend/internal/modules/auth/routes.go`
  - `apps/backend/internal/modules/share/routes.go:44`
- **Analisis Bukti:**
  Rate limiter untuk login dan unlock public share menyimpan hitungan request dalam struktur data `map[string]*ipBucket` in-memory. Jika container atau service direstart, batas percobaan kembali ke nol. Selain itu, pada konfigurasi multi-instance di balik load balancer, state rate limiter tidak tersinkronisasi antar-instance.
- **Rekomendasi:**
  Untuk kebutuhan self-hosted single-instance saat ini, status ini dapat diterima namun operator perlu mengetahui batasannya di dokumentasi operasional (`SECURITY.md`). Jika CubyCloud mendukung clustering di masa depan, gunakan penyimpanan terpusat atau reverse proxy rate-limiting (seperti Nginx atau Caddy).

---

### Temuan 3 (Tingkat: Low)
**Pengecekan SuperAdminGuard Bergantung pada Claim JWT Tanpa Query Role DB Tambahan**

- **Lokasi Kode:**
  - `apps/backend/internal/middleware/super_admin_guard.go:25-33`
- **Analisis Bukti:**
  `SuperAdminGuard` mengevaluasi context `user_role` yang diisi oleh `JWTAuth` langsung dari `claims["role"]`. Jika seorang super admin diubah hak aksesnya secara manual di database tanpa memanggil API yang menaikkan `token_version`, token JWT yang masih aktif tetap dapat mengakses endpoint admin hingga masa berlaku token habis (maksimal 24 jam).
- **Rekomendasi:**
  Selalu pastikan perubahan role di database menyertakan pembaruan `token_version = token_version + 1`, atau ubah `SuperAdminGuard` agar selalu mengambil data role terbaru dari query database aktif.

---

### Temuan 4 (Tingkat: Low)
**Potensi Penumpukan File Sementara TUS Jika Klien Membatalkan Unggahan**

- **Lokasi Kode:**
  - `apps/backend/internal/modules/mystorage/tus.go:183-189`
- **Analisis Bukti:**
  Unggahan TUS yang terhenti atau tidak diselesaikan menyimpan file `.part` dan `.info` di bawah direktori `temp/tus/`. Pembersihan saat ini (`tusSweep`) dipicu saat ada request unggahan baru, tetapi belum memiliki job terjadwal otomatis di latar belakang (background cron) seperti halnya `TrashPurgeJob` untuk file sampah.
- **Rekomendasi:**
  Tambahkan cron worker terjadwal sederhana untuk menghapus file di `temp/tus/` yang usianya melebihi 24 atau 48 jam.

---

## 4. Kesimpulan untuk Go-Live Checklist

Dari hasil tinjauan keamanan pada lima area kode:
- Tidak ditemukan kerentanan Critical seperti remote code execution atau SQL injection tak terfilter (seluruh query menggunakan parameterized query Bun / SQLite).
- Tidak ditemukan celah path traversal pada manipulasi file lokal.
- Temuan Medium terkait derivasi enkripsi S3 memerlukan keputusan migrasi kunci sebelum diubah pada deployment dengan data eksisting.
