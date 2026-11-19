# Development image: Go with air hot reload. The web UI (Vite) runs on the host.
# docker-compose.yml bind-mounts the repo at /app and runs the container as your
# host user, so every file written into the repo stays owned by you. Only
# go.mod/go.sum are copied here to warm the module cache.
FROM golang:1.27-alpine3.23

# HOME, the Go caches and /config must be writable for
# any UID, because the container runs as the UID of the host user.
ENV HOME=/home/dev \
    GOPATH=/go \
    GOCACHE=/go/cache \
    CGO_ENABLED=0 \
    GOFLAGS=-buildvcs=false \
    XDG_CONFIG_HOME=/config

RUN apk add --no-cache bash ca-certificates curl git make tzdata && \
    go install github.com/air-verse/air@latest && \
    git config --system --add safe.directory /app

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && \
    mkdir -p "$HOME" "$GOCACHE" /config && \
    chmod -R a+rwX /go "$HOME" /config

# 7476: Go backend (Vite on the host proxies /api here)
EXPOSE 7476

CMD ["bash", "/app/docker-dev/dev.sh"]
