# Getting Started — CubyCloud Alpha

> ⚠️ **Ini adalah rilis alpha (`v0.0.1-alpha.1`).** Jangan gunakan untuk data produksi atau data yang tidak bisa kehilangan. Fitur dan API masih dapat berubah tanpa pemberitahuan.

---

## Persyaratan

- Docker Engine >= 24.0
- Docker Compose >= 2.20
- Arsitektur: **ARM64** (Apple Silicon, Ampere) atau **AMD64**
- Minimal RAM: 512 MB
- Port `8383` tidak sedang digunakan

---

## Instalasi Cepat

### 1. Buat file `compose.yaml`

```yaml
services:
  cubycloud:
    image: ghcr.io/septian-padli/cubycloud:v0.0.1-alpha.1
    restart: unless-stopped
    ports:
      - "8383:8383"
    environment:
      - CUBYCLOUD_VERSION=v0.0.1-alpha.1
    volumes:
      - cubycloud_data:/app/data

volumes:
  cubycloud_data:
```

### 2. Jalankan

```bash
docker compose up -d
```

### 3. Buka browser

Akses **http://localhost:8383** dan ikuti wizard setup pertama kali.

---

## Konfigurasi Penyimpanan (MinIO / S3)

CubyCloud membutuhkan object storage kompatibel S3 sebagai backend penyimpanan file. Kamu bisa menggunakan:

- **MinIO** (self-hosted, direkomendasikan untuk alpha)
- **AWS S3**
- **Cloudflare R2**
- Layanan S3-kompatibel lainnya

Konfigurasi storage dilakukan melalui halaman **Setup** saat pertama kali membuka aplikasi.

---

## Keterbatasan di Versi Alpha Ini

Harap baca dan pahami keterbatasan berikut sebelum menggunakan:

### 🔴 Backup & Pemulihan Data

**Belum tersedia fitur backup penuh dari dalam aplikasi.** Untuk menjaga data sementara:

1. **Database** — backup manual volume Docker:
   ```bash
   docker run --rm \
     -v cubycloud_data:/data \
     -v $(pwd):/backup \
     alpine tar czf /backup/cubycloud-db-backup.tar.gz /data
   ```
2. **File pengguna** — backup langsung dari bucket MinIO/S3 kamu menggunakan tool seperti `mc mirror` atau `aws s3 sync`.

Fitur backup terpadu dari antarmuka aplikasi direncanakan untuk rilis mendatang.

### 🟡 Performance Belum Diuji

Belum dilakukan *performance testing* dan *load testing* secara formal. Penggunaan di lingkungan dengan banyak pengguna bersamaan atau file berukuran sangat besar mungkin mengalami penurunan performa yang belum teridentifikasi. Gunakan dengan beban wajar selama alpha.

### 🟡 Fitur Lain yang Masih dalam Pengembangan

- Notifikasi email belum dikonfigurasi secara otomatis
- Undangan pengguna masih memerlukan pengaturan SMTP manual
- Antarmuka administrasi masih terbatas

---

## Update ke Versi Baru

```bash
# Tarik image versi terbaru
docker compose pull

# Restart container
docker compose up -d
```

---

## Melaporkan Bug

Temukan bug atau perilaku aneh? Laporkan melalui:

👉 **[Buka Bug Report](https://github.com/septian-padli/cubycloud/issues/new?template=bug_report.md)**

Sertakan:
- Versi CubyCloud yang digunakan
- Langkah untuk mereproduksi bug
- Log dari container (`docker compose logs cubycloud`)

---

## Mengusulkan Fitur

Punya ide untuk pengembangan? Kami sangat terbuka untuk masukan di fase alpha ini.

👉 **[Buka Feature Request](https://github.com/septian-padli/cubycloud/issues/new?template=feature_request.md)**

---

## Mendapatkan Log Container

```bash
# Log real-time
docker compose logs -f cubycloud

# Log 100 baris terakhir
docker compose logs --tail=100 cubycloud
```

---

## Menghentikan & Menghapus

```bash
# Hentikan container (data tetap aman)
docker compose down

# Hentikan dan hapus semua data (tidak bisa dikembalikan)
docker compose down -v
```

---

*Dokumentasi ini akan terus diperbarui seiring perkembangan CubyCloud. Pantau [CHANGELOG](https://github.com/septian-padli/cubycloud/releases) untuk informasi rilis terbaru.*
