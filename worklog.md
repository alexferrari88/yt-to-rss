# Worklog

## Current goal

Implement, verify, privately deploy and publish the approved service. The user invoked implement and explicitly requested gpt-6.1-sol subagents at high reasoning effort. Implementation is underway on main from baseline 7c66bfab33e627b5d812491eebffdd38718e2dbe.

## Settled

- Android, Telegram submission/status replies, this always-on server, public/unlisted sources, low unspecified volume, CLI management, no dashboard.
- Standard podcast RSS; no particular player dependency. User explicitly says Pocket Casts is unnecessary and accepts AntennaPod for private-feed use.
- Full-video MP3, active-source deduplication, bounded retries plus CLI retry, one conversion at a time, automatic expiry 30 days after publication, pause when storage is full, secret read URLs.
- Tailscale-only installation using existing Traefik and DNS-only Cloudflare subdomain under thealexferrari.com. No public Funnel/Tunnel/listener. ADR-0001 and ADR-0002 record access decisions.
- Go service/CLI and hostname 2pod.thealexferrari.com. ADR-0003 records Go with on-demand yt-dlp/FFmpeg extraction; SQLite is the proposed durable-state implementation.
- Public alexferrari88/yt-to-rss repository, MIT, Docker Compose. Read-only GitHub checks found no matching accessible repo yet.
- The complete design is confirmed; do not ask for the same design or implementation authorization again.
- The operator explicitly confirmed application-level testing through CLI/bot commands and RSS/media HTTP, replacing only external YouTube/Telegram traffic in automated tests. Do not ask for those boundaries again.
- The six-ticket breakdown and dependency structure are approved. Do not request approval of the same breakdown again.

## Last verified facts

- AntennaPod's official docs establish direct feed fetching; phone playback is untested.
- Existing private Traefik/DNS/TLS convention is reusable. Extraction prerequisites exist, but a current yt-dlp must be tested and RAM measured.
- Git is initialized on main with the confirmed planning baseline 7c66bfa and implementation snapshot eb9291e committed. Independent code review is complete; two validated P2 fixes are resolved and the final application gates pass.
- Canonical implementation spec: .scratch/initial-service/spec.md, Status: ready-for-agent. It contains 44 user stories, implementation/testing decisions, accepted scope, and separate live acceptance requirements.
- Six tickets are published under .scratch/initial-service/issues/, all ready-for-agent, with acceptance criteria and the approved blockers. The parent spec remains unchanged.
- Rootless Docker and private proxy/Serve routes are available; the GitHub repository does not yet exist. Preserve unrelated dirty work in the services checkout.
- Current nightly yt-dlp/EJS/Deno successfully extracted the complete 213-second official public Rick Astley video without cookies or a proxy; decoded MP3 verification passed. The earlier short source failed with a source-specific provider bot check. Application/container extraction and processing-memory measurements remain to be verified.
- Dedicated Telegram credentials/operator configuration are absent. A setup preference question is pending; tokens must be entered into protected server configuration, not chat. Android playback remains a required operator/device check.
- Application implementation is stable: 14 core CLI/HTTP tests and four Telegram boundary tests pass; typechecking/vet passes. Packaged CLI/HTTP, container replacement and stopped-volume backup/restore checks passed using a controlled external extractor fixture. Linux-specific process/file limits and possible brief aggregate-budget overshoot are documented.
- Created the approved DNS-only A record for 2pod.thealexferrari.com to the Tailscale address. Prepared /opt/services/yt-to-rss private Compose/runbook and scoped inventory commands; Compose validates, protected .env is ignored, no container or public listener activated yet. Unrelated services WIP is preserved.
- Secure setup helper is ready: scripts/configure-telegram.py accepts a hidden token and nonce private-message identity check into /opt/services/yt-to-rss/.env. The operator has been asked to run it and reply ready; continue independent work meanwhile.
- Independent standards/spec review findings are resolved: inherited application settings are stripped from tests, decoded MP3 completeness is validated, and availability logic is shared. The full suite passed in 54.967s with both daemon and tests race-instrumented; go vet ./... passed. Core/Telegram/resource tickets 01–04 are resolved; live bot acceptance remains in ticket 06.

## Approved tickets

1. 01-cli-to-a-playable-feed.md — no blockers.
2. 02-recovery-retries-and-deletion.md — blocked by 01.
3. 03-telegram-submission-and-replies.md — blocked by 01.
4. 04-expiry-and-storage-limits.md — blocked by 02.
5. 05-reproducible-docker-compose-installation.md — blocked by 01.
6. 06-private-deployment-and-public-release.md — blocked by 03, 04 and 05; 02 is required through 04.

## Next action

Build the reviewed revision and repeat package replacement/backup/restore acceptance. Activate only the new private service, measure real processing and publish the source repository. Telegram setup and Android playback remain explicit operator-dependent acceptance; continue all independently possible work meanwhile.
