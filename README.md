# kokorobox-service

The system service for KokoroBox. It manages the proxy core, system proxy settings, and DNS
configuration through a local HTTP API.

Independently maintained and originally based on
[xishang0128/sparkle-service](https://github.com/xishang0128/sparkle-service).

## Features

- Runs and supervises the proxy core.
- Manages system proxy and DNS settings.
- Supports Windows x64 application routing with Process Router.

## Build and test

Go 1.26 or later is required.

```bash
go build -o kokorobox-service .
go test ./...
```

Install with administrator privileges on Windows or root privileges on Linux and macOS:

```bash
kokorobox-service service install
```

On Windows, `service install` copies the executable and its verified Process Router bundle into a
content-addressed directory below `%ProgramFiles%\KokoroBox Service` before registering it with the
Service Control Manager. This keeps the privileged runtime outside user-writable Desktop
installations and permits safe replacement without overwriting a running executable.

When the Windows service launches a bundled Mihomo core from a current-user Desktop installation,
it likewise copies the verified executable into `%ProgramData%\KokoroBox\core-runtime` and applies
the privileged ACL there. It does not make the user's application directory read-only.

## Localization

Supports `en` (default), `zh-CN`, and `zh-TW`. Set `KOKOROBOX_LOCALE` or use `--locale`; HTTP
clients can use `Accept-Language`.

## Upstream

- [UruhaLushia/sysproxy-go](https://github.com/UruhaLushia/sysproxy-go) powers system-proxy integration.

## License

See [LICENSE](LICENSE) for licensing information.
