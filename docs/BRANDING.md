# ReelPing — Branding Guide

The ReelPing name and logo are **original works** created for this project.
They do not use the Plex chevron/logo, the Discord logo, the Unraid logo, or any
trademarked or copyrighted artwork.

## The mark

The logo combines three ideas — **media + status + notification**:

- A simplified **film reel** (a ring with a central hub and six perforations)
  represents the media server.
- Two concentric **pulse/signal arcs** radiating from the upper-right represent
  an outgoing status notification (the "ping").
- A small **availability status dot** (green, with a light border) represents
  service state.

It is built from geometric primitives only (circles, a ring, arcs), reads
clearly at 48×48 px, uses a transparent background, and contains no text.

## Files

| File | Use |
| --- | --- |
| `assets/reelping.svg` | Canonical full-colour vector logo |
| `assets/reelping-monochrome.svg` | Single-colour (`currentColor`) variant |
| `assets/favicon.svg` | Rounded-tile favicon |
| `assets/favicon.ico` | Multi-size ICO (16, 32) |
| `assets/reelping-64/128/256/512.png` | Raster icons |
| `assets/reelping-social-card.png` | 1280×640 social/OpenGraph card |

The SVGs use real vector paths and shapes — no embedded raster images, remote
assets, base64 blobs, tracking metadata, or external links.

## Regenerating the raster assets

The PNGs, `favicon.ico`, and the social card are produced deterministically by a
small Go generator that reproduces the canonical mark geometry:

```bash
go run ./tools/genassets        # or: scripts/generate-assets.sh
```

Re-running produces byte-identical output. (If you prefer to rasterise directly
from the SVG, any standard renderer such as `rsvg-convert` or Inkscape will also
work; the generator exists so no external tool is required.)

## Colours

| Role | Hex | Notes |
| --- | --- | --- |
| Teal (primary) | `#0f9d9d` | Reel + brand |
| Cyan (accent/online) | `#17b8b8` | Signal waves, links |
| Green (status: online) | `#2ecc71` | Availability dot |
| Orange (maintenance) | `#e08a1e` | Maintenance states |
| Amber (degraded) | `#f1c40f` | Degraded state |
| Red (offline) | `#e74c3c` | Outage states |
| Charcoal (dark bg) | `#12161c` | Dark surfaces |
| White | `#ffffff` | Monochrome-on-dark, dot border |

Status is always conveyed by **text and symbol**, never colour alone
(🟢 online, 🟡 maintenance, 🟠 degraded, 🔴 offline, 🔵 information).

## Usage

- **Light backgrounds:** use the full-colour or monochrome (dark ink) logo.
- **Dark backgrounds:** use the full-colour or monochrome (white ink) logo.
- **Clear space:** keep padding of at least 25% of the logo height on all sides.
- **Minimum size:** 24 px; the favicon variant is optimised for ≤ 32 px.
- **Monochrome:** `reelping-monochrome.svg` inherits `currentColor` — set the
  CSS/text colour of the container.

## Prohibited modifications

Do not:

- Recolour the mark to imitate Plex, Discord, or Unraid branding.
- Add the Plex chevron, Discord, or Unraid logos to it.
- Stretch, skew, rotate, or add effects that reduce legibility.
- Place text inside the icon.
- Imply official affiliation or endorsement.

## Third-party trademark disclaimer

Plex®, Discord®, and Unraid® are trademarks of their respective owners, used
only to describe compatibility. **ReelPing is an independent community project
and is not affiliated with or endorsed by Plex, Inc., Discord Inc., or Lime
Technology, Inc.**
