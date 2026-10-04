# 06: Private deployment and public release

**What to build:** Operate the complete service on Alex's always-on server at 2pod.thealexferrari.com, accessible only through Tailscale, and publish the reusable MIT project as alexferrari88/yt-to-rss. Verify real extraction, Telegram delivery, memory use and Android podcast playback, keeping server evidence and device acceptance distinct.

**Blocked by:** 03 — Telegram submission and replies; 04 — Expiry and storage limits; 05 — Reproducible Docker Compose installation.

**Status:** needs-info

**Spec coverage:** User stories 1, 5–7, 19–22, 39–44; live acceptance validates the assembled service.

- [x] Reinspect current service guidance, Docker/Traefik/Tailscale state and the exact Cloudflare hostname record before changes. Apply only this service's configuration and preserve unrelated services, existing private routes and boot behavior.
- [x] Deploy the packaged service with persistent state on the existing private proxy network without publishing an application host port. Use the existing Traefik HTTPS/certificate convention and private Tailscale ingress.
- [x] Configure only the selected DNS-only Cloudflare hostname to the current Tailscale address. Do not enable Cloudflare proxying, public listeners, Funnel or Tunnel. Verify trusted HTTPS for the selected hostname without certificate exceptions.
- [ ] From within the tailnet, verify the feed and MP3 GET/HEAD/ranges through the deployed hostname. Verify feed/audio are inaccessible from outside the tailnet even with a known hostname and valid read secret; record the actual test vantage and listener/routing evidence. Existing private routes remain healthy.
- [x] Complete a real public/unlisted YouTube extraction on this server using the packaged dependencies and confirm complete playable audio, correct metadata and publication in the feed. Any provider/network failure is diagnosed and its resolution verified before claiming success.
- [x] Measure idle service memory and peak memory during a documented real processing workload. Report the persistent Go process and total processing workload including extraction subprocesses/container memory, with measurement scope and workload; make no unmeasured RAM claim.
- [ ] With the configured Telegram operator, verify actual submission and queued/published/failure reply delivery. Credentials remain private, and delivery is reported only from verified outcomes. Published reply receipt is operator-confirmed; earlier queued/failure receipt remains unconfirmed.
- [ ] On the operator's Android phone with Tailscale connected, subscribe in AntennaPod and verify feed refresh, download, playback and seeking. Record the actual operator/device confirmation separately from server/HTTP checks; pending phone acceptance remains explicit and does not count as a completed check. The feed remains standard and player-independent.
- [x] Run the assembled service's automated acceptance through the confirmed CLI/bot and HTTP boundaries, including recovery, deduplication, retry, deletion, expiry and storage pause/resume. Use controlled external traffic for automated tests and distinguish them from live acceptance.
- [x] Publish source, MIT licensing, portable Compose setup and complete operator instructions in the public alexferrari88/yt-to-rss repository. Verify the remote publication and exclude credentials, private service configuration, generated media and persistent runtime state.
- [x] The handoff records the running version, public repository, verified private access, extraction/Telegram/device results, measured resource use and supported operating/update commands. Report any unresolved acceptance item accurately rather than presenting an attempted action as complete.

## Answer

Server implementation, private deployment and source publication are complete. The running image records source revision e391521eadf5832ea9cd8e7a0d32fef98cd3c3f5 (label e391521). Trusted private-hostname RSS/media GET/HEAD/ranges, read-secret rejection, real full MP3 extraction/decoding, measured idle/processing memory and unchanged Serve routes are verified. The public MIT repository is https://github.com/alexferrari88/yt-to-rss. Complete evidence and limitations are in docs/verification.md.

The operator confirmed receiving the Published reply for the retried source on 2026-10-04. Remaining operator-dependent acceptance: receipt of the earlier queued/failure replies; Android subscription/download/play/seek; an independent outside-tailnet negative test using the valid private feed URL. Current listener/routing checks establish private configuration and successful on-server tailnet access, and do not substitute for that external/device evidence. These items remain unchecked.

Telegram configuration is activated and preserved private access/state through recreation. The new real source hit a reproducible direct-route YouTube bot challenge. The operator approved using the existing extraction proxy and configurable support for public-repo users. The setting defaults empty; the personal value remains private. The reviewed image passed full race tests, vet, both review axes and package replacement/backup/restore checks. After an owned stopped-state/settings backup and private activation, an actual application CLI retry published the source on its first attempt in 57.03 seconds. Its complete 40,237,820-byte MP3 decoded cleanly and private HTTPS GET/HEAD/ranges/auth checks passed. Idle Go RSS was 16.7 MiB; total processing peak was 236.9 MiB including tools and page cache. The subscription, prior episode, Telegram settings and Serve routes were preserved.

Next action: complete Android playback and independent disconnected-network acceptance; record earlier queued/failure reply receipt if confirmed. A CLI retry after a terminal bot failure does not itself reattach a publication notification.
