# qui

A fast, modern web interface for qBittorrent. Supports managing multiple qBittorrent instances from a single, lightweight application.

<div align="center">
  <img src=".github/assets/qui.png" alt="qui" width="100%" />
</div>

## Documentation

Full documentation is in [`documentation/docs`](documentation/docs).

## Quick Start

### Linux x86_64

```bash
# Download and extract the latest release
wget $(curl -s https://api.github.com/repos/gjhuerte/qui/releases/latest | grep browser_download_url | grep linux_x86_64 | cut -d\" -f4)
sudo tar -C /usr/local/bin -xzf qui*.tar.gz qui

# Run
qui serve
```

The web interface will be available at http://localhost:7476

### Docker

```bash
docker run -d \
  -p 7476:7476 \
  -v $(pwd)/config:/config \
  ghcr.io/gjhuerte/qui:latest
```

## Features

- **Single Binary**: No dependencies, just download and run
- **Multi-Instance Support**: Manage all your qBittorrent instances from one place
- **Fast & Responsive**: Optimized for performance with large torrent collections
- **Cross-Seed**: Automatically find and add matching torrents across trackers
- **Automations**: Rule-based torrent management with conditions and actions
- **Backups & Restore**: Scheduled snapshots with multiple restore modes
- **Reverse Proxy**: Transparent qBittorrent proxy for external apps
- **Multi-Language**: Available in English, German, French, Italian, Czech, Ukrainian, Korean, Brazilian Portuguese, Simplified Chinese, Traditional Chinese, and Catalan, with automatic browser-language detection

## Community

Join the community on [GitHub Discussions](https://github.com/gjhuerte/qui/discussions).

## Support

- [GitHub Discussions](https://github.com/gjhuerte/qui/discussions/new/choose) - Feature requests and bug reports
- [GitHub Issues](https://github.com/gjhuerte/qui/issues) - Confirmed bugs and planned work

## Contributing

[`CONTRIBUTING.md`](.github/CONTRIBUTING.md) tells who can contribute and how. It also gives the development and test workflow.

## Alternatives

If qui does not fit your setup, these projects offer different approaches:

- [VueTorrent](https://github.com/VueTorrent/VueTorrent) is a modern, responsive alternative WebUI.
- [iQbit](https://github.com/ntoporcov/iQbit) is a mobile-focused WebUI and PWA.
- [Flood](https://github.com/jesec/flood) supports qBittorrent and other torrent clients.
- [qBitController](https://github.com/Bartuzen/qBitController) is a native app for Android, iOS, Linux, macOS, and Windows.

The qBittorrent wiki includes a longer [list of community WebUIs](https://github.com/qbittorrent/qBittorrent/wiki/List-of-known-alternate-WebUIs).

## License

GPL-2.0-or-later
