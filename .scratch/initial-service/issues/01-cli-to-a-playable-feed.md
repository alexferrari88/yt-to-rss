# 01: CLI to a playable feed

**What to build:** Let the operator submit a public or unlisted YouTube video through a local CLI and listen to its complete MP3 through a standard podcast feed. Provide the first usable path with one Go service, SQLite-persisted submissions, add/list/status commands, and secret read URLs. Follow the confirmed spec and ADRs.

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

**Spec coverage:** User stories 2, 4, 7–17, 19, 20, 23, 24, 26, 27, 30, 39 and 42; local operation contributes to 44.

- [ ] The service and CLI build and run locally. Add acknowledges a submission only after durable acceptance; list/status return stable identifiers, observable states, sanitized errors and scriptable exit outcomes.
- [ ] Ordinary YouTube video URL variants normalize to one source identity. Start timestamps and tracking parameters are ignored; a video URL containing playlist context processes that video alone. Malformed links, unsupported origins and playlist-only links are rejected clearly. Login-dependent content and active livestreams produce clear unsupported outcomes.
- [ ] Repeated submissions of a queued, processing or published source return the existing status without another extraction or feed identity.
- [ ] One extraction runs at a time, invoking maintained yt-dlp and FFmpeg with explicit arguments and bounded processing time. Submitted text is never interpreted as shell code. Working audio and HTTP responses use disk-backed processing rather than buffering a whole episode in RAM.
- [ ] A successful extraction publishes the complete, playable MP3. Failed extraction produces an inspectable failure and does not publish a partial file or prevent unrelated submissions from being processed.
- [ ] The RSS 2.0 feed has configurable title, description, link and external base URL. Each episode has a stable GUID, a date based on successful publication, available title/uploader/duration/source metadata, and an MP3 enclosure with the correct URL, byte length and MIME type. Source text cannot break XML.
- [ ] Authorized HTTP GET, HEAD and byte-range requests support streaming, download, seeking and resumption with correct statuses, lengths, types and bytes. Missing or incorrect read secrets cannot retrieve feed or media, including partial responses; missing media has a clear unavailable response.
- [ ] Read URLs are long and unguessable and grant no management privileges. Submission and management remain local operator commands. Normal logs and errors omit credentials and secret read URLs.
- [ ] A clean service restart using the same persistent state retains accepted submissions and published episodes, including their identities and playable media.
- [ ] Automated acceptance submits through the public CLI and observes command/status results and RSS/media over HTTP. It replaces only external YouTube traffic, independently parses RSS, and checks known audio bytes and headers against independently chosen expectations. No database-row or private-helper test seams are introduced.
- [ ] Local build, start, configuration, add/list/status and test instructions let an operator reproduce this path. Dependency choices are recorded for the later packaged installation.
