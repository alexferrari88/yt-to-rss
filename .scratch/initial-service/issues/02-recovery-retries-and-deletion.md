# 02: Recovery, retries and deletion

**What to build:** Make accepted work recover safely from interruption and let the operator recover or remove it through the CLI. Add bounded automatic retries, explicit retry, and deletion of queued, running or published work while preserving stable episode identity.

**Blocked by:** 01 — CLI to a playable feed.

**Status:** resolved

**Spec coverage:** User stories 18, 24, 25, 28, 29, 31, 32 and 44.

- [x] Restart after interruption recovers accepted work without manual database repair. Interrupting extraction or publication never exposes partial media or creates duplicate feed entries, and previously published episodes remain usable.
- [x] Transient failures receive a finite, configurable retry budget and bounded scheduling. Attempts and the next eligible action are inspectable; exhaustion produces a final sanitized failure. Waiting for a retry does not indefinitely prevent unrelated eligible work from proceeding.
- [x] CLI retry explicitly requeues failed work after an operator fixes a dependency or provider problem. It preserves source/episode identity, respects the single-worker constraint, and reports invalid retry requests clearly.
- [x] CLI delete safely removes queued work, cancels running extraction, or withdraws a published episode and its server audio. It removes only artifacts belonging to the target, and repeated deletion is safe.
- [x] Deletion during processing prevents later publication even if extraction finishes concurrently or the service restarts. Extraction subprocesses are stopped and cleaned up without leaving an extra worker running.
- [x] A deleted source can be submitted again and successfully published using the same stable episode identity. Documentation explains that a podcast player controls whether this restored identity appears as new.
- [x] Failed and interrupted working artifacts are cleaned up safely without removing another episode's media or accepted work.
- [x] Automated acceptance drives restart, retry exhaustion, manual retry and deletion in queued/running/published states through CLI and HTTP boundaries. Controlled external YouTube traffic provides delayed completion and failures; assertions use public outcomes rather than database rows or private helpers.
- [x] Operator instructions cover crash recovery, retry/delete commands, bounded retry defaults and actionable dependency/provider troubleshooting.

## Answer

Public CLI/HTTP acceptance verifies clean and abrupt restart recovery, cancellation of orphan extraction process groups, finite retries and explicit retry, safe deletion in queued/running/published states, cleanup and stable identity after resubmission. SIGTERM on the only allowed attempt preserves accepted work rather than exhausting its retry budget.

Validation: the complete application acceptance suite passes with both daemon and tests race-instrumented (`GOFLAGS=-race GORACE=atexit_sleep_ms=0 go test -race ./...`); `go vet ./...` passes. See docs/review.md for independent standards/spec findings and resolutions.
