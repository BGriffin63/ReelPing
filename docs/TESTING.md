# ReelPing — Testing

This document describes ReelPing's test suite, how to run it, and — honestly —
what has and has **not** yet been verified.

## Running the tests

```bash
go test ./...                     # unit + integration + full-UI-render tests
go vet ./...
gofmt -l .                        # must print nothing
scripts/test.sh                   # adds coverage + race (needs CGO for -race)
```

The race detector (`-race`) requires a C toolchain (`CGO_ENABLED=1`). CI runs it
on the concurrency-sensitive packages; it is skipped in pure-static
environments.

## What the automated suite covers (all currently passing)

### State machine (`internal/monitoring`)
- One/two failures then success → **no** outage.
- Three consecutive failures → **exactly one** outage effect and one incident.
- Ten further failures → **no** duplicate alerts.
- One success after outage → recovering (no alert); second success → **exactly
  one** recovery alert; further successes → no duplicate.
- No recovery alert when the outage was never notified.
- Restart during `suspect` → no false outage, no incident created.
- Restart during a confirmed outage → **same** incident continued, no duplicate.
- Maintenance mode suppresses outage alerts and creates no incident.

### Plex checks (`internal/plex`, fake HTTP servers)
- Valid `/identity` → online (machine ID parsed).
- Invalid body, non-2xx status, oversized body → classified, never a crash.
- Connection refused → `plex_service_unreachable` (host reachable).
- Timeout → timeout-class failure.
- Identity mismatch, auth failure on `/status/sessions`, stream count, friendly
  name.
- TLS failure (self-signed + verify on) and success with verify off.
- DNS failure.
- **Token never appears in the request URL.**

### Discord (`internal/discord`, fake webhook server)
- `204` success; `404` (and other `4xx`) **not** retried; `429` with
  `retry_after` retried then succeeds; persistent `500` retried then gives up
  after the retry cap.
- `allowed_mentions`: none → empty parse & no content; everyone → parse
  everyone; role → whitelisted role only.
- Markdown/`@everyone` escaping in embed text.
- Webhook URL redacted out of error strings.
- Embed total-character budget (6000) enforced.

### Auth & security (`internal/auth`, `internal/security`)
- Argon2id hash/verify round-trip; wrong password rejected; salts differ;
  garbage hashes rejected.
- Password strength policy (length/common-password rejection).
- URL validation: valid Plex/Discord URLs accepted; credential-in-URL,
  wrong-scheme, wrong-host, CRLF, bad-port variants rejected.
- Role-ID and machine-ID validation; control-character stripping; redaction hint.

### Web/UI (`internal/web`, `httptest` end-to-end)
- `/healthz` returns 200 regardless of Plex state.
- First run redirects to `/setup`; the setup page renders.
- **Full setup flow**, then **every authenticated page renders** (proving all
  templates parse and execute).
- Login success/failure; dashboard reachable after login.
- **CSRF-less POST is rejected (403).**
- **Diagnostics download contains no secrets** (asserts the Plex token, Discord
  webhook, and password hash are absent).

## Mapping to the spec's required test sections

The scenarios above cover the required §37 (state machine), §38 (Plex
monitoring), §39 (one-minute outage timing, via an injected virtual clock),
§40 (Discord), §41 (security escaping/redaction), §42 (auth/CSRF), and §43
(persistence across restart) behaviours. Timing tests use an injected clock —
**no test waits real minutes**.

## Not yet done (honest status)

The following require real hardware/registry access and are **pending**; they
must be completed before any "Community Applications submission ready" claim:

- Clean install / upgrade / uninstall / Previous-Apps reinstall on a **real
  Unraid** server.
- Real Discord delivery to a live channel.
- Real Plex **Docker** outage + recovery, and real Plex **VM** outage + recovery.
- Building and **publishing** the multi-arch container image to GHCR, and
  installing the published image via the Unraid template.
- Container vulnerability scan (Trivy) and SBOM on the published image.
- Browser-automation UI pass (mobile/desktop, dark/light, keyboard, screen
  reader) — CI hooks are provided but not yet run against a deployed instance.

The full manual matrix lives in
[COMMUNITY-APPS-SUBMISSION.md](COMMUNITY-APPS-SUBMISSION.md). Do **not** mark any
of these passed until actually performed.
