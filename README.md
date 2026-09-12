# KokoroBox Service

KokoroBox Service is the privileged system component for KokoroBox. It exposes a local HTTP API
that manages the proxy core, system proxy settings, DNS configuration, and application routing.

> This project is independently maintained and was originally based on
> [UruhaLushia/sparkle-service](https://github.com/UruhaLushia/sparkle-service).

## Features

- Starts, stops, and supervises the proxy core.
- Configures system proxy and DNS settings.
- Provides authenticated local communication over a named pipe or Unix socket.
- Supports application routing on Windows x64 and Linux x64/arm64.

## Platform overview

| Platform | Local endpoint | Service integration |
| --- | --- | --- |
| Windows | `\\.\pipe\kokorobox\service` | Windows Service Control Manager |
| Linux | `/tmp/kokorobox-service.sock` | System service |
| macOS 13+ | `/tmp/kokorobox-service.sock` | `SMAppService` LaunchDaemon |

## Requirements

- Go 1.26 or later for building from source.
- Administrator privileges on Windows or root privileges on Linux for service installation.
- The signed KokoroBox Desktop host app for normal registration on macOS 13 or later.

## Build and test

```bash
go build -o kokorobox-service .
go test ./...
```

## Install and manage the service

### Windows and Linux

From an elevated terminal, install and start the service:

```bash
kokorobox-service service install
```

Manage an installed service with:

```bash
kokorobox-service service status
kokorobox-service service start
kokorobox-service service stop
kokorobox-service service restart
kokorobox-service service uninstall
```

### macOS 13+

KokoroBox Desktop embeds the executable and registers it as a privileged LaunchDaemon with
`SMAppService`. Registration and removal belong to the signed host app. The service CLI commands
`start`, `stop`, `restart`, `status`, and `init` control the registered
`system/KokoroBoxService` launchd job without requiring a copied plist in
`/Library/LaunchDaemons`.

The `service init --ensure-running` recovery flow initializes authentication and performs any
required start or restart within the same privileged process. This avoids a second administrator
authorization request during first-time setup.

## Platform security details

### macOS bootstrap

The preferred macOS setup flow uses a one-time `POST /bootstrap` operation on the newly approved
daemon's Unix socket instead of launching the elevated CLI. The daemon:

1. Obtains the caller UID and audit token from the kernel.
2. Validates the live client against the Developer ID requirement for app
   `com.amamiyakokoro.app` and team `755TNLRN92`.
3. Stores the public key and UID without replacing existing state.
4. Restricts the socket to the authorized UID with mode `0600`.

Request data cannot select the authorized UID. Linux and Windows do not expose this endpoint. The
elevated CLI remains available for explicit recovery and compatibility.

### Windows runtime isolation

During installation, the service copies its executable and verified Process Router bundle into a
content-addressed directory below `%ProgramFiles%\KokoroBox Service` before registering with the
Service Control Manager. This keeps the privileged runtime outside user-writable Desktop
installations and allows safe replacement without overwriting a running executable.

When launching a bundled Mihomo core from a current-user Desktop installation, the service also
copies the verified executable into `%ProgramData%\KokoroBox\core-runtime` and applies a privileged
ACL. The user's application directory remains writable.

## Localization

The service supports `en` (default), `zh-CN`, and `zh-TW`.

- Set `KOKOROBOX_LOCALE` or pass `--locale` to select the CLI language.
- Send `Accept-Language` to select the language used by HTTP responses.

## Release policy

Pushes to `main` update the mutable `pre-release` used by KokoroBox Desktop rolling builds. Stable
Desktop builds pin an independent service release such as `v0.1.0`. Stable tags and their assets
must never be moved or replaced.

Every release binary includes a `.sha256` file that KokoroBox Desktop verifies before packaging the
service. Create stable releases with a `vMAJOR.MINOR.PATCH` tag:

- Before `v1.0.0`, bump the minor version for incompatible CLI, API, authentication, or wire-format
  changes.
- Bump the patch version for compatible fixes.
- Keep service versions independent from KokoroBox Desktop versions.

## Related projects

- [amamiyakokoro/sysproxy-go](https://github.com/amamiyakokoro/sysproxy-go) provides system proxy
  integration.

## License

See [LICENSE](LICENSE) for licensing information.
