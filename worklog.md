# Worklog

## Current goal

Implement and publish the confirmed minimal personal YouTube-to-audio RSS service. The user approved the six-ticket breakdown and blocking edges; to-tickets is complete, with one ready-for-agent file per ticket published to the local tracker.

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
- Planning/domain/worklog docs and six local tickets have been created or updated. No application, Git repo, DNS record, service change, or live extraction has been created.
- Read-only checks confirm Go/Rust toolchains are installed and Go's standard library covers HTTP media, XML, and subprocess needs. Candidate names currently do not resolve; check Cloudflare records before creating the chosen one. No Go/Rust RAM comparison has been measured.
- A/AAAA/CNAME queries for the selected 2pod.thealexferrari.com return NXDOMAIN. Exact Cloudflare-zone state still needs checking before any record mutation.
- Canonical implementation spec: .scratch/initial-service/spec.md, Status: ready-for-agent. It contains 44 user stories, implementation/testing decisions, accepted scope, and separate live acceptance requirements.
- Six tickets are published under .scratch/initial-service/issues/, all ready-for-agent, with acceptance criteria and the approved blockers. The parent spec remains unchanged.

## Approved tickets

1. 01-cli-to-a-playable-feed.md — no blockers.
2. 02-recovery-retries-and-deletion.md — blocked by 01.
3. 03-telegram-submission-and-replies.md — blocked by 01.
4. 04-expiry-and-storage-limits.md — blocked by 02.
5. 05-reproducible-docker-compose-installation.md — blocked by 01.
6. 06-private-deployment-and-public-release.md — blocked by 03, 04 and 05; 02 is required through 04.

## Next action

Use implement on the published tickets, starting with 01, through the already confirmed TDD boundaries and the repository's code-review workflow. Tickets 02, 03 and 05 become eligible after 01; 04 after 02; 06 after 03, 04 and 05. The to-tickets stage has not started application implementation, infrastructure changes or GitHub publication.
