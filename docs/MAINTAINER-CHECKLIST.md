# ReelPing — Maintainer Checklist

Before publishing your fork/instance or submitting to Unraid Community
Applications, replace the placeholders and complete the checks below.

## Placeholders to replace

Search the repository for each token and replace it:

| Placeholder | Where it appears | Replace with |
| --- | --- | --- |
| `YOUR_GITHUB_USERNAME` | docs (generic examples) | Your GitHub username/org |
| `YOUR_DISPLAY_NAME` | `LICENSE`, `templates/ca_profile.xml` | Your name/handle |
| `YOUR_SUPPORT_THREAD_URL` | `templates/*.xml`, `.github/SECURITY.md`, docs | Your Unraid forum support thread URL |
| `YOUR_PROJECT_URL` | docs (generic) | Your project homepage/repo URL |
| `YOUR_CONTAINER_REGISTRY` | docs (generic) | Your registry, e.g. `ghcr.io/<you>` |

> The concrete references to `BGriffin63` and `ghcr.io/BGriffin63/reelping` match
> the suggested image name. If you fork under a different owner, update:
> `go.mod` module path is fine to keep, but change the **Repository**,
> **Registry**, **Icon**, **TemplateURL**, and **Project** URLs in
> `templates/reelping.xml` and `templates/ca_profile.xml`, the image name in
> `compose.yaml` and `README.md`, and the release workflow image reference.

## Identity / branding

- [ ] Logo and icons are the original ReelPing marks (do not add third-party
      logos).
- [ ] The non-affiliation disclaimer appears in README, footer, CA overview, and
      docs.

## Publishing

- [ ] Repository is public with the MIT `LICENSE`.
- [ ] GHCR package is public (`ghcr.io/<owner>/reelping`).
- [ ] A tagged release has been built and pushed (see `docs/RELEASE.md`).
- [ ] `templates/reelping.xml` `TemplateURL`, `Icon`, `Repository`, `Registry`,
      `Project`, and `Support` point at real URLs.
- [ ] `templates/ca_profile.xml` filled in.

## Verification (must actually be performed)

- [ ] `go test ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] Container builds; starts; `/healthz` healthy; `-healthcheck` exits 0.
- [ ] Real Discord test message delivered.
- [ ] Real Plex outage + recovery observed (Docker **and** VM).
- [ ] Clean install / upgrade / uninstall tested on real Unraid.
- [ ] Trivy scan clean of HIGH/CRITICAL (fixable); SBOM generated.

See [COMMUNITY-APPS-SUBMISSION.md](COMMUNITY-APPS-SUBMISSION.md) for the full
gate.
