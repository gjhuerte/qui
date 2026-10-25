# qui

A fast, modern web interface for qBittorrent. Manage one or many qBittorrent instances from a single, lightweight app, and keep your VPN and MyAnonamouse seedbox IP in check from the same place.

<div align="center">
  <img src=".github/assets/qui.png" alt="qui" width="100%" />
</div>

## Contents

- [Features](#features)
- [Quick start](#quick-start)
- [Full stack: gluetun, qBittorrent, qui](#full-stack-gluetun-qbittorrent-qui)
- [Set up gluetun in qui](#set-up-gluetun-in-qui)
- [Set up the MyAnonamouse seedbox IP](#set-up-the-myanonamouse-seedbox-ip)
- [Troubleshooting](#troubleshooting)
- [Documentation](#documentation)
- [License](#license)

## Features

**Torrents**

- **Multi-instance:** manage every qBittorrent instance from one place, with a unified view across instances.
- **Fast with large libraries:** live updates over a single stream, virtualized tables, and cached file lists.
- **Torrent management:** add, move, rename, recheck, edit trackers, set priorities, and run bulk actions with filters and saved views.
- **Search:** search your Torznab/Jackett/Prowlarr indexers and add results directly.
- **RSS:** manage qBittorrent feeds and auto-download rules.
- **Dashboard:** transfer speeds, tracker breakdown, and per-instance statistics.

**Automation**

- **Cross-seed:** find and add matching torrents across trackers, from RSS, webhooks, completed torrents, your library, or a directory scan.
- **Automations:** rule-based actions (speed limits, tags, categories, deletes, moves) with conditions.
- **Reannounce:** re-announce stalled torrents automatically.
- **Orphan scan:** find and clean up files on disk that no torrent uses.
- **External programs:** run your own scripts from the torrent list or from automations.
- **Notifications:** send events to Discord, Telegram, email, and more.

**VPN and seedbox** (when qui runs behind [gluetun](https://github.com/qdm12/gluetun))

- **Gluetun:** see the VPN status, public IP and location, forwarded port, container health, CPU and memory, and the container log. Restart the VPN or the whole container from the browser.
- **MyAnonamouse dynamic seedbox IP:** keeps MAM's seedbox IP up to date with your VPN IP, with an activity history. No separate seedboxapi container needed.

**Operations**

- **Backups and restore:** scheduled snapshots of each instance with several restore modes.
- **qBittorrent proxy:** give Sonarr, Radarr, autobrr and other apps a qBittorrent-compatible endpoint with their own API keys.
- **Single binary:** one file with the web UI built in. SQLite by default, Postgres optional.
- **Sign-in:** built-in account or OpenID Connect.
- **Themes and languages:** custom themes, and English, German, French, Italian, Czech, Ukrainian, Korean, Brazilian Portuguese, Simplified and Traditional Chinese, and Catalan.

## Quick start

### Docker

```bash
docker run -d \
  --name qui \
  -p 7476:7476 \
  -v "$(pwd)/config:/config" \
  ghcr.io/gjhuerte/qui:latest
```

### Linux x86_64

```bash
wget $(curl -s https://api.github.com/repos/gjhuerte/qui/releases/latest | grep browser_download_url | grep linux_x86_64 | cut -d\" -f4)
sudo tar -C /usr/local/bin -xzf qui*.tar.gz qui
qui serve
```

Open **http://localhost:7476**, create your account, and add your qBittorrent instance under **Settings → Instances**.

## Full stack: gluetun, qBittorrent, qui

This stack sends all torrent traffic through the VPN. qBittorrent and qui share gluetun's network (`network_mode: "service:gluetun"`), so their ports are published on the **gluetun** service. qui reaches qBittorrent and gluetun at `127.0.0.1`.

### 1. Prepare the folder

```bash
mkdir -p gluetun qbittorrent qui-config docker-proxy downloads
id -u                                  # your user id (1000 below)
getent group docker | cut -d: -f3      # docker group id (999 below)
docker run --rm qmcgaw/gluetun genkey  # API key for gluetun's control server
```

### 2. `docker-compose.yml`

```yaml
services:
  gluetun:
    image: qmcgaw/gluetun
    container_name: gluetun
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun:/dev/net/tun
    volumes:
      - ./gluetun:/gluetun
    environment:
      - VPN_SERVICE_PROVIDER=protonvpn
      - VPN_TYPE=wireguard
      - WIREGUARD_PRIVATE_KEY=your-wireguard-private-key
      - SERVER_COUNTRIES=Singapore
      - FIREWALL=on
      - VPN_PORT_FORWARDING=on
      # Send the forwarded port to qBittorrent whenever it changes
      - VPN_PORT_FORWARDING_UP_COMMAND=/bin/sh -c 'wget -O- --retry-connrefused --post-data "json={\"listen_port\":{{PORTS}}}" http://127.0.0.1:8080/api/v2/app/setPreferences 2>&1'
      # Lets qui use the control server. No quotes around the JSON.
      - HTTP_CONTROL_SERVER_AUTH_DEFAULT_ROLE={"auth":"apikey","apikey":"your-gluetun-api-key"}
      - HEALTH_TARGET_ADDRESS=1.1.1.1
    ports:
      - "7476:7476" # qui
      - "8080:8080" # qBittorrent WebUI
    healthcheck:
      test: ["CMD", "ping", "-c", "1", "1.1.1.1"]
      interval: 30s
      timeout: 10s
      retries: 3
    restart: always

  qbittorrent:
    image: lscr.io/linuxserver/qbittorrent
    container_name: qbittorrent
    network_mode: "service:gluetun"
    environment:
      - PUID=1000
      - PGID=1000
      - WEBUI_PORT=8080
    volumes:
      - ./qbittorrent:/config
      - ./downloads:/downloads
    depends_on:
      - gluetun
    restart: unless-stopped

  # Gives qui only the Docker calls it needs (read status and logs, restart).
  # Optional: without it, qui still shows the VPN status, IP and port.
  socket-proxy:
    image: wollomatic/socket-proxy:1.13.1
    container_name: socket-proxy
    user: "1000:999"            # your user id : docker group id
    network_mode: none
    read_only: true
    cap_drop: [ALL]
    security_opt: [no-new-privileges:true]
    mem_limit: 64m
    command:
      - '-proxysocketendpoint=/socket/docker.sock'
      - '-proxysocketendpointfilemode=0600'
      - '-allowGET=/(v[0-9.]+/)?(_ping|containers/json|containers/[a-zA-Z0-9_.-]+/(json|logs|stats)|images/[a-zA-Z0-9_.:@/-]+/json)'
      - '-allowPOST=/(v[0-9.]+/)?containers/[a-zA-Z0-9_.-]+/restart'
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./docker-proxy:/socket
    restart: unless-stopped

  qui:
    image: ghcr.io/gjhuerte/qui:latest
    container_name: qui
    network_mode: "service:gluetun"
    user: "1000:1000"
    volumes:
      - ./qui-config:/config
      - ./downloads:/downloads     # same path as in qBittorrent
      - ./docker-proxy:/var/run/docker-proxy
    depends_on:
      - gluetun
    restart: unless-stopped
```

```bash
docker compose up -d
```

### 3. Connect qui to qBittorrent

1. Open **http://localhost:7476** and create your account.
2. Go to **Settings → Instances → Add instance**. Use `http://127.0.0.1:8080` as the host, with your qBittorrent WebUI user and password. The linuxserver image prints a temporary password in `docker logs qbittorrent` on the first start.
3. In qBittorrent, open **Options → WebUI** and turn on **Bypass authentication for clients on localhost**. The port-forwarding command in gluetun needs it to update the listening port.

## Set up gluetun in qui

Open **Services → Gluetun → Connection**:

| Setting | Value |
| --- | --- |
| Control server URL | `http://127.0.0.1:8000` |
| Authentication | API key: the key from `HTTP_CONTROL_SERVER_AUTH_DEFAULT_ROLE` |
| Use the Docker API | On, if you added the socket proxy |
| Docker host | `unix:///var/run/docker-proxy/docker.sock` |
| Gluetun container name | `gluetun` |

Click **Save**, then open **Overview**. It should show the VPN as **Running** with your public IP, location and forwarded port.

What you can do there:

- **Restart VPN:** stops and starts the tunnel. Containers keep their network. Try this first when the VPN is stuck.
- **Start/Stop VPN:** turns the tunnel on or off.
- **Restart container:** restarts gluetun through Docker, waits until it is healthy, then restarts the containers that share its network (qBittorrent and qui). qui reloads by itself when it is back.
- **Logs:** the gluetun log, with level filter, search and auto refresh.
- **Configuration:** your gluetun environment variables and VPN settings, with keys and passwords hidden.

## Set up the MyAnonamouse seedbox IP

qui calls MAM's dynamic seedbox endpoint on a schedule. Because qui runs inside gluetun, MAM registers your **VPN** IP.

1. Open **Services → Seedbox (MAM) → Dynamic IP** and click **Update now**. Before you add a cookie, this only shows the IP and network MAM sees.
2. On MyAnonamouse, go to **Preferences → Security** and create a session:
   - **IP:** the IP from step 1, not your browser's IP.
   - **IP vs ASN locked session:** **ASN**, so the IP can change within your VPN provider.
   - **Allow Session to set Dynamic Seedbox:** **Yes**.
   - **Label:** for example `qui`. Use a separate session for qui.
3. Copy the cookie value MAM shows, paste it into **Session (mam_id)**, and click **Save and update**. The result should be **Updated** or **No change**.
4. Under **Schedule**, keep automatic updates on. Every 10 minutes is fine: MAM answers *No change* when the IP is the same.

qui stores the cookie encrypted and saves the new one whenever MAM rotates it. **Activity** lists every request from the last 14 days.

If you used the `seedboxapi` container before, remove it and its port from gluetun, and create a new MAM session for qui.

## Troubleshooting

### qui does not open

- Check that the container runs and read its log: `docker ps`, `docker logs qui`.
- With `network_mode: "service:gluetun"`, publish port **7476 on gluetun**, not on qui. If gluetun is unhealthy or restarting, qui is unreachable too.
- Behind a reverse proxy with `QUI__ALLOWED_HOSTS` set, add your hostname to it. Over HTTPS, set `QUI__SESSION_COOKIE_SECURE=true`.

### I forgot my password

```bash
docker exec -it qui qui change-password --username <your-user>
```

### qui cannot connect to qBittorrent

- Inside gluetun's network, the host is `http://127.0.0.1:8080`, not the container name.
- Check the WebUI user and password in qBittorrent. Too many failed logins ban the IP for a while; restart qBittorrent to clear the ban.
- After gluetun restarts, containers that share its network lose their network. Restart them: `docker compose up -d --force-recreate qbittorrent qui`, or use **Restart container** in qui, which does this for you.

### Gluetun: "Unreachable" or "HTTP 401, check the control server authentication"

The control server rejects qui's request.

1. Test the key from the host (publish `8000` on gluetun, for example `"8003:8000"`):
   ```bash
   curl -i -H "X-API-Key: your-gluetun-api-key" http://localhost:8003/v1/vpn/status
   ```
2. **401 there too:** gluetun does not have the key. Check that `HTTP_CONTROL_SERVER_AUTH_DEFAULT_ROLE` is on the **gluetun** service, without quotes around the JSON, then recreate gluetun (`docker compose up -d gluetun`) and the containers that share its network.
3. **200 there:** qui has the wrong setting. In **Connection**, set **Authentication** to **API key**, paste the key again, and save.

Gluetun v3.40 and later refuse every control-server request until you set up authentication.

### Gluetun: "Docker access is turned off" or no logs, health or stats

Logs, health, resource use and container restart need the Docker API. The VPN status, IP and port work without it.

| Message | Fix |
| --- | --- |
| `no such file or directory` | The socket is not mounted. Check the `./docker-proxy:/var/run/docker-proxy` volume on qui and the Docker host in **Connection**. |
| `permission denied` | qui's user cannot use the socket. The socket file must be owned by the same user id that qui runs as (`user:` on both services). |
| `no container named "gluetun"` | Set **Gluetun container name** to the `container_name` of your gluetun service. |
| `403` or `forbidden` from the proxy | The proxy blocked a call. Copy the `-allowGET` and `-allowPOST` lines exactly. |

The socket proxy itself logs `bind: permission denied` when it cannot create its socket. Give the folder to your user and start it again:

```bash
sudo chown "$(id -u):$(id -g)" docker-proxy
docker compose up -d socket-proxy
```

Do not mount `/var/run/docker.sock` into qui directly unless you accept the risk: anything that can use the raw socket controls the host.

### Gluetun: no forwarded port

- Your provider must support port forwarding (Proton: a Plus plan and a P2P server), and `VPN_PORT_FORWARDING=on` must be set.
- Check **Logs** for `port forwarded is …`. If qBittorrent does not pick it up, turn on **Bypass authentication for clients on localhost** in qBittorrent.

### Seedbox (MAM) results

| Result | What to do |
| --- | --- |
| **No mam_id** | Paste a cookie in **Session (mam_id)**. |
| **Updated** / **No change** | Working. |
| **Too recent** | MAM limits how often the IP can change. Nothing to do; the next check retries. |
| **Rejected** (for example *ASN mismatch* or *IP mismatch*) | The session does not allow updates from this connection. Create a new session locked to **ASN**, with dynamic seedbox allowed, and paste the new cookie. |
| **Unreachable** | Usually the VPN is down. Check **Gluetun → Overview** and the gluetun log. |

### Collect information for a bug report

- qui version: shown at the bottom of the sidebar, or `docker exec qui qui version`.
- qui log: **Logs** in the sidebar, or `docker logs qui`.
- Gluetun log: **Services → Gluetun → Logs**, or `docker logs gluetun`.
- Turn on incognito mode (the eye icon in the torrent list status bar) before you take screenshots, so torrent names and trackers stay private.

## Documentation

Detailed guides are in [`documentation/docs`](documentation/docs):

- [Docker](documentation/docs/getting-started/docker.md) and [installation](documentation/docs/getting-started/installation.md)
- [Environment variables](documentation/docs/configuration/environment.md) and [CLI commands](documentation/docs/configuration/cli-commands.md)
- [Gluetun and MyAnonamouse seedbox](documentation/docs/features/services.md)
- [Cross-seed](documentation/docs/features/cross-seed), [automations](documentation/docs/features/automations.md), [backups](documentation/docs/features/backups.md)

## Support

- [GitHub Discussions](https://github.com/gjhuerte/qui/discussions/new/choose): questions, feature requests and bug reports
- [GitHub Issues](https://github.com/gjhuerte/qui/issues): confirmed bugs and planned work

## Contributing

[`CONTRIBUTING.md`](.github/CONTRIBUTING.md) explains who can contribute and how, and the development and test workflow.

## License

GPL-2.0-or-later. See [`LICENSE`](LICENSE) and [`LICENSE-GPL-2.0`](LICENSE-GPL-2.0).
