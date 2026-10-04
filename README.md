# 2pod: YouTube to podcast RSS

A small self-hosted Go service that turns selected public or unlisted YouTube videos into full-length MP3 episodes in a standard podcast feed. Submit links through the local CLI or an optional Telegram bot. One video is processed at a time; SQLite and audio stay on disk.

Active sources are deduplicated. Episodes expire 30 days after publication by default, independently of playback or downloads. Extraction failures have bounded retries and can be retried manually. Storage pressure pauses processing while preserving queued work and unexpired episodes. There is no dashboard.

## Install

Requires Docker Engine and Docker Compose on Linux. The image includes the compiled application, yt-dlp with yt-dlp-ejs, Python, FFmpeg/ffprobe, and Deno; no host Go or extraction tools are needed. The Dockerfile builds for the host architecture; Linux amd64 is the tested platform.

```sh
git clone https://github.com/alexferrari88/yt-to-rss.git
cd yt-to-rss
cp .env.example .env
chmod 600 .env
# Edit .env: set TWOPOD_BASE_URL to the URL your player will use.
docker compose config --quiet
docker compose build --pull
docker compose up -d
```

Compose publishes **no host port** by default. Configure HTTPS through a reverse proxy, or use the opt-in localhost mapping in [operations](docs/operations.md#hosting). Set the base URL before submitting episodes so enclosure URLs point to the correct host.

```sh
docker compose exec -T 2pod 2pod add 'https://www.youtube.com/watch?v=VIDEO_ID'
docker compose exec -T 2pod 2pod list
docker compose exec -T 2pod 2pod status
docker compose exec -T 2pod 2pod status VIDEO_ID
docker compose exec -T 2pod 2pod retry VIDEO_ID
docker compose exec -T 2pod 2pod delete VIDEO_ID
docker compose exec -T 2pod 2pod feed-url
```

Commands return JSON for scripting; `feed-url` prints the subscription URL. Treat that URL as a credential. Read access does not authorize management: CLI commands use a local Unix socket accessible to the service's operating-system user.

Subscribe to the returned URL in a player that can reach your chosen hosting network. A private Tailscale installation requires the fetching device to be connected to that tailnet. AntennaPod fetches feeds on the phone; a player that fetches through its own servers may not reach a private feed. The feed uses ordinary RSS 2.0 and MP3 enclosures.

An optional HTTP(S) extraction proxy can help when YouTube blocks the server's network. Set `TWOPOD_EXTRACTOR_PROXY` in your protected `.env`, then recreate the container. For example, `http://proxy.example:3128` is a placeholder for your own reachable proxy. Leave the setting empty to use the normal route. It applies only to extraction; Telegram and feed/media hosting keep their existing routes. See [configuration](docs/operations.md#configuration) for details.

See [operations](docs/operations.md) for settings, optional Telegram setup, persistent-state backup, updates, rollback, and troubleshooting. Private videos, login-dependent content, active livestreams, playlist imports, and trimming are outside the supported scope. Shared start timestamps and playlist parameters on an individual video are ignored.

## Development

Requires Go, a C compiler for SQLite, Python 3 for the controlled extractor fixture, and FFmpeg with `ffmpeg` and `ffprobe` available on `PATH`. Ordinary Go tests use a real MP3 fixture and verify its audio before publication. They replace external extraction/Telegram traffic at their integration boundaries and do not require live YouTube access.

```sh
go test ./...
go vet ./...
```

Rebuild the image from the current checkout before each package check; the driver runs the already-built image. This check requires host Python 3 and Docker Compose:

```sh
docker compose build --pull
python3 scripts/check-package.py
```

The driver checks packaged CLI/RSS/media behavior, container replacement, and stopped-state backup/restore against the known audio fixture. It creates isolated temporary state, publishes no host port, and removes its test resources when finished. It does not contact YouTube or Telegram.

See [the glossary](GLOSSARY.md) and [architecture decisions](docs/adr/) for the domain and access model. The project is [MIT licensed](LICENSE); bundled third-party tools retain their own licenses.
