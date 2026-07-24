# Contributing to ReelPing

Thanks for your interest! ReelPing aims to be a small, dependable, secure,
self-hosted tool. Contributions that keep it that way are very welcome.

## Ground rules

- Be kind. See the [Code of Conduct](CODE_OF_CONDUCT.md).
- Keep dependencies minimal and actively maintained.
- No telemetry, analytics, external fonts/CDNs, or developer-hosted calls.
- Never weaken the security posture (auth, CSRF, secret redaction, mention
  safety) without discussion.
- Don't add code that controls Plex (start/stop/restart) in v1.

## Development setup

Requirements: Go (see `go.mod` for the pinned version).

```bash
git clone https://github.com/BGriffin63/reelping
cd reelping
go build ./...
go test ./...
```

Run locally:

```bash
RP_CONFIG_DIR=./config RP_ADDR=:8080 go run ./cmd/reelping
```

Regenerate branding assets after editing `assets/reelping.svg` / the generator:

```bash
go run ./tools/genassets
```

## Before you open a PR

Run the checks (see [docs/TESTING.md](docs/TESTING.md)):

```bash
gofmt -l .            # must print nothing
go vet ./...
go test ./...
```

- Add tests for new behaviour. Monitoring/state changes need state-machine
  tests; anything touching secrets needs a redaction test.
- Match the surrounding code style and comment density.
- Keep the web UI server-rendered, accessible, and dependency-free.
- Update the relevant docs and `CHANGELOG.md`.

## Commit / PR conventions

- Small, focused commits with clear messages.
- Describe the "why", not just the "what".
- Link related issues.

## Reporting security issues

Please do **not** open a public issue for vulnerabilities. Follow
[.github/SECURITY.md](.github/SECURITY.md).
