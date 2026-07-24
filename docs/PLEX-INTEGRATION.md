# ReelPing — Plex Integration

ReelPing talks to Plex over HTTP(S) only. It never touches Plex's host OS,
configuration files, media, or Docker container.

## What works without a token

Basic availability monitoring needs **no token**. ReelPing uses Plex's
`GET /identity` endpoint, which returns a small XML document and does not
require authentication:

```xml
<MediaContainer size="0" claimed="1"
  machineIdentifier="6a646027a56abb6d..." version="1.40.0.1234" />
```

ReelPing treats a well-formed `MediaContainer` with a `machineIdentifier` as
"Plex is responding". Plex Pass is **not** required.

## What a token unlocks

Providing a Plex token (optional) enables:

- **Server identity verification** — the `machineIdentifier` is compared to an
  expected value you can store, so a *different* server answering (e.g. after a
  DNS/reverse-proxy mishap) is flagged as `identity_mismatch`.
- **Friendly server name** and **version** display.
- **Active stream count** — read from `GET /status/sessions` (only the `size`
  attribute is used).
- **Authentication-failure detection** — an expired/invalid token is reported as
  `authentication_failure` rather than a false outage.

## How ReelPing handles the token safely

- Sent **only** as the `X-Plex-Token` HTTP header — never in a URL or query
  string, so it can't leak via logs or redirects.
- Never displayed after saving (only a redacted hint like `••••abcd`).
- Never written to logs, diagnostics, or exports.
- Never placed in process arguments.
- Stored in the `/config` database with restrictive permissions.

ReelPing also sends `X-Plex-Client-Identifier: reelping-monitor` and
`X-Plex-Product: ReelPing` headers to identify itself.

## Privacy: what ReelPing does NOT read or send

Even with a token, ReelPing reads **only** the stream *count* from
`/status/sessions`. It never parses, stores, or transmits:

- Viewer usernames or email addresses
- Client IP addresses or device identifiers
- Media titles, viewing history, or playback positions

An optional privacy setting lets you include **only the active-stream count** in
maintenance announcements. Individual viewer data is never posted to Discord.

## Getting a Plex token

Follow Plex's official guide:
<https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/>

In short: open the Plex web app, view XML for any library item, and copy the
`X-Plex-Token` value from the URL. Paste it into ReelPing's setup wizard or
Settings → Plex → *Replace token*.

## Reverse proxies and TLS

- If Plex is behind HTTPS with a valid certificate, keep **Verify TLS** on.
- For self-signed certificates on a LAN, you may disable TLS verification (an
  explicit, logged choice). ReelPing still bounds redirects, response size, and
  time.
- ReelPing follows a limited number of redirects and never follows a redirect to
  a non-HTTP(S) scheme.

## Version 1 does not control Plex

ReelPing only *monitors and announces*. It never starts, stops, restarts, or
updates Plex, and never changes Plex settings or libraries. Automatic recovery
actions may be considered as a future, clearly-optional feature.
