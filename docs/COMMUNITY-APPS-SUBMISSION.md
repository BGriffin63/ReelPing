# ReelPing — Unraid Community Applications Submission

This is the gate and checklist for submitting ReelPing to Unraid Community
Applications (CA). **Do not submit — and do not claim "CA submission ready" —
until every item is genuinely done.**

Validate everything against the current official CA documentation:
<https://docs.unraid.net/community-applications/> and the CA submission help.

## 1. Public repository prerequisites

- [ ] Public GitHub repository.
- [ ] OSI-approved license present (`LICENSE`, MIT).
- [ ] README, security policy, support policy, privacy statement, changelog.
- [ ] No secrets committed anywhere (CI secret scan green).

## 2. License verification

- [ ] `LICENSE` is MIT and dependency licenses are compatible
      (`THIRD_PARTY_NOTICES.md`).

## 3. Container publishing

- [ ] Multi-arch image published to a public registry
      (`ghcr.io/<owner>/reelping`), at least `linux/amd64` (mandatory) and
      ideally `linux/arm64`.
- [ ] Exact-version tag, `major.minor`, and `latest` (stable only), plus digest.
- [ ] SBOM and provenance attached.

## 4. Template validation

- [ ] `templates/reelping.xml` is `<Container version="2">` and well-formed
      (`xmllint --noout`).
- [ ] Only documented fields are used (no invented XML fields).
- [ ] Maps only port `8080`→`8787` and `/config`; `TZ` variable; no secrets.
- [ ] `Privileged=false`; no Docker socket, host networking, or host paths.
- [ ] `WebUI`, `Icon`, `TemplateURL`, `Repository`, `Registry`, `Project`,
      `Support` are correct, reachable URLs.

## 5. ca_profile.xml validation

- [ ] `templates/ca_profile.xml` well-formed and fields filled in.

## 6. Icon validation

- [ ] Icon is a reachable HTTPS PNG (`assets/reelping-256.png`), square,
      readable at small sizes.

## 7. Listing preview

- [ ] Category, overview, and description render correctly in a CA preview.
- [ ] Non-affiliation disclaimer present in the overview.

## 8. Clean-install testing (real Unraid)

- [ ] Fresh install via the template pulls the **published** image and starts.
- [ ] WebUI reachable at `http://<IP>:8787`; setup wizard completes.
- [ ] Container health shows healthy; a Plex outage does not flip it unhealthy.

## 9. Upgrade testing

- [ ] Update to a newer tag preserves appdata; migration runs with a backup.

## 10. Uninstall testing

- [ ] Remove container; reinstall via **Previous Apps** reuses appdata.
- [ ] Deleting appdata removes all data.

## 11. Security review

- [ ] `docs/SECURITY.md` threat model reviewed.
- [ ] Trivy scan: no unfixed HIGH/CRITICAL.
- [ ] Secret-redaction tests pass; diagnostics/exports confirmed secret-free.
- [ ] Runs as non-root (`99:100`), `no-new-privileges`.

## 12. Submission portal steps

- [ ] Follow the current CA submission process (template repo / submission form
      per the official docs). Do not rely on outdated forum instructions.

## 13. Moderator-feedback workflow

- [ ] Monitor the submission thread; address moderator feedback promptly; keep
      the template `TemplateURL` stable.

## 14. Post-approval maintenance

- [ ] Keep the image updated (Dependabot PRs), respond to issues, publish
      changelogs, and keep the template in sync with new options.

## 15. Deprecation / removal process

- [ ] If ever discontinued, mark the template deprecated, communicate in the
      support thread, and follow CA's removal guidance so users aren't stranded.

---

### Real-hardware items still pending (as of this beta)

Sections **3, 8, 9, 10, 11 (Trivy on the published image)** require building and
publishing the image and testing on a real Unraid server. These are **not yet
done** in this repository and must be completed before submission. See
[TESTING.md](TESTING.md).
