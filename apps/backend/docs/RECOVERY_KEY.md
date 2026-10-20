# Recovery Key

Recovery Key adalah kredensial pemulihan yang ditampilkan satu kali setelah setup berhasil. Simpan di tempat aman; server hanya menyimpan hash.

## Setup

`POST /api/v1/setup/complete` menyelesaikan setup dan mengembalikan `recovery_key` beserta redirect. Key dibuat client-side dengan CSPRNG dalam format enam grup lima karakter uppercase alfanumerik:

```text
XXXXX-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX
```

UI setup menampilkan key pada success dialog. Pengguna dapat menyalin atau mengunduhnya dan harus mengakui telah menyimpannya sebelum melanjutkan ke `/drive/home`. Key tidak dapat ditampilkan kembali.

## Reset password

Kirim `POST /api/v1/auth/reset-password`:

```json
{
  "username": "admin",
  "recovery_key": "ABCDE-FGHJK-LMNPQ-RSTUV-WXYZ2-34567",
  "new_password": "newpassword456"
}
```

Key dinormalisasi case-insensitively dengan mengabaikan hyphen dan whitespace. Recovery key disimpan sebagai SHA-256 dari 30 karakter kanonis tanpa separator; password tetap di-hash dengan bcrypt. Reset yang berhasil mengonsumsi key secara atomik dan response menyertakan `new_recovery_key`, key baru berformat 6×5 yang harus disimpan. Key lama tidak dapat digunakan kembali.

## Kompatibilitas migrasi

Legacy 64-character hexadecimal key hanya diterima jika hash recovery yang tersimpan adalah bcrypt. Reset legacy yang berhasil merotasi kredensial ke format 6×5 dan SHA-256. Ini adalah fallback migrasi saja; format aktif bukan 64-hex dan recovery hash baru bukan bcrypt.

Response reset berhasil memuat pesan sukses dan `new_recovery_key`. Key tidak valid ditolak; user yang tidak ditemukan menghasilkan error user-not-found.
