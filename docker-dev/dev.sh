#!/usr/bin/env bash
# Runs the backend (air). Vite runs on the host: make dev-frontend
set -euo pipefail
cd /app

# go:embed needs at least one file in web/dist.
mkdir -p web/dist
[ -n "$(ls -A web/dist)" ] || touch web/dist/.gitkeep

exec air -c .air.toml
