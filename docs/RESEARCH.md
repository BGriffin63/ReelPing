# ReelPing — Research Notes

This document records the official and reference sources consulted while
designing ReelPing, the relevant findings, and the date each source was
accessed. It exists so that reviewers can verify that ReelPing's behaviour is
grounded in current documentation rather than in outdated forum posts or
assumptions.

> All sources below were accessed on **2026-07-24** unless otherwise noted.

ReelPing is an independent community project and is not affiliated with or
endorsed by Plex, Inc., Discord Inc., or Lime Technology, Inc. Third-party
names are used only to describe compatibility.

---

## 1. Plex Media Server API

### 1.1 Server identity — primary health signal

- **Source:** Plexopedia — *Plex API: Server Identity*
  <https://www.plexopedia.com/plex-media-server/api/server/identity/>
- **Source:** Plex developer docs — <https://developer.plex.tv/pms/>

**Findings:**

- `GET /identity` returns a small XML document and, critically, **does not
  require a Plex token**. It can be reached from any host that can talk to the
  server over HTTP. This makes it the ideal lightweight, unauthenticated
  availability probe.
- The response is a `MediaContainer` element with attributes:
  - `machineIdentifier` — a stable unique server ID (used by ReelPing for
    identity verification).
  - `version` — the running Plex version.
  - `claimed` — `1`/`0`.
  - `size` — number of child items.
- Because `/identity` needs no authentication and returns the machine
  identifier, ReelPing uses it as the **Stage 4 HTTP probe** and, when an
  expected machine identifier is configured, as an **identity check** as well.

**Design decision:** ReelPing's default HTTP probe path is `/identity`. It
parses the returned XML defensively (bounded reader, tolerant of extra fields),
and treats a well-formed `MediaContainer` with a `machineIdentifier` as
"Plex is responding".

### 1.2 Token authentication

- **Source:** Plex Support — *Finding an authentication token / X-Plex-Token*
  <https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/>

**Findings:**

- Plex accepts the token either as the `X-Plex-Token` HTTP header or as an
  `X-Plex-Token` query parameter.
- Plex accepts additional `X-Plex-*` headers; `X-Plex-Client-Identifier`,
  `X-Plex-Product`, and `X-Plex-Version` identify the calling client.

**Design decision:** ReelPing **always sends the token in the `X-Plex-Token`
HTTP header**, never in the URL/query string, so the token never appears in
logs, redirect targets, or rendered URLs. ReelPing also sends
`X-Plex-Client-Identifier`, `X-Plex-Product: ReelPing`, and
`X-Plex-Version` headers.

### 1.3 Active sessions

- **Source:** Plex developer docs, `/status/sessions`
  <https://developer.plex.tv/pms/>

**Findings:**

- `GET /status/sessions` (token required) returns a `MediaContainer` whose
  `size` attribute is the count of active playback sessions, and whose children
  describe each session (user, device, media — all sensitive).

**Design decision:** When a token is present, ReelPing reads `/status/sessions`
**only to obtain the `size` (stream count)**. It never parses, stores, or
transmits per-session child data (usernames, IPs, media titles). The count is
the only value surfaced, and only when the administrator explicitly opts in.

---

## 2. Discord incoming webhooks

- **Source:** Discord Developer Documentation — *Webhook Resource*
  <https://docs.discord.com/developers/resources/webhook>
- **Source:** Discord Webhook Guide (limits reference)
  <https://discord-webhook.com/en/discord-webhook-guide/>

**Findings:**

- **URL format:** `https://discord.com/api/webhooks/{webhook.id}/{webhook.token}`.
  The token segment is a secret. `discordapp.com` is a legacy alias host.
- **Execute:** `POST` the webhook URL with a JSON body. Adding `?wait=true`
  makes Discord return the created message (and validation errors) rather than
  a bare `204`.
- **`allowed_mentions` object** controls which mentions actually ping:
  - `parse`: array that may contain `"roles"`, `"users"`, `"everyone"`.
    An **empty `parse` array suppresses all mentions**, including literal
    `@everyone`/`@here` text in the content.
  - `roles`: explicit list of role IDs allowed to ping.
  - `users`: explicit list of user IDs allowed to ping.
- **Embed limits:** title ≤ 256, description ≤ 4096, field name ≤ 256, field
  value ≤ 1024, ≤ 25 fields, footer ≤ 2048, author name ≤ 256, ≤ 10 embeds per
  message, and **≤ 6000 total characters across all embeds**.
- **Content limit:** 2000 characters.
- **Rate limiting:** roughly 30 requests/minute per webhook route. On `429`,
  Discord returns a JSON body with `retry_after` (seconds, may be fractional)
  and a `Retry-After` header.

**Design decisions:**

- ReelPing **always sends an explicit `allowed_mentions`** on every message.
  The default is `{"parse": []}` (no mentions). `@everyone`/`@here` is only
  possible when the administrator explicitly enables it, and role mentions
  require a numeric role ID that ReelPing places in `allowed_mentions.roles`.
- ReelPing validates that the webhook host is one of Discord's known hosts
  (`discord.com`, `discordapp.com`, `canary.discord.com`, `ptb.discord.com`)
  and that the scheme is HTTPS, before ever sending.
- All embed fields are length-clamped to the documented limits and the total
  6000-character budget before sending.
- On `429`, ReelPing honours `retry_after` (bounded), uses exponential backoff
  with jitter for other transient errors, and enforces a hard maximum retry
  count so it can never enter a retry storm.

---

## 3. Unraid Community Applications

- **Source:** Unraid Docs — *Community Applications*
  <https://docs.unraid.net/community-applications/>
- **Source:** Community Apps — *Repository XML Format*
  <https://ca.unraid.net/submit/help/repository-xml>
- **Source:** Selfhosters.net — *Writing a template compatible for Unraid*
  <https://selfhosters.net/docker/templating/templating/>

**Findings:**

- A container template is an XML file rooted at `<Container version="2">`.
- Common top-level elements: `Name`, `Repository`, `Registry`, `Network`,
  `Privileged`, `Support`, `Project`, `Overview`, `Category`, `WebUI`,
  `TemplateURL`, `Icon`, `ExtraParams`, `Requires`.
- `WebUI` uses the pattern `http://[IP]:[PORT:8080]`.
- `<Config>` elements describe ports, paths, variables, labels and devices with
  attributes: `Name`, `Target`, `Default`, `Mode`, `Description`,
  `Type` (`Path|Port|Variable|Label|Device`), `Display`
  (`always|advanced|always-hide|advanced-hide`), `Required`, `Mask`.
- `ca_profile.xml` describes the developer/maintainer profile shown in CA.
- Deprecated/discouraged elements (`DateInstalled`, `DonateText`, `MyIP`) are
  omitted.

**Design decisions:**

- ReelPing ships a single `<Container version="2">` template
  (`templates/reelping.xml`) that maps only:
  - Port `8080` → host `8787` (`Type="Port"`).
  - `/config` → `/mnt/user/appdata/reelping` (`Type="Path"`, `Mode="rw"`).
  - `TZ`, `PUID`, `PGID`, `UMASK` environment variables.
- **No secrets** (Plex token, Discord webhook) are present in the template.
  They are entered through the authenticated WebUI only.
- The template requests **no** privileged mode, Docker socket, host networking,
  or extra host paths — matching the Unraid acceptance constraints.

---

## 4. Password hashing — Argon2id

- **Source:** OWASP Password Storage Cheat Sheet (Argon2id guidance).
- **Source:** `golang.org/x/crypto/argon2` package documentation.

**Findings & chosen parameters:**

- Argon2id is the recommended password KDF. A reasonable modern configuration
  for a self-hosted single-admin service: memory 64 MiB, iterations (time) 3,
  parallelism 2–4, 16-byte salt, 32-byte key.

**Design decision:** ReelPing uses `argon2.IDKey` with `time=3`, `memory=64MiB`,
`threads=4`, 16-byte random salt, 32-byte output. Hashes are stored in the
standard PHC string format (`$argon2id$v=19$m=...,t=...,p=...$salt$hash`) so the
parameters travel with the hash and can be upgraded later.

---

## 5. Embedded storage — bbolt

- **Source:** `go.etcd.io/bbolt` package documentation and README.

**Findings:**

- bbolt is a pure-Go, single-file, B+tree key/value store with fully
  serializable ACID transactions and no external service. It is actively
  maintained (etcd project).

**Design decisions:**

- ReelPing stores everything in a single bbolt file at `/config/reelping.db`.
- A `meta` bucket stores a schema version; migrations run inside a transaction
  after an atomic file backup is taken.
- No SQLite/cgo is used, keeping the container fully static and shell-free.

---

## 6. Container / Go security baseline

- **Source:** Go release notes (current stable **go1.26.5**, pinned).
- **Source:** Docker/OCI multi-stage build and non-root guidance.

**Design decisions:**

- Multi-stage Docker build: `golang:1.26` builder → `gcr.io/distroless/static`
  (or `scratch` + CA bundle) final image, `CGO_ENABLED=0`, static binary.
- Runs as a non-root numeric UID, exposes only `8080`, persists only `/config`,
  ships a self-contained `/reelping -healthcheck` command for the Docker
  `HEALTHCHECK`, and handles `SIGTERM` for graceful shutdown.

---

## 7. Things deliberately NOT done (and why)

- **No Discord bot / gateway** — one-way webhooks satisfy every notification
  requirement with far less attack surface (see the product spec).
- **No reliance on Plex outgoing webhooks** — a crashed/frozen/powered-off Plex
  cannot send a "goodbye" webhook, so ReelPing polls actively instead.
- **No ICMP as the primary check** — a host can answer ping while Plex is down;
  ICMP is only a *supplemental* signal used to distinguish "service down" from
  "host unreachable", and is skipped entirely when unprivileged ICMP is not
  available.
- **No Docker socket / privileged mode / host paths** — ReelPing reaches Plex
  purely over the network, so it needs none of these, satisfying Unraid CA
  security expectations.
