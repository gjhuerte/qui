#!/usr/bin/env bash
# Runs the backend (air) and frontend (Vite) together; stops both if either exits.
set -euo pipefail
cd /app

# go:embed needs at least one file in web/dist.
mkdir -p web/dist
[ -n "$(ls -A web/dist)" ] || touch web/dist/.gitkeep

(cd web && pnpm install --frozen-lockfile)

trap 'kill 0' EXIT

air -c .air.toml &
(cd web && pnpm dev --host 0.0.0.0 --port 5173) &

wait -n
