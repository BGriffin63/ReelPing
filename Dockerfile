# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26 AS build

WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=mod

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build.
COPY . .
ARG VERSION=0.5.0-beta
ARG COMMIT=unknown
ARG DATE=unknown
RUN go build -trimpath \
      -ldflags "-s -w \
        -X github.com/BGriffin63/reelping/internal/version.Version=${VERSION} \
        -X github.com/BGriffin63/reelping/internal/version.Commit=${COMMIT} \
        -X github.com/BGriffin63/reelping/internal/version.Date=${DATE}" \
      -o /out/reelping ./cmd/reelping

# Prepare a /config directory owned by the runtime user (Unraid appdata is
# typically owned 99:100 = nobody:users).
RUN mkdir -p /out/config && chown -R 99:100 /out/config

# ---- Final stage ----
# scratch keeps the image tiny and shell-free. CA certificates are copied for
# outbound HTTPS (Plex/Discord); the time-zone database is embedded in the
# binary (time/tzdata), so no system zoneinfo is required.
FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/reelping /reelping
COPY --from=build --chown=99:100 /out/config /config

# Non-root by default. 99:100 matches Unraid's nobody:users, which owns the
# appdata share, so /config is writable out of the box on Unraid.
USER 99:100

ENV RP_ADDR=":8787" \
    RP_CONFIG_DIR="/config" \
    TZ="UTC"

EXPOSE 8787
VOLUME ["/config"]

# The health check probes ReelPing itself. A Plex outage never marks the
# container unhealthy.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD ["/reelping", "-healthcheck"]

ENTRYPOINT ["/reelping"]
