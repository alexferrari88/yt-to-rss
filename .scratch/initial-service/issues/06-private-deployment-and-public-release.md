# 06: Private deployment and public release

**What to build:** Operate the complete service on Alex's always-on server at 2pod.thealexferrari.com, accessible only through Tailscale, and publish the reusable MIT project as alexferrari88/yt-to-rss. Verify real extraction, Telegram delivery, memory use and Android podcast playback, keeping server evidence and device acceptance distinct.

**Blocked by:** 03 — Telegram submission and replies; 04 — Expiry and storage limits; 05 — Reproducible Docker Compose installation.

**Status:** ready-for-agent

**Spec coverage:** User stories 1, 5–7, 19–22, 39–44; live acceptance validates the assembled service.

- [ ] Reinspect current service guidance, Docker/Traefik/Tailscale state and the exact Cloudflare hostname record before changes. Apply only this service's configuration and preserve unrelated services, existing private routes and boot behavior.
- [ ] Deploy the packaged service with persistent state on the existing private proxy network without publishing an application host port. Use the existing Traefik HTTPS/certificate convention and private Tailscale ingress.
- [ ] Configure only the selected DNS-only Cloudflare hostname to the current Tailscale address. Do not enable Cloudflare proxying, public listeners, Funnel or Tunnel. Verify trusted HTTPS for the selected hostname without certificate exceptions.
- [ ] From within the tailnet, verify the feed and MP3 GET/HEAD/ranges through the deployed hostname. Verify feed/audio are inaccessible from outside the tailnet even with a known hostname and valid read secret; record the actual test vantage and listener/routing evidence. Existing private routes remain healthy.
- [ ] Complete a real public/unlisted YouTube extraction on this server using the packaged dependencies and confirm complete playable audio, correct metadata and publication in the feed. Any provider/network failure is diagnosed and its resolution verified before claiming success.
- [ ] Measure idle service memory and peak memory during a documented real processing workload. Report the persistent Go process and total processing workload including extraction subprocesses/container memory, with measurement scope and workload; make no unmeasured RAM claim.
- [ ] With the configured Telegram operator, verify actual submission and queued/published/failure reply delivery. Credentials remain private, and delivery is reported only from verified outcomes.
- [ ] On the operator's Android phone with Tailscale connected, subscribe in AntennaPod and verify feed refresh, download, playback and seeking. Record the actual operator/device confirmation separately from server/HTTP checks; pending phone acceptance remains explicit and does not count as a completed check. The feed remains standard and player-independent.
- [ ] Run the assembled service's automated acceptance through the confirmed CLI/bot and HTTP boundaries, including recovery, deduplication, retry, deletion, expiry and storage pause/resume. Use controlled external traffic for automated tests and distinguish them from live acceptance.
- [ ] Publish source, MIT licensing, portable Compose setup and complete operator instructions in the public alexferrari88/yt-to-rss repository. Verify the remote publication and exclude credentials, private service configuration, generated media and persistent runtime state.
- [ ] The handoff records the running version, public repository, verified private access, extraction/Telegram/device results, measured resource use and supported operating/update commands. Report any unresolved acceptance item accurately rather than presenting an attempted action as complete.
