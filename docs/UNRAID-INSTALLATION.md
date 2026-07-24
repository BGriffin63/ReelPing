# ReelPing — Unraid Installation

ReelPing installs as a standard Docker container. It requires **no** privileged
mode, Docker socket, host networking, VM access, or media/Plex-config mappings.

## A. Install via Community Applications (once listed)

1. In Unraid, go to the **Apps** tab.
2. Search for **ReelPing** and click **Install**.
3. Review the template:
   - **WebUI Port** — host port for the UI (default `8787`).
   - **Appdata** — `/mnt/user/appdata/reelping` (container path `/config`).
   - **Time Zone** — set to your zone (e.g. `America/New_York`).
   - Leave PUID/PGID/UMASK at defaults unless you have a reason to change them.
4. Click **Apply** and wait for the image to pull and start.
5. Click the ReelPing icon → **WebUI**, or browse to
   `http://<UNRAID-IP>:8787`.

## B. Install during beta (template repository / manual template)

Until ReelPing is listed in CA, you can add it manually:

1. **Apps → (gear) → Manage Repositories** and add the template repository, **or**
2. Docker tab → **Add Container**, set:
   - **Repository:** `ghcr.io/BGriffin63/reelping:latest`
   - **Network Type:** `Bridge`
   - **WebUI:** `http://[IP]:[PORT:8080]`
   - **Port:** container `8080` → host `8787` (TCP)
   - **Path:** container `/config` → host `/mnt/user/appdata/reelping` (RW)
   - **Variable:** `TZ` → your time zone
   - Leave **Privileged** off. Add extra param `--security-opt no-new-privileges`.
3. **Apply**.

The provided template is `templates/reelping.xml`.

## First run

Open the WebUI and complete the setup wizard (administrator account → Plex URL →
Discord webhook → monitoring preset → enable). See the README's *First-run
setup*. Enter secrets **only** in the WebUI.

### Pointing ReelPing at Plex

If Plex is an Unraid Docker container, use the **Unraid host's LAN IP** and
Plex's published port, e.g. `http://192.168.1.50:32400` — not the container's
internal Docker IP. For other hosting, see
[PLEX-DEPLOYMENT-TYPES.md](PLEX-DEPLOYMENT-TYPES.md).

## Upgrade

Unraid → Docker → **check for updates** → **apply update** (or `docker compose
pull && up -d`). ReelPing runs any database migration automatically and takes a
backup first. Your settings persist in appdata.

## Backup & restore

- **Backup:** WebUI → Settings → Data → **Download backup**, or copy
  `/mnt/user/appdata/reelping/reelping.db`. The backup contains secrets — store
  it securely. (The Unraid **CA Appdata Backup** plugin covers appdata too.)
- **Restore:** stop the container, replace `reelping.db` with your backup, start.

## Uninstall

Remove the container from the Docker tab. To delete all data, also remove
`/mnt/user/appdata/reelping`. Reinstall via **Previous Apps** to reuse existing
appdata.

## Health

Unraid shows the container health from ReelPing's built-in health check, which
tests **ReelPing itself**. A Plex outage will **not** mark the container
unhealthy.

## Troubleshooting

See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for networking, firewall, DNS,
reverse-proxy, and TLS guidance.
