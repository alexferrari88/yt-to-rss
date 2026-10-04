# 04: Expiry and storage limits

**What to build:** Keep server storage bounded without routine manual cleanup. Expire episodes after the retention period and pause/resume processing under storage pressure while retaining accepted submissions and unexpired episodes.

**Blocked by:** 02 — Recovery, retries and deletion.

**Status:** resolved

**Spec coverage:** User stories 18, 33–38 and 44.

- [x] Default retention is 30 days from successful publication and is configurable. Queuing, retries, feed refreshes and media requests neither start nor extend that period.
- [x] Expiry withdraws the feed entry and removes its server audio safely; subsequent feed/media responses agree that the episode is unavailable. Cleanup also handles overdue episodes after restart without deleting unrelated or unexpired media.
- [x] GET, HEAD, partial downloads and playback activity do not trigger early deletion. Cleanup does not depend on a player reporting download or listening state.
- [x] An expired source can be submitted again using its stable episode identity, with a new publication time and retention period after successful extraction.
- [x] Configurable storage budgets and free-space checks account for completed media and extraction working files. Processing pauses when those checks fail; resource enforcement and cleanup prevent uncontrolled working-file growth from filling the host disk.
- [x] Storage pressure preserves queued work and unexpired episodes, leaves existing playable media available, and exposes an actionable paused reason through public CLI status rather than silently discarding work.
- [x] Eligible queued processing resumes when deletion/expiry frees space or configured limits are increased. A storage pause is not counted as a failed extraction attempt, and restart preserves accepted work and the resource constraints.
- [x] Failed/interrupted artifact cleanup releases accounted storage while respecting cancellation and safe deletion from ticket 02.
- [x] Automated acceptance uses constrained storage, known audio fixtures and controlled elapsed time or short configured retention. CLI and HTTP observations prove expiry, preserved unexpired media, paused/resumed processing, and stable identity on resubmission without inspecting SQLite rows or private helpers.
- [x] Operator instructions document retention/resource defaults, storage accounting, paused-state remedies and expiry behavior, including the distinction between server audio and copies already downloaded by a player.

## Answer

Implemented independent retention maintenance, 30-day defaults and storage/free-space pause/resume without losing queued work or evicting unexpired episodes. Public-boundary acceptance covers short retention/restart, unchanged retention after downloads, working-file growth cancellation and resumed stable identities. Operations document per-file limits and the possible brief aggregate overshoot between polling checks.

Validation: the complete application acceptance suite passes with both daemon and tests race-instrumented (`GOFLAGS=-race GORACE=atexit_sleep_ms=0 go test -race ./...`); `go vet ./...` passes. See docs/review.md for independent standards/spec findings and resolutions.
