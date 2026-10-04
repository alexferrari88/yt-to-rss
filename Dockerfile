# syntax=docker/dockerfile:1
FROM golang:1.27.1-trixie@sha256:3b77fc618ec235a1ab412de7737f120dd507c57e8d87de4cbb7994fb94275ed5 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/2pod ./cmd/2pod

# The upstream Unix zip executable includes the matching yt-dlp-ejs scripts.
ARG YT_DLP_VERSION=2026.09.27.232945
ARG YT_DLP_SHA256=36de87e6276c4bbaa2d9fd4d1295b25f4e824fe9bbfa20e05e175fd0cecee858
RUN curl --fail --location --retry 3 "https://github.com/yt-dlp/yt-dlp-nightly-builds/releases/download/${YT_DLP_VERSION}/yt-dlp" -o /out/yt-dlp \
    && echo "${YT_DLP_SHA256}  /out/yt-dlp" | sha256sum --check \
    && chmod 0755 /out/yt-dlp

FROM denoland/deno:bin-2.9.7@sha256:bc5aa4466e21b6d3021226a85ba2e1911f7c386254d97b9d797903ab74edace2 AS deno

FROM debian:trixie-slim@sha256:a99cfc517144bc59b1978475ec53b46ecabec7e43635402ee5b77cc54cd1b20a
ARG VCS_REF=unknown
LABEL org.opencontainers.image.source="https://github.com/alexferrari88/yt-to-rss" \
      org.opencontainers.image.revision="${VCS_REF}"
# Freeze Debian dependencies as well as the direct tools. Advance this snapshot
# and the pins together when updating; rebuild to receive security fixes.
ARG DEBIAN_SNAPSHOT=20261004T000000Z
ARG FFMPEG_VERSION=7:7.1.5-0+deb13u1
RUN rm /etc/apt/sources.list.d/debian.sources \
    && printf 'deb [check-valid-until=no] http://snapshot.debian.org/archive/debian/%s/ trixie main\ndeb [check-valid-until=no] http://snapshot.debian.org/archive/debian-security/%s/ trixie-security main\n' "$DEBIAN_SNAPSHOT" "$DEBIAN_SNAPSHOT" > /etc/apt/sources.list \
    && apt-get update \
    && apt-get install --no-install-recommends -y ca-certificates python3 "ffmpeg=${FFMPEG_VERSION}" \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir /data \
    && chown 10001:10001 /data
COPY --from=build /out/2pod /usr/local/bin/2pod
COPY --from=build /out/yt-dlp /usr/local/bin/yt-dlp
COPY --from=deno /deno /usr/local/bin/deno
COPY LICENSE /usr/share/doc/2pod/LICENSE
ENV TWOPOD_STATE_DIR=/data TWOPOD_LISTEN=0.0.0.0:8080 DENO_DIR=/tmp/deno
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["2pod"]
CMD ["serve"]
