# ReelPing — Troubleshooting

Start with **Test connection** (setup wizard or Settings → Plex). ReelPing's
multi-stage check tells you *which stage* failed, which usually pinpoints the
cause. Match the classification below.

## Reading the classification

| Classification | Meaning | Look at |
| --- | --- | --- |
| `dns_failure` | Hostname didn't resolve | DNS / split-DNS / hostname typo |
| `plex_service_unreachable` | Host up, Plex port closed/refused | Plex running? correct port? firewall |
| `host_unreachable` | Host didn't answer at all | Host powered off? network/routing |
| `request_timeout` | Connection/HTTP too slow | Firewall dropping packets? overloaded host |
| `tls_failure` | HTTPS/cert problem | Cert validity / disable TLS verify on LAN |
| `response_error` | Unexpected HTTP status | Reverse proxy returning an error page |
| `invalid_response` | Not recognisably Plex | Wrong URL/port (hitting another service) |
| `authentication_failure` | Token rejected | Replace the Plex token |
| `identity_mismatch` | Different server answered | DNS/proxy pointing elsewhere; fix or clear expected machine ID |

## Docker / Unraid networking

- **Use the LAN IP, not a container IP.** For Plex in an Unraid Docker
  container, use `http://UNRAID_LAN_IP:32400`. Internal Docker IPs
  (`172.x`) are not stable.
- **Bridge network:** ReelPing on the default bridge can reach other hosts on
  your LAN. It does not need to share Plex's network.
- **Custom Docker networks:** if you insist on container-name addressing, both
  containers must be on the same user-defined network. The LAN address is
  simpler and more robust.
- **Port publishing:** the Plex container must publish `32400` (or your chosen
  port) to the host.

## VM networking

- **Bridged:** the VM has its own LAN IP — use it directly.
- **NAT:** you likely need a host→VM port forward for `32400`.
- Confirm the guest firewall allows inbound `32400` (see below).

## Firewalls

- **Windows Defender Firewall:** add an inbound rule allowing TCP `32400`.
- **Linux (`ufw`):** `sudo ufw allow 32400/tcp`.
- **Linux (`firewalld`):** `sudo firewall-cmd --add-port=32400/tcp --permanent && sudo firewall-cmd --reload`.
- **NAS:** enable the Plex package's port in the NAS firewall/security panel.

## DNS

- The name must resolve **from ReelPing's container**, which may differ from
  your workstation (split DNS). Test with an IP first; if the IP works but the
  name doesn't, it's DNS.
- Set the container `TZ` correctly so timestamps read right (does not affect
  reachability).

## Reverse proxy

- Point ReelPing at the proxy URL (`https://plex.example.com`). ReelPing follows
  a bounded number of redirects and refuses non-HTTP(S) redirect targets.
- A `response_error` often means the proxy returned an error/landing page — check
  the proxy's upstream and path.

## TLS / HTTPS

- Keep **Verify TLS** on with valid certificates.
- For self-signed certs on a trusted LAN, disable TLS verification (an explicit,
  logged choice). A `tls_failure` with verification on usually means an untrusted
  or hostname-mismatched certificate.

## Discord

- **Test webhook fails:** re-copy the webhook URL (Server Settings → Integrations
  → Webhooks). It must be `https://discord.com/api/webhooks/{id}/{token}`.
- **Deleted/expired webhook:** ReelPing reports a `4xx` and does not retry — create
  a new webhook and replace it in Settings → Discord.
- **No ping when expected:** check the mention policy (default is *No mention*).
  `@everyone`/`@here` must be explicitly selected.

## The container won't stay healthy

The health check tests ReelPing, not Plex. If it's unhealthy, ReelPing itself has
a problem (usually `/config` not writable). Check that the appdata path exists
and is writable by `99:100`, and read the container logs.

## Locked out (password recovery)

Use the local recovery flag; it clears only the admin credential (not your Plex/
Discord config or history):

```bash
# Unraid: stop the container, then run once against the same appdata:
docker run --rm -v /mnt/user/appdata/reelping:/config ghcr.io/BGriffin63/reelping:latest -reset-admin
# Compose:
docker compose run --rm reelping -reset-admin
```

Start ReelPing again and the first-run wizard will let you create a new admin.

## Still stuck?

Download **Diagnostics** (Settings → Data) — it contains no secrets — and open a
support request per [SUPPORT.md](SUPPORT.md). Never paste your token or webhook.
