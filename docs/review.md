# Implementation review

Reviewed implementation snapshot `eb9291e66dc4bf44f73794f36cf80ab27f2288d0`
against planning baseline `7c66bfab33e627b5d812491eebffdd38718e2dbe` using
`git diff 7c66bfab33e627b5d812491eebffdd38718e2dbe...HEAD`.
Two independent reviewers used the documented standards plus the skill's
Fowler smell baseline, and the approved initial-service spec and tickets.

## Standards

- **P2, documented test isolation:** the acceptance harness inherited production
  `TWOPOD_*` settings, potentially enabling real Telegram polling or breaking
  unrelated cases. This contradicted the documented controlled external
  traffic boundary. Resolved by stripping all inherited application settings
  before setting fixture configuration. The regression deliberately supplies
  invalid production settings and verifies successful CLI-to-feed publication.
- **P3, possible Duplicated Code:** feed and media repeated the publication and
  expiry predicate. Resolved with one small availability predicate shared by
  both endpoints.

The reviewer also observed that `go test -race` alone did not instrument the
daemon built by the harness. Final validation sets `GOFLAGS=-race` so the
daemon and tests are both instrumented; `GORACE=atexit_sleep_ms=0` avoids an
artificial one-second exit delay during short-retention acceptance.

## Spec

- **P2, incomplete audio publication:** ticket 01 requires a "complete,
  playable MP3." A controlled extractor returning success with only 2,255 of
  the 4,510 fixture bytes was published because its MP3 header still advertised
  the full duration. Resolved by decoding/counting frames from disk and checking
  duration against both the header and source metadata with fixed codec/rounding
  allowances. CLI/HTTP regressions reject the truncated file and a 30-second
  source mismatch, while complete fixture and real 213-second audio pass.

No other confirmed missing application behavior or scope creep was reported.
Live deployment and device acceptance are tracked separately; automated
acceptance does not establish real Telegram delivery or Android playback.

Initial findings: Standards 2 (worst P2), Spec 1 (worst P2). All three code
findings are addressed; final test and deployment evidence is recorded in
[verification](verification.md).
