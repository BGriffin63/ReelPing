# ReelPing — Privacy Statement

ReelPing is **fully self-hosted**. It runs entirely on your hardware and talks
to nothing except the services **you** configure.

## No data collection

ReelPing contains **no**:

- Analytics or usage tracking
- Telemetry
- Advertising
- Crash-report uploads
- Developer-controlled API calls
- License checks or activation
- Remote feature flags
- Cloud accounts
- Tracking pixels
- External fonts, scripts, or CDN assets

The web UI uses the system font stack and locally-served CSS/JS only. There are
no third-party requests from your browser.

## Outbound connections (complete list)

ReelPing makes outbound network connections **only** to:

1. The **Plex server** you configure (monitoring, and optional token calls).
2. The **Discord webhook** you configure (notifications).

There are no other outbound connections.

## What ReelPing reads from Plex

- Always: the unauthenticated `GET /identity` response (server machine ID and
  version).
- With a token (optional): the friendly server name, and **only the count** of
  active sessions from `GET /status/sessions`.

ReelPing never reads or stores viewer usernames, emails, IP addresses, device
IDs, media titles, viewing history, or playback positions.

## What ReelPing sends to Discord

By default, only **service-status information**: whether Plex is up/down, outage
duration, and any maintenance/announcement text you write. Individual viewer
data is never sent. Optionally, you may include only the **active-stream count**
in maintenance announcements.

## Secrets

The Plex token and Discord webhook are stored in the `/config` database with
restrictive permissions and are never rendered, logged, exported, or placed in
URLs or process arguments. See [SECURITY.md](SECURITY.md) for the honest
at-rest limitations (anyone who can read `/config` can read the secrets).

## Your responsibilities

- Protect the `/config` volume (permissions, host-disk encryption).
- Treat backup files and the Discord webhook like passwords.
