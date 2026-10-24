# Development image: Node (Vite + pnpm) and Go (air hot reload) in one container.
# docker-compose.yml bind-mounts the repo at /app and runs the container as your
# host user, so every file written into the repo stays owned by you. Only
# go.mod/go.sum are copied here to warm the module cache.
FROM golang:1.27-alpine3.23 AS go

FROM node:24.15-alpine

COPY --from=go /usr/local/go /usr/local/go

# HOME, the Go caches, Corepack, node_modules, and /config must be writable for
# any UID, because the container runs as the UID of the host user.
ENV HOME=/home/dev \
    GOPATH=/go \
    GOCACHE=/go/cache \
    PATH=/go/bin:/usr/local/go/bin:$PATH \
    CGO_ENABLED=0 \
    GOFLAGS=-buildvcs=false \
    COREPACK_HOME=/opt/corepack \
    COREPACK_ENABLE_DOWNLOAD_PROMPT=0 \
    XDG_CONFIG_HOME=/config

RUN apk add --no-cache bash ca-certificates curl git make tzdata && \
    npm install -g corepack@latest && \
    corepack enable && \
    go install github.com/air-verse/air@latest && \
    git config --system --add safe.directory /app

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && \
    mkdir -p "$HOME" "$GOCACHE" "$COREPACK_HOME" /config /app/web/node_modules && \
    chmod -R a+rwX /go "$HOME" "$COREPACK_HOME" /config /app/web/node_modules

# 7476: Go backend, 5173: Vite dev server (open this one; it proxies /api to 7476)
EXPOSE 7476 5173

CMD ["bash", "/app/docker-dev/dev.sh"]
