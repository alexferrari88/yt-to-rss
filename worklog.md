# Worklog

## Current goal

Implement, verify, privately deploy and publish the approved service. The user invoked implement and explicitly requested gpt-6.1-sol subagents at high reasoning effort. Core implementation and release are complete. Finish real Telegram/device acceptance and resolve the source-specific direct YouTube route block without adding unnecessary infrastructure.

## Settled

- Android, Telegram submission/status replies, this always-on server, public/unlisted sources, low unspecified volume, CLI management, no dashboard.
- Standard podcast RSS; no particular player dependency. User explicitly says Pocket Casts is unnecessary and accepts AntennaPod for private-feed use.
- Full-video MP3, active-source deduplication, bounded retries plus CLI retry, one conversion at a time, automatic expiry 30 days after publication, pause when storage is full, secret read URLs.
- Tailscale-only installation using existing Traefik and DNS-only Cloudflare subdomain under thealexferrari.com. No public Funnel/Tunnel/listener. ADR-0001 and ADR-0002 record access decisions.
- Go service/CLI and hostname 2pod.thealexferrari.com. ADR-0003 records Go with on-demand yt-dlp/FFmpeg extraction; SQLite provides durable state.
- Public alexferrari88/yt-to-rss repository, MIT, Docker Compose. The repository is published.
- The complete design is confirmed; do not ask for the same design or implementation authorization again.
- The operator explicitly confirmed application-level testing through CLI/bot commands and RSS/media HTTP, replacing only external YouTube/Telegram traffic in automated tests. Do not ask for those boundaries again.
- The six-ticket breakdown and dependency structure are approved. Do not request approval of the same breakdown again.

## Last verified facts

- Main contains baseline 7c66bfa, implementation eb9291e and reviewed fixes 2ae45c8. Independent standards/spec findings are resolved; the full acceptance suite passed in 54.967s with both daemon and tests race-instrumented, and go vet ./... passed.
- Tickets 01–05 are resolved. The final rootless image passed exact CLI/RSS/media fixture checks, container replacement, generated-token persistence and whole stopped-state backup/restore; its isolated resources were cleaned.
- Personal service is healthy under /opt/services/yt-to-rss, source revision 2ae45c8880b3ee81fd7b055f069f496917ee14a0, UID10001, persistent 2pod-state, one CPU/512 MiB ceiling, no host ports, existing private Traefik. Approved DNS-only hostname resolves to the Tailscale address; trusted HTTPS, secret rejection and GET/HEAD/ranges pass through it.
- All existing Serve routes were preserved; Traefik remains loopback-bound and port 443 tailnet-bound. Rootless Docker/user lingering are active. Hister/Taste private HTTPS remain healthy. Scoped infrastructure/configuration commit 5a31272 is saved locally in /opt/services; unrelated WIP is preserved and the services repository was not pushed.
- Packaged real 213s public YouTube conversion published first attempt in 9.61s; complete 3,744,644-byte MP3 downloaded/decoded independently without errors. Idle Go RSS 11.5 MiB, postconversion 13.5 MiB, peak total container 364.2 MiB including extractor tools/probes/CLI sampling/page cache. Earlier isolated short-video source hit a provider bot check; no cookie/proxy change was needed for the successful source.
- Public MIT repository created/pushed at https://github.com/alexferrari88/yt-to-rss. Source and final evidence/status documentation are committed and pushed; local/tracking/remote main parity was verified. The source checkout is clean and runtime source matches the final published code.
- Operator completed the protected Telegram helper. Official bot identity is valid and no webhook conflicts with polling. Recreated only 2pod with its new credentials; healthy, generated feed URL and published audio retained, private HTTPS and all Serve routes preserved. No secrets are stored in tracked docs; subscription is in protected /opt/services/yt-to-rss/subscription.txt.

- A new real operator submission was accepted through Telegram, then reached final failure after three attempts. The same-source isolated metadata probe reproduces YouTube bot-signin on the server direct route in 2.16s; known public control still succeeds, three alternate clients fail, and pinned nightly is current/master identical. The same source is public/non-live and succeeds through the existing private Beelink proxy in 3.33s. No production extraction route changed.
- Committed optional child-only extraction proxy setting e391521 (default disabled) and fixed sanitized bot-challenge diagnostic. Full daemon/test race suite passed in 17.205s, vet passed, both review axes found 0 issues, and rebuilt package replacement/backup/restore acceptance passed. Isolated full same-source conversion through the existing proxy passed: 40,237,820-byte MP3, decoded 3,660.504s, full decode clean, 63.33s elapsed; temporary container removed. Enabling it would add Beelink as an availability dependency; An explicit routing-choice question is pending after presenting verified conversion and gates; do not activate the proxy until answered.
- Actual queued/failure reply confirmation, Android playback and independent outside-tailnet access remain pending. An operator reply question is open; ticket 06 is still needs-info.

## Approved tickets

1. 01-cli-to-a-playable-feed.md — no blockers.
2. 02-recovery-retries-and-deletion.md — blocked by 01.
3. 03-telegram-submission-and-replies.md — blocked by 01.
4. 04-expiry-and-storage-limits.md — blocked by 02.
5. 05-reproducible-docker-compose-installation.md — blocked by 01.
6. 06-private-deployment-and-public-release.md — blocked by 03, 04 and 05; 02 is required through 04.

## Next action

Await the operator's explicit Beelink route choice (all implementation/review/package/full-conversion preparation passed). If selected, set the protected extraction proxy, deploy only 2pod using the reviewed image, retry the failed source through CLI and verify final feed/media. Confirm actual bot replies, then Android feed refresh/download/play/seek and the external negative access check. The verified code follow-up e391521 and review/evidence docs are published on main with local/remote parity checked; keep operator-dependent ticket 06 open. The prepared image b7a8ff1cff114164352429efd66bec2b9bd91d0da139fa5db64c70ef8900a390 has revision label e391521. The current live container remains 2ae45c8 with enabled Telegram and direct extraction until the routing choice arrives.
