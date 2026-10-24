---
sidebar_position: 18
title: Incognito mode for screen sharing
sidebar_label: Incognito Mode
description: Disguise torrent names, trackers, and save paths for screen sharing.
---

# Incognito Mode

Incognito mode disguises your torrent data so you can share your screen. qui replaces torrent names with Linux ISO names. It also replaces categories, tags, trackers, and save paths with matching fake values.

The disguise is display-only. qui does not change any data in qBittorrent. Each torrent maps to the same fake values every time, based on its info hash. The pool of fake names is small, so two torrents can show the same fake name.

## Turn it on

Incognito mode is one global toggle:

- On the torrent list, click the eye icon in the status bar at the bottom. The label reads **Incognito off** or **Incognito on**.
- On mobile, tap the **Incognito** button in the torrent view.
- The eye icons next to instance URLs on the Dashboard and under **Settings > Instances** toggle the same setting.

qui saves the toggle with your client settings and syncs it to the server. The setting applies across tabs and browsers, and it is off by default.

## What it hides

| Data | Shown instead |
|------|---------------|
| Torrent names | Linux ISO names, for example `debian-12.7.0-amd64-DVD-1.iso` |
| Categories | Linux-themed categories, for example `distributions` and `server-editions` |
| Tags | Linux-themed tags, for example `lts` and `rolling-release` |
| Trackers | Linux mirror domains, for example `mirrors.kernel.org` |
| Save paths | Linux paths, for example `/home/downloads/distributions` |
| File and folder names | Linux ISO and distribution names |
| Torrent metadata | Fake "Created by", comment, and info hash |
| Peer addresses | `192.168.x.x` with masked ports |
| Ratios and sidebar filter counts | Deterministic fake numbers |

qui blurs some values instead of replacing them:

- Instance URLs and usernames on instance cards
- External IP addresses in the status bar and on the Dashboard
- Instance hosts and proxy URLs under **Settings > Client Proxy**

## What it does not hide

Sizes, speeds, progress, states, and dates stay real. Incognito mode is a visual disguise for casual viewers, not a privacy control.

:::warning
If you export a `.torrent` file while incognito mode is on, qui renames the downloaded file with the fake name. The file still contains the real torrent data. Do not share exported files as if they were disguised.
:::
