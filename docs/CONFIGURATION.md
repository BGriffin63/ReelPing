# ReelPing — Configuration Reference

All configuration is done through the **authenticated web UI** and stored in
`/config/reelping.db`. Only a few environment variables affect startup; secrets
are **never** set via environment variables.

## Environment variables (startup only)

| Variable | Default | Purpose |
| --- | --- | --- |
| `RP_ADDR` | `:8080` | Listen address (`host:port`). |
| `RP_CONFIG_DIR` | `/config` | Directory for the database and backups. |
| `TZ` | `UTC` | Container time zone (also selectable in the UI). |
| `PUID` / `PGID` | `99` / `100` | Informational on Unraid; the image runs as `99:100`. |
| `UMASK` | `022` | File-creation mask. |

> Do **not** put the Plex token or Discord webhook in environment variables,
> Docker labels, or the Unraid template. Enter them in the WebUI.

## Settings (WebUI)

### General
Site title · time zone · date format · theme (system/light/dark) · retention
(days) for announcements/audit/notifications · trusted proxies.

### Plex
Display name · base URL · request timeout · expected machine ID · verify TLS ·
active-session integration · include-stream-count privacy toggle · token
(replace/remove) · connection test.

### Monitoring
Preset (Fast/Balanced/Conservative/Custom) · check interval · request timeout ·
failure threshold · recovery threshold · startup stabilization · notification
cooldown · latency-warning threshold · supplemental host diagnostics · degraded
tracking.

**Bounds:** interval ≥ 10 s; timeout ≥ 1 s and `< interval`; failures ≥ 2;
recovery ≥ 1.

### Discord
Webhook (replace/remove, shown only as a redacted hint) · display-name override
· avatar URL · default manual mention · role ID · automatic outage/recovery
notify toggles · automatic outage/recovery mention policies · test message.

### Security
Change administrator password · view active sessions · sign out other sessions ·
trusted proxies. (Session idle/absolute timeouts and login rate-limit windows
have safe defaults.)

### Data
Download backup · export incidents (JSON/CSV) · download diagnostics · clear
selected history · (retention configured under General).

## Retention defaults

| Data | Default |
| --- | --- |
| Incidents | kept indefinitely (unless a max count is set) |
| Announcement history | 365 days |
| Audit history | 90 days |
| Notification records | 90 days |
| Detailed monitor errors | 14 days (rolling) |

A background sweeper enforces retention hourly, so the database cannot grow
without bound. ReelPing does **not** write a record per successful poll — only
state transitions, incidents, notifications, announcements, and audit events.

## Quiet hours (optional)

Suppress non-critical notifications during a daily window (with day-of-week
selection). Critical outage alerts can be allowed to bypass quiet hours.
Suppressed notifications are **recorded with a reason**, never silently dropped.

## Trusted proxies

If ReelPing runs behind a reverse proxy that terminates TLS, add the proxy's
IP/CIDR under General → Trusted proxies. Only then does ReelPing honour
`X-Forwarded-Proto`/`X-Forwarded-For`; otherwise those headers are ignored to
prevent spoofing.
