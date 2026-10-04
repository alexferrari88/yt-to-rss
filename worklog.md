# Worklog

## Current goal

Implement, verify, privately deploy and publish the approved service. The user invoked implement and explicitly requested gpt-6.1-sol subagents at high reasoning effort. Implementation, public release and the authorized configurable extraction-proxy activation are complete. The source-specific YouTube block is resolved and publication is verified; finish operator-dependent Telegram/device/network acceptance.

## Settled

- Android, Telegram submission/status replies, this always-on server, public/unlisted sources, low unspecified volume, CLI management, no dashboard.
- Standard podcast RSS; no particular player dependency. User explicitly says Pocket Casts is unnecessary and accepts AntennaPod for private-feed use.
- Full-video MP3, active-source deduplication, bounded retries plus CLI retry, one conversion at a time, automatic expiry 30 days after publication, pause when storage is full, secret read URLs.
- Tailscale-only installation using existing Traefik and DNS-only Cloudflare subdomain under thealexferrari.com. No public Funnel/Tunnel/listener. ADR-0001 and ADR-0002 record access decisions.
- Go service/CLI and hostname 2pod.thealexferrari.com. ADR-0003 records Go with on-demand yt-dlp/FFmpeg extraction; SQLite provides durable state.
- Public alexferrari88/yt-to-rss repository, MIT, Docker Compose. The repository is published.
- Optional HTTP(S) extraction proxy, empty by default for portable installations. The operator explicitly authorized using the existing Beelink proxy for the personal deployment; its value remains only in protected private settings. Telegram and feed/media hosting keep their existing routes.
- The complete design is confirmed; do not ask for the same design or implementation authorization again.
- The operator explicitly confirmed application-level testing through CLI/bot commands and RSS/media HTTP, replacing only external YouTube/Telegram traffic in automated tests. Do not ask for those boundaries again.
- The six-ticket breakdown and dependency structure are approved. Do not request approval of the same breakdown again.

## Last verified facts

- Main contains baseline 7c66bfa, implementation eb9291e and reviewed fixes 2ae45c8. Independent standards/spec findings are resolved; the full acceptance suite passed in 54.967s with both daemon and tests race-instrumented, and go vet ./... passed.
- Tickets 01–05 are resolved. The final rootless image passed exact CLI/RSS/media fixture checks, container replacement, generated-token persistence and whole stopped-state backup/restore; its isolated resources were cleaned.
- Personal service is healthy under /opt/services/yt-to-rss, source revision e391521eadf5832ea9cd8e7a0d32fef98cd3c3f5 (label e391521), UID10001, persistent 2pod-state, one CPU/512 MiB ceiling, no host ports, existing private Traefik. Approved DNS-only hostname resolves to the Tailscale address; trusted HTTPS, secret rejection and GET/HEAD/ranges pass through it.
- All existing Serve routes were preserved; Traefik remains loopback-bound and port 443 tailnet-bound. Rootless Docker/user lingering are active. Hister/Taste private HTTPS remain healthy. Scoped infrastructure/configuration commit 5a31272 and proxy runbook follow-up 103c721 are saved locally in /opt/services; unrelated WIP is preserved and the services repository was not pushed.
- Packaged real 213s public YouTube conversion published first attempt in 9.61s; complete 3,744,644-byte MP3 downloaded/decoded independently without errors. Idle Go RSS 11.5 MiB, postconversion 13.5 MiB, peak total container 364.2 MiB including extractor tools/probes/CLI sampling/page cache. Earlier isolated short-video source hit a provider bot check; no cookie/proxy change was needed for the successful source.
- Public MIT repository created/pushed at https://github.com/alexferrari88/yt-to-rss. Source and final evidence/status documentation are committed and pushed; local/tracking/remote main parity was verified. The source checkout is clean and runtime source matches the final published code.
- Operator completed the protected Telegram helper. Official bot identity is valid and no webhook conflicts with polling. Recreated only 2pod with its new credentials; healthy, generated feed URL and published audio retained, private HTTPS and all Serve routes preserved. No secrets are stored in tracked docs; subscription is in protected /opt/services/yt-to-rss/subscription.txt.

- A new real operator submission was accepted through Telegram, then reached final failure after three direct attempts. Diagnosis isolated a source-specific YouTube bot challenge on the server route; the same public/non-live source worked through the existing residential proxy. No cookies or account access were used.
- Optional child-only extraction proxy setting e391521 and the sanitized bot-challenge diagnostic passed the full daemon/test race suite in 17.205s, vet, both review axes with zero findings, and rebuilt package replacement/backup/restore acceptance. The operator explicitly approved activation and portable configuration. Saved a complete stopped-state/settings backup outside both repositories and rebuilt/retained the earlier source revision as a rollback image because Docker no longer held the old image. Recreated only 2pod with the reviewed prepared image and protected proxy setting; subscription, submissions, prior media, Telegram settings and Serve routes were preserved.
- The configured application's actual CLI retry published the failed bot source on its first attempt in 57.03s. Trusted private HTTPS RSS metadata/GUID, full 40,237,820-byte MP3, HEAD/exact 206 ranges and wrong-secret 404 passed. SHA-256 8303d7e146ed173ed40702ebccd3c28d4c826e51305cbbf03fbdae56265b4a2e; decoded duration 3,660.504s and independent full FFmpeg decode passed. Idle Go RSS with Telegram 16.7 MiB; immediately after publication 24.6 MiB; reset cgroup peak 236.9 MiB including tools, CLI sampling and page cache. Private detailed receipt is in verification.private.json; rollback/state backup is outside Git.
- Actual queued/failure/publication reply confirmation, Android playback and independent outside-tailnet access remain pending. Ticket 06 is still needs-info.

## Approved tickets

1. 01-cli-to-a-playable-feed.md — no blockers.
2. 02-recovery-retries-and-deletion.md — blocked by 01.
3. 03-telegram-submission-and-replies.md — blocked by 01.
4. 04-expiry-and-storage-limits.md — blocked by 02.
5. 05-reproducible-docker-compose-installation.md — blocked by 01.
6. 06-private-deployment-and-public-release.md — blocked by 03, 04 and 05; 02 is required through 04.

## Next action

Await operator confirmation of actual bot replies, Android feed refresh/download/play/seek and fresh retrieval failure with Tailscale disconnected. Two concise live-acceptance questions are open. Resharing the now-published source should produce an immediate Published reply without a duplicate; a CLI retry after terminal bot failure does not itself reattach the publication notification. Keep ticket 06 needs-info until those results arrive. Server implementation, configurable proxy, activation and publication checks are complete; preserve unrelated service WIP and do not push /opt/services.
