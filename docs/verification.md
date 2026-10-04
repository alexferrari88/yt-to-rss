# Verification

Validated on Linux amd64, 2026-10-04. Application image source revision:
`2ae45c8880b3ee81fd7b055f069f496917ee14a0`. Later documentation commits record the results; they do not change the running application code.

## Automated application acceptance

The full CLI/bot/HTTP suite passed:

```sh
GOFLAGS=-race GORACE=atexit_sleep_ms=0 go test -race ./...
go vet ./...
```

Acceptance took 54.967 seconds. The daemon built by the harness was race-instrumented as well as the tests. Only external extractor and Telegram traffic were controlled; SQLite, files, process lifecycle, CLI and HTTP were real. Cases cover canonical/durable acceptance, XML metadata, exact fixture bytes, read authorization, GET/HEAD/ranges, restart and killed workers, finite/manual retries, safe deletion, retention, storage pause/resume, Telegram authorization/replay/reconnection/replies, production-environment isolation and recognizable but incomplete MP3 rejection.

Python helper/driver syntax, Compose validation and diff whitespace checks passed. Independent standards/spec review and resolved findings are in [review](review.md).

## Packaged installation

The reviewed image built with Docker Compose and passed `scripts/check-package.py` under rootless Docker. The CLI submitted a controlled source; RSS/media matched the known 4,510-byte MP3, GUID and HTTP expectations. Container replacement and stopped-state backup/restore into a fresh volume retained the same media and generated read access. The driver published no host port and removed its own temporary containers/volume.

The image includes pinned Go, yt-dlp/EJS, Python, Deno and FFmpeg/ffprobe; current versions and rebuild/rollback instructions are in [operations](operations.md). It runs as UID/GID 10001 with persistent state, dropped capabilities and no-new-privileges.

## Personal private installation

The new service runs behind the existing Traefik proxy at `2pod.thealexferrari.com`. The selected Cloudflare A record is DNS-only and resolves to the server's Tailscale IPv4 address. No application host port is published. Traefik's HTTPS host port remains loopback-only; port 443 listens on the server's Tailscale IPv4/IPv6 addresses. All existing Tailscale Serve routes were unchanged. Rootless Docker is active with user lingering and the new service's unless-stopped restart policy. Hister and Taste Explorer still return trusted HTTPS 200 responses.

From this server through the actual Tailscale-address hostname, trusted TLS, RSS, complete audio GET, HEAD and 206 ranges passed. Missing/wrong read credentials returned 404, including a media range request. Verification used normal certificate validation.

The generated subscription URL is stored in an owner-only file under the private service directory and excluded from Git. No credential or secret URL is included in this report or public source. Private deployment configuration is maintained separately from the portable project.

## Real extraction and memory

Workload: [Rick Astley's official public video](https://www.youtube.com/watch?v=dQw4w9WgXcQ), 213 seconds, one CLI submission, using the actual packaged default tools without cookies or a proxy. Published on its first attempt after 9.61 seconds at 2026-10-04T11:17:47Z. The feed contained its title/uploader, stable `youtube:dQw4w9WgXcQ` GUID and audio enclosure. Downloaded MP3 size: 3,744,644 bytes. Independent FFmpeg decoding completed without errors.

Audio SHA-256: `3344c6492e6a576e57d294a8aab0c8f88e621a25b3840292610c0fd5d789ab92`.

| Measurement | Observed | Scope |
| --- | ---: | --- |
| Idle Go process | 11.5 MiB | Host `/proc` VmRSS of the persistent Go daemon before submission. |
| Go process after conversion | 13.5 MiB | Same process after publication. |
| Peak container during conversion | 364.2 MiB | Reset cgroup v2 `memory.peak`; includes Go, yt-dlp/Python, Deno, FFmpeg/probes, CLI status sampling and charged filesystem page cache. |

The personal container has a 512 MiB memory ceiling and one CPU. This is one measured workload, not a guarantee for every source or dependency release. RSS excludes many resources charged to the container; do not compare its value directly with the complete-workload peak.

An earlier separate short public video hit YouTube's bot-confirmation check. The official 213-second source passed both an isolated tool probe and the final application/container path. Provider availability can differ by source; retry behavior and actionable operator diagnostics are implemented.

## Remaining live acceptance

- Dedicated Telegram configuration is still absent. Automated bot acceptance passes; actual operator submission and queued/published/failure delivery have not been verified. Use the protected interactive setup helper, then recreate only this service.
- Android/AntennaPod subscription, refresh, download, playback and seeking require operator/device confirmation with Tailscale connected. The feed is standard and player-independent.
- A negative request with a valid read URL from an independent outside-tailnet network remains untested. Current DNS/listener/Serve evidence verifies private configuration; successful on-server tailnet HTTP checks do not establish this external result. On the phone, verify fresh retrieval fails with Tailscale disconnected and succeeds when connected; cached/downloaded audio is not a network test.

Ticket 06 remains `needs-info` with those acceptance items unchecked.
