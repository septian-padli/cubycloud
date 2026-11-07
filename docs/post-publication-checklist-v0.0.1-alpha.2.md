# CubyCrate v0.0.1-alpha.2 Post-Publication Verification Checklist

This checklist is to be executed by the repository owner immediately following the publication of `v0.0.1-alpha.2` in T8.

---

## 1. GitHub Container Registry (GHCR) Verification

- [ ] **Package Visibility:**
  - Verify package exists at `https://github.com/septian-padli/cubycrate/pkgs/container/cubycrate`
  - Ensure package visibility is set to **Public** so users can pull without authentication tokens.
- [ ] **Tag Resolution:**
  - Confirm tag `v0.0.1-alpha.2` exists.
  - Confirm tag `alpha` exists and points to the same image manifest / digest.
  - Confirm tag `latest` **WAS NOT** created or updated (prerelease policy check).
- [ ] **Multi-Architecture Manifest:**
  - Inspect multi-arch manifest using Docker Buildx or crane:
    ```bash
    docker buildx imagetools inspect ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2
    ```
  - Verify both platforms are present in the list:
    - [ ] `linux/amd64`
    - [ ] `linux/arm64`

---

## 2. Remote Image Pull & Fresh Installation

- [ ] **Pull Image:**
  ```bash
  docker pull ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2
  ```
- [ ] **Start Fresh Container via Compose using Published GHCR Image:**
  ```bash
  CUBYCRATE_IMAGE=ghcr.io/septian-padli/cubycrate CUBYCRATE_VERSION=v0.0.1-alpha.2 docker compose up -d
  ```
- [ ] **Health Probe:**
  ```bash
  curl -fsS http://127.0.0.1:8383/health
  # Expected: {"status":"ok"} with HTTP 200
  ```
- [ ] **Setup Wizard:**
  - Open `http://localhost:8383/setup`
  - Create admin account and copy recovery key.
  - Complete first-run initialization.
- [ ] **Authentication & Cookies:**
  - Check developer tools › Cookies:
    - Verify cookie name is `cubycrate_token`.
    - Flags: `HttpOnly: true`, `SameSite: Lax`.
- [ ] **File Operations Smoke Test:**
  - Create folder in My Storage.
  - Upload file (small image or text document).
  - Preview file in browser.
  - Download uploaded file and verify content matches.
  - Delete file (moves to Trash) and permanently delete.
- [ ] **Container Restart Persistence:**
  ```bash
  docker compose restart cubycrate
  ```
  - Verify logged-in session survives restart or login works without re-triggering `/setup`.
  - Confirm uploaded files remain accessible.
- [ ] **Backup & Restore Smoke Test:**
  ```bash
  docker compose exec cubycrate tar -czf /tmp/test-backup.tar.gz /app/data
  docker cp cubycrate:/tmp/test-backup.tar.gz ./test-backup.tar.gz
  tar -tzf ./test-backup.tar.gz | grep "db/cubycrate.db"
  tar -tzf ./test-backup.tar.gz | grep "secrets/master.key"
  ```

---

## 3. GitHub Release Asset & Link Verification

- [ ] Check `https://github.com/septian-padli/cubycrate/releases/tag/v0.0.1-alpha.2`
  - Release title: `CubyCrate v0.0.1-alpha.2`
  - Marked as **Prerelease** (yellow tag indicator).
  - Release body includes Compose quickstart with `v0.0.1-alpha.2`.
- [ ] Check README badges on `https://github.com/septian-padli/cubycrate`:
  - CI badge shows passing status.
  - Release badge links to release workflow.
  - GHCR badge links to container package page.
