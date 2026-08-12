package web

import "embed"

// Dist is the SvelteKit SPA staging dir (synced from apps/frontend/build/ at release build).
// Scaffolding ships a placeholder so go:embed compiles.
//
//go:embed all:dist
var Dist embed.FS
