# Panduan Reverse Proxy Production CubyCloud (Caddy & Nginx)

Dokumen ini menyediakan panduan konfigurasi reverse proxy HTTPS menggunakan **Caddy** atau **Nginx** di depan container CubyCloud.

---

## 1. Persiapan Environment CubyCloud

Saat dijalankan di balik reverse proxy HTTPS, ubah konfigurasi environment container CubyCloud di `compose.yaml`:

```yaml
services:
  cubycloud:
    image: ghcr.io/septian-padli/cubycloud:latest
    restart: unless-stopped
    # Jika reverse proxy berada dalam host atau network yang sama:
    ports:
      - "127.0.0.1:8383:8383"
    volumes:
      - cubycloud_data:/app/data
    environment:
      - ENV=production
      - TRUST_PROXY=true
      - COOKIE_SECURE=true
      - ALLOWED_ORIGINS=https://drive.domainanda.com
```

* **`TRUST_PROXY=true`**: Memungkinkan server membaca header `X-Forwarded-For` dan `X-Forwarded-Proto: https`.
* **`COOKIE_SECURE=true`**: Memastikan cookie auth sesi (`cuby_token`) hanya dikirim melalui koneksi HTTPS aman.
* **`ALLOWED_ORIGINS`**: Origin HTTPS publik yang diizinkan untuk request CORS.

---

## 2. Health & Readiness Probe Endpoint

CubyCloud menyediakan dua probe endpoint HTTP:

* **`GET /api/health`** (Readiness probe): Memvalidasi koneksi SQLite database dan ketersediaan semua storage node fisik yang terdaftar.
  * Respon **HTTP 200** `{"status":"ok"}` jika siap menerima beban kerja.
  * Respon **HTTP 503** `{"status":"unavailable"}` jika SQLite atau salah satu storage node mengalami gangguan.
* **`GET /health`** (Storage probe kompatibel): Memvalidasi ketersediaan storage node fisik.

Contoh verifikasi kesehatan via reverse proxy:
```bash
curl -i https://drive.domainanda.com/api/health
```

---

## 3. Opsi A: Deployment dengan Caddy (Rekomendasi)

Caddy otomatis mengelola sertifikat TLS (Let's Encrypt / ZeroSSL) dan konfigurasi HTTP/2 / HTTP/3.

### File Konfigurasi: `deploy/caddy/Caddyfile`

```caddy
drive.domainanda.com {
    encode gzip zstd

    # Dukungan upload file besar (TUS chunked & standard upload)
    request_body {
        max_size 10GB
    }

    reverse_proxy cubycloud:8383 {
        # Streaming real-time SSE tanpa buffering
        flush_interval -1
    }

    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "SAMEORIGIN"
        Referrer-Policy "strict-origin-when-cross-origin"
    }

    log {
        output stdout
        format console
    }
}
```

### Docker Compose Contoh dengan Caddy:

```yaml
services:
  cubycloud:
    image: ghcr.io/septian-padli/cubycloud:latest
    restart: unless-stopped
    volumes:
      - cubycloud_data:/app/data
    environment:
      - ENV=production
      - TRUST_PROXY=true
      - COOKIE_SECURE=true
      - ALLOWED_ORIGINS=https://drive.domainanda.com

  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./deploy/caddy/Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      - cubycloud

volumes:
  cubycloud_data:
  caddy_data:
  caddy_config:
```

---

## 4. Opsi B: Deployment dengan Nginx

Template konfigurasi lengkap tersedia di `deploy/nginx/cubycloud.conf`.

### Poin Kunci Konfigurasi Nginx:

1. **Upload File Besar:**
   ```nginx
   client_max_body_size 10G;
   proxy_request_buffering off;
   ```
2. **Server-Sent Events (SSE):**
   ```nginx
   location = /api/v1/events {
       proxy_pass http://cubycloud_backend;
       proxy_http_version 1.1;
       proxy_set_header Connection '';
       proxy_buffering off;
       proxy_cache off;
       proxy_read_timeout 86400s;
   }
   ```
3. **Readiness Probe:**
   ```nginx
   location ~ ^/(api/)?health$ {
       proxy_pass http://cubycloud_backend;
       proxy_connect_timeout 5s;
       proxy_read_timeout 5s;
   }
   ```
