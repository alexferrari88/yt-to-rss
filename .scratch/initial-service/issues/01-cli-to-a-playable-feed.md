# 01: CLI to a playable feed

**What to build:** Let the operator submit a public or unlisted YouTube video through a local CLI and listen to its complete MP3 through a standard podcast feed. Provide the first usable path with one Go service, SQLite-persisted submissions, add/list/status commands, and secret read URLs. Follow the confirmed spec and ADRs.

**Blocked by:** None (can start immediately).

**Status:** resolved

**Spec coverage:** User stories 2, 4, 7–17, 19, 20, 23, 24, 26, 27, 30, 39 and 42; local operation contributes to 44.

- [x] The service and CLI build and run locally. Add acknowledges a submission only after durable acceptance; list/status return stable identifiers, observable states, sanitized errors and scriptable exit outcomes.
- [x] Ordinary YouTube video URL variants normalize to one source identity. Start timestamps and tracking parameters are ignored; a video URL containing playlist context processes that video alone. Malformed links, unsupported origins and playlist-only links are rejected clearly. Login-dependent content and active livestreams produce clear unsupported outcomes.
- [x] Repeated submissions of a queued, processing or published source return the existing status without another extraction or feed identity.
- [x] One extraction runs at a time, invoking maintained yt-dlp and FFmpeg with explicit arguments and bounded processing time. Submitted text is never interpreted as shell code. Working audio and HTTP responses use disk-backed processing rather than buffering a whole episode in RAM.
- [x] A successful extraction publishes the complete, playable MP3. Failed extraction produces an inspectable failure and does not publish a partial file or prevent unrelated submissions from being processed.
- [x] The RSS 2.0 feed has configurable title, description, link and external base URL. Each episode has a stable GUID, a date based on successful publication, available title/uploader/duration/source metadata, and an MP3 enclosure with the correct URL, byte length and MIME type. Source text cannot break XML.
- [x] Authorized HTTP GET, HEAD and byte-range requests support streaming, download, seeking and resumption with correct statuses, lengths, types and bytes. Missing or incorrect read secrets cannot retrieve feed or media, including partial responses; missing media has a clear unavailable response.
- [x] Read URLs are long and unguessable and grant no management privileges. Submission and management remain local operator commands. Normal logs and errors omit credentials and secret read URLs.
- [x] A clean service restart using the same persistent state retains accepted submissions and published episodes, including their identities and playable media.
- [x] Automated acceptance submits through the public CLI and observes command/status results and RSS/media over HTTP. It replaces only external YouTube traffic, independently parses RSS, and checks known audio bytes and headers against independently chosen expectations. No database-row or private-helper test seams are introduced.
- [x] Local build, start, configuration, add/list/status and test instructions let an operator reproduce this path. Dependency choices are recorded for the later packaged installation.

## Answer

Implemented the Go daemon/local CLI, durable canonical submissions, one disk-backed extraction worker, verified MP3 publication, stable standard RSS and secret read-only GET/HEAD/range endpoints. Public-boundary acceptance passes, including rejecting truncated audio that retains a valid duration header. Installation and command instructions are in README.md and docs/operations.md.

Validation: the complete application acceptance suite passes with both daemon and tests race-instrumented (`GOFLAGS=-race GORACE=atexit_sleep_ms=0 go test -race ./...`); `go vet ./...` passes. See docs/review.md for independent standards/spec findings and resolutions.
