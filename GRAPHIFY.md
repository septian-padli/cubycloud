# Graphify — CubyCloud knowledge graph

## Setup (sekali per mesin)

```bash
uv tool install "graphifyy[mcp,openai]"
cp .env.graphify.example .env.graphify.local   # isi OPENAI_* (gitignored)
graphify install --platform cursor --project
graphify hook install
```

## Full rebuild (AST + semantic docs + labeling)

```bash
./scripts/graphify-extract.sh
graphify cluster-only .    # optional jika report belum ada; script extract sudah menulis graph.json
graphify export html
```

Variabel di `.env.graphify.local` (OpenAI-compatible gateway):

| Variable | Contoh |
| -------- | ------ |
| `OPENAI_API_KEY` | `sk-...` |
| `OPENAI_BASE_URL` | `https://api-ai.septianpadli.my.id/v1` |
| `OPENAI_MODEL` | `custom-one` |

Override backend: `GRAPHIFY_BACKEND=openai ./scripts/graphify-extract.sh`

## Query (wajib sebelum explore)

```bash
graphify query "How does quota enforcement work?"
graphify path "VFS" "USER_QUOTA_EXCEEDED"
graphify explain "Organization Drive"
```

Visual: `open graphify-out/graph.html`

## Maintenance

| Perubahan | Perintah |
| --------- | -------- |
| File kode saja | `graphify update .` (post-commit hook, gratis) |
| Docs + kode | `./scripts/graphify-extract.sh` |

Output `graphify-out/` **gitignored**. Jangan commit `.env.graphify.local`.
