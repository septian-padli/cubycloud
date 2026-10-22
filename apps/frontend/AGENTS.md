# CubyCloud — Frontend Agent Guidance

> Scope: `apps/frontend/`. Follow the repository-root `AGENTS.md` as well. This document owns **frontend implementation conventions**, not product requirements, route contracts, or design tokens.

## 1. Source of truth and selective reading

| Concern | Authoritative reference |
| --- | --- |
| Business behavior, permissions, quotas, lifecycle | `docs/prd.md` |
| Routes, page behavior, URL parameters | `apps/frontend/SITEMAP.md` |
| UI tokens, components, styling, interactions | `apps/frontend/DESIGN.md` |
| System architecture, backend integration | `docs/architecture.md` |
| Frontend coding conventions | This file |

**Progressive context loading:** Before implementation, identify affected features/routes and inspect the relevant headings or index of each applicable source. Read **only the sections needed** for the task and dependencies; expand when modifying a contract, investigating ambiguity, or changing shared behavior. Do not dump entire PRD, SITEMAP, or DESIGN into context by default. A direct change to a document requires reading the affected sections and cross-references before editing. These instructions govern reading efficiency, **not** permission to ignore applicable contracts. Root-level instructions take precedence if stricter.

Prefer Codegraph for initial structural exploration according to root guidance; then read relevant source and tests. When delegating, give subagents scoped tasks, owned files, API contracts, and precise references rather than requiring every subagent to explore the whole monorepo.

## 2. Stack and build boundary

- SvelteKit SPA (`adapter-static`), Svelte 5, TypeScript, Tailwind CSS v4.
- Svelte 5 Runes (`$state`, `$derived`, `$effect`, `$props`) for new component/local state.
- shadcn-svelte / bits-ui UI primitives, `@lucide/svelte` icons, Bun package manager.
- Static frontend assets are embedded in the Go backend for production; do not require another frontend runtime service.
- Follow existing SvelteKit route-group, loader, and file naming conventions; use descriptive `kebab-case` for Svelte component files.

## 3. Components, routes, and state

- Reuse existing components under `src/lib/components/`, especially `src/lib/components/ui/`, before building a new primitive. Install a missing shadcn component using the project's established `bunx shadcn-svelte@latest add <component>` flow when appropriate.
- Put reusable UI and feature components in `src/lib/components/`; route files compose pages, load/orchestrate data, and declare route metadata.
- Use the **smallest viable state scope**: component → feature → route/layout → shared app state. Reuse existing context/state utilities; do not add global state for local concerns or introduce legacy stores where Runes fit.
- Check relevant `SITEMAP.md` sections before changing route structure, access, parameters, navigation, or page behavior. Update that document when a route contract changes; never silently invent a route.
- Avoid breaking existing component APIs, keyboard behavior, or tests during refactoring.

## 4. API and data boundary

- Keep backend access behind typed clients in `src/lib/api/`; no scattered raw `fetch()` calls in presentational components.
- Prefer: **component → feature/page logic → API client → backend**.
- Follow existing backend endpoints and any applicable API contract; coordinate frontend/backend changes where behavior differs.
- Use typed data/models in `src/lib/types/`; temporary fixtures in `src/lib/mocks/`, not embedded inside `.svelte` components.
- Mocks are for explicit demos or unimplemented surfaces. Do not return mock success for production flows or substitute mock data when a real API is available.
- On mock-to-live migrations, adapt data in the API/data layer and preserve stable UI contracts where practical.
- Frontend guards and hidden controls are UX only; **authentication, authorization, ownership, and permission checks belong to the backend**.

## 5. Visual implementation

- Follow the **relevant** `DESIGN.md` sections and existing `src/routes/layout.css` tokens. Use standard Tailwind v4 utilities and existing project tokens.
- No arbitrary Tailwind values for one-off pixel matching. If a genuine token is missing, update the design source and token definitions before use.
- Reuse existing components and visual patterns for flat surfaces, typography, spacing, focus/hover, dialogs, file lists/grids, and transfer feedback.
- Preserve responsive layouts and accessibility: semantic elements, keyboard operation, visible focus, icon-button labels, appropriate ARIA, and sufficient contrast.
- Do not replace shadcn-svelte/bits-ui accessibility semantics with ad-hoc controls.

## 6. Async behavior and errors

- Provide relevant loading, success, empty, recoverable error, and unavailable states for asynchronous UI.
- Reuse existing toasts, errors, and UI status components. Do not present an operation as successful until the backend confirms it.
- Preserve documented storage-offline behavior, transfer state, quota errors, permission feedback, and account-scoped language behavior **where the affected feature uses them**. Refer to SITEMAP/DESIGN; do not copy their specifications here.
- Handle network errors, stale responses, concurrent actions, and navigation/reload behavior when relevant.

## 7. Implementation workflow

1. Determine task scope, affected routes/modules, and relevant authoritative sections.
2. Use Codegraph for targeted structural discovery; inspect existing components, API clients, state, and tests.
3. Confirm API and permission boundaries, failure cases, and existing conventions.
4. Implement the smallest correct change without unrelated refactors or new dependencies unless justified.
5. Add/adjust focused tests. Run checks relevant to touched code (`bun run check`, `bun run lint`, `bun run build`, and feature-specific tests/Playwright as warranted).
6. Update **only the owning documents** when their contracts actually change: product → PRD; route/page → SITEMAP; visuals → DESIGN; implementation convention → this file; system/API → architecture/API contract.
7. Report changes, actual test results, and unresolved risks. Follow root Git-operation restrictions; **do not commit or push without explicit authorization**.

**Goal:** Reduce repeated document reading without relaxing product behavior, design consistency, accessibility, authorization, or testing.
