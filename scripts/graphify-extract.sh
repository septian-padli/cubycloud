#!/usr/bin/env bash
# Full Graphify rebuild (AST + semantic docs) using .env.graphify.local
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ENV_FILE="${GRAPHIFY_ENV_FILE:-$ROOT/.env.graphify.local}"
if [[ ! -f "$ENV_FILE" ]]; then
	echo "Missing $ENV_FILE — copy .env.graphify.example and set OPENAI_* vars." >&2
	exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

export OPENAI_API_KEY OPENAI_BASE_URL OPENAI_MODEL
BACKEND="${GRAPHIFY_BACKEND:-openai}"

echo "Graphify extract: backend=$BACKEND model=${OPENAI_MODEL:-default} base=${OPENAI_BASE_URL:-default}"
graphify extract . --backend "$BACKEND" --timing "$@"
