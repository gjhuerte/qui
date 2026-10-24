---
sidebar_position: 3
title: Run qui in Docker with docker compose
sidebar_label: Docker
description: Run qui with Docker Compose or docker run, with the volumes and environment variables it needs.
---

import LocalFilesystemDocker from "../_partials/_local-filesystem-docker.mdx";

# Docker

## Docker Compose

```yaml title="docker-compose.yml"
services:
  qui:
    image: ghcr.io/gjhuerte/qui:latest
    container_name: qui
    restart: unless-stopped
    ports:
      - "7476:7476"
    volumes:
      - ./qui:/config
    #environment:
      # Run as this user/group and chown /config (set both or neither).
      # Alternative to the standard "user:" option, see the Permissions docs.
      #PUID: "1000"
      #PGID: "1000"
      # Permission mask for new files and directories, for example 002 for group write
      #UMASK: "002"
```

```bash
docker compose up -d
```

## Docker Compose (Postgres)

```yaml title="docker-compose.postgres.yml"
services:
  postgres:
    image: postgres:16-alpine
    container_name: qui-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: qui
      POSTGRES_PASSWORD: qui
      POSTGRES_DB: qui
    volumes:
      - ./postgres:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER -d $$POSTGRES_DB"]
      interval: 10s
      timeout: 5s
      retries: 5

  qui:
    image: ghcr.io/gjhuerte/qui:latest
    container_name: qui
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "7476:7476"
    volumes:
      - ./qui:/config
    environment:
      QUI__DATABASE_ENGINE: postgres
      QUI__DATABASE_DSN: postgres://qui:qui@postgres:5432/qui?sslmode=disable
```

```bash
docker compose -f docker-compose.postgres.yml up -d
```

## Build from source

The repository has two Compose setups:

- `docker-dev/` runs the backend with air and the web UI with Vite hot reload. Start it with `make docker/dev` and open http://localhost:5173.
- `docker-prod/` builds the production image from your checkout. Start it with `make docker/prod`, or `make docker/prod-postgres` for Postgres.

## Standalone

```bash
docker run -d \
  -p 7476:7476 \
  -v $(pwd)/config:/config \
  ghcr.io/gjhuerte/qui:latest
```

## macOS container

On macOS, [Apple Container](https://github.com/apple/container/releases) runs the same image. Create the host folders first, then use `container` in place of `docker`:

```bash
container run -d \
  -p 7476:7476 \
  -v $(pwd)/config:/config \
  ghcr.io/gjhuerte/qui:latest
```

## Permissions

By default, the container runs as root. You can run qui as a different user in two ways. Use one or the other, not both.

With either method, qui needs write access to more than `/config`. Cross-seed hardlink and reflink mode create files in their base directory, and orphan scan deletes files from your scan paths. Those paths live on the data volumes (see [Local filesystem access](#local-filesystem-access)). Run qui as the user that owns that data, or as a member of its group.

### `user:` (standard Docker)

Set `user:` in compose, or `--user` in docker run. Docker starts the container as that user.

```yaml title="docker-compose.yml"
services:
  qui:
    image: ghcr.io/gjhuerte/qui:latest
    user: "1000:1000"
    volumes:
      - ./qui:/config
    ports:
      - "7476:7476"
```

If you use this method, make sure that the host folder mounted at `/config` is writable for that user:

```bash
chown -R 1000:1000 ./qui
```
### PUID/PGID (automatic ownership)

Set both `PUID` and `PGID` environment variables. If you set only one variable, the container refuses to start. The entrypoint then:

1. Creates a user and a group with those IDs
2. Corrects the owner of every file under `/config` that does not match those IDs
3. Runs qui as that user

The result matches `user:`, but the entrypoint corrects the ownership of `/config` for you. If `/config` still contains root-owned files from an earlier run, or if the host folder has the wrong owner, this fixes the ownership.

```yaml title="docker-compose.yml"
services:
  qui:
    image: ghcr.io/gjhuerte/qui:latest
    environment:
      PUID: "1000"
      PGID: "1000"
    volumes:
      - ./qui:/config
    ports:
      - "7476:7476"
```

```bash
docker run -d \
  -e PUID=1000 \
  -e PGID=1000 \
  -p 7476:7476 \
  -v $(pwd)/config:/config \
  ghcr.io/gjhuerte/qui:latest
```

:::note
Do not combine `user:` with `PUID`/`PGID`. If the container does not start as root, the entrypoint cannot create users or change ownership. If you switch to `PUID`/`PGID`, remove any `user:` or `--user` setting first.
:::

The entrypoint walks `/config` only, never your data volumes. As a result, a wrong `PUID` cannot chown your media library. A switch from root needs one manual step for the same reason. If qui already created hardlink or reflink trees as root, chown those directories once yourself:

```bash
find /data/cross-seed -type d -exec chown 1000:1000 {} +
```

Chown the directories only. Hardlinked files share their inode with the source download. If you run a recursive `chown -R`, that command also changes the owner of your library files. qui needs write access to the directories only.

### UMASK

Optional. The qui binary reads `UMASK` at startup and applies it to new files and directories, such as cross-seed hardlink and reflink trees. If the value is not valid octal in the range 000 to 777, qui logs a warning and keeps the inherited umask.

The binary applies `UMASK`, not the entrypoint. As a result, `UMASK` works with both methods above. If you start a non-root container with `user:` or `--user`, `UMASK` also works.

Common values:

- `022`: owner read/write, group and others read-only (typical default)
- `002`: owner and group read/write, others read-only (group-writable)
- `077`: owner only, no access for group and others (private)

Two exceptions:

- Regardless of `UMASK`, qui always creates security-sensitive files (the database, `config.toml`, backup manifests) with owner-only mode (`0600`).
- Hardlinked files share the inode with the source file. They keep the owner and permissions of the original download. See [Directory permissions and umask](../features/cross-seed/troubleshooting.md#directory-permissions-and-umask).

## Local filesystem access

<LocalFilesystemDocker />

## Unraid

The release workflow builds images for `linux/amd64`, `linux/arm64`, and ARM v6/v7, and publishes them to `ghcr.io/gjhuerte/qui`. The container runs on Unraid without extra steps.

### Deploy from the Docker tab

1. Open **Docker → Add Container**
2. Set **Name** to `qui`
3. Set **Repository** to `ghcr.io/gjhuerte/qui:latest`
4. Keep the default **Network Type** (`bridge` works for most setups)
5. Add a port mapping: **Host port** `7476` → **Container port** `7476`
6. Add a path mapping: **Container Path** `/config` → **Host Path** `/mnt/user/appdata/qui`
7. Enable **Advanced View** (top right)
8. Set **Icon URL** to `https://raw.githubusercontent.com/gjhuerte/qui/main/web/public/icon.png`
9. Set **WebUI** to `http://[IP]:[PORT:7476]`
10. Add environment variables `PUID` = `99` and `PGID` = `100`. The entrypoint then corrects the ownership of `/config` and runs qui as uid 99 (`nobody` on Unraid). If **Extra Parameters** contains `--user`, remove it first. If qui ran as root before, fix your data directories once (see [Permissions](#permissions))
11. (Optional) add environment variables for advanced configuration (for example `QUI__BASE_URL`, `QUI__LOG_LEVEL`, `TZ`)
12. Click **Apply** to pull the image and start the container

By default, the `/config` mount stores `config.toml`, logs, the tracker icon cache, and other runtime assets. If you use the default SQLite engine, qui stores `qui.db` there too. An absolute `logPath` or a custom `dataDir` moves those files. `config.toml` always stays in `/config` (see the [configuration reference](../configuration/reference.md)). Point the mount at your appdata share so your configuration survives upgrades.

qui logs to stdout by default. Read the logs under **Docker → qui → Logs**. If you configure a relative log file path, qui writes it under `/config`.

### Updating

- Pull a newer `latest` image with Unraid's **Check for Updates** action
- If you pinned a version tag, edit the repository field to the new tag
- Restart the container after the image update to load the new binary

## Updating

```bash
docker compose pull && docker compose up -d
```
