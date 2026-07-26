# Security Policy

## Supported versions

ReelPing is in beta. Security fixes target the latest released version and
`main`. Please keep your container updated.

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Instead, report privately using GitHub's
[private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
on this repository (Security → Report a vulnerability), or open a minimal
non-sensitive issue asking a maintainer to make contact at
<https://github.com/BGriffin63/ReelPing/issues>.

Please include:

- A description of the issue and its impact.
- Steps to reproduce (a minimal proof of concept if possible).
- Affected version(s) and configuration.

We aim to acknowledge reports within a few days and to provide a remediation
timeline after triage. Please give us a reasonable window to fix the issue
before public disclosure.

## Scope highlights

ReelPing's threat model is documented in
[docs/SECURITY.md](../docs/SECURITY.md). Of particular interest:

- Authentication, session, and CSRF handling.
- Secret redaction (Plex token, Discord webhook) across logs, diagnostics,
  exports, and rendered HTML.
- Discord mention safety (`allowed_mentions`).
- URL/SSRF handling for the admin-configured Plex target.

## Out of scope

- Anyone with read access to the `/config` volume can read stored secrets by
  design (documented). Protect that volume.
- Issues requiring a pre-compromised host or a malicious administrator.
