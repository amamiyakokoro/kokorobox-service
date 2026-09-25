# Local API

KokoroBox Desktop talks to the service over `\\.\pipe\kokorobox\service` on Windows or `/tmp/kokorobox-service.sock` on Linux and macOS. The transport carries HTTP requests; it is not a public TCP server. Paths below are relative to that local endpoint.

## Authentication and responses

`GET /ping` needs no authentication. `POST /bootstrap` is available only during initial macOS setup and verifies the signed Desktop process through the socket peer identity. Every other endpoint requires the authorized OS user (SID on Windows, UID on Linux/macOS) and an Ed25519 request signature.

Signed requests use these headers:

| Header | Value |
| --- | --- |
| `X-Auth-Version` | `3` (or legacy `2`) |
| `X-Timestamp` | Unix time in milliseconds, within 30 seconds of service time |
| `X-Nonce` | Unique value within the replay window |
| `X-Key-Id` | Registered public key ID |
| `X-Content-SHA256` | Lowercase hex SHA-256 of the raw request body, including an empty body |
| `X-Signature` | Base64 Ed25519 signature of the canonical request |

The canonical request is the following fields joined with newline characters, including the empty query line: domain (`KOKOROBOX-AUTH-V3` for version 3, `SPARKLE-AUTH-V2` for version 2), timestamp, nonce, key ID, uppercase method, escaped path, normalized query, and body hash. Normalize the query by sorting keys and their values, then joining URL-escaped `key=value` pairs with `&`. Desktop provisions the key and authorized principal during setup.

Responses use JSON unless an endpoint returns `204 No Content`, a WebSocket stream, or a proxied core response. Simple results and errors have `status` (`success` or `error`) and a localized `message`. `Accept-Language` supports `en`, `zh-CN`, and `zh-TW`; the selected language appears in `Content-Language`.

## Service and metadata

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/ping` | Unauthenticated health check. |
| `POST` | `/bootstrap` | macOS initial key setup; body: `{ "public_key": "<base64-DER>" }`. |
| `GET` | `/meta` | Service version, API version, and platform capabilities. |
| `GET` | `/test` | Verify authenticated access. |
| `POST` | `/service/stop` | Stop the service asynchronously. |
| `POST` | `/service/restart` | Restart the service asynchronously. |

`/service/stop` and `/service/restart` normally return `202 Accepted` before the action completes.

## Core

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/core/` | Current core process status. |
| `GET` | `/core/desired` | Desired state (`running` or `stopped`). |
| `GET` | `/core/events` | Authenticated WebSocket stream of core events. |
| `GET` | `/core/profile` | Saved launch profile. |
| `POST` | `/core/profile` | Replace the saved launch profile. |
| `PATCH` | `/core/profile` | Update supported launch profile fields. |
| `POST` | `/core/start` | Start and maintain the desired core state. Optional body: launch profile. |
| `POST` | `/core/stop` | Stop the core and clear the desired running state. |
| `POST` | `/core/restart` | Restart the core. Optional body: launch profile. |
| Any | `/core/controller` and `/core/controller/*` | Forward a request to the active core controller. |

A full launch profile can contain `core_path`, `mode` (`auto`, `sandbox`, or `direct`), `args`, `safe_paths`, `env`, `mihomo_cpu_priority`, `log_path`, `save_logs`, and `max_log_file_size_mb`. `PATCH /core/profile` accepts only `mode`, `log_path`, `save_logs`, and `max_log_file_size_mb`.

## System proxy

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/sysproxy/status` | Query current system proxy settings. |
| `GET` | `/sysproxy/events` | Authenticated WebSocket stream of proxy guard events. |
| `POST` | `/sysproxy/proxy` | Set a proxy; body includes `server` and optional `bypass`. |
| `POST` | `/sysproxy/pac` | Set PAC; body includes `url`. |
| `POST` | `/sysproxy/disable` | Disable the system proxy. |
| `POST` | `/sysproxy/renew` | Renew the service-owned proxy lease. |

Mutation bodies may also include `device`, `only_active_device`, `use_registry`, and `guard`. Successful mutations and renewal return `204 No Content`. Desktop must renew an active lease; the service restores prior settings when the lease expires.

## DNS and system settings

| Method | Path | Platform | Purpose |
| --- | --- | --- | --- |
| `POST` | `/network/dns/lease` | macOS | Set a DNS lease; body: `{ "servers": ["1.1.1.1"] }` (one to four IP addresses). |
| `POST` | `/network/dns/renew` | macOS | Renew the active DNS lease. |
| `DELETE` | `/network/dns/lease` | macOS | Release the lease and restore prior DNS settings. |
| `POST` | `/sys/dns/set` | macOS | Set DNS directly; body: `device` and `servers`. |
| `PUT` | `/sys/uwp-loopback` | Windows | Set a UWP app container loopback exemption; body: `id` (hex) and `enabled` (boolean). |

DNS lease operations return `204 No Content` on success. A DNS lease lasts 60 seconds unless renewed.

## Application routing

Supported on Windows x64 and Linux x64/arm64.

| Method | Path | Purpose |
| --- | --- | --- |
| `PUT` | `/process-router/rules` | Validate and replace rules; returns router status. |
| `POST` | `/process-router/start` | Start routing; returns router status. |
| `GET` | `/process-router/status` | Read status and renew the client lease. |
| `POST` | `/process-router/stop` | Stop routing but retain rules; returns `204`. |
| `POST` | `/process-router/firewall/repair` | Verify and repair firewall state; returns router status. |
| `POST` | `/process-router/cleanup` | Stop routing and remove saved rules; returns `204`. |

The rules body requires `version: 1`, `platform` (`windows` or `linux`), `proxy_port` (`7891` on Windows, `7894` on Linux), `fail_closed: true`, and `rules`. Optional flags are `proxy_udp_dns` and `diagnostic_logging`. Each rule has `id`, `executable_path` or (on Linux) `executable_name`, `protocol` (`tcp`, `udp`, `both`), `action` (`proxy`, `direct`, `block`), `enabled`, and `priority`. Linux also accepts `match_kind: "process_name"`; Windows requires executable path matching. Rule IDs, targets, and priorities must be unique; the limit is 256 rules.
