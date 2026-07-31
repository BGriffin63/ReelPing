# Changelog

All notable changes to ReelPing are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Additional Discord-compatible webhook** destination (e.g. Root, Guilded,
  Revolt, or any service that accepts a Discord-style webhook). Every alert
  fans out to both the primary Discord webhook and this one. Configure it under
  Settings → Discord, with its own label, enable toggle, and test button. The
  URL is stored as a secret (redacted like the Discord webhook).

### Changed

- The container now listens on port **8787** by default (previously 8080), so the
  container and host ports match and are less confusing. Compose/Unraid examples
  map `8787:8787`. Override with `RP_ADDR` if needed. **If you run the container
  manually, update your port mapping to `-p 8787:8787`.**

### Added

- Optional **auto-finish maintenance**: when enabled, ReelPing announces Plex is
  back and clears the maintenance window automatically once Plex returns (after
  it actually went down and stayed up for the recovery threshold).
- Discord avatar asset (`assets/reelping-avatar.png`).

### Fixed

- `.gitignore` no longer excludes the `cmd/reelping` package.
- Setup-wizard **Test connection** / **Send test** buttons now read the wizard's
  field names (`plex_url` / `discord_webhook`).
- GHCR image references are lowercase; icon/template URLs use the correct repo case.

## [0.5.0-beta] - 2026-07-24

First public beta.

### Added

- Multi-stage Plex availability monitoring (URL → DNS → TCP → HTTP `/identity`
  → optional authenticated identity + stream count) with classified failures.
- Persisted, hysteretic monitoring state machine (suspect/offline/recovering,
  maintenance-online/offline) with restart-safe incident continuity.
- Automatic outage and recovery Discord notifications (one per event, with
  outage duration on recovery).
- Discord incoming-webhook provider with explicit `allowed_mentions`, embed
  limit/character-budget enforcement, escaping, bounded retries with
  backoff/jitter, and `429` `retry_after` handling.
- Planned-maintenance workflow: schedule, start-now, going-offline, delay
  updates, service-restored (with down-server warning/override), and custom
  announcements — all with idempotency protection.
- Authenticated web UI: first-run wizard, dashboard, maintenance, announcements,
  incidents, notification history, audit log, settings, and diagnostics.
- Argon2id auth, server-side sessions, CSRF protection, login rate limiting,
  strict security headers, and trusted-proxy handling.
- Embedded bbolt storage with schema versioning, atomic backup-before-migrate,
  retention sweeping, and secret-free diagnostics/exports.
- Original branding (SVG logo, favicon, monochrome, PNG icons, social card) and
  a deterministic asset generator.
- Multi-stage Dockerfile (non-root, scratch-based, embedded tzdata, health
  check), Compose file, and Unraid Community Applications template + profile.
- Documentation set and CI workflows.

### Security

- Secrets (Plex token, Discord webhook) are stored with restrictive permissions
  and never rendered, logged, exported, or placed in URLs/process args.
- Automated tests assert no secrets appear in diagnostics.

[Unreleased]: https://github.com/BGriffin63/reelping/compare/v0.5.0-beta...HEAD
[0.5.0-beta]: https://github.com/BGriffin63/reelping/releases/tag/v0.5.0-beta
