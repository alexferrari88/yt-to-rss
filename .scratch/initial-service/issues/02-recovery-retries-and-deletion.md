# 02: Recovery, retries and deletion

**What to build:** Make accepted work recover safely from interruption and let the operator recover or remove it through the CLI. Add bounded automatic retries, explicit retry, and deletion of queued, running or published work while preserving stable episode identity.

**Blocked by:** 01 — CLI to a playable feed.

**Status:** ready-for-agent

**Spec coverage:** User stories 18, 24, 25, 28, 29, 31, 32 and 44.

- [ ] Restart after interruption recovers accepted work without manual database repair. Interrupting extraction or publication never exposes partial media or creates duplicate feed entries, and previously published episodes remain usable.
- [ ] Transient failures receive a finite, configurable retry budget and bounded scheduling. Attempts and the next eligible action are inspectable; exhaustion produces a final sanitized failure. Waiting for a retry does not indefinitely prevent unrelated eligible work from proceeding.
- [ ] CLI retry explicitly requeues failed work after an operator fixes a dependency or provider problem. It preserves source/episode identity, respects the single-worker constraint, and reports invalid retry requests clearly.
- [ ] CLI delete safely removes queued work, cancels running extraction, or withdraws a published episode and its server audio. It removes only artifacts belonging to the target, and repeated deletion is safe.
- [ ] Deletion during processing prevents later publication even if extraction finishes concurrently or the service restarts. Extraction subprocesses are stopped and cleaned up without leaving an extra worker running.
- [ ] A deleted source can be submitted again and successfully published using the same stable episode identity. Documentation explains that a podcast player controls whether this restored identity appears as new.
- [ ] Failed and interrupted working artifacts are cleaned up safely without removing another episode's media or accepted work.
- [ ] Automated acceptance drives restart, retry exhaustion, manual retry and deletion in queued/running/published states through CLI and HTTP boundaries. Controlled external YouTube traffic provides delayed completion and failures; assertions use public outcomes rather than database rows or private helpers.
- [ ] Operator instructions cover crash recovery, retry/delete commands, bounded retry defaults and actionable dependency/provider troubleshooting.
