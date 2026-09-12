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

Install with administrator privileges on Windows or root privileges on Linux:

```bash
kokorobox-service service install
```

On macOS 13 and later, KokoroBox Desktop embeds the executable and registers it as a privileged
LaunchDaemon with `SMAppService`. Registration and removal therefore belong to the signed host app;
the service CLI's `start`, `stop`, `restart`, `status`, and `init` commands control the registered
`system/KokoroBoxService` launchd job without depending on a copied plist in
`/Library/LaunchDaemons`.

`service init --ensure-running` performs authentication initialization and the required start or
restart in the same privileged process. Desktop clients use it to avoid requesting administrator
authorization twice during first-time setup.

The preferred macOS path no longer launches that CLI. A freshly approved daemon exposes a
one-time `POST /bootstrap` operation on its Unix socket. The daemon obtains the caller UID and
audit token from the kernel, validates the live client against the Developer ID requirement for
`com.amamiyakokoro.app` and team `755TNLRN92`, commits the public key and UID without replacing
existing state, then restricts the socket to that UID with mode `0600`. Request data can never
select the authorized UID, and Linux and Windows do not expose this endpoint. The elevated CLI is
retained only for explicit recovery and compatibility.

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

## Releases

Pushes to `main` update the mutable `pre-release` used by KokoroBox Desktop rolling builds. Stable
Desktop builds pin an independent service release such as `v0.1.0`; stable tags and their assets
must never be moved or replaced. Every release binary is accompanied by a `.sha256` file, which
Desktop verifies before packaging the service.

Create stable releases with a `vMAJOR.MINOR.PATCH` tag. Until the service reaches `v1.0.0`, bump the
minor version for incompatible CLI, API, authentication, or wire-format changes, and the patch
version for compatible fixes. Service versions are independent from KokoroBox Desktop versions.

## Upstream

- [amamiyakokoro/sysproxy-go](https://github.com/amamiyakokoro/sysproxy-go) powers system-proxy integration.

## License

See [LICENSE](LICENSE) for licensing information.
