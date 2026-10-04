# Verification

Validated on Linux amd64, 2026-10-04.

- Full CLI/bot/HTTP application acceptance passed: `GOFLAGS=-race GORACE=atexit_sleep_ms=0 go test -race ./...` (54.967s). The daemon built by the harness was race-instrumented as well as the tests. External extractor and Telegram traffic were controlled; SQLite and disk state were real.
- `go vet ./...`, Python helper/driver syntax, private Compose validation and `git diff --check` passed.
- Independent standards/spec review and its resolved findings are recorded in [review](review.md).

Final container replacement/backup/restore, private deployment and packaged real extraction are being verified. Real Telegram delivery, Android playback and an independent outside-tailnet test still require operator acceptance; they are not established by automated tests.
