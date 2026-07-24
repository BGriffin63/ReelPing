# ReelPing — Plex Deployment Types

ReelPing monitors a **network endpoint**. It does not care how or where Plex is
hosted, as long as ReelPing's container can reach the URL you give it.

Three things are always distinct:

1. **Where ReelPing runs** (e.g. an Unraid Docker container).
2. **Where Plex runs** (Docker, a VM, another NAS, another server…).
3. **The URL ReelPing uses to reach Plex.**

Below, `UNRAID_LAN_IP`, `VM_LAN_IP`, etc. are the **LAN IP addresses** of those
machines (not Docker-internal IPs). Prefer a stable LAN address or hostname, not
a temporary container IP.

## Plex as an Unraid Docker container

```
http://UNRAID_LAN_IP:32400
```

Use the Unraid host's LAN IP and Plex's **published** port. Do **not** use the
container's internal Docker IP (it can change). If ReelPing and Plex are on the
same custom Docker network you *could* use the container name, but the LAN
address is the robust default.

## Plex in Docker on another host

```
http://SERVER_LAN_IP:32400
```

Ensure the port is published on that host and any firewall allows it.

## Plex in a Windows VM

```
http://VM_LAN_IP:32400
```

- **Bridged networking:** the VM gets its own LAN IP — use that.
- **NAT networking:** you may need a port forward from the host to the VM.
- Check **Windows Defender Firewall** allows inbound `32400`.

## Plex in a Linux VM

```
http://VM_LAN_IP:32400
```

- Bridged vs NAT as above.
- Check the Linux firewall (`ufw`, `firewalld`, `iptables`, or `nftables`)
  allows inbound `32400`.

## Plex installed directly on Windows

```
http://HOST_LAN_IP:32400
```

Allow inbound `32400` in Windows Defender Firewall.

## Plex installed directly on Linux

```
http://HOST_LAN_IP:32400
```

Allow inbound `32400` in the host firewall.

## Plex on another NAS (Synology, QNAP, etc.)

```
http://NAS_LAN_IP:32400
```

Enable the Plex package's port and any NAS firewall rule.

## Plex on another physical server

```
http://SERVER_LAN_IP:32400
```

## Plex behind a reverse proxy

```
https://plex.example.com
https://plex.example.internal
```

- Keep **Verify TLS** on when the certificate is valid.
- If you use split DNS, make sure the name resolves correctly **from ReelPing's
  container** — it may differ from your workstation.
- ReelPing follows a bounded number of redirects and refuses non-HTTP(S)
  redirect targets.

## Reachable by hostname or IP

Both work:

```
http://plex-vm.local:32400
http://plex-server.internal:32400
http://192.168.1.10:32400
http://10.1.9.100:32400
```

## Verifying reachability

Use ReelPing's **Test connection** button (setup wizard or Settings → Plex). It
runs the full multi-stage check and reports exactly which stage failed (DNS,
TCP, TLS, HTTP, identity, or auth), which usually points straight at the fix.
See [TROUBLESHOOTING.md](TROUBLESHOOTING.md).
