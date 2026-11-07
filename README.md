# CubyCrate

**Your files. Your server. Your rules.**

CubyCrate is a self-hosted file drive for people who want a familiar way to organize and share files while choosing where those files live. It combines a Go API, a SvelteKit web app, SQLite, local storage, and optional S3-compatible storage.

[![CI Quality Gates](https://github.com/septian-padli/cubycrate/actions/workflows/ci.yml/badge.svg)](https://github.com/septian-padli/cubycrate/actions/workflows/ci.yml)
[![Release Automation](https://github.com/septian-padli/cubycrate/actions/workflows/release.yml/badge.svg)](https://github.com/septian-padli/cubycrate/actions/workflows/release.yml)
[![GHCR Package](https://img.shields.io/badge/container-ghcr.io-24292e?logo=docker&logoColor=white)](https://github.com/septian-padli/cubycrate/pkgs/container/cubycrate)
![Multi-Arch](https://img.shields.io/badge/arch-amd64%20%7C%20arm64-blue)
[![Go 1.25](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Svelte 5](https://img.shields.io/badge/Svelte-5-FF3E00?logo=svelte&logoColor=white)](https://svelte.dev/)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL_3.0-blue.svg)](LICENSE)
![Self-hosted](https://img.shields.io/badge/self--hosted-yes-61764B)

![CubyCrate Drive home showing quick access, recent activity, and storage summary](docs/assets/drive-home.png)

## Why CubyCrate?

- **Single self-contained container.** The SvelteKit 5 frontend is embedded directly into the Go binary. No Node.js runtime container, PHP-FPM, or separate web server is required.
- **Zero external dependencies.** Powered by SQLite in WAL mode with embedded migrations. You do not need to install, configure, or maintain PostgreSQL, MySQL, Redis, or external brokers.
- **Minimal footprint.** The Alpine runtime image weighs under 150 MB and idles around 30 to 50 MB of RAM, making it fast to deploy and ideal for low-power hardware.
- **Multi-architecture ready.** Native builds for both `linux/amd64` (PC, VPS, Intel/AMD servers) and `linux/arm64` (Raspberry Pi 4/5, Orange Pi, ARM VPS).
- **Keep control of your data.** Store files on local disks you manage or connect S3-compatible object storage.
- **Start in a browser.** The first-run wizard creates the initial administrator and configures storage.
- **Separate personal and shared workspaces.** My Storage and Organization are distinct drives with independent quota policies.

## Features

- **File management:** folders, uploads, rename, move, duplicate, trash, restore, and starred items.
- **Resumable uploads:** browser uploads to My Storage use TUS chunking.
- **File previews:** images, video, PDF, and text/code files can be previewed in the browser.
- **Sharing:** share files and folders with registered users using Viewer or Editor permissions, or create public links with optional password protection.
- **Organization drive:** a shared workspace for registered users, separate from personal My Storage quota accounting.
- **Flexible storage:** local disk and S3-compatible storage, with a MinIO-backed Playwright scenario.
- **Administration:** manage storage nodes, user quotas, system settings, and storage usage.
- **Capacity controls:** My Storage and Organization uploads are checked against the capacity of the physical storage nodes they share.

## Screenshots

### File manager

![My Storage file manager showing a folder and a text file](docs/assets/my-storage.png)

### First-run setup

![CubyCrate first-run setup wizard](docs/assets/setup-wizard.png)

### Storage administration

![Admin Settings page showing a storage node and system settings](docs/assets/admin-settings.png)

The screenshots were captured from the running application using synthetic local data. The interface currently shown in these captures is Indonesian.

## Quick start

### Docker Compose (recommended)

CubyCrate runs as one container with the frontend embedded in the Go backend.

> **Image availability:** The official container image `ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2` is staged for the upcoming release and **is not yet published to GHCR** (blocked on T8 release gate).
>
> - **Once published to GHCR (Quick Start):** Users only need Docker + Docker Compose and a downloaded `compose.yaml`. No repository clone or local build is required:
>   ```bash
>   curl -fsSLO https://raw.githubusercontent.com/septian-padli/cubycrate/main/compose.yaml
>   export CUBYCRATE_IMAGE=ghcr.io/septian-padli/cubycrate
>   export CUBYCRATE_VERSION=v0.0.1-alpha.2
>   docker pull ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2
>   docker compose pull
>   docker compose up -d
>   curl -f http://127.0.0.1:8383/health
>   ```
>   The health probe returns `{"status":"ok"}` with HTTP 200. Open `http://<server-ip>:8383/setup` to configure the first administrator.
> - **Until GHCR publish / Local Development:** Clone the full source repository because Docker builds need the complete build context:
>   ```bash
>   git clone https://github.com/septian-padli/cubycrate.git
>   cd cubycrate
>   docker build -t cubycrate:local .
>   docker compose up -d
>   ```
Compose keeps the container port at `8383` and publishes host port `8383` by default. Set `CUBYCRATE_PORT=9000` to override the host port; set `CUBYCRATE_VOLUME_NAME` before Compose commands to change the named volume from `cubycrate_data`. Restrict direct HTTP access with a firewall.


#### Updating CubyCrate

The official `ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2` image will be published via GitHub Release in T8; it is not available before that workflow succeeds. Once published, `docker compose pull` then `docker compose up -d` updates the instance. For development, run `git pull` from the cloned repository root and rebuild with `docker build -t cubycrate:local .` before `docker compose up -d`.

#### Docker Volume & Storage Configuration

- **Named volume (default):** `cubycrate_data:/app/data` stores SQLite (`db/cubycrate.db`, WAL), generated secrets (`secrets/jwt.key`, `secrets/master.key`), and local file storage (`storage/default`).
- **Host bind mount:** You may replace the named volume with a host directory mount. Ensure the host directory is writable and included in backups.
- **Secondary storage disks:** A host path mounted under `/app/data/storage/<node>` is external to the named volume and needs a separate backup.
#### Reverse Proxy & HTTPS

The Compose file does not include a reverse proxy. To use an existing HTTPS reverse proxy, point its upstream to the Docker host's IP address and published port (8383 by default, or your `CUBYCRATE_PORT`). Configure the public domain and TLS certificate in the proxy. Restrict access to the direct HTTP port to the proxy and trusted LAN clients with a firewall. A proxy on the same Docker network may instead reach `cubycrate:8383`; joining that network requires configuration outside this Compose file.

For HTTPS-only login, enable `COOKIE_SECURE=true` in the Compose environment. This prevents login over direct HTTP on the LAN. With the current application code, allowing authenticated HTTP LAN and HTTPS proxy access at the same time cannot provide `Secure` cookies on HTTPS without trusting a client-supplied `X-Forwarded-Proto` header on the published LAN port. Do not enable `TRUST_PROXY=true` on that exposed port as a workaround. The embedded frontend uses same-origin API requests, so `ALLOWED_ORIGINS` is not needed for a normal same-domain proxy setup. Use `GET /api/health` for readiness checks; see the [Reverse Proxy Guide](docs/reverse-proxy.md) for proxy examples.
#### Backup and Restore

The persistent application tree under `/app/data` contains the SQLite database (`db/cubycrate.db`, WAL mode, with `-wal`/`-shm` sidecars while active), local blobs (`storage/`), and generated secrets (`secrets/jwt.key`, `secrets/master.key`). The named volume defaults to `${CUBYCRATE_VOLUME_NAME:-cubycrate_data}`.

> **Database-only is not full backup:** `GET /api/v1/admin/backup` returns a consistent SQLite database snapshot only. It does not include local file blobs or secrets. External local mounts, S3/MinIO objects, and externally supplied master/JWT secrets need separate backup procedures.

Use the [backup and disaster recovery runbook](docs/backup-restore.md) for cold full-volume backup and isolated restore. Never archive only the live SQLite database file, restore directly over an existing volume, or run `docker compose down -v` for backup/restore.

Detailed backup, disaster recovery, upgrade, and rollback instructions are in [docs/backup-restore.md](docs/backup-restore.md) and [docs/release-guide.md](docs/release-guide.md).
### Run from source

Requirements: **Go 1.25 or newer**, **Bun 1.x**, and OpenSSL to generate a random development secret. Run the backend and frontend in separate terminals from a local checkout.

**Terminal 1: backend**

```bash
cd apps/backend
PORT=8383 \
DATA_DIR_PATH="$HOME/cubycrate-data" \
go run ./cmd/server/main.go
```

**Terminal 2: frontend**

```bash
cd apps/frontend
bun install
VITE_API_PROXY_TARGET=http://localhost:8383 \
bun run dev --host 127.0.0.1 --port 5188
```

Open [http://localhost:5188/setup](http://localhost:5188/setup), create the first administrator, and complete the setup wizard.
## System requirements

CubyCrate is designed to run efficiently on low-power homelab servers, single-board computers, and virtual private servers.

- **Minimum hardware:** 1 CPU core, 1 GB RAM (suitable for single-user personal storage on Raspberry Pi 4 or entry-level VPS).
- **Recommended hardware:** 2 CPU cores, 2 GB RAM (optimized for Intel Celeron mini PCs, small offices, or active family drives).
- **Supported architectures:** `linux/amd64` and `linux/arm64`.
- **Disk space:** 200 MB for container image and SQLite metadata, plus whatever storage capacity you plan to allocate for uploaded files.
- **Source development:** Go 1.25 or newer and Bun 1.x (if building without Docker).
## Architecture and tech stack

| Area           | Implementation                                           |
| -------------- | -------------------------------------------------------- |
| Backend        | Go 1.25, Echo, Bun ORM                                   |
| Database       | SQLite in WAL mode with embedded SQL migrations          |
| Frontend       | SvelteKit 2, Svelte 5, TypeScript, Tailwind CSS 4        |
| Local storage  | Filesystem storage nodes                                 |
| Object storage | S3-compatible services through the AWS SDK for Go v2     |
| Packaging      | Static frontend assets embedded into the Go server build |

The intended production package is a single Go service serving both the API and the built frontend embedded in the binary.

## Development and testing

For the full development setup, configuration, and test details, see [Development](docs/development.md) and [Testing](docs/testing.md).

```bash
make test
```

`make test` runs backend Go tests and the frontend Svelte check. Playwright end-to-end tests are run separately; the S3 integration test requires the optional MinIO development service.

## Documentation

- [API Documentation & Swagger UI (OpenAPI 3.1)](docs/openapi/README.md)
- [Architecture and runtime model](docs/architecture.md)
- [Development setup](docs/development.md)
- [Testing guide](docs/testing.md)
- [Contributing](CONTRIBUTING.md)
- [Security policy and reporting](SECURITY.md)
## Security and privacy

CubyCrate runs on infrastructure you control and stores application data beneath its configured data directory. Implemented safeguards include bcrypt password hashing, HTTP-only `SameSite=Lax` session cookies, rate limiting for login and public-share unlock requests, and encryption of saved S3-compatible storage credentials.

These safeguards do not replace TLS, host security, or backups. CubyCrate does not claim end-to-end encryption for uploaded file contents. See [SECURITY.md](SECURITY.md) for the current vulnerability-reporting guidance; a dedicated private contact channel is not published yet.

## Contributing and community

Bug reports, reproducible examples, and focused pull requests are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md) before preparing a change. Do not include secrets, recovery keys, or real user data in issues, logs, screenshots, or test fixtures.

## License

Copyright (C) 2026 CubyCrate contributors. CubyCrate is licensed under the GNU Affero General Public License, version 3 or (at your option) any later version. See [LICENSE](LICENSE).

Modified versions used over a network must offer their users access to the corresponding source code under AGPL-3.0 section 13. The source repository is available at [github.com/septian-padli/cubycrate](https://github.com/septian-padli/cubycrate).
