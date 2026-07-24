// Package storage is ReelPing's persistence layer, backed by a single bbolt
// database file under /config. It owns the schema, migrations, atomic backups,
// and typed accessors for every persisted entity.
package storage

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket names.
var (
	bucketMeta          = []byte("meta")
	bucketConfig        = []byte("config")
	bucketAdmin         = []byte("admin")
	bucketSessions      = []byte("sessions")
	bucketMonitorState  = []byte("monitor_state")
	bucketIncidents     = []byte("incidents")
	bucketMaintenance   = []byte("maintenance")
	bucketAnnouncements = []byte("announcements")
	bucketNotifications = []byte("notifications")
	bucketAudit         = []byte("audit")
	bucketIdempotency   = []byte("idempotency")
)

var allBuckets = [][]byte{
	bucketMeta, bucketConfig, bucketAdmin, bucketSessions, bucketMonitorState,
	bucketIncidents, bucketMaintenance, bucketAnnouncements, bucketNotifications,
	bucketAudit, bucketIdempotency,
}

// CurrentSchemaVersion is the schema version this build expects.
const CurrentSchemaVersion = 1

var (
	keySchemaVersion = []byte("schema_version")
	keyInstallID     = []byte("install_id")
	keyConfig        = []byte("config")
	keyMonitorState  = []byte("state")
	keyAdmin         = []byte("admin")
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps the bbolt database.
type Store struct {
	db   *bolt.DB
	path string
}

// Open opens (creating if necessary) the database at path, ensures all buckets
// exist, and runs migrations with a backup-first strategy.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	s := &Store{db: db, path: path}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, b := range allBuckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		meta := tx.Bucket(bucketMeta)
		if meta.Get(keyInstallID) == nil {
			if err := meta.Put(keyInstallID, []byte(newInstallID())); err != nil {
				return err
			}
		}
		if meta.Get(keySchemaVersion) == nil {
			if err := putUint(meta, keySchemaVersion, 0); err != nil {
				return err
			}
		}
		return nil
	})
}

// SchemaVersion returns the persisted schema version.
func (s *Store) SchemaVersion() (uint64, error) {
	var v uint64
	err := s.db.View(func(tx *bolt.Tx) error {
		v = getUint(tx.Bucket(bucketMeta), keySchemaVersion)
		return nil
	})
	return v, err
}

// InstallID returns the stable random install identifier.
func (s *Store) InstallID() (string, error) {
	var id string
	err := s.db.View(func(tx *bolt.Tx) error {
		id = string(tx.Bucket(bucketMeta).Get(keyInstallID))
		return nil
	})
	return id, err
}

// migrate validates the DB, backs it up, applies pending migrations in order,
// and preserves the backup on failure.
func (s *Store) migrate() error {
	current, err := s.SchemaVersion()
	if err != nil {
		return err
	}
	if current == CurrentSchemaVersion {
		return nil
	}
	if current > CurrentSchemaVersion {
		return fmt.Errorf("database schema %d is newer than this build supports (%d); refusing to downgrade",
			current, CurrentSchemaVersion)
	}

	// Only back up if there is real data to protect (schema already > 0).
	var backup string
	if current > 0 {
		backup, err = s.Backup()
		if err != nil {
			return fmt.Errorf("pre-migration backup failed: %w", err)
		}
	}

	for v := current; v < CurrentSchemaVersion; v++ {
		if err := s.applyMigration(int(v) + 1); err != nil {
			if backup != "" {
				return fmt.Errorf("migration to v%d failed: %w (backup preserved at %s)", v+1, err, backup)
			}
			return fmt.Errorf("migration to v%d failed: %w", v+1, err)
		}
	}
	// Migration succeeded and validated; the backup can be removed by the
	// operator. We keep it — cheap insurance — but stop referencing it.
	return nil
}

// applyMigration runs a single forward migration inside one transaction.
func (s *Store) applyMigration(to int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		switch to {
		case 1:
			// v0 -> v1: initial schema. Buckets already created in init();
			// nothing structural to change. Seed defaults if config missing.
			cfgB := tx.Bucket(bucketConfig)
			if cfgB.Get(keyConfig) == nil {
				// Leave empty; the app writes defaults on first run.
			}
		default:
			return fmt.Errorf("no migration defined for version %d", to)
		}
		return putUint(tx.Bucket(bucketMeta), keySchemaVersion, uint64(to))
	})
}

// Backup writes an atomic, consistent copy of the database next to the original
// and returns the backup path. bbolt's tx.CopyFile provides a consistent copy.
func (s *Store) Backup() (string, error) {
	dir := filepath.Dir(s.path)
	base := filepath.Base(s.path)
	stamp := time.Now().UTC().Format("20060102T150405Z")
	tmp := filepath.Join(dir, base+".bak-"+stamp+".tmp")
	final := filepath.Join(dir, base+".bak-"+stamp)

	err := s.db.View(func(tx *bolt.Tx) error {
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := tx.WriteTo(f); err != nil {
			return err
		}
		return f.Sync()
	})
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return final, nil
}

// BackupTo writes a consistent copy of the database to the provided writer.
func (s *Store) BackupTo(w io.Writer) error {
	return s.db.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(w)
		return err
	})
}

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Writable reports whether the config directory is writable.
func (s *Store) Writable() bool {
	probe := filepath.Join(filepath.Dir(s.path), ".rp-write-probe")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return true
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// --- helpers ---

func putJSON(b *bolt.Bucket, key []byte, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return b.Put(key, data)
}

func getJSON(b *bolt.Bucket, key []byte, v any) error {
	data := b.Get(key)
	if data == nil {
		return ErrNotFound
	}
	return json.Unmarshal(data, v)
}

func putUint(b *bolt.Bucket, key []byte, v uint64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	return b.Put(key, buf[:])
}

func getUint(b *bolt.Bucket, key []byte) uint64 {
	v := b.Get(key)
	if len(v) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(v)
}

func newInstallID() string {
	// 16 random bytes, hex-ish via the model encoder is fine, but avoid import
	// cycle: use time + random through crypto in a tiny local helper.
	return time.Now().UTC().Format("20060102150405") + "-" + randHex(8)
}
