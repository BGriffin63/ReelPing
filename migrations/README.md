# Database migrations

ReelPing uses an embedded [bbolt](https://github.com/etcd-io/bbolt) database, so
migrations are **code-defined** rather than SQL files. They live in
[`internal/storage/storage.go`](../internal/storage/storage.go) in
`applyMigration`, and the current expected schema version is
`CurrentSchemaVersion`.

## How migrations run

On startup, `storage.Open` compares the on-disk schema version (stored in the
`meta` bucket) with `CurrentSchemaVersion` and, if they differ:

1. **Validates** the existing database (bbolt opens it consistently).
2. Takes an **atomic backup** (`*.bak-<timestamp>`) when there is existing data.
3. Applies each pending migration **inside a transaction**.
4. **Validates** the result (bumps the stored version only on success).
5. **Refuses to downgrade** if the on-disk schema is newer than the binary
   (prevents an older build from corrupting a newer database).

If a migration fails, the pre-migration backup is preserved and the error names
its path.

## Adding a migration

1. Increment `CurrentSchemaVersion`.
2. Add a `case <newVersion>:` in `applyMigration` that performs the change inside
   the provided transaction.
3. Add a persistence test in `internal/storage` (fresh DB, migrate, verify) and,
   where relevant, an interrupted-migration/backup test.

This directory exists to document the approach and to hold any future SQL-style
migration artifacts should the storage backend ever change.
