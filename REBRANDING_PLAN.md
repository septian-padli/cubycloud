# REBRANDING_PLAN.md — CubyCloud → CubyCrate

> **Version:** 1.0 · **Prepared:** 2026-11-03  
> **Status:** Implementation-ready plan, **not authorization to merge or deploy**.  
> **Scope:** Two independently managed OMP workstreams: `septian-padli/cubycloud` (application) and `septian-padli/cubycloud-page` (website/docs).  
> **Brand authority:** The identical `BRAND.md` v1.1 and the same owner-supplied `public/logos/` source assets must be accessible to both workstreams.  
> **Inputs:** `cubycrate-rebranding-audit.md`, `REBRANDING_AUDIT_PAGES.md`, each repository's current `DESIGN.md` and governing `AGENTS.md`/product contracts.  
> **Result:** Brand-facing CubyCrate identity with **no destructive or unnecessary technical migration**.

## 1. Decisions already settled

| Concern | Decision |
| --- | --- |
| Official display name | **CubyCrate** (one word; exact capitalization) |
| Canonical future marketing domain | `https://cubycrate.com` (owner-controlled; DNS/Cloudflare active) |
| Positioning | Lightweight, open-source, self-hosted file drive |
| Existing tagline | *Your files. Your server. Your rules.* — optional/provisional |
| Color system, fonts, tone, UI/UX, motion | **No changes**; obey the corresponding repository design system |
| Logo | Owner-selected 18-variant logo family, supplied in both repos under **root** `public/logos/`; SVG and WebP exports; use **original files** |
| App data and runtime identity | `cubycloud` technical identifiers remain unchanged in this wave |
| GitHub repos and GHCR | Existing repository names, URLs, image paths and release contracts remain unchanged |
| Cloudflare Worker | Keep `cubycloud-page` and its existing Workers Builds integration |
| Production deployment | Explicit approval required; currently held during rebrand |
| DNSSEC | Owner reports Cloudflare success and registrar confirmation; unrelated to code authorization |

**Hard limits:** No blanket search-and-replace; no redesign; no feature additions; no new colors, fonts, brand voice or logo variations; no repository rename; no migration of database, object keys, cookie/storage state, Compose volumes, Go module or runtime env names; no unapproved production action. Record apparent conflicts and request specific owner approval rather than guessing.

## 2. Execution model, branch isolation, and single source of truth

The two OMP sessions are **separate**. They may operate concurrently once the shared contract is copied, but each owns only one repository. There is **no shared working tree** and no assumption that a file edited in one repo appears in the other.

| Agent | Repository | Start from | Proposed feature branch | Merge target | Responsibility |
| --- | --- | --- | --- | --- | --- |
| **OMP A** | `septian-padli/cubycloud` | latest `origin/main` | `feature/rebrand-cubycrate` | `main` via PR | Product Go/SvelteKit public identity, localized UI, docs, Swagger text, assets and safety tests |
| **OMP B** | `septian-padli/cubycloud-page` | latest `origin/staging` | `feature/rebrand-cubycrate` | `staging` via PR | Astro/React landing, Starlight, Pagefind, SEO, logos/favicons, staging tests |

Same branch name in different repositories is harmless. Agents must `fetch` and confirm their actual base SHA and worktree state before branching; preserve unrelated local/uncommitted user changes (including any design/figcraft edits). Do not delete, reset, force-push, or merge unrelated work. If a branch already exists, inspect it and choose a safe continuation rather than overwriting it.

**Contract sync:** Place the identical `BRAND.md` v1.1 and `REBRANDING_PLAN.md` content at each repository root, ideally committed with the workstream PR. The owner should circulate the same documents to both OMPs. If either agent proposes a changed brand rule, **stop and obtain a single approved contract revision for both** before editing.

**Governance:** `BRAND.md` governs branding and transition boundaries; app `docs/prd.md`, `docs/architecture.md`, `apps/frontend/SITEMAP.md` and implementation/tests govern behavior and security; each repo's `DESIGN.md` governs its visuals. Existing website `MOTION.md`, `DECISIONS.md`, `BRIEF.md`, `CONTENT.md`, and `MIGRATION_CONTRACT.md` continue to govern their respective concerns, with historical brand references superseded only for presentation.

## 3. Shared asset inventory and packaging gate

The owner reports the *same* `public/logos/` source tree in each repository. Source SVGs are the visual authority; do not reconstruct approved logos using CSS text, generated graphics, or screenshot crops. Audit the **actual** file list, contents and checksums before coding.

**Expected variant families** (each `color`, `black`, `white`, each intended `svg` and `webp`):

| Family | Source path / basename pattern | Common usage |
| --- | --- | --- |
| Icon | `public/logos/icon/cubycrate-icon-*` | favicon source, compact icon |
| Horizontal default | `public/logos/horizontal/default/cubycrate-horizontal-*` | navbar, app header |
| Horizontal with tagline | `public/logos/horizontal/with-tagline/cubycrate-horizontal-tagline-*` | large brand panel only |
| Vertical default | `public/logos/vertical/default/cubycrate-vertical-*` | centered/prominent placement |
| Vertical with tagline | `public/logos/vertical/with-tagline/cubycrate-vertical-tagline-*` | large editorial use only |
| Wordmark | `public/logos/wordmark/cubycrate-wordmark-*` | text-only brand compositions |

The screenshot is not a full disk manifest. **Gate A0: every referenced source file must be present**; report missing `horizontal/default` variants or other discrepancies rather than creating lookalikes. Compare SVG and WebP to confirm expected presentation. Validate each SVG for `viewBox`, font portability, safe external references and expected transparency. Where feasible, compare file hashes across repos to ensure source variants are identical; if different, resolve before committing.

**Deployment nuance:** Astro serves `public/` as root static assets. The application, however, builds SvelteKit under `apps/frontend/` and embeds its output in Go. Root-level `public/logos/` may **not** be served by its app build. OMP A must confirm the actual static-directory/import convention and implement a reproducible inclusion/copy process (e.g. from root `public/logos/` to the frontend's valid static build input) without altering the original logo sources. Verify those files inside the final embedded Go binary/container as well as in dev mode.

**Derived deliverables** (only if required and visually reviewed): favicon SVG/ICO, 16/32/48-px tests, Apple touch icon 180×180, appropriately sized raster icon, and 1200×630 OG card. Do not claim that a logo SVG automatically covers every browser/icon/social requirement. Do not generate a new icon design. Any derived asset must be generated from the approved logo and approved brand palette, and stored in a documented output path.

## 4. Workstream A — application (`cubycloud`)

### A1. Preflight

1. Read root/backend/frontend `AGENTS.md` and affected PRD, architecture, `SITEMAP.md`, `DESIGN.md` sections; inspect current Git tree and untouched source logo folder.
2. Establish baseline: `git status`, current SHA, Go/backend tests, frontend checks, and available Docker/E2E baseline (record pre-existing failures honestly).
3. Compare references from the application audit to *current* code; the audit is an inventory, not automatic permission to refactor every match.

### A2. Apply **public-facing brand only**

- Replace **display strings** in app title, login, setup, sidebar/drive, header, public share, error screens, metadata, descriptions, tooltips, accessibility labels, email-/download-facing presentation names where safe, and both English and Indonesian localizations.
- Use approved SVG/WebP lockups instead of recreating the logo in ordinary markup. Keep the existing Flat Earthy geometry, spacing, interaction, light/dark tokens and language behavior.
- Adapt/replace any visible old-brand placeholder/fallback icons only with the official provided sources or derived files confirmed in §3. Do not leave a default Svelte/framework favicon in production.
- Update README, public docs, release guide wording, CONTRIBUTING and applicable issue templates/Swagger/OpenAPI **display text**; retain real install commands, links, repository paths and API contracts. Regenerate bundled OpenAPI docs with the repo's existing process and verify route/spec consistency.
- Keep actual license, copyright attribution, release provenance and historical contribution records accurate. No unsupported AGPL or security claims; verify implementation truth before changing marketing statements.
- Keep existing home/landing UI and features intact: this is not permission to build a new marketing site inside the app.

### A3. **Protected runtime identifiers: do not rename**

| Area | Preserve exactly unless owner authorizes a separately tested migration |
| --- | --- |
| Backend module | `github.com/cubycloud/backend` and Go imports |
| Physical object keys | `cubycloud/{instance_id}/{node_id}/blobs/...`, scanner/orphan behavior |
| Database and WAL | `db/cubycloud.db` and sidecars, existing migrations and data layout |
| Storage volume | Docker named volume `cubycloud_data` and its host/bind path behavior |
| Docker contracts | `cubycloud` Compose service/container, `cubycloud:8383` reverse-proxy upstream and published paths |
| Distributable | `ghcr.io/septian-padli/cubycloud` and existing image/tags/package names |
| Env and secret names | Existing `CUBYCLOUD_*`, cookie/security flags, encryption secret files |
| Authentication | `cuby_token` cookie, active sessions, tokens |
| Browser state | `cubycloud-theme`, `cubycloud-drive-view`, `cubycloud-language:*`, `cubycloud_setup_draft`, IndexedDB `cubycloud-transfer-queue`, Web Locks and BroadcastChannels |
| Binary/deployment | Persistent data directories, `/app/cubycloud` binary path/entrypoint unless a separate approved migration exists |

The application audit proposes optional Waves 2–5 (new env aliases, cookie dual-read, namespace migration, Compose and Go module rename). These are **explicitly deferred**, not part of this workstream. New *public presentation text* may be renamed; do not change stored keys simply to make strings look consistent.

### A4. Product tests and deliverables

- Run Go tests (`cd apps/backend && go test ./...`; include `-race` where the CI environment supports it), OpenAPI bundle/coverage checks, frontend `bun install --frozen-lockfile`, `bun run check`, `bun run lint`, appropriate unit tests and Playwright E2E.
- Build Docker with `push: false`; inspect the final container's embedded frontend and logo asset URLs. Test a local instance against a **safe copy** of representative pre-rebrand data, without modifying production user volumes.
- Regression samples: onboarding, sign-in/session, file upload/download/preview, sharing, trash, local/S3 storage, quotas, app language/theme, and all affected logo placements on dark/light and responsive screens.
- Verify upgrade compatibility: existing `cubycloud.db` opens unchanged, `cubycloud_data` stays attached, legacy storage keys remain readable, existing encrypted S3 credentials decrypt, and no surprise setup/reset/logout/data loss occurs.
- Open a PR to `main` with file summary, before/after images, test evidence, preserved identifier list, and explicit statement **no package release or deployment performed**. The merge/release needs owner approval.

## 5. Workstream B — website/docs (`cubycloud-page`)

### B1. Preflight

1. Fetch latest `origin/staging`, confirm current Astro/Starlight baseline, old PR #2 state and branch SHA; record working tree and preserve unrelated design edits.
2. Read website `AGENTS.md`, `DESIGN.md`, `MOTION.md`, `DECISIONS.md`, `BRIEF.md`, `CONTENT.md`, `MIGRATION_CONTRACT.md`, website audit and the shared brand contract.
3. Verify approved `public/logos/` files against §3 and current build/output paths.

### B2. Landing, docs and asset integration

- Replace display name in `src/components/SiteNavigation.tsx`, `HeroExperience.tsx`, `Sections.tsx`, `FileExplorerDemo.tsx`, `StorageSelectorDemo.tsx`, `src/pages/index.astro` and relevant synthetic labels/copy. Keep Three.js `StorageCoreScene`, GSAP narrative, demos, breakpoints and layout exactly as before.
- Replace obsolete CSS/text wordmarks and placeholder favicon with actual approved logos; choose `color`/`black`/`white` based on real contrast at light/dark positions. Remove unverified `®`; keep tagline variants optional, especially in tiny views.
- Update Starlight title and **all seven** currently audited MDX articles under `src/content/docs/docs/` while preserving *working* `cubycloud` GitHub/GHCR/Compose/DB commands. Add a concise transition note where necessary; do not fabricate an existing CubyCrate Docker image or repo.
- If legacy `src/docs/` prototype content remains part of the shipped build or referenced tooling, align **public-facing** names only and preserve `cubycloud-docs-theme` persisted key. Do not accidentally activate/replace the old React docs prototype.
- Ensure `src/config.ts` and footer CTAs point to the **existing live product repo and verified installation docs**, not speculative future repository paths.
- Set production Astro `site`/canonical to `https://cubycrate.com`; update social metadata and accurate OG-card references. Keep marketing/docs aesthetics and text voice unchanged.
- Inspect whether Starlight already generates a sitemap with the configured `site` or whether additional integration is needed. Verify actual `dist` files and URLs; only then publish the correct `Sitemap:` directive in robots. Do not presume a nonexistent sitemap exists.
- Use a **production-indexable** robots policy only where appropriate. Prevent staging/preview indexing by **environment-specific** verified controls; never add a global `Disallow: /`/`noindex` that leaks to production. The Worker `workers.dev` URL is a public endpoint until verified otherwise.
- Preserve `wrangler.jsonc` Worker name `cubycloud-page`, existing static asset routing and GitHub Workers Builds, including its temporary `staging` production branch during QA.

### B3. Website validation and deliverables

- `pnpm install --frozen-lockfile`, `pnpm check`, `pnpm build` (includes expected Pagefind patch/build), verify HTML assets and expected doc routes, and `pnpm exec wrangler deploy --dry-run`.
- Test resolved logo paths, no SVG/font problems, canonical/OG output, Pagefind worker and search, robots/sitemap, direct deep-link refresh and 404 page, all Docs routes, mobile drawer, Starlight light/dark, GSAP, Three.js and interactive demos.
- Browser screenshot tests at narrow mobile, typical mobile, tablet and desktop; assess accessible logo labels, meaningful alt text, visual parity, no horizontal overflow, no JS errors or missing network assets.
- Open a PR from `feature/rebrand-cubycrate` into `staging`, not `main`; obtain CI evidence. Keep current `staging → main` release PR #2 open for owner-controlled promotion. **Do not merge or deploy production without authorization.**

## 6. Integration, QA and release gates

| Gate | Must be true | Responsible |
| --- | --- | --- |
| **G0 — Preflight** | Both repos have identical `BRAND.md` and logo originals; working trees safe; branches and base SHAs recorded | Both OMPs |
| **G1 — Code/asset review** | No protected internal identifiers renamed; source logos untouched; all public branding aligned | Both OMPs |
| **G2 — App tests** | App build, Go/Frontend tests, Docker package inspection, functional upgrade/data-compat tests passed or blockers disclosed | OMP A |
| **G3 — Website tests** | `pnpm` checks/build/Pagefind, docs routes, asset URLs, accessibility/screen tests and CI passed or blockers disclosed | OMP B |
| **G4 — Cross-repo truth** | Same logo/wordmark and positioning; feature claims and install examples match verified product release | Owner + both OMPs |
| **G5 — Staging** | Website PR merged to `staging` **with owner approval**; Cloudflare auto-deploy from exact SHA; live regression PASS | Owner + OMP B |
| **G6 — Production approval** | Owner explicitly approves app release timing, website `main` merge, Cloudflare branch switch and domain cutover separately | Owner |
| **G7 — Live production** | `main` SHA verified deployed; HTTPS/canonical/robots/sitemap/analytics if any; app docs/downloads valid; www behavior validated | Owner + OMP B |

### Operational detail: today's Cloudflare topology

At the time of the website audit, `staging` was Cloudflare's **temporary production branch** on existing Worker `cubycloud-page`. Its `workers.dev` URL displayed the staging build. **Merging into `staging` can change that publicly accessible deployment even before a custom domain is connected.** Verify the current dashboard before release actions.

The website `staging → main` release **PR #2** was held open in the audit. Do not assume the status is unchanged; verify before action. The intended sequence:

1. Owner reviews and merges the **website feature PR into `staging`**. This triggers a Cloudflare build under the existing integration; verify the deployed SHA, logo URLs and QA before proceeding.
2. Owner reviews the updated **PR #2** (`staging → main`) and product release readiness. Any commit added to `staging` should appear in the PR, but check actual diff and CI status.
3. On explicit approval, merge PR #2 into `main`. In Cloudflare Worker **Settings → Builds**, change the production branch from `staging` to `main` as appropriate and **ensure a fresh build for the intended `main` commit actually runs**. Changing a branch setting alone is not proof the correct version was deployed; use a supported build trigger or authorized Wrangler deployment if needed.
4. Verify `workers.dev` serves the exact `main` commit; confirm that `staging` now has a separate **Worker Preview** deployment/URL and does not overwrite production. Don't invent preview URLs.
5. After the branded production Worker is verified, attach **`cubycrate.com`** as an existing Worker's custom domain via Cloudflare dashboard; verify Cloudflare-managed DNS/SSL, HTTPS, canonical apex and optional `www → apex` redirect. Do not create a second Worker or buy external SSL.
6. Verify production `robots.txt`, sitemap, canonical, OG image, all docs routes, download/install links, and direct refresh. Keep HSTS off until the HTTPS/domain setup is proven stable and owner opts in.

**Product application sequencing:** Coordinate the app's owner-approved branding merge/release and image availability with marketing launch. The public website may use the new brand while legacy GHCR/repo identifiers remain accurate; it must not claim the app's new branded release is published until it exists.

### Rollback

- **Website:** Identify the last known-good Worker deployment ID *before* each cutover. If the new release fails, use supported Cloudflare rollback to that deployment (or authorized `wrangler rollback`), verify live URLs and restore branch/build routing consistency. Avoid relying solely on Git revert as an instant deployment rollback.
- **App:** Keep last known-good image tag and an offline, verified backup of data directory/SQLite/secrets. Roll back the container image without changing its named volume, database filenames, object key namespace or encryption secrets. Never use a new empty volume as a workaround.
- **Domain:** Custom-domain/DNS changes require owner approval; maintain a clear path back to the last working Worker deployment. DNSSEC is already enabled per owner confirmation; do not change nameservers or DS records as a casual rollback action.
- **Git:** Revert or follow-up PR rather than resetting published branches. Preserve existing PR history and unrelated file modifications.

## 7. Explicitly deferred: technical identity migration

Not included in the approved rebranding wave: renaming GitHub repositories, GHCR image namespaces, Docker Compose service/container/volume, data directories or SQLite file; blob prefixes, orphan scanner, S3 keys, Go module path; cookies, browser storage, IndexedDB, environment names or encryption secrets; Cloudflare Worker ID; independent website redesign or app architecture changes. If desired later, draft a **separate** migration RFC with backups, dual-read/fallback rules, explicit upgrade tests, versioning, communication plan and rollback.

## 8. Handoff requirements (what each OMP must report)

The end-of-task report from **each** agent must include:

1. Repository, base/head branch and commit SHAs, working-tree preservation statement, PR link if created.
2. Actual logo source manifest, validation results, served paths, and any missing or malformed files.
3. Diff summary distinguishing public branding from untouched protected identifiers, plus before/after screenshots.
4. Explicit commands executed and passing/failing results; never report unrun tests as PASS.
5. Legacy-name search report categorized as **approved internal legacy / historical text / accidental public leak**. A raw `grep cubycloud` count of zero is neither required nor desirable.
6. Discrepancies in docs/product truth, risks, blockers, and dependencies on the other repository.
7. Owner-controlled next actions and rollback-ready deployment/package identifiers (where applicable).

**Completion definition:** CubyCrate appears consistently wherever the public experience exposes brand identity, both repositories use the same supplied assets, all old technical data/install contracts still function, the staging build is verified, and no production change has happened without explicit owner approval.
