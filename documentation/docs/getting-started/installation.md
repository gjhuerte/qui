---
sidebar_position: 1
title: Install qui, the web UI for qBittorrent
sidebar_label: Installation
description: Install qui on Linux with one command, download the binary for Linux, macOS, or Windows, or run the Docker image.
---

# Installation

## Quick install (Linux x86_64)

```bash
# Download and extract the latest release
wget $(curl -s https://api.github.com/repos/gjhuerte/qui/releases/latest | grep browser_download_url | grep linux_x86_64 | cut -d\" -f4)
```

### Unpack

Extract the archive to `/usr/local/bin`:

```bash
tar -C /usr/local/bin -xzf qui*.tar.gz qui
```

If the command fails with a permission error, run it again with `sudo`. If you do not have root, or you are on a shared system, extract qui to a directory in your home directory, for example `~/.bin`.

## Manual download

Download the latest release for your platform from the [releases page](https://github.com/gjhuerte/qui/releases). Windows users should follow the [Windows guide](./windows.md). For containers, see the [Docker guide](./docker.md).

On Linux or macOS, extract the archive:

```bash
tar -xzf qui*.tar.gz qui
```

## Run

If you extracted the archive to `/usr/local/bin`, the binary is on your PATH:

```bash
qui serve
```

If you extracted to `~/.bin` or downloaded the archive by hand, change to that directory first:

```bash
chmod +x qui
./qui serve
```

The web interface is available at http://localhost:7476. To change the port or other settings, see [environment variables](../configuration/environment.md).

## Updating

qui does not update itself. When the web UI shows that a new release is available, update by hand:

1. Back up the qui config directory. A new version can migrate the database, and the old version cannot always read a migrated database.
2. Stop qui.
3. Download the archive for your platform from [GitHub Releases](https://github.com/gjhuerte/qui/releases/latest) and replace the `qui` binary with the one from the archive.
4. Start qui again.

To roll back, stop qui, put the old binary back, and restore the config directory backup if the new version migrated the database.

For Docker, pull a new image instead (see [Docker](./docker.md#updating)). On Windows, follow [Updating](./windows.md#updating) in the Windows guide.

## First setup

1. Open your browser at http://localhost:7476
2. Create your account
3. Add your qBittorrent instances
4. Manage your torrents

Create the account before you expose qui to the Internet, and read [Sessions](../configuration/reference.md#sessions) before you put qui behind HTTPS.
