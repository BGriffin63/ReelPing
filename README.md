<p align="center">
  <img src="assets/reelping.svg" width="96" height="96" alt="ReelPing logo">
</p>

<h1 align="center">ReelPing</h1>

<p align="center"><strong>Keep your viewers in the loop.</strong></p>

<p align="center">
  A lightweight, self-hosted <em>Plex Media Server status monitor</em> and <em>Discord notifier</em>, optimized for Unraid.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-beta-orange" alt="beta">
  <img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT">
  <img src="https://img.shields.io/badge/go-1.26-00ADD8" alt="Go 1.26">
  <img src="https://img.shields.io/badge/no%20telemetry-100%25%20self--hosted-2ecc71" alt="no telemetry">
</p>

> ReelPing watches your media server and keeps your Discord community informed.
> Announce planned maintenance with one click, automatically detect unexpected
> outages, and let everyone know when streaming is available again.

> [!NOTE]
> **ReelPing is an independent community project and is not affiliated with or
> endorsed by Plex, Inc., Discord Inc., or Lime Technology, Inc.**
> ReelPing is currently in **beta**: the automated test suite is green, but the
> "real Unraid" test matrix has not yet been field-completed — see
> [Release readiness](#release-readiness).

---

## Why ReelPing?

Two problems, one small app:

1. **Your users don't know when Plex is going down for maintenance.** ReelPing
   gives you one-click, pre-formatted Discord announcements.
2. **You often don't know Plex crashed until people complain.** ReelPing polls
   Plex from its own container and posts one clear alert when a *confirmed*
   outage happens — and a recovery notice (with duration) when it's back.

It feels like a focused media-server status console, not a heavyweight
observability stack.

## Features

- 🔴 **Automatic outage detection** — confirmed in ~1 minute with defaults,
  with hysteresis so a single blip never pages your whole server.
- 🟢 **Automatic recovery alerts** — including the outage duration.
- 🟡 **Planned-maintenance workflow** — schedule, start-now, "going offline",
  delay updates, and service-restored, all one click.
- 🔵 **Custom announcements** with styled Discord embeds.
- 🔒 **Authenticated web UI** — Argon2id, sessions, CSRF, security headers. No
  default password; the UI is never unauthenticated.
- 🧭 **Multi-stage checks** — URL → DNS → TCP → HTTP `/identity` → optional
  authenticated identity/stream-count. Never relies on ICMP.
- 🔗 **Discord incoming webhooks** — no bot, no gateway, minimal attack surface.
- 🧰 **Runs anywhere Plex is reachable** — Docker, VM, another NAS, another
  server, reverse proxy, private hostname, or IP.
- 🕵️ **Private by design** — no telemetry, analytics, or external assets. Only
  talks to *your* Plex and *your* Discord webhook.

## Supported Plex hosting methods

ReelPing monitors a **network endpoint**, so it does not care how Plex is
hosted. All of these work:

| Where Plex runs | Example URL |
| --- | --- |
| Unraid Docker container | `http://UNRAID_LAN_IP:32400` |
| Docker on another host | `http://SERVER_LAN_IP:32400` |
| Windows VM | `http://VM_LAN_IP:32400` |
| Linux VM | `http://VM_LAN_IP:32400` |
| Directly on Windows/Linux | `http://HOST_LAN_IP:32400` |
| Another NAS / physical server | `http://NAS_LAN_IP:32400` |
| Behind a reverse proxy | `https://plex.example.com` |

See [docs/PLEX-DEPLOYMENT-TYPES.md](docs/PLEX-DEPLOYMENT-TYPES.md).

## Screenshots

The web UI includes a first-run setup wizard, a status dashboard (live Plex
state, monitoring config, Discord config, and any active incident/maintenance),
one-click maintenance/announcement controls, and history views for incidents,
notifications, and the audit log.

> To add screenshots to this README and the Unraid listing, drop PNGs into
> `docs/screenshots/` and reference them here (and optionally add `<Screenshot>`
> tags to `templates/reelping.xml`).

## How outage detection works (the one-minute story)

ReelPing polls Plex on an interval. A single failed check does **not** alert —
it moves Plex to a `suspect` state. Only after **N consecutive failures**
(default 3, every 20 s) is an outage *confirmed*: one incident is opened and one
Discord alert is sent. Recovery needs **M consecutive successes** (default 2)
before a single recovery alert goes out.

`confirmation time ≈ check interval × failure threshold` → `20 s × 3 ≈ 60 s`.

A successful HTTP check proves Plex is **reachable and responding**, not that
every stream will play perfectly. See
[docs/INCIDENT-STATE-MACHINE.md](docs/INCIDENT-STATE-MACHINE.md).

### Why no Discord bot?

One-way notifications only need an **incoming webhook**. A bot would add token
management, a gateway connection, hosting, and permissions — all attack surface
you don't need. ReelPing uses webhooks and sets an explicit `allowed_mentions`
on every message so `@everyone`/`@here` can only fire when you deliberately
enable it.

### Why not Plex outgoing webhooks?

A Plex server that has crashed, frozen, or lost power **cannot** send a final
"I'm down" webhook. ReelPing checks actively from its own container instead.
Plex Pass and a Plex token are **not** required for basic monitoring (a token
just unlocks identity verification and stream counts).

## Install

### Unraid (Community Applications)

Full guide: [docs/UNRAID-INSTALLATION.md](docs/UNRAID-INSTALLATION.md).

1. Apps → search **ReelPing** → **Install** (or add the template repository
   during beta).
2. Set the **WebUI Port** (default host `8787`) and **Appdata** path
   (`/mnt/user/appdata/reelping`).
3. Set **TZ** to your time zone.
4. Apply, then open `http://<UNRAID-IP>:8787` and complete the setup wizard.

No privileged mode, Docker socket, host networking, or media mappings are
requested. Enter your Plex token and Discord webhook **in the web UI**, never in
the template.

### Docker Compose

```bash
mkdir reelping && cd reelping
curl -O https://raw.githubusercontent.com/BGriffin63/ReelPing/main/compose.yaml
docker compose up -d
```

Then open `http://<host>:8787`.

### Build locally

```bash
go build -o reelping ./cmd/reelping
RP_CONFIG_DIR=./config ./reelping
```

## First-run setup

Open the WebUI and the wizard walks you through:

1. **Administrator account** — pick a username and a strong password (min 12
   chars). There is no default login.
2. **Plex connection** — enter the base URL ReelPing should use to reach Plex,
   e.g. `http://192.168.1.10:32400`. Optionally add a token. Click **Test**.
3. **Discord** — paste your channel's incoming-webhook URL. Click **Send test**.
4. **Monitoring** — pick a preset (Balanced recommended) and time zone.
5. **Finish** — enable monitoring.

### Getting a Discord webhook

Server Settings → **Integrations** → **Webhooks** → **New Webhook** → choose the
channel → **Copy Webhook URL**. Paste it into ReelPing. Details:
[docs/DISCORD-SETUP.md](docs/DISCORD-SETUP.md).

### Getting a Plex token (optional)

A token enables identity verification and active-stream counts. See Plex's
[official article](https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/)
and [docs/PLEX-INTEGRATION.md](docs/PLEX-INTEGRATION.md). ReelPing only ever
sends it as an `X-Plex-Token` header and never shows, logs, or exports it.

## One-minute monitoring configuration

Use the **Balanced** preset (the default): check interval 20 s, 3 failures to
confirm, 2 successes to recover, 60 s startup stabilization. That confirms an
outage in about a minute while ignoring one-off blips.

## Configuration, presets, and backups

- Configuration reference: [docs/CONFIGURATION.md](docs/CONFIGURATION.md)
- Monitoring internals: [docs/MONITORING.md](docs/MONITORING.md)
- **Backup:** Settings → Data → **Download backup** (a copy of the database;
  treat it like a password vault — it contains your secrets).
- **Restore:** stop ReelPing, replace `/config/reelping.db` with your backup,
  start ReelPing.
- **Upgrade:** pull the new image / update the container. Migrations run
  automatically and take a backup first.
- **Uninstall:** remove the container. Delete `/mnt/user/appdata/reelping` to
  remove all data.

## Security & privacy

- Threat model and hardening: [docs/SECURITY.md](docs/SECURITY.md)
- Privacy statement: [docs/PRIVACY.md](docs/PRIVACY.md)

ReelPing makes outbound connections **only** to your configured Plex server and
Discord webhook. No telemetry, analytics, crash reporting, license checks,
fonts, or CDN assets.

## Troubleshooting

Platform-neutral guidance (Docker networking, firewalls, DNS, reverse proxies,
TLS) is in [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md).

## Known limitations

- A successful check proves reachability, not perfect playback.
- v1 monitors only; it never starts, stops, or restarts Plex.
- One Plex server and one Discord webhook per instance in v1.
- Automated `@everyone` is opt-in and never enabled by default.
- The "real Unraid" acceptance matrix is not yet field-completed (beta).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Issues and PRs welcome.

## Support

See [docs/SUPPORT.md](docs/SUPPORT.md). For security issues, follow
[.github/SECURITY.md](.github/SECURITY.md).

## Release readiness

This repository is **beta**. The automated test suite (unit, integration,
state-machine, security-redaction, and full-UI-render tests) passes, and the
container is designed to the Unraid Community Applications constraints. Items
still pending real-hardware verification are tracked in
[docs/TESTING.md](docs/TESTING.md) and
[docs/COMMUNITY-APPS-SUBMISSION.md](docs/COMMUNITY-APPS-SUBMISSION.md).

## License

[MIT](LICENSE). Third-party notices: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Trademark & non-affiliation

Plex® is a trademark of Plex, Inc. Discord® is a trademark of Discord Inc.
Unraid® is a trademark of Lime Technology, Inc. These names are used only to
describe compatibility. **ReelPing is an independent community project and is
not affiliated with or endorsed by Plex, Inc., Discord Inc., or Lime
Technology, Inc.** The ReelPing name and logo are original works.
