# RTK — Rust Token Killer (CubyCloud)

Token-optimized CLI proxy (~60–90% savings on dev command output). **Wajib** dipakai agent AI saat menjalankan perintah shell di repo ini.

## Setup (sekali per mesin)

```bash
brew install rtk
rtk init -g --agent cursor --auto-patch
```

Verifikasi: `rtk --version` dan `rtk gain`.

## Aturan

1. **Shell:** Cursor hook (`rtk hook cursor`) otomatis mem-prefix perintah Shell — tidak perlu `rtk` manual kecuali hook gagal.
2. **Output besar:** gunakan `rtk read`, `rtk grep`, `rtk find`, `rtk git …` alih-alih membaca file/log mentah panjang.
3. **Debug filter:** `rtk proxy <cmd>` untuk output mentah.

## Perintah umum di monorepo ini

| Tujuan | Perintah |
| ------ | -------- |
| Frontend check | `cd apps/frontend && rtk bun run check` |
| Frontend build | `cd apps/frontend && rtk bun run build` |
| Backend test | `cd apps/backend && rtk go test ./...` |
| Backend build | `cd apps/backend && rtk go build ./cmd/server` |
| Git status/diff | `rtk git status` / `rtk git diff` |
| Cari simbol | `rtk grep "pattern" apps/` |

## Meta

```bash
rtk gain
rtk gain --history
rtk proxy <cmd>
```

Docs: [rtk-ai.app/guide](https://www.rtk-ai.app/guide)
