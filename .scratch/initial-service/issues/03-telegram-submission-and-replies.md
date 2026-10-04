# 03: Telegram submission and replies

**What to build:** Let the operator share a YouTube link from Android to a Telegram bot and receive queued, published or failed replies. Use the same durable submission and episode lifecycle as the CLI, with authorization checked before accepting work.

**Blocked by:** 01 — CLI to a playable feed.

**Status:** resolved

**Spec coverage:** User stories 1, 3–6, 9, 17, 39, 42 and 44.

- [x] The service uses outbound Telegram polling with configurable bot credentials and operator identity. It needs no inbound webhook or public management endpoint; the CLI path remains usable when Telegram is not configured.
- [x] Messages from anyone other than the configured operator cannot create or manage submissions. Authorization occurs before state changes or acknowledgement of accepted work.
- [x] A supported link enters the same durable lifecycle as a CLI submission. The queued acknowledgement follows persistence, and malformed or unsupported input receives a concise actionable outcome.
- [x] Link variants, repeated sharing, replayed polling updates and cross-CLI/bot submissions return the existing active-source status without duplicate extraction or publication.
- [x] Publication replies are sent only when the episode is available in the feed with complete media. Failure replies reflect a final failed outcome and contain an actionable sanitized reason. Reply delivery failures are reported accurately in safe diagnostics rather than treated as successful delivery.
- [x] Polling reconnection and service restart preserve accepted submissions. Terminal outcomes of accepted bot work remain inspectable and are reported through the bot when delivery is available.
- [x] Bot credentials and secret read URLs are omitted from routine logs and error replies. The feed's read credentials confer no bot or CLI management permission.
- [x] Automated acceptance exercises the public Telegram input adapter and observes replies, CLI status and feed/media. Only external Telegram and YouTube traffic are controlled; unauthorized input, replay, duplicate sharing, publication and failure are covered without mocking internal application helpers.
- [x] Setup instructions explain safe bot credential/operator configuration and Android sharing. Real Telegram delivery is verified in the deployment ticket and is distinguished from automated adapter acceptance.

## Answer

Implemented optional outbound polling with operator/private-chat authorization, durable update intents/cursor/pending replies, canonical CLI/bot deduplication and safe queued/published/failure messages. Four controlled Telegram boundary cases pass for authorization, replay/deduplication, delivery failure and restart behavior. Setup is documented with a protected interactive helper; actual Telegram delivery remains a separate pending live requirement in ticket 06.

Validation: the complete application acceptance suite passes with both daemon and tests race-instrumented (`GOFLAGS=-race GORACE=atexit_sleep_ms=0 go test -race ./...`); `go vet ./...` passes. See docs/review.md for independent standards/spec findings and resolutions.
