# sparkle-service

This is a fork of [xishang0128/sparkle-service](https://github.com/xishang0128/sparkle-service) maintained for KokoroBox.

Written in Go, this system service manages the proxy core process, system proxy settings, and DNS configuration. It exposes a local HTTP API over a Unix socket on Linux and macOS or a named pipe on Windows.

## Fork-specific features

- Starts and supervises the KokoroBox Process Router on Windows x64.
- Changes enabled Proxy rules to Block when Mihomo is unavailable, preventing unintended direct connections.
- Validates and persists Process Router rules, with APIs for status, shutdown, and cleanup.
- Uses a client lease to release Router and WinDivert resources after an abnormal client exit.

The native Process Router components must be placed in the `process-router` directory next to `sparkle-service.exe`. This integration currently supports Windows 10/11 x64 only.

## Build and test

Go 1.26 or later is required.

```bash
go build -o sparkle-service .
go test ./...
```

Install the system service with administrator privileges on Windows or root privileges on Linux and macOS:

```bash
sparkle-service service install
```

## Documentation

- [Upstream project](https://github.com/xishang0128/sparkle-service)
- [Archived full README](README.original.md)

## License

See [LICENSE](LICENSE) for licensing information.
