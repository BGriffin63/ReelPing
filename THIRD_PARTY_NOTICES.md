# Third-Party Notices

ReelPing bundles or depends on the following third-party open-source components.
Each is used under its own license; ReelPing is grateful to their maintainers.

## Runtime dependencies (compiled into the ReelPing binary)

| Component | Purpose | License |
| --- | --- | --- |
| [go.etcd.io/bbolt](https://github.com/etcd-io/bbolt) | Embedded key/value database | MIT |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | Argon2id password hashing | BSD-3-Clause |
| [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) | Low-level OS primitives (indirect) | BSD-3-Clause |
| Go standard library | HTTP, TLS, XML/JSON, `time/tzdata`, crypto | BSD-3-Clause |

## Build/tooling-only dependencies (NOT in the runtime image)

| Component | Purpose | License |
| --- | --- | --- |
| [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) | Raster asset generator (`tools/genassets`) | BSD-3-Clause |
| [golang.org/x/text](https://pkg.go.dev/golang.org/x/text) | Transitive dep of x/image | BSD-3-Clause |
| [Go Fonts](https://go.dev/blog/go-fonts) (gobold, goregular) | Wordmark on the social card | BSD-3-Clause |

The Go Fonts are used only to render the social-card PNG; they are not shipped
in the container image and are not embedded in the served web UI (which uses the
system font stack).

## Fonts in the web UI

The ReelPing web interface uses the operating system / browser default font
stack (`system-ui`, Segoe UI, Roboto, Helvetica, Arial). **No external or
proprietary fonts are downloaded or bundled.**

## Branding

The ReelPing name and logo are original works created for this project. They do
not incorporate the Plex, Discord, or Unraid logos or any trademarked artwork.

## Trademarks

Plex®, Discord®, and Unraid® are trademarks of their respective owners and are
used only to describe compatibility. ReelPing is not affiliated with or endorsed
by Plex, Inc., Discord Inc., or Lime Technology, Inc.
