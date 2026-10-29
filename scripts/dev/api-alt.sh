#!/usr/bin/env bash
# Second API instance on :8082 for verification next to a running dev API (L-044).
# Loads the root .env (same DB, JWT secret, vault key) and only overrides the address.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
set -a; [ -f "$ROOT/.env" ] && . "$ROOT/.env"; set +a
export APP_ENV="${APP_ENV:-development}" HTTP_ADDR=":8082"
cd "$ROOT/backend"
exec go run ./cmd/api serve
