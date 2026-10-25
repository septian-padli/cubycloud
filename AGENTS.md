# CubyCloud — Repository-Wide Agent Instructions

> Scope: repository root and all descendants. This file contains **mandatory cross-project engineering constraints**. Area-specific instructions may add requirements but must not relax security, product, or architecture contracts.

## 1. Source of truth and precedence

| Concern | Canonical source |
| --- | --- |
| Product scope, permissions, behavior, acceptance criteria | `docs/prd.md` |
| Architecture, system boundaries, infrastructure | `docs/architecture.md` |
| Frontend routes, page actions, URL state | `apps/frontend/SITEMAP.md` |
| Visual design and components | `apps/frontend/DESIGN.md` |
| Frontend implementation conventions | `apps/frontend/AGENTS.md` |
| Go/SQLite/storage implementation conventions | `apps/backend/AGENTS.md` |
| Global agent workflow and constraints | This file |
| Agent mindset, non-binding decision principles | `SOUL.md` (read selectively, if relevant) |
| Git permission and documentation placement details | `.cursor/WORKFLOW_RULES.md` (read selectively) |

A newer, explicit user decision for the active task takes precedence over older documentation. Do not silently overwrite an established contract or treat outdated dates and targets as evidence of delivered functionality. When sources genuinely conflict, flag the specific contradiction and its consequences; request a decision **only if** the task cannot safely proceed. Record accepted decisions in the owning document. Implementation alone is not a substitute for an approved product decision.

## 2. Progressive context loading (required)

**Read the instructions that govern the work, not the full documentation corpus by default.**

1. Understand the request and identify affected modules, data flows, and risk areas.
2. Read the applicable scoped `AGENTS.md` when working in its directory. Root guidance always applies.
3. Locate relevant headings in PRD, architecture, SITEMAP and DESIGN; read **only the pertinent sections and necessary cross-references**.
4. Expand to other sections, source code, and tests when dependencies, shared contracts, security, migration, or ambiguity require it.
5. For edits to a documentation file, inspect the affected section, dependent contracts, and references first. For major rewrites, review the entire source document to prevent requirement loss.
6. Do not repeatedly dump the same documents into the context. Reuse accurate section references and concise findings, but verify important details against the original source.

Never use selective reading to skip a relevant permission, quota, data-integrity, or security requirement.

### Typical task scopes

| Task | Initial reading |
| --- | --- |
| Frontend-only | Frontend `AGENTS.md`, affected SITEMAP/DESIGN/PRD sections |
| Backend-only | Backend `AGENTS.md`, affected PRD/architecture sections |
| Cross-project API or security | Both scoped `AGENTS.md`; related PRD, architecture, route, and source contracts |
| Documentation-only | Owning document's affected sections plus referenced contracts |
| Deployment/storage | Architecture, affected PRD sections, build/storage sources and tests |

`README.md`, `SOUL.md`, `RTK.md`, `.cursor/WORKFLOW_RULES.md`, `CONTRIBUTING.md`, and other reference files are **not universal full-read prerequisites**. Read them when the task or a specific applicable rule calls for them. Any instructions actually auto-loaded by the active agent runtime still apply.

## 3. Product and platform invariants

- CubyCloud is a lightweight, self-hosted cloud-storage application: **Go backend + embedded SvelteKit SPA + SQLite WAL**, local and S3-compatible storage, zero-CLI web onboarding.
- Production distribution remains **one executable / one container** with embedded static frontend; do not require an additional frontend application server.
- No mandatory Redis, RabbitMQ, Kafka, MinIO, PostgreSQL, external AI services, or other new runtime infrastructure without an explicitly accepted architecture change.
- Respect the resource target: **Intel Celeron 2-core, 2 GB RAM; idle < 200 MB; active transfer peak < 600 MB; image < 150 MB target**. These are targets requiring measurement, not guarantees or reasons to weaken correctness.
- Preserve storage abstraction (`StorageDriver` and virtual filesystem), quota and capacity separation, transactional boundaries, and the existing local/S3 transfer architecture.
- Backend owns authentication, authorization, business rules, data integrity, and permissions; frontend guards are for user experience only.
- Do not introduce features excluded by the PRD or rewrite product requirements while fixing unrelated code.

## 4. Tooling and code exploration

- **Codegraph:** Use for initial structural discovery when configured and available. Prefer targeted `codegraph orient --root . --budget small`, `explore`, `search`, `deps/rdeps`, or `impact`; then inspect actual source/tests. Run `codegraph sync .` after branch changes if needed, and `doctor` when stale. Do not repeatedly index or orient for every known file. If unavailable, continue with targeted code navigation and report the limitation; never pretend it was used.
- **RTK:** Use repository-configured token-efficient shell wrappers where available (e.g. `rtk git diff`, `rtk go test ./...`). Cursor shell hooks are **not assumed to run in OMP**. Run a tool directly if RTK is absent or suppresses diagnostic information; keep output focused.
- **Ponytail:** Prefer smallest **correct** change, existing abstractions, standard libraries, and installed dependencies. Avoid speculative abstraction or broad refactors. Preserve testing, security, and operational correctness.
- See `.cursor/rules/` and `RTK.md` only for the active integration's detailed setup or troubleshooting, not as mandatory reading for every task.

## 5. Security and data-integrity baseline

Treat authentication, session invalidation, CSRF, IDOR, ownership, role boundaries, inherited share permissions, path traversal, share tokens, secret storage, rate limiting, and resource abuse as applicable threat surfaces. Never expose credentials, recovery keys, private physical-node information, or inaccessible files in responses or logs.

For data mutations consider concurrency, idempotency, rollback, partial failures, restart, retry, orphaned physical objects, quota reservations, and metadata/physical-storage consistency. Do not claim success before server confirmation or claim a test passed without executing it.

## 6. Delegation with native OMP subagents

For independently scoped work, use **native OMP subagents** when they materially reduce total effort; do not use Orca or require subagents for trivial tasks. The lead owns contracts, integration, and the final result.

- Give each worker a bounded objective, relevant source/section pointers, specific file ownership, dependencies, expected tests, and an explicit no-commit/no-push constraint.
- Parallelize independent modules; designate one owner or the lead for files that multiple workers would otherwise edit. Avoid concurrent edits to shared interfaces and migrations without coordination.
- Workers must verify important source contracts independently; avoid redundant whole-repository exploration.
- Integrate and validate worker results before reporting completion. Worker self-reports do not prove end-to-end success.

## 7. Engineering workflow and documentation ownership

1. Define acceptance criteria and affected contracts. Inspect relevant code, APIs, permissions, and tests.
2. Implement the smallest correct change; avoid changing unrelated behaviors or adding dependencies without justification.
3. Test appropriate levels (Go tests, frontend checks/build, targeted integration/E2E); re-run failed checks after fixes. Report exact PASS/FAIL/SKIPPED/NOT RUN results and limitations.
4. Update **only** documents that own genuinely changed contracts: product → PRD; system → architecture; routes → SITEMAP; visuals → DESIGN; implementation conventions → scoped AGENTS.
5. Communicate with the user in **Bahasa Indonesia**. Keep code identifiers, file paths, API names and commands in their original form.

**Git safety rule:** Do not commit, push, force-push, reset destructively, delete branches/worktrees, or publish without specific authorization. Implement, verify, and report first. The detailed workflow is in `.cursor/WORKFLOW_RULES.md` and need not be read in full for every task.

<!-- CODEGRAPH_START -->
## CodeGraph

In repositories indexed by CodeGraph (a `.codegraph/` directory exists at the repo root), reach for it BEFORE grep/find or reading files when you need to understand or locate code:

- **MCP tool** (when available): `codegraph_explore` answers most code questions in one call — the relevant symbols' verbatim source plus the call paths between them, including dynamic-dispatch hops grep can't follow. Name a file or symbol in the query to read its current line-numbered source. If it's listed but deferred, load it by name via tool search.
- **Shell** (always works): `codegraph explore "<symbol names or question>"` prints the same output.

If there is no `.codegraph/` directory, skip CodeGraph entirely — indexing is the user's decision.
<!-- CODEGRAPH_END -->
