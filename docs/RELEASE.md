# ReelPing — Release Process

ReelPing uses [Semantic Versioning](https://semver.org/). Suggested progression:

```
0.1.0-alpha → 0.5.0-beta → 0.9.0-rc1 → 1.0.0
```

Pre-release tags (containing `-`) never get the `latest` Docker tag.

## Versioning sources

- `VERSION` holds the human-readable version.
- The build injects `Version`, `Commit`, and `Date` into
  `internal/version` via `-ldflags` (see `scripts/build.sh` and the `Dockerfile`
  build args).

## Cutting a release

1. Ensure `main` is green: `go test ./...`, `go vet ./...`, `gofmt -l .`.
2. Update `VERSION` and `CHANGELOG.md` (move Unreleased → the new version).
3. Commit, then tag:

   ```bash
   git tag -a v0.5.0-beta -m "ReelPing 0.5.0-beta"
   git push origin v0.5.0-beta
   ```

4. The `release` workflow then:
   - Builds and pushes a **multi-arch** image (`linux/amd64`, `linux/arm64`) to
     GHCR with exact-version, `major.minor`, and (for stable) `latest` tags,
     plus an image **digest**.
   - Attaches an **SBOM** and build **provenance**.
   - Creates a GitHub Release with generated notes and attaches the SBOM and the
     Unraid templates.

## What gets published

- `ghcr.io/BGriffin63/reelping:<version>`
- `ghcr.io/BGriffin63/reelping:<major.minor>`
- `ghcr.io/BGriffin63/reelping:latest` (stable only)
- Image digest, SBOM (SPDX), source archive (GitHub auto), changelog/notes.

## Database migrations

Migrations run automatically on startup and are **safe**:

1. Validate the existing database.
2. Take an **atomic backup** (kept on failure).
3. Apply the migration inside a transaction.
4. Validate the result.
5. Refuse to downgrade if the on-disk schema is newer than the binary.

Configuration is never silently discarded.

## Post-release

- Verify the published image runs and is healthy.
- Update the Unraid template `TemplateURL`/tag if needed.
- Announce in the support thread.

See [COMMUNITY-APPS-SUBMISSION.md](COMMUNITY-APPS-SUBMISSION.md) for the CA
submission gate.
