# CubyCloud — Global AI Agent Guidance

## 1. Scope & Responsibility

This file contains **only rules that apply to the entire CubyCloud monorepo**.

It defines:

- repository-wide engineering constraints
- source-of-truth hierarchy
- mandatory agent tooling
- global product boundaries
- cross-project rules

It does NOT define implementation details that belong exclusively to:

- `apps/backend/AGENTS.md`
- `apps/frontend/AGENTS.md`

When a local `AGENTS.md` exists, follow it together with this file. The local file may add constraints for its own scope, but must not contradict global rules or product contracts.

Do not duplicate product requirements, UI specifications, route definitions, or backend/frontend implementation details here.

---

## 2. Repository Structure

CubyCloud is a monorepo:

```text
cubycloud/
├── AGENTS.md
├── docs/
│   ├── prd.md
│   └── architecture.md
├── apps/
│   ├── frontend/
│   │   ├── AGENTS.md
│   │   ├── DESIGN.md
│   │   └── SITEMAP.md
│   └── backend/
│       └── AGENTS.md
├── scripts/
├── Makefile
└── Dockerfile
```

The main application consists of:

- `apps/frontend/` — SvelteKit SPA
- `apps/backend/` — Go backend
- `docs/` — product and architecture documentation

---

## 3. Source of Truth

Each document has a specific ownership boundary.

| Document                   | Owns                                                                                      |
| -------------------------- | ----------------------------------------------------------------------------------------- |
| `docs/prd.md`              | Product requirements, feature scope, business rules, acceptance criteria                  |
| `docs/architecture.md`     | System architecture, backend architecture, infrastructure and data-flow decisions         |
| `apps/frontend/DESIGN.md`  | UI/UX design system, visual tokens, component appearance, interaction styling             |
| `apps/frontend/SITEMAP.md` | Frontend route tree, page surfaces, route-level UI contracts                              |
| `apps/frontend/AGENTS.md`  | Frontend implementation conventions and AI-agent workflow                                 |
| `apps/backend/AGENTS.md`   | Backend implementation conventions and AI-agent workflow                                  |
| Root `AGENTS.md`           | Repository-wide agent rules and cross-project constraints                                 |

### Precedence

When documents conflict:

1. Explicit product requirements in `docs/prd.md`
2. Explicit architectural decisions in `docs/architecture.md`
3. Area-specific design/route contracts (`DESIGN.md`, `SITEMAP.md`)
4. Area-specific `AGENTS.md`
5. Agent inference or recommendation

Do not silently change a product or architectural decision because a different implementation appears more conventional.

If a conflict materially affects implementation, stop and surface it before changing the contract.

### Source-of-truth rule

Do not copy a specification from another document into an `AGENTS.md` unless it is genuinely an **implementation rule required by the agent**.

When a rule belongs to another document:

1. Reference that document.
2. Do not reproduce its full specification.
3. Do not create a second version of the same rule.

---

## 4. Scope Rules

Before modifying files, determine which project owns the change.

### Frontend

For changes under `apps/frontend/`, read and follow:

- `apps/frontend/AGENTS.md`
- `apps/frontend/DESIGN.md`
- `apps/frontend/SITEMAP.md`
- `docs/prd.md`

Read `docs/architecture.md` when the frontend change depends on backend/system behavior.

### Backend

For changes under `apps/backend/`, read and follow:

- `apps/backend/AGENTS.md`
- `docs/prd.md`
- `docs/architecture.md`

Read frontend documentation only when the backend change affects a frontend contract.

### Cross-project changes

For changes spanning frontend and backend:

- Read both project-level `AGENTS.md` files.
- Treat `docs/prd.md` as the product contract.
- Treat `docs/architecture.md` as the system architecture contract.
- Keep frontend/backend contracts synchronized.
- Do not solve cross-project inconsistencies by duplicating rules into `AGENTS.md`.

---

## 5. AI Agent Toolchain

The following tools are mandatory for agents working on this repository.
| Tool         | Purpose                                                         | Configuration                                                   |
| ------------ | --------------------------------------------------------------- | --------------------------------------------------------------- |
| **Codegraph** | Codebase knowledge graph, symbol navigation, and architectural exploration | `.cursor/rules/codegraph.mdc`, `.cursor/mcp.json`, `codegraph init` |
| **RTK**      | Reduce shell command output and token usage                     | `.cursor/rules/rtk.mdc`, `RTK.md`                               |
| **Ponytail** | Keep changes small and avoid unnecessary abstraction            | `.cursor/rules/ponytail.mdc`                                    |

## 5.1. Bahasa dan Komunikasi

Semua komunikasi langsung dengan pengguna **harus dalam Bahasa Indonesia**.

- Jawaban, penjelasan, ringkasan, dan dialog interaktif → Bahasa Indonesia.
- istilah teknis, kode, path file, command, API names → tetap dalam bentuk aslinya (English).
: ponynail: section 5.1 sudah benar, hanya hapus duplikasi 5.1 table

These tools are workflow requirements, not product architecture.

### Codegraph

Use Codegraph before exploring unfamiliar code or architecture.

For structural questions, prefer:

```bash
codegraph orient --root . --budget small
codegraph explore "<question>"
codegraph search "<symbol or query>" --json
codegraph path "<file A>" "<file B>"
codegraph explain "<file or symbol>"
codegraph deps <file>          # rdeps for reverse dependencies
codegraph impact --base <ref> --head <ref>
```

Use the result to identify relevant files before performing broad exploration.

After a pull, merge, or large branch switch:

```bash
codegraph sync .
```

After modifying source code no manual rebuild is required — the `.codegraph/cache/index-v1/` cache invalidates on demand. Use `codegraph doctor` when results look stale.

Do not treat generated Codegraph output as source code or documentation. Do not use Codegraph as a substitute for reading the actual source files when implementation details matter.

> Graphify is **deprecated for this repository**. Do not invoke graphify commands, rebuild graphify graphs, or follow graphify skill instructions here; Codegraph is the only mandatory graph tool.

### RTK

Use the repository-configured RTK integration for shell commands where applicable.

Avoid commands that produce unnecessarily large output.

### Ponytail

Prefer:

- small diffs
- existing abstractions
- explicit code
- the simplest implementation that satisfies the contract

Do not use Ponytail as justification for violating the established architecture or stack.

---

## 6. Repository-Wide Product Constraints

CubyCloud v1.0 is a self-hosted modular cloud storage application designed around:

- single-container deployment
- Go backend
- embedded SvelteKit SPA
- SQLite
- local and S3-compatible storage
- zero-CLI web onboarding
- low resource consumption

### Deployment

- Production distribution must remain a single Docker image / executable deployment.
- Frontend is built separately and embedded into the Go binary at build time.
- Do not introduce a required external runtime service without explicitly changing the product/architecture contract.

### Resource Target

The primary target hardware is:

- Intel Celeron 2-Core
- 2 GB RAM

Target resource budgets:

- Idle RAM: `< 200 MB`
- Active transfer peak: `< 600 MB`
- Docker image: `< 150 MB` target

These are product/engineering targets, not permission to sacrifice correctness.

### Runtime Dependencies

v1.0 must not introduce mandatory external infrastructure such as:

- Redis
- RabbitMQ
- Kafka
- MinIO
- external message brokers
- AI/RAG/Ollama infrastructure

unless the product and architecture contracts are explicitly changed.

---

## 7. Cross-Project Contracts

### Database

CubyCloud uses SQLite as the primary database.

Backend-specific database implementation belongs to:

- `docs/architecture.md`
- `apps/backend/AGENTS.md`

Do not define database implementation conventions in this file.

### Storage

CubyCloud abstracts physical storage behind the storage architecture defined by `docs/architecture.md`.

Do not introduce a second storage abstraction or bypass the established storage layer.

### Frontend / Backend Boundary

Frontend and backend communicate through explicit application contracts.

- Backend owns API behavior and business enforcement.
- Frontend owns presentation and interaction.
- Shared behavior must be derived from the product/API contracts rather than duplicated as independent rules.

If an API contract is missing or ambiguous, identify it explicitly instead of inventing incompatible behavior.

---

## 8. Security Baseline

Security requirements defined by the product and architecture are mandatory.

When implementing or reviewing a feature, consider at minimum:

- authentication
- authorization
- ownership
- role boundaries
- session security
- CSRF
- IDOR
- path traversal
- share-token security
- brute-force protection
- rate limiting
- upload/resource abuse
- permission escalation
- sensitive data exposure

Do not weaken an existing security contract for implementation convenience.

---

## 9. Engineering Rules

When implementing changes:

- Follow the established project stack.
- Do not introduce dependencies without a concrete reason.
- Do not introduce infrastructure without a concrete reason.
- Avoid premature abstraction.
- Prefer simple, explicit, testable code.
- Preserve existing contracts unless the user explicitly asks to change them.
- Consider error handling, concurrency, transaction boundaries and idempotency.
- Keep changes scoped to the requested task.
- Do not refactor unrelated code merely because it could be improved.

Before changing architecture, explain the trade-off and obtain agreement when the change affects an established contract.

---

## 10. Documentation & Contract Changes

When implementation reveals that a requirement or architecture decision is incomplete:

1. Identify the ambiguity.
2. Explain the implementation impact.
3. Propose the smallest reasonable decision.
4. Update the appropriate source-of-truth document only after the decision is accepted.

Do not use `AGENTS.md` as a replacement for PRD or architecture documentation.

When implementation changes invalidate documentation:

1. Update the document that owns the affected information.
2. Do not place the information into `AGENTS.md` merely to make it visible to agents.
3. Keep documentation references accurate.

Examples:

- Product behavior → `docs/prd.md`
- Backend architecture → `docs/architecture.md`
- UI appearance → `apps/frontend/DESIGN.md`
- Routes/page contracts → `apps/frontend/SITEMAP.md`
- Frontend coding convention → `apps/frontend/AGENTS.md`
- Backend coding convention → `apps/backend/AGENTS.md`

---

## 11. Working With Multiple AGENTS.md Files

Agent instructions are hierarchical:

```text
/
└── AGENTS.md
    ├── apps/backend/AGENTS.md
    └── apps/frontend/AGENTS.md
```

Interpret them as:

- Root: **what applies everywhere**
- Backend: **how to work inside backend**
- Frontend: **how to work inside frontend**

A child `AGENTS.md` should contain only rules that are genuinely specific to that area.

If a rule is useful for both backend and frontend, it belongs here.

If a rule only concerns one area, it belongs in that area's `AGENTS.md`.

---

## 12. General Change Workflow

For non-trivial work:

1. Identify the affected project and source-of-truth documents.
2. Query Codegraph before broad code exploration.
3. Read the relevant project-level `AGENTS.md`.
4. Read the source-of-truth documentation relevant to the change.
5. Make the smallest correct implementation.
6. Run appropriate validation/tests.
7. Run `codegraph sync .` after code changes if the graph metadata looks stale.
8. Update the owning documentation when the contract itself changed.

Never use `AGENTS.md` as a substitute for understanding the product or architecture.
