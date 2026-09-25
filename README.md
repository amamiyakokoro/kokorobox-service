<div align="center">

# KokoroBox Service

A privileged companion service for [KokoroBox Desktop](https://github.com/amamiyakokoro/KokoroBox-Desktop), providing an authenticated local API for system operations.

[Desktop](https://github.com/amamiyakokoro/KokoroBox-Desktop) · [License](LICENSE)

</div>

## Features

- Manages the Mihomo core process, system proxy settings, and macOS DNS settings
- Provides per-application routing on Windows and Linux
- Exposes service health, capabilities, and events to Desktop over a protected local connection

## Supported platforms

KokoroBox Service runs on Windows, macOS 13+, and Linux. Application routing supports Windows x64 and Linux x64/arm64. Desktop connects through a named pipe on Windows or a Unix socket on macOS and Linux.

## Get started

[KokoroBox Desktop](https://github.com/amamiyakokoro/KokoroBox-Desktop/releases) handles normal installation and setup. For manual service management on Windows or Linux, run these commands with elevated privileges:

```sh
kokorobox-service service install
kokorobox-service service status
kokorobox-service service start
kokorobox-service service stop
kokorobox-service service restart
kokorobox-service service uninstall
```

On macOS, Desktop registers the daemon through `SMAppService`. Run `kokorobox-service --help` for other commands and options.

## Development

Requires Go 1.26 or later.

```sh
go test ./...
go build -o kokorobox-service .
```

## Documentation

- [Local API, request bodies, and authentication](docs/API.md)

## License

KokoroBox Service is based on [sparkle-service](https://github.com/UruhaLushia/sparkle-service) and licensed under [GNU GPLv3](LICENSE).
