# CubyCloud

**Your files. Your server. Your rules.**

CubyCloud is a self-hosted file drive for people who want a familiar way to organize and share files while choosing where those files live. It combines a Go API, a SvelteKit web app, SQLite, local storage, and optional S3-compatible storage.

[![Go 1.25](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Svelte 5](https://img.shields.io/badge/Svelte-5-FF3E00?logo=svelte&logoColor=white)](https://svelte.dev/)
![Self-hosted](https://img.shields.io/badge/self--hosted-yes-61764B)

![CubyCloud Drive home showing quick access, recent activity, and storage summary](docs/assets/drive-home.png)

## Why CubyCloud?

- **Keep control of your data.** Store files on disks you manage or connect an S3-compatible service you choose.
- **Start in a browser.** The first-run wizard creates the initial administrator and configures storage.
- **Keep the stack simple.** The application uses a Go server and SQLite; it does not require a separate database or message broker.
- **Separate personal and shared workspaces.** My Storage and Organization are distinct drives with different quota behavior.
- **Self-host on your terms.** Choose the host, storage location, backup process, and network exposure.

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

![CubyCloud first-run setup wizard](docs/assets/setup-wizard.png)

### Storage administration

![Admin Settings page showing a storage node and system settings](docs/assets/admin-settings.png)

The screenshots were captured from the running application using synthetic local data. The interface currently shown in these captures is Indonesian.

## Quick start

### Run from source

Requirements: **Go 1.25 or newer**, **Bun 1.x**, and OpenSSL to generate a random development secret. Run the backend and frontend in separate terminals from a local checkout.

**Terminal 1 — backend**

```bash
cd apps/backend
PORT=8383 \
DATA_DIR_PATH="$HOME/cubycloud-data" \
JWT_SECRET="$(openssl rand -hex 32)" \
go run ./cmd/server/main.go
```

**Terminal 2 — frontend**

```bash
cd apps/frontend
bun install
VITE_API_PROXY_TARGET=http://localhost:8383 \
bun run dev --host 127.0.0.1 --port 5188
```

Open [http://localhost:5188/setup](http://localhost:5188/setup), create the first administrator, and complete the setup wizard. Save the recovery key somewhere safe. Application data is written under the `DATA_DIR_PATH` you selected; keep that directory backed up.

### Container build

The multi-stage `Dockerfile` uses pinned versions for all stages (`oven/bun:1.3.14-alpine`, `golang:1.25.14-alpine`, and `alpine:3.21.9`). To build and run the container locally:

```bash
docker build -t cubycloud .
docker run -d --name cubycloud -p 8383:8383 \
  -e JWT_SECRET=your-random-32-byte-secret \
  -e COOKIE_SECURE=true \
  -e ALLOWED_ORIGINS=http://localhost:8383 \
  -v $(pwd)/data:/app/data \
  cubycloud
```
## System requirements

- **Source development:** Go 1.25 or newer and Bun 1.x. Air is only needed for the `make dev` convenience script.
- **Storage:** a writable persistent data directory for the SQLite database, file data, and temporary upload chunks. S3-compatible storage is optional.
- **Operating systems:** no official OS compatibility matrix is published. Use a Go/Bun toolchain available for your development host.
- **CPU and memory:** no measured minimum requirements or performance benchmarks are published. Plan capacity for your workload and monitor the host.

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

CubyCloud runs on infrastructure you control and stores application data beneath its configured data directory. Implemented safeguards include bcrypt password hashing, HTTP-only `SameSite=Lax` session cookies, rate limiting for login and public-share unlock requests, and encryption of saved S3-compatible storage credentials.

These safeguards do not replace TLS, host security, or backups. CubyCloud does not claim end-to-end encryption for uploaded file contents. See [SECURITY.md](SECURITY.md) for the current vulnerability-reporting guidance; a dedicated private contact channel is not published yet.

## Contributing and community

Bug reports, reproducible examples, and focused pull requests are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md) before preparing a change. Do not include secrets, recovery keys, or real user data in issues, logs, screenshots, or test fixtures.

## License

There is currently no license file in this repository. No license has been selected or granted; do not assume that copying, modifying, or redistributing this project is permitted.
