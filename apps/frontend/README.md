# CubyCloud Frontend

SvelteKit SPA for CubyCloud (`adapter-static` + SPA fallback for Go `go:embed`).

## Stack

- SvelteKit 2 + Svelte 5 (runes)
- TypeScript
- Tailwind CSS v4
- shadcn-svelte (vega style, neutral base)
- ESLint + Prettier

## Commands

```bash
bun install
bun run dev        # http://localhost:5173
bun run build      # output: build/
bun run preview
bun run check
bun run lint
```

## shadcn-svelte

Add components:

```bash
bunx shadcn-svelte@latest add button
```

Config: [`components.json`](./components.json)

## Design tokens

Marketing pages use tokens from [`DESIGN.md`](./DESIGN.md) via CSS variables (`--cc-*` in `src/routes/layout.css`).

## Scaffold command (reference)

```bash
bun x sv@0.17.0 create --template minimal --types ts \
  --add eslint prettier tailwindcss="plugins:none" sveltekit-adapter="adapter:static" \
  --no-download-check --install bun frontend
```
