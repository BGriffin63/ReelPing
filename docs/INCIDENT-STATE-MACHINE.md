# ReelPing — Monitoring State Machine

ReelPing models Plex availability as an explicit, persisted state machine. The
machine is deliberately hysteretic: a single failed or successful check never
changes the world. Outages are only *confirmed* after N consecutive failures,
and recovery only after M consecutive successes. This is what turns a noisy
20-second poll into "approximately one clear notification per real event".

> Defaults referenced below are the **Balanced** preset: check interval 20 s,
> failure threshold 3, recovery threshold 2, startup stabilisation 60 s.

---

## 1. States

| State | Meaning |
| --- | --- |
| `disabled` | Monitoring is off (setup incomplete or admin disabled it). |
| `initializing` | Worker is starting; inside the startup stabilisation window. |
| `unknown` | No confident reading yet. |
| `online` | Plex responded successfully on the most recent confirmed reading. |
| `suspect` | At least one recent failure, but the failure threshold is not yet met. No notification sent. |
| `offline` | Failure threshold met; an incident is open and (if enabled) an outage notification was sent. |
| `recovering` | Plex responded again after an outage, but the recovery threshold is not yet met. |
| `degraded` | Plex is reachable but impaired (e.g. authenticated API failing, sustained high latency). Informational; no auto-notification by default. |
| `maintenance-online` | Maintenance mode active and Plex currently reachable. |
| `maintenance-offline` | Maintenance mode active and Plex currently unreachable (expected — no outage alert). |

---

## 2. Automatic outage / recovery transitions

```mermaid
stateDiagram-v2
    [*] --> initializing
    initializing --> online: first success (after stabilization)
    initializing --> unknown: stabilization ends, no clear reading

    online --> suspect: check fails (1st)
    suspect --> suspect: check fails (< threshold)
    suspect --> online: check succeeds (reset counters)
    suspect --> offline: consecutive failures == threshold\n(open incident, send ONE outage alert)

    offline --> offline: still failing\n(update incident duration, no new alert)
    offline --> recovering: check succeeds (1st)
    recovering --> offline: check fails (reopen same incident)
    recovering --> online: consecutive successes == recovery threshold\n(close incident, send ONE recovery alert)

    online --> degraded: impaired over threshold
    degraded --> online: healthy again
```

### Worked example (Balanced preset)

| Tick | Result | State before → after | Notification |
| --- | --- | --- | --- |
| 1 | fail | `online` → `suspect` | none |
| 2 | fail | `suspect` → `suspect` | none |
| 3 | fail | `suspect` → `offline` | **1 outage alert**, incident opened |
| 4–20 | fail | `offline` → `offline` | none (duration updated) |
| 21 | success | `offline` → `recovering` | none |
| 22 | success | `recovering` → `online` | **1 recovery alert**, incident closed with duration |

With a 20 s interval and threshold 3, the outage is confirmed roughly
`interval × threshold = 60 s` after Plex actually goes down (actual time varies
by where the failure lands relative to the polling schedule).

---

## 3. Maintenance transitions

When the administrator starts or schedules maintenance, ReelPing enters
maintenance mode. While in maintenance, an unreachable Plex is *expected* and
does **not** raise an unexpected-outage alert.

```mermaid
stateDiagram-v2
    online --> maintenance_online: admin starts maintenance
    maintenance_online --> maintenance_offline: Plex becomes unreachable\n(recorded, NO outage alert)
    maintenance_offline --> maintenance_online: Plex reachable again
    maintenance_online --> online: admin ends maintenance (Plex up)
    maintenance_offline --> suspect: admin ends maintenance (Plex down)\n(re-enters normal detection)
```

- Ending maintenance while Plex is still down drops back into the normal
  detection path (`suspect`), so a genuine post-maintenance outage will still be
  caught.
- A recovery/"service restored" announcement can be sent automatically when Plex
  returns during a maintenance window, if the admin enabled that option.

---

## 4. Restart safety

On startup the worker:

1. Loads the persisted `monitor_state` snapshot and any open incident.
2. Enters `initializing` for the stabilisation window and performs fresh checks.
3. **Does not** emit an outage alert merely because ReelPing restarted.
4. **Does not** open a duplicate incident — if an incident was already open, it
   continues that same incident.
5. Restores an active maintenance window if one was in effect.

```mermaid
stateDiagram-v2
    [*] --> load_state: process start
    load_state --> initializing: stabilization window
    initializing --> online: confirmed up
    initializing --> offline: incident already open AND still failing\n(continue SAME incident, no new alert)
    initializing --> maintenance_online: maintenance was active
```

---

## 5. Counters and idempotency

- `consecutiveFailures` / `consecutiveSuccesses` are persisted so thresholds
  survive restarts.
- Each incident has a unique short ID. The outage alert and recovery alert for
  an incident each carry an idempotency key derived from the incident ID and
  phase, so a retry after recovery can never double-send.
- A notification cooldown prevents rapid re-alerting for flapping services.

---

## 6. Edge cases explicitly handled

- **Out-of-order / duplicate checks:** the worker is single-threaded and
  ticker-driven; a slow check cannot overlap the next one (runs are skipped, not
  stacked), so ordering is guaranteed.
- **Clock jumps / DST:** durations use a monotonic clock; only display uses wall
  time in the configured zone.
- **Threshold changes mid-incident:** applied to future evaluations; an already
  open incident stays open and is evaluated against the current recovery
  threshold.
- **Flapping:** cooldown + hysteresis prevent alert storms.
