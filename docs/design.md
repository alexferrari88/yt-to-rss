# YouTube-to-RSS design

Status: confirmed by the operator on 2026-10-04. Implementation, testing, private deployment, and public project publication are authorized within this design.

## Confirmed requirements

- Send selected public or unlisted YouTube videos from an Android phone and receive them as audio episodes in a standard podcast RSS feed.
- One user per installation, for personal use. Other users can run their own instances.
- Keep the service simple and memory use low; expected volume is low but unspecified.
- Use Go for the compiled service and CLI instead of a persistent Python daemon.
- Manage it through a CLI; no dashboard is needed.
- Run this installation on the current always-on server, accessible only through the operator's Tailscale network.
- Reuse the existing Traefik and DNS-only Cloudflare hostname `2pod.thealexferrari.com`.
- Long secret HTTPS read URLs are acceptable in addition to the network boundary; they grant no operator write access.
- Publish a public `alexferrari88/yt-to-rss` repository under MIT with Docker Compose installation.
- Use AntennaPod as an example acceptance player. The application protocol remains standard RSS and does not depend on any particular podcast app.

## Accepted behavior

- Use Telegram as the primary phone submission path. The operator accepted either Share-menu submission or Telegram; Telegram provides the accepted queued/published/failed replies.
- Allow submissions only from the configured operator. The bot receives updates through outbound polling; it requires no public inbound webhook.
- Accept one source video per URL and publish its full audio as MP3. Ignore shared start timestamps; playlist imports, channel subscriptions, trimming, and active livestreams are outside the initial scope.
- Process one video at a time. Save work durably so restart does not lose queued submissions.
- Repeated submissions of an active source reuse its existing episode/status rather than creating duplicates. Retry transient failures a bounded number of times; permit explicit retry through the CLI.
- Publish only completed audio. Preserve episode identity and use publication time as the feed episode date.
- Expire an episode 30 days after successful publication: remove it from the feed and delete its server-side audio. Copies already saved by a player remain under that player's control.
- Keep retention and storage limits configurable. Pause new processing when storage is full rather than deleting episodes before their retention ends.
- Published means ready in this service's feed; the player controls refresh timing.

## Proposed implementation defaults

- One Go service with SQLite for durable submission/episode state, on-disk audio, and yt-dlp/FFmpeg subprocesses; process files without loading whole episodes into RAM.
- Go's standard library provides seekable-file HTTP serving with HEAD/range support, XML generation, and subprocess execution. This supports a small service with few application dependencies; it is a simplicity rationale, not a measured RAM comparison against Rust. Both toolchains are available on this server. [HTTP serving](https://pkg.go.dev/net/http#ServeContent), [XML encoding](https://pkg.go.dev/encoding/xml#Marshal), [Process execution](https://pkg.go.dev/os/exec#CommandContext)
- Bundle the current yt-dlp YouTube prerequisites, including yt-dlp-ejs and a supported JavaScript runtime, in the Docker deployment.
- yt-dlp remains Python-based and runs only during extraction. The Go service removes the persistent Python daemon; it does not make the extraction toolchain Python-free. Measure the daemon and total processing memory separately.
- A local CLI provides add, list/status, retry, and delete operations. A separate management HTTP API is unnecessary for the API-or-CLI requirement.
- Docker Compose packages the service with persistent state outside the image. Keep deployment-specific secrets and generated audio out of Git.
- Configuration provides the externally visible base URL, feed identity, operator/bot credentials, retention, and resource limits. The application itself does not require Traefik, Cloudflare, or Tailscale.
- Measure idle and active processing RAM during acceptance; no RAM bound has been measured or promised yet.

These implementation defaults and the complete design are confirmed. No application code, Git repository, deployment, or DNS change has been made yet.

## Personal deployment

Selected hostname: `2pod.thealexferrari.com`.

Read-only A, AAAA, and CNAME queries returned NXDOMAIN on 2026-10-04. This does not prove that the Cloudflare zone has no record; inspect the exact name in the zone before creating or changing it. No DNS record or route has been created.

Use the verified private convention already present on this server:

- DNS-only Cloudflare record to the server's Tailscale address.
- Existing private Tailscale TCP 443 route to loopback Traefik HTTPS.
- Traefik Host rule on `websecure`, using the existing `cloudflare` DNS-01 certificate resolver.
- Application on the private Docker `proxy` network, without a published host application port.
- Preserve all existing private routes, including the separate Serve route on 8443.

The operator explicitly requires Tailscale-only access. Do not activate a Funnel, Cloudflare Tunnel, public listener, or Cloudflare proxy/CDN for this installation. Cloudflare supplies DNS and certificate validation rather than carrying feed/audio traffic. DNS/certificate publication does not imply public service reachability. [Service instructions](/opt/services/AGENTS.md)

Secret-URL access is recorded in ADR-0001; the personal network boundary is recorded in ADR-0002; Go and the external extractor boundary are recorded in ADR-0003.

## Player compatibility and protocol facts

- Use RSS 2.0 with stable episode GUIDs and audio enclosures containing URL, byte length, and MIME type. [RSS specification](https://www.rssboard.org/rss-specification)
- Serve audio with HTTP HEAD and byte-range support for streaming and seeking, and conventional podcast metadata. Apple's technical requirements are a compatibility reference, not a directory-publication requirement. [Podcast feed requirements](https://podcasters.apple.com/support/823-podcast-requirements)
- RSS/HTTP have no acknowledgement that a podcast player saved a durable, playable offline copy. A complete transfer or partial/range requests cannot establish that fact; this is an inference from the protocol semantics. Use the operator's 30-day fallback rather than download-triggered deletion. [RSS enclosures](https://www.rssboard.org/rss-specification#ltenclosuregtSubelementOfLtitemgt), [HTTP GET](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.3.1), [Partial Content](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.7)
- AntennaPod accepts RSS addresses and uses a distributed model that fetches feeds from the podcast host. It can therefore use this private deployment while the phone has Tailscale access; this is an inference from its documented direct-fetch behavior. Actual phone refresh/download/playback remains untested. [Subscription instructions](https://antennapod.org/documentation/getting-started/subscribe), [Distributed model](https://antennapod.org/documentation/general/central-distributed)
- Standards compatibility and network reachability are separate: a player must fetch through a device or server allowed into this installation's network. Other installations may choose different network exposure without changing the feed format.

## Feasibility and acceptance

Read-only checks on 2026-10-04 found Python, yt-dlp, yt-dlp-ejs, FFmpeg/FFprobe, and Deno installed. The host's yt-dlp is stale; use a current isolated/package version for implementation checks rather than treating installation as proof of working extraction.

Full YouTube support needs the external JS runtime and yt-dlp-ejs; FFmpeg/FFprobe support audio extraction. Public/unlisted scope avoids login support but does not guarantee requests from this server will succeed. Verify a real extraction before accepting the application, and record measured resource use. [yt-dlp README](https://github.com/yt-dlp/yt-dlp), [EJS guide](https://github.com/yt-dlp/yt-dlp/wiki/EJS), [PO-token guide](https://github.com/yt-dlp/yt-dlp/wiki/PO-Token-Guide)

Acceptance covers real extraction, durable restart recovery, deduplication, bounded retries, expiry, disk-full handling, RSS validity, media HEAD/range requests, authenticated bot submission, private routing, and measured RAM. Phone subscription/download/playback requires an actual AntennaPod test on the operator's device and must not be claimed from server checks alone.

## Next step

The confirmed design has been synthesized into the [local implementation spec](../.scratch/initial-service/spec.md), marked ready-for-agent. The operator confirmed the application-level command and RSS/media HTTP testing boundaries. Use to-tickets next, then implement, verify, privately deploy at `2pod.thealexferrari.com`, and publish the reusable project. No further confirmation of this design or these test boundaries is required.
