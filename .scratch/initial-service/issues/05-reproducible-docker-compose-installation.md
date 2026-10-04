# 05: Reproducible Docker Compose installation

**What to build:** Give another operator a reproducible Docker Compose installation of the CLI-to-feed path with persistent state and explicit extraction dependencies. Keep hosting and configuration portable; later application tickets extend the same installation and its instructions as their features land.

**Blocked by:** 01 — CLI to a playable feed.

**Status:** resolved

**Spec coverage:** User stories 24, 41 and 42; installation, licensing and operation contribute to 43 and 44.

- [x] Compose configuration validates, and a fresh documented build/start can run CLI add/list/status and serve RSS/audio without relying on host-global Go or extraction tools.
- [x] The package contains the compiled Go application, maintained yt-dlp, FFmpeg, yt-dlp-ejs and a supported JavaScript runtime. yt-dlp's Python prerequisites run as needed for extraction; Go remains the persistent application service.
- [x] Tested dependency versions and an explicit update/rebuild procedure are documented. Verification uses the packaged dependencies rather than the stale downloader previously found on the host.
- [x] Writable application state and audio survive container replacement. Recreating the container preserves submissions, episode identities, playable media and configured read access.
- [x] Feed identity, external base URL, read secrets and bind/network settings are configurable. Installation works on another operator's host without Alex's domain or infrastructure; the package can join a reverse-proxy network without publishing an application host port.
- [x] CLI operation works without Telegram credentials. The configuration documentation is extended by the Telegram, recovery and resource tickets when their respective features land; those tickets are not prerequisites for packaging the core path.
- [x] Credentials, persistent databases, media and working outputs remain outside version control. Checked-in examples use placeholders, and documented diagnostics do not disclose secrets.
- [x] The project includes the MIT license and concise installation, configuration, startup, update and troubleshooting instructions for the implemented commands. Private-feed documentation explains that the subscribing player must be able to reach the configured hosting network.
- [x] Package acceptance verifies the CLI-to-feed/media path through the agreed public boundaries and container replacement with retained state. Ordinary automated checks control external YouTube traffic; real packaged extraction and memory measurement occur in the deployment ticket.

## Answer

The final reviewed image builds from pinned dependencies and passes scripts/check-package.py under rootless Docker. Public CLI/RSS/GET/HEAD/range checks verify the exact 4,510-byte MP3 fixture, persistent identity/read access after container replacement, and whole stopped-state backup restored into a fresh volume. The check publishes no host port and cleans only its isolated test project. README.md and docs/operations.md cover portable configuration, private network requirements, optional Telegram setup, updates and rollback.
