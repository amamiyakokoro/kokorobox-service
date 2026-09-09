# kokorobox-service

KokoroBox Service is the system service for KokoroBox. It is independently maintained and was
originally based on [xishang0128/sparkle-service](https://github.com/xishang0128/sparkle-service).

Written in Go, this system service manages the proxy core process, system proxy settings, and DNS configuration. It exposes a local HTTP API over a Unix socket on Linux and macOS or a named pipe on Windows.

## KokoroBox features

- Starts and supervises the KokoroBox Process Router on Windows x64.
- Changes enabled Proxy rules to Block when Mihomo is unavailable, preventing unintended direct connections.
- Validates and persists Process Router rules, with APIs for status, shutdown, and cleanup.
- Uses a client lease to release Router and WinDivert resources after an abnormal client exit.
- Creates, verifies, repairs, and removes the narrowly scoped Windows Firewall rules required by
  the Process Router TCP and UDP relays.

The native Process Router components must be placed in the `process-router` directory next to `kokorobox-service.exe`. This integration currently supports Windows 10/11 x64 only.

## Build and test

Go 1.26 or later is required.

```bash
go build -o kokorobox-service .
go test ./...
```

Install the system service with administrator privileges on Windows or root privileges on Linux and macOS:

```bash
kokorobox-service service install
```

The same privileged helper used by portable KokoroBox builds can manage the application-routing
firewall rule group explicitly:

```bash
kokorobox-service process-router firewall ensure
kokorobox-service process-router firewall check
kokorobox-service process-router firewall remove
```

## Localization

User-visible CLI output, service logs, and HTTP API messages use English source messages (`en`,
the default) and support Simplified Chinese (`zh-CN`) and Traditional Chinese (`zh-TW`). Set
`KOKOROBOX_LOCALE=zh-TW` before starting the service, or pass `--locale zh-TW` to a CLI command.
HTTP clients can select a response language per request with the standard `Accept-Language`
header; responses include the selected `Content-Language`.

## Upstream dependencies

- [UruhaLushia/sysproxy-go](https://github.com/UruhaLushia/sysproxy-go) provides the cross-platform
  system-proxy integration used by the `sysproxy` CLI commands and HTTP API.

## Documentation

- [Original project](https://github.com/xishang0128/sparkle-service)
- [Original README](README.original.md)

## License

See [LICENSE](LICENSE) for licensing information.
