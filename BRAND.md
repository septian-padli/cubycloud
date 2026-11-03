# BRAND.md — CubyCrate Brand Identity Contract

> **Version:** 1.1 · **Updated:** 2026-11-03  
> **Status:** Brand identity locked; owner-selected logo family supplied in both repositories. Source-vector integrity, small-size rendering, and deployment mapping remain verification gates.  
> **Applies to:** `septian-padli/cubycloud` (product application) and `septian-padli/cubycloud-page` (marketing website and documentation).  
> **Purpose:** Shared, implementation-independent brand contract for two separately operated OMP agents. This document authorizes **no code change, merge, repository rename, domain cutover, or deployment by itself**.

## 1. Non-negotiable decisions

| Field | Approved rule |
| --- | --- |
| Official product and brand name | **CubyCrate** — one word, capital **C** in both parts |
| Machine-readable spelling | `cubycrate` (lowercase, no separator) when a **new** technical identifier is explicitly authorized |
| Website domain | `https://cubycrate.com` (registered and under owner control; attachment to Worker is a separate release action) |
| Category / positioning | **Lightweight, open-source, self-hosted file drive** |
| Previous identity | CubyCloud; transitional technical identifiers may still contain `cubycloud` |
| Legacy tagline | **Your files. Your server. Your rules.** — retained as **provisional/optional**, not a mandatory brand lockup or required text on every surface |
| Product visual design | **Unchanged**; preserve the existing application's Flat Earthy system |
| Website visual design | **Unchanged**; preserve existing Creative Editorial / immersive marketing and branded Starlight systems |
| Brand colors, typography, voice, interaction style | **Unchanged**; do not introduce a new palette, typeface, tone, or redesign through rebranding |
| Official logo family | **Owner-selected CubyCrate artwork is supplied** at `public/logos/` in both repositories; 18 visual variants (SVG/WebP exports). Do not redraw or reinterpret the source artwork. |
| Phase-one scope | Public-facing name, approved copy/metadata/docs and associated assets only; **legacy runtime compatibility must remain intact** |
| Technical rename | Deferred to a separately reviewed, tested migration with explicit owner approval |
| GitHub repos and Cloudflare Worker | Retain existing technical names during phase one |

**Naming rule:** Write **CubyCrate** in natural-language titles, prose, UI strings, documentation, accessibility labels, and public metadata. Never write `Cuby Crate`, `Cubycrate`, or `CubyCloud` for *new public branding*. `cubycrate` is for suitable new machine identifiers, not ordinary prose. Styling may use CSS `text-transform` or an existing uppercase display treatment without changing the canonical brand spelling.

**Trademark:** Do **not** append `®` or imply registered-trademark status without documentary confirmation. Do not introduce `™` automatically. Registration of the domain is not a trademark clearance decision.

## 2. Positioning and product truth

**Short descriptor:** Lightweight, open-source, self-hosted file drive.  
**One-sentence description:** CubyCrate is an open-source file drive for organizing, accessing, and sharing files on infrastructure its operator controls.  
**Core promise:** Familiar file workflows without requiring a managed cloud provider.  
**Primary audiences:** Small teams and organizations; secondary audiences include homelab operators, developers, system administrators, and privacy-conscious individuals.

**Messaging hierarchy (preserve existing strategy):**

1. Familiar, focused file organization and sharing for small teams.
2. Local filesystem storage with optional S3-compatible object storage.
3. Infrastructure ownership and practical self-hosting.
4. Implementation detail and operational proof when the reader needs it.

**Claim discipline:** All feature claims, technical specifications, compatibility notes, installation instructions, screenshots, benchmarks, and release statuses must be verified against the actual product repository, tests, and released artifacts. A planned feature or PRD requirement is not proof of implementation.

- Do not invent native sync clients, real-time coediting, AI capabilities, file-version history, automatic replication, seamless migration between storage providers, advanced group RBAC, or enterprise controls.
- Do not present the Organization drive as multiple isolated team workspaces unless verified; the documented current model is a shared tree.
- Do not infer end-to-end encryption, guaranteed security/privacy, or unmeasured speed/RAM advantages merely from self-hosting or the word **lightweight**.
- Public product demonstrations must clearly remain illustrative when they do not interact with a live backend. Screenshots and flows must reflect real product behavior.
- Preserve verified open-source license and attribution wording; the source currently indicates **AGPL-3.0-or-later**, but agents must verify the current `LICENSE` before publication.

**Tagline policy:** The existing line, *Your files. Your server. Your rules.*, may remain where it fits the established copy. It is **not** a required footer, hero, logo, or metadata element. Do not generate a replacement tagline or revise the messaging tone without a separate approval.

## 3. Brand voice and language — preserve, do not reinvent

**Website/marketing:** Preserve the existing **Bold & Disruptive**, anti-complexity, product-led editorial voice specified in the website's `BRIEF.md` and `CONTENT.md`: concise, confident, direct, memorable, and factually conservative. Challenge needless complexity, not customers or competitors. Do not add vague SaaS superlatives, invented proof, or unsupported comparisons. Keep the approved narrative and CTAs: **Get Started** to verified installation docs; **View on GitHub** to the actual product repository.

**Documentation:** Preserve clear, accurate, procedural, approachable technical writing. Existing Starlight navigation, article structure, and real commands take precedence over promotional language. Documentation is English-first.

**Product application:** Preserve existing application UI microcopy, information hierarchy, terminology, and interaction tone. The app supports English and Indonesian; update affected localized labels consistently without changing product behavior or language preference logic.

**No cross-surface tone replacement:** Marketing can be expressive; file management and admin workflows must stay functional and concise. Rebranding does not authorize rewriting copy into a new personality.

## 4. Existing color system — immutable

These are the **existing approved semantic color roles**, shared as a reference across the two design systems. Use the actual project CSS variables/tokens as the implementation authority. **Do not recolor, remap, introduce hues, or change light/dark modes.**

| Semantic role | Light | Dark |
| --- | --- | --- |
| Background / `bg-app` | `#F0EEE3` | `#0F130E` |
| Surface / `bg-surface` | `#FFFFFF` | `#182017` |
| Primary | `#606C38` | `#99AB5E` |
| Primary accent | `#283618` | `#ADCB8A` |
| Secondary | `#DDA15E` | `#E1AB70` |
| Secondary accent | `#BC6C25` | `#D78032` |
| Main text | `#283618` | `#F0EEE3` |
| Muted text | `#5A6B46` | `#9CA88F` |
| Subtle border | `rgba(0,0,0,0.06)` | `rgba(255,255,255,0.08)` |
| Danger (functional only) | `#EC003F` | `#FF3366` |

The website may already use approved alpha variations, depth, and gradients **between these existing values**; the application must retain its flat surface rules. Neither repo may reinterpret the other repo's visual constraints.

## 5. Typography, layout, motion, and accessibility — immutable

- **Common body/UI/docs font:** `Montserrat`, sans-serif. Keep existing weights, letter spacing, size scale, and loaded assets.
- **Marketing display headings:** Preserve the currently implemented font and hierarchy. The original website design document did not finalize a distinct display family; do not silently choose or introduce one as part of rename work.
- **Product application:** Maintain Flat Earthy: flat surfaces, no decorative shadows/glassmorphism/gradients, existing rounded controls, list/grid behavior, responsive patterns, accessible focus states, and Tailwind v4 tokens. See the app `apps/frontend/DESIGN.md` and actual `src/routes/layout.css`.
- **Marketing website:** Preserve current editorial composition, approved palette-based depth, GSAP scroll narrative, Three.js / React Three Fiber scenes, responsive recomposition, navigation, and interactive demos. Honor the more specific approved **no scroll hijack / no forced snapping / no pinning** motion decision wherever old exploratory specifications conflict. Refer to website `DESIGN.md`, `MOTION.md`, and `DECISIONS.md`.
- **Starlight:** Preserve standard reading ergonomics, sidebar hierarchy, Pagefind search, code blocks, copy controls, keyboard navigation, and light/dark themes. Do not copy experimental marketing motion into docs.
- **Accessibility:** Preserve keyboard/touch parity, visible focus, contrast, semantic labels, reduced-motion behavior, and responsive usability. The rename must not introduce clipping or wordmark line wraps on narrow screens.

## 6. Official logo system — approved source family

**Status:** The owner has selected a CubyCrate logo family and has copied the `public/logos/` folder into **both** repositories. This replaces the former placeholder-only decision. The design depicts an angular **C** enclosing an isometric crate/storage geometry; the supplied full-color variant uses the existing green and warm orange/sand brand family. The **actual SVG files**, not a screenshot or this description, are authoritative for exact geometry, color values, paths, stroke widths, and wordmark letterforms. This agreement is **not** permission for agents to redesign the mark.

### 6.1 Source paths and naming conventions

The following is the **intended repository-root layout** reported by the owner. The uploaded file-tree screenshot shows these directories and representative names but is **not an exhaustive file manifest**. Each OMP must inspect its actual local copy, verify the files, and report any missing pair before use.

```text
public/logos/
├── icon/
│   └── cubycrate-icon-{color|black|white}.{svg|webp}
├── horizontal/
│   ├── default/
│   │   └── cubycrate-horizontal-{color|black|white}.{svg|webp}
│   └── with-tagline/
│       └── cubycrate-horizontal-tagline-{color|black|white}.{svg|webp}
├── vertical/
│   ├── default/
│   │   └── cubycrate-vertical-{color|black|white}.{svg|webp}
│   └── with-tagline/
│       └── cubycrate-vertical-tagline-{color|black|white}.{svg|webp}
└── wordmark/
    └── cubycrate-wordmark-{color|black|white}.{svg|webp}
```

- **Six families:** icon, horizontal/default, horizontal/with-tagline, vertical/default, vertical/with-tagline, wordmark. Each has **three treatments**: `color`, `black`, `white`. Total intended **18 visual variants**, each ideally available in both `SVG` and `WebP` (up to 36 assets); actual files must be enumerated, not assumed.
- The **source SVG** is the preferred scalable asset; **WebP** is an approved raster export for specific presentation/performance needs. Do not recreate an SVG by tracing a WebP, or substitute a generated approximation.
- Do not rename, move, overwrite, simplify, optimize destructively, or recolor source assets in `public/logos/` without explicit owner approval. Reproducible deployment derivatives (e.g. favicon, PNG social card) may be generated **from** source in the appropriate destination after visual review.
- Validate SVGs for safe rendering, proper `viewBox`, expected font handling/outlined paths, embedded external resources, and whether source backgrounds are transparent. Never rely on a locally installed proprietary font for the logo to render correctly on a user's device.
- **No `®` / automatic `™`**. If a supplied asset unexpectedly contains such a glyph, flag it for owner review rather than editing the approved original without consent.

### 6.2 Variant usage rules

| Surface | Preferred asset | Rule |
| --- | --- | --- |
| App navigation / sidebar / login | Horizontal default `color` on compatible light surface; `white` on verified dark surface; icon if width constrained | Preserve existing sizing/alignment; do not add new decoration |
| Marketing website navigation / footer | Horizontal default, treatment matched to actual section background | Use the **supplied** lockup rather than reconstructing CubyCrate with ordinary font styling |
| Docs Starlight header | Horizontal default or icon at compact breakpoints | Keep Starlight reading ergonomics and existing theme toggles |
| Small UI mark, favicon source, PWA/app icon source | Icon family | Test silhouette and inner lines at 16/32/48 px; use owner-approved small-size derivative if necessary |
| Hero or centered branded panel | Vertical default if layout calls for it | Do not force vertical lockup into every section |
| Branded illustration or text-led composition | Wordmark | Avoid duplicating the brand name in accessible text unnecessarily |
| Campaign/brand panel explicitly requesting tagline | Horizontal/vertical `with-tagline` | Tagline remains **optional**; test readability at displayed size |
| Open Graph/social card | New composition from approved logo assets and existing color system | Separate 1200×630 asset needs review; do not treat logo variant as finished OG card |

- `color` is the normal brand treatment when contrast works. `black` suits light surfaces and monochrome contexts; `white` suits dark surfaces. **Do not infer that the SVG has an opaque background from a screenshot containing a preview tile.** Verify asset alpha/background directly.
- Use unchanged brand colors from §4; **the SVG's own approved artwork colors take precedence for the logo**. Never add CSS `filter: hue-rotate`, arbitrary recolors, glow, 3D extrusion, shadows, outlines, or gradient fills to the supplied logo.
- Preserve aspect ratio and original geometry. No stretching, rotation, horizontal mirroring, text retypesetting, cropping, or separating icon layers for new artwork without approval.
- Keep sufficient clear space relative to the symbol and test no cropping at target breakpoints. **No numeric minimum-size or clear-space specification is claimed yet**; the implementing OMP must propose evidence-backed values for owner approval if needed.
- Avoid tagline variants in navigation, favicon, compact mobile layouts, and any environment where lettering is illegible.
- Logo files are **brand source assets**, not a visual redesign mandate for the site's existing Three.js Storage Core or the app's components.

### 6.3 Serving and derived-asset constraints (different for each repository)

- **Website (`cubycloud-page`)**: Astro uses `public/` for static root assets; verify resolved URLs like `/logos/icon/cubycrate-icon-color.svg` against actual build and Cloudflare deployment.
- **Product (`cubycloud`)**: The provided `public/logos/` is at repository root, while the SvelteKit app lives under `apps/frontend/`. **Do not assume root `public/` is automatically served.** The app OMP must inspect `svelte.config.*`, Vite, `apps/frontend/static/`, asset imports, and Go `embed.FS` packaging. Choose a minimal, deterministic, non-destructive approach to serve/copy the approved files into the shipped app; preserve the root source directory.
- Derived `favicon.ico`, PNG touch icons, and OG image may be generated if required by the actual browsers/frameworks. Their output paths and dimensions must be verified in the production build. No such derivative is claimed to exist just because the icon SVG/WebP files exist.
- Both repos must render **the same canonical artwork**, not produce separate hand-adjusted variants.

## 7. Public names versus protected technical identities

**Fundamental principle:** `CubyCloud` in public *presentation* becomes `CubyCrate`; `cubycloud` inside existing runtime identifiers remains **unchanged in phase one**. Do not perform blanket, case-insensitive search-and-replace.

### 7.1 Public-facing targets (safe after normal review)

- UI/app names, accessible labels, browser titles, descriptions, welcome/setup/login/sidebar/footers, user-facing messages, documentation page titles, feature descriptions, README introductions, and OpenAPI/Swagger *display titles*.
- Website navigation/hero/sections/FAQ/footer, Starlight titles and prose, SEO descriptions, canonical/site metadata, Open Graph/Twitter *text*, and editorial alt text.
- Brand-oriented sample/demo filenames when they are clearly synthetic and do not affect serialized data or fixtures.
- `©` display names only after reviewing attribution and license implications; do not rewrite historical legal notices or contributions inaccurately.

### 7.2 Protected identifiers (MUST retain in phase one)

| Category | Protected examples / required treatment |
| --- | --- |
| GitHub repositories | `septian-padli/cubycloud`, `septian-padli/cubycloud-page`; retain current URLs and real destination links |
| Container distribution | `ghcr.io/septian-padli/cubycloud`, existing tags, pinned versions and install commands |
| Docker & networking | `cubycloud` Compose service/container, `cubycloud_data` named volume, network upstream `cubycloud:8383` |
| Database & secrets | `db/cubycloud.db` and its `-wal` / `-shm`, persisted secrets, master key material, backup paths |
| File/object storage | `cubycloud/{instance_id}/{node_id}/blobs/{blob_id}`, all existing blob keys, path parsers and orphan/reconciliation semantics |
| Environment & setup | `CUBYCLOUD_*` variables, defaults, installed Compose `.env` expectations |
| Go/backend | `github.com/cubycloud/backend` module and imports, internal binary/config paths and existing stable API contracts |
| Sessions | Existing `cuby_token` cookie and session handling |
| Browser state | `cubycloud-theme`, `cubycloud-drive-view`, `cubycloud-language:*`, `cubycloud_setup_draft`, IndexedDB `cubycloud-transfer-queue`, Web Locks/BroadcastChannels and other persisted keys |
| Website infrastructure | Existing Cloudflare Worker `cubycloud-page`, `wrangler.jsonc` resource identity, current GitHub Builds integration |
| Documentation commands | Real `git clone`, GHCR, Docker, volumes, database, environment and reverse-proxy commands must retain working legacy spelling |

**Documentation presentation rule:** Public prose should say **CubyCrate**, but installation examples must use the actual existing `cubycloud` commands/paths until a separately approved technical release changes them. Add a short factual transition note **only where readers could otherwise be confused**. Never publish a fictional `ghcr.io/.../cubycrate` image or GitHub URL.

**Upgrade compatibility:** Merely changing a volume or database name can silently select empty data; changing a blob prefix can make files unreadable or trigger orphan cleanup; changing master-key configuration can break decryption; changing browser-state keys can reset preferences or in-progress transfers. Preserve current behavior, data, credentials, and sessions. No dual-key/dual-prefix migration is needed for phase-one cosmetic rebranding.

## 8. URLs, SEO, and deployment boundaries

- **Canonical target for the future production site:** `https://cubycrate.com` (apex). Treat `https://www.cubycrate.com` as an optional redirect alias to configure and verify at release; do not presume it already exists.
- **Astro:** Set the production `site` property and accurate canonical/OG metadata when implementing the website rebrand. Verify which sitemap integration/generation actually exists; do not assume setting `site` alone produces XML. If a sitemap is generated, its published URL must be verified before inclusion in `robots.txt`.
- **Indexing:** Production may be indexable after release. Worker previews/staging must be protected from search indexing using verified **environment-specific** behavior; do **not** publish a global `Disallow: /` or `noindex` policy that would also suppress production.
- **External links:** Preserve live GitHub repository and image URLs until technical migration. Update navigation/documentation cross-links only to destinations that actually exist.
- **Cloudflare:** Preserve Worker `cubycloud-page`, existing Wrangler Static Assets routing, and GitHub Builds wiring. The custom domain is **not** attached as part of authoring this file.
- **Release order:** Website rebranding branch from `staging` → PR and CI → staging merge and live QA → owner approval → production PR/merge into `main` → verify production commit/build and domain association. Maintain existing PR #2 (`staging` → `main`) as a held release gate until reviewed; do not merge, switch the production branch, or attach `cubycrate.com` without explicit permission.
- **Product application:** Independent rebranding branch from its own `main`; build/test and package verification first. Coordinate released app/package names and documentation with website before making public install claims.
- DNSSEC status/registrar operational steps are infrastructure concerns, **not** permission to deploy.

## 9. Two-repository OMP handoff

The same **unchanged** `BRAND.md` content must be available to both agents. A common document may be copied to each repository root, or referenced from a pinned shared artifact; agents must not independently revise the contract.

| Workstream | Base / scope | Explicitly out of scope |
| --- | --- | --- |
| **OMP A — Application** | `septian-padli/cubycloud` from `main`; SvelteKit UI strings, product-facing assets, supported localization, brand-focused documentation and OpenAPI display text | Changes to runtime storage/database/cookies/Go module/GHCR/Docker contracts; repo renames; deployment |
| **OMP B — Website** | `septian-padli/cubycloud-page` from `staging`; Astro/React brand copy, Starlight prose, SEO targets, favicon/OG after asset approval | GSAP/3D redesign, feature inventions, Worker/resource rename, production merge, DNS change, custom-domain cutover |

**Source-of-truth and conflict precedence:**

1. Explicit owner-approved decisions recorded in this `BRAND.md` govern **brand names, approved positioning, legacy tagline status, and rename boundaries**.
2. Product behavior/security/permissions belong to product PRD, architecture, `SITEMAP.md`, and current implementation/tests; this document cannot override them.
3. Application visuals belong to its `apps/frontend/DESIGN.md`; website visuals to its own `DESIGN.md`, `MOTION.md`, and approved `DECISIONS.md`; product storytelling/copy to the website `BRIEF.md` and `CONTENT.md` except for the **new name and explicitly approved brand decisions**.
4. If a document still says CubyCloud, treat its historical name as superseded **only in public-brand references**; do not reinterpret technical paths or silently rewrite history. Log substantive contradictions instead of inventing decisions.

The audit files `cubycrate-rebranding-audit.md` (app) and `REBRANDING_AUDIT_PAGES.md` (website) are **impact inventories and recommendations**, not blanket authorization for every proposed rename/migration or for logo generation.

## 10. Acceptance criteria and review gates

**Identity consistency**

- [ ] All current public-facing references use **CubyCrate** as one word, with correct capitalization and no unapproved `®`.
- [ ] Copy retains the established tone, tagline remains optional, existing messaging hierarchy is intact, and current feature claims are verified.
- [ ] Both repos use the **unchanged** approved color and typography systems; the distinct app/website visual behaviors remain intact.
- [ ] Wordmarks, nav, docs, mobile layouts, keyboard focus, and accessible names remain usable; no new CSS/layout regressions.
- [ ] Use official source logo family from `public/logos/`, verify actual exported files and correct light/dark usage; no obsolete brand/framework icon slips into production.
- [ ] Test small-size icon visibility, font/path portability, logo asset HTTP responses, and no tagline text overflow.

**Application safety**

- [ ] Legacy database, storage blobs, Docker named volume, env vars, Go module, GHCR paths, cookies, and persisted browser state remain functional and are not renamed in this wave.
- [ ] Existing-installation upgrade/smoke test proves user data and file downloads remain intact; S3 credentials remain decryptable; no repeat setup or unexpected logout.
- [ ] Go tests, frontend typecheck/lint/unit tests, Docker build, and applicable Playwright E2E tests pass, with evidence or clearly reported blockers.

**Website & release**

- [ ] `pnpm check`, `pnpm build`, Pagefind search, Starlight routes, direct refresh, `wrangler deploy --dry-run`, and CI pass.
- [ ] GSAP/Three.js, demos, desktop/mobile responsive behavior, reduced motion, light/dark docs, and accessible navigation remain unchanged and tested.
- [ ] Canonical/OG metadata use **CubyCrate**; sitemap, robots, and staging indexing are verified in actual output/environment.
- [ ] Existing resource names and links still work; no accidental production deploy, branch cutover, Worker creation, or domain attachment.
- [ ] Owner reviews cross-repo output, staging evidence, remaining asset decisions, and the release plan **before** approving any production action.

## 11. Deferred decisions and explicit change control

**Not decided / requires a separate approval:**

1. **Logo artwork is selected and supplied**; what remains open is verification of the actual export files, approved small-size derivative if needed, browser-specific favicon/touch output, Open Graph composition, and quantitative safe-space/minimum-size rules.
2. Any replacement tagline, revised product positioning, new brand palette, new display font, or altered content tone.
3. GitHub repository renames, GHCR image namespace migration, Docker/container/volume naming migration, Go module rename, database path migration, file blob prefix migration, environment key renames, cookie changes, and browser-storage migrations.
4. Actual production merge/cutover timing, verified release assets, WWW redirect, and any new infrastructure resources.
5. Trademark filings/clearance; no registration or exclusive rights are asserted by this document.

**Change process:** Record the proposal, affected surfaces, backward-compatibility impact, acceptance tests, and owner approval before updating this contract. A successful build or an agent recommendation does not count as approval.

---

**Operating rule:** *Apply the owner-approved logo and rebrand the experience, not the stored data or the underlying compatibility contract.* Both OMP workstreams must use the same canonical assets, preserve their own DESIGN/behavior contracts, and follow `REBRANDING_PLAN.md` for execution order and release gates.
