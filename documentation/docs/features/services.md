---
sidebar_position: 19
title: Gluetun, MyAnonamouse seedbox and media apps
sidebar_label: Services
description: Watch and restart the gluetun VPN, keep the MyAnonamouse dynamic seedbox IP up to date, and track Sonarr, Radarr, Prowlarr, Bazarr and Tautulli.
---

# Services

The **Services** page is for a qui that runs behind [gluetun](https://github.com/qdm12/gluetun), with `network_mode: "service:gluetun"`. It has three parts:

- **Gluetun**: the VPN status, the public IP and its location, the forwarded port, the container health, CPU and memory use, the container log, and buttons to restart the VPN or the container.
- **Seedbox (MAM)**: keeps your MyAnonamouse dynamic seedbox IP up to date. It does the same job as the `myanonamouse/seedboxapi` container, so you do not need that container.
- **Media apps**: one place for any number of Sonarr, Radarr, Prowlarr, Bazarr and Tautulli instances, inside gluetun or outside it: a status dashboard, one list of issues, and your movies, series and wanted subtitles.

## Compose example

```yaml
services:
  gluetun:
    image: qmcgaw/gluetun
    container_name: gluetun
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun:/dev/net/tun
    environment:
      - VPN_SERVICE_PROVIDER=protonvpn
      - VPN_TYPE=wireguard
      - WIREGUARD_PRIVATE_KEY=...
      - VPN_PORT_FORWARDING=on
      # Lets qui use the control server (gluetun v3.40 and later need authentication).
      - HTTP_CONTROL_SERVER_AUTH_DEFAULT_ROLE={"auth":"apikey","apikey":"change-me"}
    ports:
      - "7476:7476" # qui
      - "8080:8080" # qBittorrent
    healthcheck:
      test: ["CMD", "ping", "-c", "1", "1.1.1.1"]
      interval: 30s
    restart: always

  qui:
    image: ghcr.io/gjhuerte/qui:latest
    container_name: qui
    network_mode: "service:gluetun"
    # The entrypoint drops extra groups when PUID/PGID are set, so use user: with group_add.
    user: "1000:1000"
    group_add:
      - "${DOCKER_GID}" # getent group docker | cut -d: -f3
    volumes:
      - ./qui-config:/config
      - /var/run/docker.sock:/var/run/docker.sock
    restart: unless-stopped
```

The Docker socket is optional. Without it, the page still shows the VPN status, the public IP and the forwarded port, and it can restart the VPN. The log, the container health and resource use, the list of containers in gluetun's network, and the container restart need the socket.

:::warning
Anyone who can use the Docker socket controls the host. Mount it only when you need those features, and protect qui with a strong password.
:::

## Gluetun

Open **Services → Gluetun → Connection** first:

| Setting | Default | Notes |
| --- | --- | --- |
| Control server URL | `http://127.0.0.1:8000` | qui shares gluetun's network, so `127.0.0.1` reaches gluetun. |
| Authentication | None | Use the API key or the user name and password from `HTTP_CONTROL_SERVER_AUTH_DEFAULT_ROLE`. |
| Docker host | `unix:///var/run/docker.sock` | Or `tcp://host:2375` for a socket proxy. Turn it off to stop all Docker calls. |
| Gluetun container name | `gluetun` | The `container_name` of the gluetun service. |

### Restart VPN or restart container

- **Restart VPN** stops and starts the tunnel through the control server. The container and its network stay up, so qui and qBittorrent keep working once the tunnel is back. Try this first.
- **Restart container** restarts gluetun through Docker, waits until its health check passes, and then restarts each container that shares gluetun's network. Without that second step, those containers keep a dead network until they restart. If qui is one of them, the page reloads when qui is back. A container restart needs the socket: a TCP socket proxy is out of reach while gluetun restarts.

### Configuration

**Configuration** shows the environment variables that the container sets on top of the image defaults (the values from your compose file), and the VPN settings from the control server. Keys and passwords are hidden.

## Seedbox (MyAnonamouse)

Every interval, qui calls MAM's `dynamicSeedbox.php` with your `mam_id` cookie. Because qui runs in gluetun's network, MAM registers the VPN's IP. MAM answers *No change* when the IP is the same, so frequent checks are harmless.

1. Open **Services → Seedbox (MAM) → Dynamic IP**. Before you add a `mam_id`, each check shows the IP MAM sees.
2. On MyAnonamouse, open **Preferences → Security** and create a session:
   - **IP**: the IP from step 1, not the IP of your browser.
   - **IP vs ASN locked session**: ASN, so the IP can change within your VPN provider.
   - **Allow Session to set Dynamic Seedbox**: Yes.
3. Paste the cookie value in **Session (mam_id)** and click **Save and update**.

qui stores the cookie encrypted. When MAM sends a new cookie, qui saves it. **Activity** lists every request to MAM from the last 14 days.

| Result | Meaning |
| --- | --- |
| Updated | MAM now has the new IP. |
| No change | MAM already had this IP. |
| Too recent | MAM limits how often the IP can change. The next check tries again. |
| Rejected | The session cannot be updated from this connection, for example after an ASN mismatch. Create a new session and paste the new `mam_id`. |
| Unreachable | Usually the VPN is down. Check the Gluetun overview. |

### Moving from the seedboxapi container

Remove the `seedboxapi` service and its port from gluetun. The seedboxapi page shows only the end of the cookie, and MAM rotates it, so create a new dynamic seedbox session on MAM and paste its `mam_id` on the Seedbox page.

## Media apps

**Media apps → Dashboard** shows one card per enabled instance. qui only reads from the apps.

| App | What the card shows |
| --- | --- |
| Sonarr, Radarr | Version, health warnings, download queue with progress, missing items, the next 7 days of the calendar, disk space |
| Prowlarr | Version, health warnings, enabled and failing indexers, grabs and queries |
| Bazarr | Version, health warnings, wanted episode and movie subtitles |
| Tautulli | Version, Plex connection, streams with user, progress, direct play or transcode, and bandwidth |

The dashboard refreshes every minute. qui caches the answers for 20 seconds, so several open tabs share one round of requests. **Refresh** skips the cache.

### Issues, movies, series and subtitles

- **Issues** lists everything that needs attention across all apps, errors first: apps that cannot be reached, health warnings, downloads Sonarr or Radarr flag as stuck or failed (with their reason), failing Prowlarr indexers, and a Tautulli that lost Plex.
- **Movies** merges the movie lists of every Radarr instance. Filter by **Missing** (monitored, released, no file), **Downloaded**, **Not released yet** and **Unmonitored**, and by instance.
- **Series** merges every Sonarr instance. It shows episodes on disk against monitored aired episodes, the missing count, the next episode and the size. Filter by **Missing episodes**, **Complete**, **Continuing**, **Ended** and **Unmonitored**.
- **Subtitles** lists what every Bazarr instance still needs subtitles for, with the missing languages (HI and forced marked).

Click a title to open it in its app. qui caches each list for one minute; **Refresh** loads it again. When one instance fails, the page names it and still shows the others.

### Add an app

1. Open **Media apps → Manage apps** and click **Add app**.
2. Choose the app and enter a name, the URL and the API key. Sonarr, Radarr, Prowlarr and Bazarr show the key under **Settings → General → Security**. Tautulli shows it under **Settings → Web Interface → API**, with **Enable API** turned on.
3. Click **Test**, then **Save**.

Add as many instances of each app as you run. The **Enabled** switch hides an instance from the dashboard without deleting it. When you edit an instance, leave the API key empty to keep the stored key. qui stores API keys encrypted and never sends them to the browser.

These instances are separate from the Sonarr and Radarr connections under **Settings → Integrations**, which cross-seed uses to look up IDs.

### Reach the apps from inside gluetun

qui shares gluetun's network, so its requests leave through gluetun:

- An app that also uses `network_mode: "service:gluetun"` is at `http://127.0.0.1:<port>`.
- For an app on your LAN or another Docker network, use the IP address of its host and add that network to `FIREWALL_OUTBOUND_SUBNETS` on gluetun, for example `FIREWALL_OUTBOUND_SUBNETS=192.168.1.0/24,172.16.0.0/12`. Without it, the gluetun firewall blocks the connection. Recreate gluetun and the containers that share its network after the change.
- Container names do not resolve: gluetun runs its own DNS.
- Include the URL base if the app has one, for example `http://192.168.1.10:8989/sonarr`.

| Error on the card | Fix |
| --- | --- |
| `i/o timeout`, `connection refused` | Check the address, and `FIREWALL_OUTBOUND_SUBNETS` on gluetun. |
| `no such host` | Use an IP address. |
| `HTTP 401: check the API key` | Copy the key again. For Tautulli, turn on **Enable API**. |
| `not found: check the URL` | Add the URL base. |
| `did not answer with JSON` | The URL points at a login page or another site. Use the app's direct address. |
