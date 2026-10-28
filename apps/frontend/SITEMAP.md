# SITEMAP.md — CubyCloud Frontend Routes & Page Contracts

> **Project:** CubyCloud v1.0.0 · **Audience:** Frontend, UI/UX, QA  
> Product rules: [`docs/prd.md`](../../docs/prd.md) · Visuals: [`DESIGN.md`](./DESIGN.md) · Frontend conventions: [`AGENTS.md`](./AGENTS.md)

## How agents should use this document

Locate the relevant route below and read its contract plus any applicable **Global contracts**. Do not reread unrelated routes for each task. Changes to route, access, URL state, or page behavior must update the appropriate section. API implementation/security rules remain owned by the backend and PRD.

## 1. Global contracts

- **Routing:** SvelteKit route groups `(onboarding)`, `(auth)`, `(drive)`, `(public)` isolate layouts/guards without changing visible URLs. `(drive)` uses the authenticated shell (sidebar, search, header upload).
- **Health:** `GET /health` is a **public backend endpoint**, not a page. HTTP `200` only if all configured storage nodes are reachable, otherwise `503`; probe every registered node. Public response must not reveal physical node identifiers, names, paths, or credentials.
- **Storage offline:** My Storage/Organization file records expose `storage_available` for files. Keep unavailable files and metadata listed; disable operations needing the offline node and explain using localized hover/click shadcn Popover. Folders remain navigable. Regular users must not see physical storage identifiers; badges/names are Super Admin only.
- **Languages:** English/Indonesian. Browser locale starting `id` defaults to Indonesian, other locales default to English. Explicit preference is **browser `localStorage` only**, keyed by authenticated user ID, never persisted in backend. Translate user-facing labels, errors, toasts, dialogs, statuses, and accessible names; retain appropriate technical/product terms and user content. Global switch: Globe icon + shadcn-svelte Select showing `id` / `en`.
- **Global search:** Header supports debounced filename/folder preview and opens `/drive/search?q=...&page=...`. Search accessible My Storage, Organization, and Shared With Me; no Trash or full-content search.
- **Global header Upload:** Normally uploads to **My Storage root**, even when browsing a My Storage/Organization subfolder. Exception: inside an active Shared With Me **folder** with effective `Editor` rights, upload to that shared folder. Shared root/list or insufficient access (e.g., Viewer): show a denial dialog **without fallback** to My Storage. Use established upload/transfer behavior and backend authorization.
- **Permissions:** UI visibility is not authorization. Backend validates ownership, roles, effective shared permissions, and direct URL access.

## 2. Route map and navigation

```text
src/routes/
├── +layout.svelte, +page.svelte                   # global providers; / landing
├── (onboarding)/setup/+page.svelte                 # /setup
├── (auth)/login/+page.svelte                       # /login
├── (auth)/recovery/+page.svelte                    # /recovery
├── (drive)/drive/
│   ├── home/+page.svelte                           # /drive/home
│   ├── search/+page.svelte                         # /drive/search
│   ├── my-storage/+page.svelte                     # /drive/my-storage
│   ├── organization/+page.svelte                   # /drive/organization
│   ├── shared/+page.svelte                         # /drive/shared
│   ├── starred/+page.svelte                        # /drive/starred
│   ├── trash/+page.svelte                          # /drive/trash
│   ├── storage-stats/+page.svelte                  # /drive/storage-stats
│   └── profile/[userId]/(+page.svelte, +page.ts)  # /drive/profile/:userId
├── (drive)/admin/
│   ├── storage/+page.svelte                        # /admin/storage
│   ├── settings/+page.svelte                       # /admin/settings
│   └── users/+page.svelte                          # /admin/users
└── (public)/s/[token]/+page.svelte                 # /s/:token
```

- `/` is prelaunch coming-soon; after launch, redirect per app configuration.
- `(drive)` sidebar (all authenticated): **Beranda** `Home` → `/drive/home`; **My Storage** `HardDrive`; **Organization** `Building2`; **Dibagikan pada saya** `Users`; **Berbintang** `Star`; **Trash** `Trash2`; **Penyimpanan** `PieChart`. All navigate to corresponding `/drive/...` routes.
- Quota Widget sits below drive navigation and above the Super Admin divider. Super Admin-only menu: **Penyimpanan** `HardDrive` → `/admin/storage`; **Admin Settings** `Settings` → `/admin/settings`; **Users & Quotas** `UserCog` → `/admin/users`.

## 3. Onboarding and authentication

### `/setup` — first boot

- **Access:** unauthenticated only while `is_setup_completed === false`; after successful setup continue to `/drive/home`.
- **Flow:** (1) Super Admin username/email/password (minimum 8 characters); (2) local disk scanner for `/app/data/storage/*`, optional Linux device, node capacity cap; (3) strategy `FILL_FIRST` / `ROUND_ROBIN` / `MOST_FREE_SPACE`; (4) optional Cloudflare R2, AWS S3, or S3-compatible endpoint/credentials; (5) generate/export Emergency Recovery Key.
- **Modal:** Recovery Key displayed as **30 uppercase alphanumeric characters in six groups of five** (`XXXXX-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX`), Copy/Download and mandatory acknowledgment. Show Device Scan Error Modal on scan/mount failure.
- **CTA:** `Selesaikan Setup & Masuk ke CubyCloud`.

### `/login` — sign in

- **Access:** unauthenticated; valid JWT session redirects to `/drive/home`.
- **UI:** username/email and password; `Masuk`; JWT is set via HTTP-only cookie. Recovery link `Pulihkan Akses Admin` → `/recovery`; forgot-password feedback explains that v1.0 uses an Emergency Recovery Key, not SMTP.

### `/recovery` — Super Admin password reset

- **Access:** unauthenticated; accepts current 6×5 recovery key and new password; **does not support legacy 64-character hexadecimal keys** in the UI.
- **Success:** show replacement `new_recovery_key`, require user to save it, then require login again; prior key is single-use. CTA `Reset Password Super Admin`.

## 4. Authenticated drive pages

### `/drive/home` — default landing

- **Access:** authenticated.
- **Components:** Quick Upload Drawer to My Storage (quota applies), URL-driven File Preview Lightbox, storage summary matching sidebar quota.
- **Quick Access:** last **five** opened files/folders for current account, newest first, persisted per account.
- **Recent activity:** last **five distinct** files/folders with upload/change events for current account, including rename/move in My Storage and Organization; deduplicate per item.

### `/drive/search` — global results

- **URL:** `?q=query&page=number`; authenticated.
- **Hybrid UX:** header preview + link to full paginated results here.
- **Scope:** accessible filenames/folder names in own My Storage, Organization, and granted Shared subtrees. Exclude Trash and file-content searching.
- **Results:** source, location, type, size, modified time. Shared paths must not expose private ancestors above the shared root.
- **Navigation:** folder → accessible source folder; file → preview in its parent folder. Access must be revalidated on navigation.

### `/drive/my-storage` — personal file manager

- **URL:** `?folder=folder_id&preview=file_id`; authenticated.
- **Explorer:** category filters; list/grid according to existing view toggle; filename, size, date, actions; physical node badge **Super Admin only**.
- **Actions:** file/folder upload via picker or drag/drop (including hierarchy and per-file errors); download; streaming ZIP download; Compress to ZIP (asynchronous persistent archive created in the folder where action was triggered, with conflict resolution and background queue progress); rename; move; recursive zero-byte-reference duplicate; delete recursively to Trash; share; star; preview image/video/text/code/PDF/audio.
- **Conflicts:** upload/duplicate automatically get next available numeric suffix; rename/move offer suggested available name or cancel. No overwrite/merge folders.
- **UI:** New Folder, Folder Delete Confirmation (all contents to Trash), Share Sheet, File Preview Lightbox, quota warnings/errors.
- **Upload queue:** floating top-right below nav; progress weighted by file size and 100% only after server confirmation; expandable queued/uploading entries until dismissed or swiped right; completion toast bottom-right.
- **Offline storage:** preserve listed metadata and folders; disable only file operations that require unavailable physical storage; show localized Popover without exposing node identity (see Global contracts).

### `/drive/organization` — shared organization drive

- **URL:** `?folder=folder_id&preview=file_id`; accessible to **all registered users** with full read/write/download/delete rights.
- **Model:** fixed Organization tree, **not** personal folders marked by an admin. Uses shared file explorer and corresponding previews/conflict modal.
- **Quota:** Organization items **do not count toward per-user quota**; enforce physical-node capacity.
- **Actions & UI:** supports same upload, streaming ZIP, Compress to ZIP (archive stored in current folder), rename, move, duplicate, and delete-to-trash actions as My Storage. Hide `Bagikan` action because internal access is already shared.

### `/drive/shared` — Shared With Me

- **URL:** `?folder=folder_id&preview=file_id`; authenticated.
- **UI:** shared items table, owner, `Viewer`/`Editor` permission badge, Share Details Modal, previews including Viewer access.
- **Actions:** Viewer can download/preview; Editor additionally can edit, delete, and share; actual rights enforced by backend.
- **Header upload:** only when inside a shared folder with effective **Editor** permission; upload into that folder. Viewer or Shared list/root → **denial dialog**, no fallback (see Global contracts).
- **Quota:** upload is charged to the **share owner**, not the recipient.

### `/drive/starred` — starred items

- **URL:** `?preview=file_id`; authenticated. Includes starred items from My Storage, Organization, and Shared; actions include unstar and Share Sheet. Double-click or `Lihat Pratinjau` opens File Preview Lightbox.

### `/drive/trash` — lifecycle

- **Access:** authenticated. Trashed files **still count toward user quota until purged**.
- **UI:** trashed items and remaining time before **30-day auto-purge**; Restore and permanent deletion; missing parent on restore → Missing Parent Folder Modal and fallback to ROOT; permanent-delete confirmation in danger styling; `Kosongkan Sampah (Purge Trash)`.

### `/drive/storage-stats` — usage

- **Access:** authenticated. Show effective used/limit GB or `Used: X GB` for unlimited; category chart (Video, Gambar, Dokumen, Arsip, Trash); top **10 largest files**, excluding Organization; Cleanup Advisor Modal; shortcut to Trash.

### `/drive/profile/:userId` — account profile

- **Access:** authenticated. Own profile editable. **Only Super Admin** may open another user's profile, and it is **read-only**. Regular users cannot access other users' profiles.
- **Read:** display name, email, username, role, effective quota, actual usage. Username, role, and quota are not editable here.
- **Edit:** display name/email; changing email requires current password, valid format, and case-insensitive uniqueness.
- **Password:** require current password, minimum-length validation, and confirmation. Success revokes **all sessions**, then login again.

## 5. Super Admin pages

### `/admin/storage` — system usage

- **Access:** Super Admin only.
- **Summary:** sum registered nodes' `capacity_bytes`; physical/file usage computed from deduplicated `disk_key` `size_bytes`; show total capacity, used space, remaining capacity, node count, and usage ratio.
- **Largest consumers:** descending user usage, avatar/username/email/role/quota, used GB and share of total; infinite pagination `limit=20`, `offset`, `has_more`, `next_offset` with Intersection Observer.
- **UI:** loading skeleton, empty/error, refresh CTA, link to `/admin/users`.

### `/admin/settings` — storage nodes and configuration

- **Access:** Super Admin only.
- **Nodes:** capacity, physical usage, free space, critical **5%** badge; node names required and case-insensitively unique; Detach Node and Rescan Nodes modals.
- **Settings:** default user quota in GB or Unlimited; audit log retention **7/14/30/60/90 days**, default **90**; worker concurrency default **2**.
- **Backup CTA:** `Download DB Backup (.db)` (database backup, not necessarily complete file/content backup).

### `/admin/users` — people and quotas

- **Access:** Super Admin only. Users table: name, email, role, effective quota, used GB, actions.
- **Actions:** Create Regular User (no password field, default password notice `12345678`, default quota inheritance); Edit Quota (GB, Unlimited, Reset to Default); Reset Password to default `12345678` with modal confirmation dialog (hidden for own account, rejected by API for self-reset); Delete User (confirmation dialog). Super Admin always Unlimited and cannot be limited.
- **Note:** `/admin/addons` was removed. ZIP downloads and file archive functionality are core, permanently enabled.

## 6. Public sharing

### `/s/:token` — public/restricted shared link

- **URL:** `?preview=file_id`; public or restricted guest access subject to link rules.
- **UI:** password challenge, restricted-login prompt → `/login?redirect=/s/{token}`, different expired/not-found states; single-file card or folder explorer.
- **Actions:** direct download, authorized stream preview, built-in ZIP download, File Preview Lightbox (including PDF).
- **Existing UI demo tokens only:** `demo-public`, `demo-folder`, `demo-password` (`demo123`), `demo-restricted`, `demo-expired`; do not treat demo tokens as production access.

## 7. Shared UI interaction contracts

### URL-driven preview

- Opening a file uses the page's `?preview=file_id` query parameter; SvelteKit may use `goto('?preview=file_id', { keepFocus: true, noScroll: true })`.
- Applies to My Storage, Organization, Shared, and public `/s/:token` within access limits.
- Formats: images PNG/JPEG/WebP/GIF/SVG; MP4/WebM video via Vidstack/native media; text/code/Markdown via plain text/PrismJS; MP3/WAV/OGG audio; PDF via PDF.js (`pdfjs-dist`), always built in.
- Preview fetches existing download endpoint with **opt-in `?preview=1`** for inline allowed MIME responses; use HTTP Range where supported. Ordinary downloads remain attachments. Page `?preview=file_id` controls lightbox state, **not** backend access rights.
- Visual overlay/container are specified in `DESIGN.md`.

### Quota widget and visibility

| State                           | Display / response                                      |
| ------------------------------- | ------------------------------------------------------- |
| Limited                         | `My Storage`, `8.2 / 15 GB`, flat progress bar          |
| ≥80% used                       | Warning-colored bar; warn on My Storage write attempts  |
| At 100% / `USER_QUOTA_EXCEEDED` | Block upload; `Kuota My Storage penuh. Hubungi admin.`  |
| Physical pool full              | `Penyimpanan server penuh.` (different from user quota) |
| Unlimited / Super Admin         | `Used: 8.2 GB`, **no cap or limit progress bar**        |

- Node badges/physical node identity in explorer: **Super Admin only**. `/admin/settings` storage-node UI is Super Admin only.
- Organization files: **never** included in personal usage/quota.
