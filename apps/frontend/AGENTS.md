# CubyCloud — Frontend AI Agent Guidance

This file contains **frontend implementation conventions only**.

Product requirements, UI design specifications, and route contracts are owned by other documents:

- `docs/prd.md` — product requirements and business behavior
- `apps/frontend/DESIGN.md` — UI/UX and visual design system
- `apps/frontend/SITEMAP.md` — route tree and page contracts
- `docs/architecture.md` — system/backend architecture when relevant

Do not duplicate those specifications here.

---

## 1. Frontend Stack

| Area            | Standard                |
| --------------- | ----------------------- |
| Framework       | SvelteKit               |
| Rendering       | SPA / static adapter    |
| Language        | TypeScript              |
| UI framework    | Svelte 5                |
| State model     | Svelte 5 Runes          |
| Styling         | Tailwind CSS v4         |
| Components      | shadcn-svelte / bits-ui |
| Icons           | `@lucide/svelte`        |
| Package manager | Bun                     |

The frontend is ultimately built as static assets and embedded into the Go backend.

---

## 2. Svelte Standards

Use Svelte 5 Runes for new local/component state:

```svelte
let count = $state(0); let doubled = $derived(count * 2);
```

Prefer:

- `$state`
- `$derived`
- `$effect`
- `$props`

Do not introduce legacy Svelte stores for new local state when Runes are appropriate.

Use existing project state/context utilities before creating another state mechanism.

---

## 3. Component Architecture

Prefer reusable components over route-local duplication.

Use:

```text
src/lib/components/
```

for reusable UI and feature components.

Use route files for:

- route composition;
- page-specific orchestration;
- loading/data composition;
- route-specific metadata.

Do not place reusable business/UI components directly inside individual route directories when they are clearly shared by multiple pages.

Before creating a new component:

1. Search for an existing component with the same responsibility.
2. Reuse it if possible.
3. Extend it when the existing abstraction is appropriate.
4. Create a new component only when the responsibility is genuinely distinct.

---

## 4. shadcn-svelte

Use the existing shadcn-svelte components under:

```text
src/lib/components/ui/
```

When a required component does not exist, use the project's established shadcn-svelte workflow:

```bash
bunx shadcn-svelte@latest add <component>
```

Do not replace existing shadcn-svelte primitives with ad-hoc implementations without a concrete reason.

When customizing a primitive, preserve its existing API and composition model unless the requirement explicitly requires a breaking change.

---

## 5. Styling

`apps/frontend/DESIGN.md` is the authoritative source for visual styling.

Agents must:

- use the project's design tokens;
- follow the documented spacing, typography, colors, radius, interaction, and surface rules;
- avoid introducing visual patterns that are not defined by the design system.

Do not copy design-token definitions or component styling specifications into this file.

### Tailwind

Use standard Tailwind CSS v4 utilities and project-defined tokens.

Do not introduce arbitrary values merely to match a particular pixel measurement.

If a required design token does not exist:

1. Check `DESIGN.md`.
2. Check the existing token definitions.
3. If the design genuinely requires a new token, update the design system first.
4. Then use the new token in components.

Do not silently create one-off visual tokens inside components.

---

## 6. Routes

`apps/frontend/SITEMAP.md` is the authoritative source for frontend routes and page-level contracts.

Before creating or modifying a route:

1. Check `SITEMAP.md`.
2. Confirm that the route belongs to the documented application structure.
3. Follow the documented page contract.

Do not invent new application routes without updating `SITEMAP.md`.

Do not copy the route tree from `SITEMAP.md` into this file.

---

## 7. Feature Implementation

`docs/prd.md` is authoritative for product behavior.

When implementing a feature:

1. Identify the relevant PRD requirement.
2. Identify the corresponding route/page contract in `SITEMAP.md`.
3. Follow the visual implementation rules in `DESIGN.md`.
4. Implement the frontend behavior using the project's component/state conventions.

Do not redefine business rules inside frontend components.

For example, quota behavior, permissions, sharing semantics, storage behavior, and lifecycle rules must come from the product contract rather than being independently invented in UI code.

---

## 8. API Boundary

Backend integration must be isolated behind:

```text
src/lib/api/
```

Do not scatter raw `fetch()` calls across page and presentational components.

Prefer:

```text
component
    ↓
feature/page logic
    ↓
API client
    ↓
backend
```

When the real API contract exists:

```text
docs/API_CONTRACT.md
```

the frontend API client must follow that contract.

If the backend API is not yet available, use mocks/fixtures behind the same conceptual boundary.

Do not design mock responses independently from the documented product/API contract.

---

## 9. Mock Data

Temporary frontend data belongs under:

```text
src/lib/mocks/
```

Keep mock data separate from UI components.

Prefer typed fixtures:

```text
src/lib/types/
src/lib/mocks/
```

Do not embed large datasets directly inside `.svelte` files.

When replacing mocks with real API calls:

- preserve the component-facing data shape where practical;
- move API-specific transformation into the API/data layer;
- avoid rewriting unrelated UI components.

---

## 10. State Management

Use the smallest appropriate state scope.

Preferred order:

1. local component state;
2. feature-level state;
3. route/layout state;
4. shared application state.

Do not introduce global state for data that belongs to a single component or route.

Before creating shared state, check whether an existing context/state utility already owns the responsibility.

---

## 11. Authentication & Authorization UI

Authentication and authorization behavior must follow the product contract.

Frontend authorization checks are for:

- controlling UI visibility;
- preventing inappropriate client-side actions;
- improving UX.

They must not be treated as the security boundary.

Actual authorization must be enforced by the backend.

Do not encode privileged operations as frontend-only security controls.

---

## 12. Loading, Error & Empty States

Every asynchronous UI should have an appropriate state for:

- loading;
- success;
- empty;
- recoverable error;
- unavailable/failed operation where applicable.

Reuse the project's existing error/loading components and patterns before creating new ones.

Global error-page behavior belongs to the documented frontend error-page implementation and design system.

---

## 13. Accessibility

New or modified UI must preserve basic accessibility:

- semantic HTML where appropriate;
- keyboard accessibility;
- visible focus state;
- accessible labels for icon-only controls;
- appropriate ARIA attributes when native semantics are insufficient;
- sufficient color contrast according to the design system.

Do not remove accessibility behavior from existing shadcn-svelte/bits-ui primitives without a concrete reason.

---

## 14. Responsive Behavior

Frontend layouts must work across the supported viewport sizes.

Prefer:

- responsive Tailwind utilities;
- existing layout primitives;
- existing mobile/desktop component variants.

Do not create separate mobile and desktop implementations when a single responsive component can handle both cleanly.

---

## 15. File & Module Naming

Follow existing repository naming conventions.

Use:

- `kebab-case` for Svelte component filenames;
- descriptive feature-oriented names;
- colocated route files according to SvelteKit conventions.

Avoid generic names such as:

```text
utils.ts
helpers.ts
common.ts
misc.ts
```

when a more specific ownership boundary is possible.

---

## 16. Frontend Change Workflow

Before implementing a non-trivial frontend change:

1. Read the relevant section of `docs/prd.md`.
2. Read the relevant section of `SITEMAP.md`.
3. Read the relevant section of `DESIGN.md`.
4. Query Codegraph before broad code exploration.
5. Inspect existing components/state/API layers.
6. Implement the smallest compatible change.
7. Run appropriate checks.

Typical validation:

```bash
bun run check
bun run lint
bun run build
```

Run the commands relevant to the actual change; do not run expensive commands unnecessarily.

---

## 17. Documentation Ownership

When a frontend change changes a contract:

| Change                             | Update                                |
| ---------------------------------- | ------------------------------------- |
| Product behavior/scope             | `docs/prd.md`                         |
| Route/page contract                | `SITEMAP.md`                          |
| Visual/design-system rule          | `DESIGN.md`                           |
| Frontend implementation convention | `AGENTS.md`                           |
| Backend/API architecture           | `docs/architecture.md` / API contract |

Do not put product requirements, route definitions, or design specifications into this file merely because the implementation happens in the frontend.
