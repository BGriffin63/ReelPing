package storage

import (
	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/model"

	bolt "go.etcd.io/bbolt"
)

// --- Config ---

// GetConfig loads the persisted config, returning config.Default() if none is
// stored yet.
func (s *Store) GetConfig() (config.Config, error) {
	cfg := config.Default()
	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketConfig).Get(keyConfig)
		if data == nil {
			return nil
		}
		return getJSON(tx.Bucket(bucketConfig), keyConfig, &cfg)
	})
	return cfg, err
}

// SaveConfig persists the config.
func (s *Store) SaveConfig(cfg config.Config) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bucketConfig), keyConfig, cfg)
	})
}

// --- Admin ---

// GetAdmin loads the administrator account.
func (s *Store) GetAdmin() (model.Admin, error) {
	var a model.Admin
	err := s.db.View(func(tx *bolt.Tx) error {
		return getJSON(tx.Bucket(bucketAdmin), keyAdmin, &a)
	})
	return a, err
}

// HasAdmin reports whether an administrator account exists.
func (s *Store) HasAdmin() (bool, error) {
	var exists bool
	err := s.db.View(func(tx *bolt.Tx) error {
		exists = tx.Bucket(bucketAdmin).Get(keyAdmin) != nil
		return nil
	})
	return exists, err
}

// SaveAdmin persists the administrator account (including its Argon2id hash).
func (s *Store) SaveAdmin(a model.Admin) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bucketAdmin), keyAdmin, a)
	})
}

// DeleteAdmin removes the administrator account (used by the recovery flow).
func (s *Store) DeleteAdmin() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketAdmin).Delete(keyAdmin)
	})
}

// --- Monitor state ---

// GetMonitorState loads the persisted monitoring state, or a zero-value state
// with ErrNotFound-safe behaviour (returns ok=false when none stored).
func (s *Store) GetMonitorState() (model.MonitorState, bool, error) {
	var st model.MonitorState
	found := false
	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketMonitorState).Get(keyMonitorState)
		if data == nil {
			return nil
		}
		found = true
		return getJSON(tx.Bucket(bucketMonitorState), keyMonitorState, &st)
	})
	return st, found, err
}

// SaveMonitorState persists the monitoring state snapshot.
func (s *Store) SaveMonitorState(st model.MonitorState) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bucketMonitorState), keyMonitorState, st)
	})
}
