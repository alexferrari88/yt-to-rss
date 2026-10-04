# Personal YouTube-to-audio podcast feed

Status: ready-for-agent
Design: confirmed on 2026-10-04.
Testing boundaries: confirmed by the operator on 2026-10-04.

## Problem Statement

The operator wants to send selected YouTube videos from an Android phone and listen to their audio through a podcast player. They need a small personal service on their existing always-on server, with modest memory use and CLI management suitable for Codex. Their installation must remain accessible only through Tailscale, while the source project should be reusable by others through a public GitHub repository.

## Solution

A single Go service receives the operator's YouTube links through Telegram or a local CLI, persists submissions in SQLite, and processes one video at a time with yt-dlp and FFmpeg. A completed MP3 becomes an episode in a standard RSS feed. The bot reports queued, published, or failed work; the CLI supports submission, inspection, retry, and deletion.

Episodes expire 30 days after publication. Existing Traefik serves the personal installation at `2pod.thealexferrari.com`, using DNS-only Cloudflare records, trusted HTTPS, and the existing private Tailscale ingress. AntennaPod is the example acceptance player; the feed format remains independent of any player. Other operators can configure their own base URL and network access. Publish the reusable project as `alexferrari88/yt-to-rss` under MIT with Docker Compose installation.

## User Stories

1. As the operator, I want to share a YouTube link from Android to my Telegram bot, so that adding an episode takes little effort.
2. As the operator, I want to submit a link through the CLI, so that Codex or a shell can add episodes without a phone.
3. As the operator, I want only my configured Telegram identity to submit work, so that other people cannot consume my server's resources through the bot.
4. As the operator, I want a queued acknowledgement after a submission is durably accepted, so that I know the link will survive a service restart.
5. As the operator, I want a publication reply when an episode is available in the feed, so that I know processing has finished.
6. As the operator, I want a concise failure reply with an actionable reason, so that I can decide whether to retry or choose another video.
7. As the operator, I want public and unlisted videos supported without storing a YouTube login, so that the initial service stays simple to maintain.
8. As the operator, I want malformed links and unsupported sources rejected clearly, so that invalid input does not create unusable episodes or stall other work.
9. As the operator, I want ordinary YouTube link variants normalized to the same source, so that sharing a shortened link does not create a duplicate of the watch-page link.
10. As the operator, I want the full audio as an MP3, so that I can listen in ordinary podcast players without the video.
11. As the operator, I want shared start timestamps ignored, so that each episode contains the complete video audio.
12. As the operator, I want a video link that includes a playlist parameter to refer to that video alone, so that sharing from a playlist does not import unrelated videos.
13. As the operator, I want source title, uploader, duration, and source link reflected in episode metadata when available, so that I can identify what I saved.
14. As the operator, I want only complete, playable audio published, so that I do not receive episodes pointing to partial downloads.
15. As the operator, I want newly published episodes dated by publication time, so that an old YouTube upload can appear as a newly added episode.
16. As the operator, I want stable episode identity, so that refreshing a feed or restarting the service does not create duplicate episodes in my player.
17. As the operator, I want repeated submissions of an active source to return its existing status, so that repeated sharing does not cause redundant downloads.
18. As the operator, I want to submit an expired or deleted source again, so that I can restore its audio without inventing a new identity merely to force player refresh.
19. As the operator, I want to subscribe by a standard RSS URL, so that the service works with players able to reach the feed rather than a proprietary player integration.
20. As the operator, I want to stream, download, seek, and resume audio through normal HTTP requests, so that the feed behaves like an ordinary podcast.
21. As the operator, I want feed and audio available over trusted HTTPS, so that my player can connect without certificate exceptions.
22. As the operator, I want this installation reachable only within my Tailscale network, so that knowing its hostname or a secret URL does not grant internet access.
23. As the operator, I want secret read URLs separated from management privileges, so that sharing a feed URL cannot authorize submissions or deletion.
24. As the operator, I want accepted submissions and published episodes retained across restarts, so that restarting or updating the service does not lose my work.
25. As the operator, I want interrupted processing recovered without duplicate publication, so that a crash does not require manual database repair.
26. As the operator, I want only one extraction running at a time, so that processing load stays predictable.
27. As the operator, I want audio processed and served from disk without loading complete files into memory, so that long videos do not require correspondingly large RAM.
28. As the operator, I want bounded automatic retries for transient failures, so that brief problems can recover without an endless retry loop.
29. As the operator, I want explicit retry of a failed submission through the CLI, so that I can recover after fixing a dependency or provider problem.
30. As the operator, I want to list submissions and inspect their current status through the CLI, so that Codex can diagnose queues and failures.
31. As the operator, I want to delete an episode or pending submission through the CLI, so that I can remove unwanted work and reclaim storage.
32. As the operator, I want deletion during processing to prevent later publication, so that a worker cannot restore something I deliberately removed.
33. As the operator, I want automatic expiry 30 days after successful publication, so that storage does not require constant manual cleanup.
34. As the operator, I want expiry to remove the feed item and server audio together, so that the service stops advertising an unavailable episode.
35. As the operator, I want cleanup independent of download requests or listening state, so that streaming or partial downloads do not cause premature deletion.
36. As the operator, I want processing to pause when the configured storage limit is reached, so that the service does not silently discard unexpired episodes or fill the host disk.
37. As the operator, I want queued work to remain inspectable while storage is full and resume when space becomes available, so that storage pressure does not lose submissions.
38. As the operator, I want retention and resource settings configurable, so that I can adjust them without changing the application code.
39. As the operator, I want normal logs and error replies to omit credentials and secret read URLs, so that routine diagnostics do not expose access secrets.
40. As the operator, I want a small persistent Go process and reported idle/processing memory measurements, so that I can assess the actual resource cost.
41. As a self-hosting user, I want a documented Docker Compose installation with persistent state and explicit dependencies, so that I can reproduce the service on my own host.
42. As a self-hosting user, I want feed identity, base URL, operator credentials, and hosting access configurable, so that Alex's domain and Tailscale setup are not required by the application.
43. As a self-hosting user, I want the source and MIT license in the public repository, so that I can use, inspect, and adapt my own installation.
44. As the operator, I want startup, update, retry, cleanup, and troubleshooting instructions, so that Codex or I can operate the service without a dashboard.

## Implementation Decisions

### Application and interfaces

- Use one Go service and CLI. Go coordinates submission, state, processing, publication, retention, and management. SQLite provides durable local state; no separate queue service is required.
- Provide operator commands for add, list/status, retry, and delete. Return clear identifiers, statuses, failures, and exit outcomes suitable for scripting; management requires local operator access rather than feed credentials.
- Use Telegram outbound polling for phone submission and queued/published/failed replies. Authorize the configured operator before accepting work. No inbound webhook or internet-accessible management API is required.
- Keep HTTP read access separate from commands: serve the RSS feed and MP3s through unguessable read URLs, with no management permission attached to those URLs.
- Configure feed identity, the externally visible base URL, operator/bot credentials, retention, storage limits, and bounded processing/retry settings. Other installations can choose different network access without changing the feed format.

### Submission and episode lifecycle

- Recognize individual public/unlisted YouTube videos, normalize supported URL variants to a canonical video identity, and discard timestamps/tracking/playlist context. Reject playlist-only URLs, unsupported origins, invalid input, login-dependent content, and active livestreams with a clear outcome.
- Persist an accepted submission before acknowledging it. Keep enough state to inspect its source, identifier, queue/processing/publication/failure outcome, retry attempts, timestamps, and sanitized failure reason.
- Deduplicate queued, processing, and published sources across CLI and Telegram. Returning an existing submission/episode does not start another extraction or create a new feed identity.
- A single worker processes queued submissions. Run yt-dlp/FFmpeg with explicit arguments and cancellation rather than interpreting submitted text as shell code.
- Recover interrupted work safely after restart. A publication must not be visible until complete audio and its metadata are available; restart must reconcile unfinished work without offering partial files or creating duplicate episodes.
- Manual deletion prevents pending or running work from publishing afterwards and removes only artifacts belonging to that submission/episode. Repeating deletion is safe.
- Expired/deleted sources may be submitted again using their stable episode identity. Players control whether a restored identity is presented as new; do not promise forced rediscovery.

### Extraction, RSS, and media

- Invoke maintained yt-dlp and FFmpeg for full-video MP3 extraction. Bundle the tested YouTube prerequisites, including yt-dlp-ejs and a supported JavaScript runtime. yt-dlp remains Python-based but runs on demand; there is no persistent Python application service.
- Use current isolated/package dependencies for verification. Document the dependency versions/update procedure rather than relying on the stale downloader found on the host.
- Store audio and intermediate work on disk; do not buffer whole episodes in RAM. Keep one extraction active and bound processing and retry behavior.
- Generate RSS 2.0 with feed title/description/link, stable episode GUIDs, publication dates, source metadata, and MP3 enclosures containing the correct URL, byte length, and MIME type. Encode source text safely so metadata cannot break XML.
- Feed entries reference completed audio at the configured base URL. Use publication time rather than the original YouTube upload date for the episode date.
- Serve MP3s with correct content length/type, HTTP HEAD, and byte-range behavior for download, seeking, and resumption. Secret access controls apply consistently to full and partial requests.
- Player polling, refresh timing, caching, downloaded copies, and listening state remain outside the service's control. Publication means available in the service's feed, not already visible in a particular player.

### Retention and resource handling

- Default retention is 30 days from successful publication, configurable by the operator. Queuing, retry, or HTTP access does not start or extend that period.
- On expiry, withdraw the episode from the feed and remove its server-side audio safely. Do not use HTTP download requests as evidence of a saved offline copy, and do not attempt to revoke copies already stored by a player.
- Account for working files as well as completed audio when enforcing storage limits. Pause admission to processing when the budget/free-space checks fail; preserve queued work and unexpired episodes.
- Expose a meaningful storage-paused reason to the operator. Resume eligible queued work once space is available or limits are adjusted.
- Clean up failed/interrupted extraction artifacts without deleting another episode's files. Resource settings beyond the accepted retention/concurrency values must have documented defaults; no exact usage volume or RAM ceiling has been agreed.

### Deployment and release

- Package the service and extraction prerequisites with Docker Compose; keep writable state persistent across container replacement and keep credentials/generated media outside version control.
- For Alex's installation, use `2pod.thealexferrari.com`, a DNS-only Cloudflare record to the current Tailscale address, existing private Tailscale ingress, and the existing Traefik HTTPS router/certificate resolver convention.
- Use the private Docker proxy network without publishing an application host port. Preserve existing private Traefik/Tailscale routes and their boot behavior. Do not enable Cloudflare proxying, a public listener, Funnel, or Tunnel.
- Inspect the exact Cloudflare hostname record before a scoped DNS change; DNS non-resolution alone does not establish zone-record absence.
- Publish the source as `alexferrari88/yt-to-rss` under MIT, with portable setup, configuration, operator commands, dependency/update instructions, and clear private-player connectivity requirements.

## Testing Decisions

### Confirmed public seams

- **Command boundary:** exercise submission/management through the public CLI and Telegram input adapter, with outcomes observed through public status/command results and feed/media responses.
- **HTTP read boundary:** exercise the RSS and MP3 endpoints through HTTP, including authenticated read access, HEAD, and ranges.
- Keep the test surface at these application boundaries. Do not add separate test seams for database rows, private worker helpers, or internal collaborator calls. The operator explicitly confirmed these boundaries under the to-spec skill; do not request the same confirmation again.

### Automated acceptance

- A good test verifies an observable requirement with independently chosen expected results. It should survive changes to database layout or internal module structure.
- Use temporary persistent state and controlled external YouTube/Telegram traffic at their integration boundaries. Do not rely on live third-party availability in the ordinary test suite or mock the application's own internal helpers.
- Drive valid/invalid source submission, operator authorization, canonical-source deduplication, publication metadata, and operator command results through the command boundary.
- Exercise restart using the same persistent state and observe preserved submissions/episodes, recovered interrupted work, and absence of duplicate or partial publication.
- Exercise bounded retry/final failure, manual retry, and deletion during queued/running/published states. Observe status and feed/media results rather than reading SQLite directly.
- Exercise expiry with controlled elapsed time/configuration and storage pressure with constrained storage conditions. Observe withdrawn entries, unavailable expired media, preserved unexpired episodes, paused queue status, and resumed processing.
- Independently parse generated RSS and check required metadata, safe text encoding, stable GUIDs, publication dates, and enclosure byte lengths/types against known fixtures.
- Use a known audio fixture to verify GET, HEAD, byte ranges, inaccessible/missing media, and access-token behavior. Compare exact bytes and independent expected headers/statuses; do not derive expectations by copying production logic.
- There is no application code or test suite in the repository yet, so no project test prior art exists. Go's standard testing/HTTP test facilities can drive these public boundaries without introducing a separate test framework.

### Live acceptance

- Verify a real YouTube audio extraction from this server using the packaged/current dependencies before accepting the service as working. Document any provider/network failure and its actual resolution.
- Measure idle Go-service RAM and peak memory during real processing, including extraction subprocesses/container workload. Report workload and measurement scope; make no unmeasured RAM claim.
- Verify the deployed private DNS/TLS/Traefik path, expected Docker/listener exposure, preserved existing routes, and inaccessible feed/audio from outside the tailnet.
- Verify Telegram submission/replies with the configured operator once credentials are available. Never claim delivery merely because a request was attempted.
- On the operator's Android phone with Tailscale connected, add the RSS URL in AntennaPod and verify refresh, download, playback, and seeking. Device acceptance requires a real phone check and must be reported separately from server/HTTP verification.

## Out of Scope

- Multi-user accounts, shared hosted service operation, dashboards, and a separate management HTTP API.
- Automatic channel subscriptions, playlist imports, trimming, timestamp clips, active livestreams, and login-dependent/private/members-only video access.
- Player-specific feed formats/APIs, podcast-directory submission, synchronization of listening state, and delete-after-download automation.
- Revocation of audio already downloaded to a podcast player.
- Distributed workers, external queue infrastructure, browser-based extraction infrastructure, or automatic proxy/cookie/account management.
- Public exposure of Alex's installation through listeners, Cloudflare proxy/CDN, Funnel, or Tunnel. Public source-code publication does not make the personal feed public.
- Invented throughput promises, fixed weekly volume, or an unmeasured memory guarantee.

## Further Notes

- The operator confirmed the design on 2026-10-04. Do not reopen Go, hostname, player choice, private access, retention, or release authorization while translating it into tickets.
- Authoritative design/domain sources are the confirmed design, glossary, and ADR-0001 (secret read URLs), ADR-0002 (Tailscale-only personal deployment), and ADR-0003 (Go with external extraction).
- Previous read-only checks established installed toolchains/extraction prerequisites and reusable private routing, but no application, Git repository, DNS route, live extraction, or phone playback has been created or verified by this spec task. Recheck mutable host/zone state during implementation.
- The selected hostname previously returned NXDOMAIN; that is not proof of Cloudflare-zone absence. Scope future deployment changes to this service and preserve unrelated service state.
- This spec is intended to become the local tracker source for a few complete vertical implementation tickets. Producing those tickets, implementing code, deploying, and publishing GitHub are subsequent stages, not actions performed by to-spec.
