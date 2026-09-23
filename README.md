# KokoroBox Service

KokoroBox Service is the privileged companion service for
[KokoroBox Desktop](https://github.com/amamiyakokoro/KokoroBox-Desktop). It exposes an authenticated
local API for operations that Desktop should not perform directly as an ordinary user.

This project is independently maintained and was originally based on
[UruhaLushia/sparkle-service](https://github.com/UruhaLushia/sparkle-service).

## Capabilities

- Starts and reconciles the desired proxy-core runtime.
- Manages leased system proxy settings, recovery, and change events.
- Manages leased DNS settings on macOS.
- Provides application routing on Windows x64 and Linux x64/arm64.
- Reports its service version, API version, and supported capabilities to Desktop.

## Platforms

| Platform | Local endpoint | Service integration |
| --- | --- | --- |
| Windows | `\\.\pipe\kokorobox\service` | Windows Service Control Manager |
| Linux | `/tmp/kokorobox-service.sock` | System service |
| macOS 13+ | `/tmp/kokorobox-service.sock` | `SMAppService` LaunchDaemon |

## Build and test

Go 1.26 or later is required.

```bash
go test ./...
go build -o kokorobox-service .
```

Release builds are produced for x64 and arm64 on Windows, Linux, and macOS.

## Service management

KokoroBox Desktop handles normal setup. For manual Windows or Linux administration, run the
following commands from an elevated terminal:

```bash
kokorobox-service service install
kokorobox-service service status
kokorobox-service service start
kokorobox-service service stop
kokorobox-service service restart
kokorobox-service service uninstall
```

On macOS, the signed Desktop app embeds and registers the daemon through `SMAppService`.
Registration and removal belong to the host app; the service CLI only controls an existing
registration or performs explicit recovery.

To inspect the current system proxy settings without changing them:

```bash
kokorobox-service sysproxy status
```

Use `kokorobox-service --help` for all commands and options. CLI and HTTP responses support `en`,
`zh-CN`, and `zh-TW`; select a CLI language with `--locale` or `KOKOROBOX_LOCALE`.

## Security model

- Windows uses a restricted named pipe; Linux and macOS use a protected Unix socket.
- Requests are authenticated with credentials provisioned by Desktop and bound to the authorized
  OS user.
- macOS bootstrap verifies the live Desktop process and its Developer ID before accepting the
  initial key.
- Windows stages privileged service, core, and Process Router files outside user-writable Desktop
  installations.

## Releases

Pushes to `main` update the moving `pre-release` used by Desktop rolling builds. Stable releases use
immutable `vMAJOR.MINOR.PATCH` tags and are versioned independently from Desktop. Every binary and
the Windows Process Router bundle includes a `.sha256` file verified during Desktop packaging.

## Related projects

- [amamiyakokoro/KokoroBox-Desktop](https://github.com/amamiyakokoro/KokoroBox-Desktop)
- [amamiyakokoro/sysproxy-go](https://github.com/amamiyakokoro/sysproxy-go)

## License

See [LICENSE](LICENSE).
