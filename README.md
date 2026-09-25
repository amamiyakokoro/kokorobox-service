# KokoroBox Service

Privileged companion service for [KokoroBox Desktop](https://github.com/amamiyakokoro/KokoroBox-Desktop). It provides an authenticated local API to manage the proxy core, system proxy, and macOS DNS settings. Application routing is available on Windows x64 and Linux x64/arm64.

Desktop handles normal installation and setup. The service runs through a Windows named pipe or a protected Unix socket on Linux and macOS.

## Build

Requires Go 1.26 or later.

```bash
go test ./...
go build -o kokorobox-service .
```

## Service commands

On Windows and Linux, run these commands with elevated privileges:

```bash
kokorobox-service service install
kokorobox-service service status
kokorobox-service service start
kokorobox-service service stop
kokorobox-service service restart
kokorobox-service service uninstall
```

On macOS, Desktop registers the daemon through `SMAppService`. Run `kokorobox-service --help` for other commands and options.

See [Local API](docs/API.md) for endpoints, request bodies, and authentication.

Originally based on [sparkle-service](https://github.com/UruhaLushia/sparkle-service). Licensed under [GPL-3.0](LICENSE).
