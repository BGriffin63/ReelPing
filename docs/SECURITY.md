# ReelPing — Security & Threat Model

ReelPing can send Discord notifications and, if configured, mention an entire
Discord server. Its web interface is therefore **never** unauthenticated, and
its secrets are treated like passwords. This document describes the security
model and the threats considered.

To report a vulnerability, see [`.github/SECURITY.md`](../.github/SECURITY.md).

> ReelPing is an independent community project and is not affiliated with or
> endorsed by Plex, Inc., Discord Inc., or Lime Technology, Inc.

---

## 1. Security posture summary

- **Authentication:** single administrator account, Argon2id password hashing
  (PHC format, `m=64MiB, t=3, p=4`, 16-byte salt, 32-byte key). No default
  credentials; the app refuses to do anything until an admin is created.
- **Sessions:** cryptographically random 256-bit session IDs, server-side
  storage, `HttpOnly` + `SameSite=Lax` cookies, `Secure` when served over
  HTTPS, rotation on login, idle + absolute expiry, "sign out other sessions".
- **CSRF:** double-submit token required on every state-changing request.
- **Rate limiting:** login attempts and manual announcements are rate limited.
- **Secrets:** Plex token and Discord webhook are stored with restrictive file
  permissions and are never rendered, logged, exported, or placed in URLs,
  process arguments, or diagnostics. Only a redacted hint (`••••abcd`) is shown.
- **Transport:** the Discord webhook must be HTTPS on a known Discord host.
- **Headers:** CSP (self only, no inline script via nonce), `X-Content-Type-Options`,
  `Referrer-Policy`, `frame-ancestors 'none'`, `Permissions-Policy`, and
  `Cache-Control: no-store` on authenticated pages.
- **Container:** non-root, no privileged mode, no Docker socket, no host paths
  beyond `/config`, static binary, minimal image.

---

## 2. Threat model

| Threat | Mitigation |
| --- | --- |
| **Stolen Discord webhook** | Stored redacted-at-read with `0600` perms; never shown/logged/exported. Host allow-list limits misuse. Rotate via WebUI. |
| **Stolen Plex token** | Same storage protections; sent only as `X-Plex-Token` header to the configured host; never in URLs/logs/exports. |
| **Weak admin password** | Strength meter + minimum policy; common-password rejection; Argon2id makes offline cracking costly. |
| **Brute-force login** | Per-username + per-IP rate limiting with lockout backoff; failures audited (never the password). |
| **CSRF** | Per-session token, required and constant-time-compared on all mutating routes; `SameSite=Lax` cookies. |
| **Stored XSS** | All persisted user text is HTML-escaped on output via `html/template`; no `innerHTML`, no inline event handlers. |
| **Reflected XSS** | Same escaping; error messages are static/whitelisted, never echo raw input into HTML. |
| **Command injection** | No shell is ever invoked; all networking uses native Go (`net`, `net/http`). No `os/exec` on user input. |
| **Header injection** | Inputs with newlines/control chars are rejected before use in URLs or outbound requests. |
| **URL parsing attacks / SSRF** | Only `http`/`https`; credentials-in-URL rejected; redirects bounded and never to non-HTTP(S); response size/time bounded. Plex target is admin-controlled by design (documented). |
| **DNS rebinding** | Checks are read-only, bounded, and never expose fetched bodies as HTML or execute them; identity is verified against an expected machine identifier when configured. |
| **Malicious / malformed Plex responses (XML/JSON)** | Size-bounded reader; tolerant decoder with limits; parse failure → `invalid_response`, never a crash. |
| **Oversized responses** | `io.LimitReader` caps every external body; excess is an error. |
| **Database corruption** | bbolt ACID; validate-before/after migration; atomic backup kept on failure. |
| **Path traversal / symlink attacks** | No user-controlled filesystem paths; `/config` is the only writable location; backups use `O_EXCL` temp files + atomic rename. |
| **Race conditions / concurrent announcements** | Single monitor goroutine owns state; storage transactions are serialised; idempotency keys dedupe. |
| **Duplicate incidents / notification floods** | Hysteresis thresholds, one-incident-per-outage, cooldown, hard retry cap, no per-poll notifications. |
| **Discord rate limiting (429)** | `retry_after` honoured (bounded), exponential backoff + jitter, max retries, then give up and record the failure. |
| **Unexpected ReelPing restart / Unraid reboot** | Persisted state + incident pointer; stabilisation window; no false outage, no duplicate incident on restart. |
| **Clock changes / DST transitions** | Monotonic durations; UTC storage; localised display. |
| **Reverse-proxy misconfiguration** | Forwarded headers ignored unless a trusted proxy is explicitly configured. |
| **Attacker with access to `/config`** | Treated as full compromise (documented). Secrets rely on filesystem permissions; the recovery procedure requires this access. Encrypt the host volume for defence in depth. |

---

## 3. Secret handling honesty statement

ReelPing stores the Plex token and Discord webhook in its bbolt database under
`/config` with restrictive permissions (`0600` file, non-root owner).

Encrypting these at rest with a key that necessarily lives **beside** the
ciphertext in the same volume would provide little real protection against an
attacker who can already read `/config`. Rather than pretend otherwise, ReelPing
relies on filesystem permissions and honest documentation: **anyone who can read
`/config` can read the secrets.** Protect that volume (host-disk encryption,
correct permissions, limited access). This is stated plainly in the UI and docs.

---

## 4. Outbound connections (complete list)

1. The **configured Plex base URL** (monitoring + optional token calls).
2. The **configured Discord webhook** (notifications only).

There are **no** other outbound connections: no telemetry, analytics, crash
reporting, license checks, update pings, remote fonts, or CDN assets.

---

## 5. Local recovery procedure

If you are locked out (lost password), you recover by proving control of
`/config`, not via email:

1. Stop the ReelPing container.
2. Back up `/config/reelping.db`.
3. Run the container (or the binary) with the documented recovery flag
   `reelping -reset-admin` mounted against the same `/config`. This clears only
   the administrator credential and forces the first-run wizard on next start;
   it does **not** touch Plex/Discord config, incidents, or history.
4. Start ReelPing and create a new administrator.

See `docs/TROUBLESHOOTING.md` for the exact commands.
