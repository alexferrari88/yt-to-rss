# Operating 2pod

## Configuration

Compose reads `.env` next to `compose.yaml`, or the file selected by `docker compose --env-file /private/path/.env`. Keep credentials outside version control and restrict that file to its owner. Commands below assume the normal Compose invocation; use the same project name, env file and overrides consistently for a separate installation.

| Setting | Default | Purpose |
| --- | --- | --- |
| `TWOPOD_BASE_URL` | `http://localhost:8080` in Compose | External feed/media URL; use your trusted HTTPS origin in production. |
| `TWOPOD_FEED_TITLE` | `My YouTube audio` | Feed title. |
| `TWOPOD_FEED_DESCRIPTION` | `Selected YouTube videos as audio episodes` | Feed description. |
| `TWOPOD_FEED_LINK` | Base URL when empty | Feed website link. |
| `TWOPOD_READ_TOKEN` | Generated and persisted | At least 32 characters if set. Generate a value with `openssl rand -hex 32`; never use a shared example token. |
| `TWOPOD_RETENTION` | `720h` | Expiry after publication: 30 days. |
| `TWOPOD_STORAGE_LIMIT_BYTES` | `10737418240` | 10 GiB budget including working files. |
| `TWOPOD_MIN_FREE_BYTES` | `536870912` | 512 MiB filesystem free-space reserve. |
| `TWOPOD_PROCESS_TIMEOUT` | `2h` | Per-extraction time bound. |
| `TWOPOD_MAX_ATTEMPTS` | `3` | Bounded automatic extraction attempts. |
| `TWOPOD_RETRY_DELAY` | `1m` | Delay between automatic attempts. |
| `TWOPOD_POLL_INTERVAL` | `1s` | Worker polling interval. |
| `TWOPOD_EXTRACTOR_PROXY` | Empty | Optional HTTP(S) outbound proxy for extraction subprocesses only. |
| `TWOPOD_TELEGRAM_BOT_TOKEN` | Empty | Optional bot credential. |
| `TWOPOD_TELEGRAM_OPERATOR_ID` | Empty | Numeric Telegram user ID permitted to submit. |
| `TWOPOD_IMAGE` | `yt-to-rss:local` | Image name, also used to select a saved rollback image. |

Durations use Go notation such as `30m`, `2h`, or `720h`, not `30d`. Recreate the container after changing settings. The application also accepts `TWOPOD_STATE_DIR` (default `./data`) and `TWOPOD_LISTEN` (default `127.0.0.1:8080`) when run directly; Compose fixes these to `/data` and the container-only `0.0.0.0:8080`. `TWOPOD_EXTRACTOR` and `TWOPOD_FFMPEG_LOCATION` select alternate extractor/FFmpeg locations for controlled testing or custom installations. Keep `ffprobe` on `PATH`: the application decodes/counts MP3 frames and checks duration against the header and available source metadata before publication. The image bundles both FFmpeg executables.

Storage accounting includes published media, metadata and extraction working files. The aggregate budget and free-space reserve are checked during processing at `TWOPOD_POLL_INTERVAL` (default one second). Multiple working files can grow between checks and briefly exceed the aggregate budget; these settings are polling thresholds rather than a filesystem quota or an exact total-byte cap. Linux child processes have a per-file size cap based on the available budget. Keep enough filesystem headroom for growth between checks when choosing the reserve.

When a storage check fails, processing is cancelled and the submission remains queued with a `PausedReason`; a storage pause does not consume a retry attempt. Delete unwanted episodes, allow expiry, or adjust limits to resume processing.

If YouTube challenges the server's network for a public video, an existing HTTP(S) outbound proxy can be configured with `TWOPOD_EXTRACTOR_PROXY`. For example, `http://proxy.example:3128` is a placeholder for an operator-owned proxy. Leave the setting empty for the normal route. The proxy must be reachable from the container. A configured proxy becomes an extraction availability dependency; it does not change where RSS/media are hosted or route Telegram through that proxy. Proxy URLs, including optional credentials, stay in the protected environment file and extraction child environment rather than command arguments or diagnostics. The application does not install a proxy or import browser cookies.

Leave both Telegram fields empty to operate entirely through the CLI. To enable phone submissions, create a bot through Telegram's BotFather, store its token privately, and set your positive numeric user ID. Both values are required together; partial or invalid settings refuse startup. The bot uses outbound polling; it needs no inbound webhook or management HTTP port. Only a private chat from the configured identity can submit links; other senders and group chats are ignored.

Send `/start` or `/help` to see instructions, then send one YouTube URL per message (surrounding title text is allowed). The bot acknowledges only after the submission is stored, then replies when it is published or reaches a final failure. `/status VIDEO_ID` inspects a submission. Pending replies survive service restarts.

After a final bot failure, a CLI retry does not reattach the bot's publication notification. Resend the link to the bot or use `/status VIDEO_ID` to check the retried submission. Resending an already published source returns its status without adding another episode.

The optional setup helper verifies the bot identity and derives your user ID from a fresh private `/start` message. Create the bot in BotFather first, then run these commands in an interactive terminal:

```sh
docker compose stop
install -d -m 700 ../2pod-private
cp .env ../2pod-private/.env
chmod 600 ../2pod-private/.env
python3 scripts/configure-telegram.py ../2pod-private/.env
docker compose --env-file ../2pod-private/.env up -d
```

The helper reads the token with a hidden prompt and shows the bot username plus a random command to send from your own private chat. It writes both settings without printing the token. The settings file and its parent must be owned by you; the file must have mode `600` and the parent must not be writable by others. Stop any existing polling process for that bot before setup. Continue using the private env file on subsequent Compose commands. After restarting, submit a real link and check the queued and publication/failure replies; helper identity checks alone do not verify delivery.

## Hosting

The supplied Compose file creates a private Docker network and publishes no port. Attach the service to an existing reverse-proxy network in a local `compose.local.yaml`:

```yaml
services:
  2pod:
    networks:
      - default
      - proxy
networks:
  proxy:
    external: true
    name: your_existing_proxy_network
```

Start with `docker compose -f compose.yaml -f compose.local.yaml up -d`. Configure your proxy to forward to `2pod:8080`, issue a trusted TLS certificate, and avoid recording secret URL paths in access logs. Restrict ingress to the network you intend to permit. A secret URL grants read access only where that network boundary permits it.

For a local-only trial, use this alternative override:

```yaml
services:
  2pod:
    ports:
      - "127.0.0.1:8080:8080"
```

With `TWOPOD_BASE_URL=http://localhost:8080`, a player on the Docker host can fetch the feed. A phone's `localhost` is the phone itself; use reachable HTTPS hosting for phone playback. Alex's deployment uses Tailscale-only Traefik routing and DNS-only Cloudflare at `2pod.thealexferrari.com`; that infrastructure is not required by the application or supplied portable Compose file.

## State and access

The named `state` volume contains SQLite state and its WAL files, published audio, temporary extraction files, the generated read token, and the local management socket. Container replacement preserves this volume. Use the whole stopped-state backup below to preserve database and media together. Do not use `docker compose down --volumes` unless deliberately removing the installation's data.

The image runs as UID/GID `10001:10001` and works with rootless Docker. Docker initializes the named volume with that ownership. If you replace it with a bind mount, arrange ownership for that container identity, including the rootless UID mapping, before startup. Do not make state world-writable.

The management socket is owner-only and has no TCP listener. Access to Docker or the state directory is operator access. The RSS and enclosure URLs share a read secret; avoid pasting them into tickets, logs or shared shell transcripts. `feed-url` deliberately reveals the subscription URL to the operator. Preserve the generated token during updates and restores. Changing the token invalidates existing feed/media URLs and requires updating subscriptions.

Expiry removes the feed item and server audio. Playback, HTTP downloads and ranges do not trigger cleanup or extend retention. Already downloaded copies remain under the player's control. A source submitted after deletion or expiry retains its stable identity; the player decides whether to rediscover it as new.

## Update and rollback

The package baseline, verified against upstream on 2026-10-04, is Go `1.27.1`, Debian trixie, FFmpeg `7:7.1.5-0+deb13u1`, yt-dlp nightly `2026.09.27.232945` with bundled yt-dlp-ejs `0.8.0`, and Deno `2.9.7`. Base images are pinned by digest, the yt-dlp download by SHA-256, and Debian packages by the `20261004T000000Z` archive snapshot. Python runs only during extraction; the persistent application is Go.

Check [yt-dlp nightly releases](https://github.com/yt-dlp/yt-dlp-nightly-builds/releases), the [official EJS guide](https://github.com/yt-dlp/yt-dlp/wiki/EJS), [Deno releases](https://github.com/denoland/deno/releases), [Go releases](https://go.dev/dl/) and [Debian's supported FFmpeg package](https://packages.debian.org/trixie/ffmpeg) before advancing pins. [Upstream recommends the nightly channel](https://github.com/yt-dlp/yt-dlp#update); this package pins a specific release instead of fetching a moving latest version. The official Unix yt-dlp executable includes its matching EJS scripts. Update its version and published checksum together; advance Deno/base-image digests and the Debian snapshot/FFmpeg package together as appropriate. Rebuilds retain pinned versions until the Dockerfile is changed; `--pull` alone does not advance digest pins or apply newer security fixes.

1. Save the current image with `docker image tag yt-to-rss:local yt-to-rss:rollback` and make the stopped-state backup below.
2. Update source and reviewed dependency pins, then run `docker compose build --pull`.
3. Run `docker compose up -d` and verify `docker compose exec -T 2pod 2pod status`, feed refresh and an enclosure's playback/seek behavior.

If the new image fails, use `TWOPOD_IMAGE=yt-to-rss:rollback docker compose up -d --no-build --force-recreate`. Keep the pre-update state backup: restoring an older image after a future incompatible state migration may also require restoring the matching backup. Do not restore a backup over a running service.

Inspect packaged versions without emitting credentials:

```sh
docker compose exec -T 2pod yt-dlp --version
docker compose exec -T 2pod deno --version
docker compose exec -T 2pod ffmpeg -version
docker compose exec -T 2pod python3 --version
```

## Backup and restore

Stop the service so SQLite and media are a consistent set. Store the archive and configuration in a private directory outside the source checkout; they include access secrets.

```sh
install -d -m 700 ../2pod-backups
docker compose stop
docker compose run --rm --no-deps -T --entrypoint tar 2pod \
  --exclude=control.sock -C /data -czf - . > ../2pod-backups/state.tar.gz
chmod 600 ../2pod-backups/state.tar.gz
cp .env ../2pod-backups/settings.env
chmod 600 ../2pod-backups/settings.env
docker compose up -d
```

To restore into a fresh installation, copy the private settings to `.env`, then run the following before starting the service. Use an empty state volume; do not combine old and restored databases or media.

```sh
docker compose run --rm --no-deps -T --entrypoint tar 2pod \
  -C /data -xzf - < ../2pod-backups/state.tar.gz
docker compose up -d
docker compose exec -T 2pod 2pod status
```

Recheck the subscription and media after restoration. Use the same Compose project name and volume when restarting an existing installation; changing the project name normally creates another volume.

## Troubleshooting

- **Submission failed:** run `2pod status VIDEO_ID` through Compose to see the sanitized reason and attempts. Check whether the video is public/unlisted, not a livestream, and available from the server's network. Login-dependent, removed, age-restricted or region-blocked content may be unavailable without being an application failure.
- **Provider/extraction errors:** check packaged versions above, DNS/connectivity, and yt-dlp's current upstream reports. Rebuild with a verified current release when required, then use `docker compose exec -T 2pod 2pod retry VIDEO_ID`. A fixed bot-challenge diagnostic means YouTube blocked automated access for that source from the chosen network. Check an existing extraction proxy's reachability if one is configured, or configure an operator-owned proxy after reviewing its availability dependency. Do not repeatedly retry an unchanged provider block or import browser cookies without reviewing the scope.
- **Queue paused for storage:** inspect `2pod status`, remove unwanted episodes with `delete`, increase the budget/reserve only if disk capacity permits, or wait for expiry. Preserve unexpired episodes; do not manually delete files behind SQLite's back.
- **Player cannot refresh:** confirm the player can reach the base URL, trusts its HTTPS certificate, and uses the current secret feed URL. For private Tailscale hosting, connect the fetching device to the tailnet. Publication means available at the service; the player's polling/cache timing is separate.
- **Service does not start:** use `docker compose logs --tail 100 2pod` and check settings/state permissions. Normal diagnostics sanitize credentials; inspect logs privately before sharing excerpts.
- **Seek/resume fails:** ensure the reverse proxy passes `Range` and `HEAD` through and preserves `Content-Length`, `Content-Type` and `Content-Range`. Avoid a proxy rewrite that changes enclosure paths.

RSS/media serving and controlled fixture tests establish server behavior. Real YouTube extraction, Telegram delivery, measured memory, private-network exposure and phone playback need separate live acceptance; a successful image build does not establish those results.
