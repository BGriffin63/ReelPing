package storage

import (
	"time"

	"github.com/BGriffin63/reelping/internal/model"

	bolt "go.etcd.io/bbolt"
)

// putRecord stores v keyed by id (JSON) in the named bucket.
func putRecord[T any](s *Store, bucket []byte, id string, v T) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bucket), []byte(id), v)
	})
}

// getRecord loads a record by id.
func getRecord[T any](s *Store, bucket []byte, id string) (T, error) {
	var v T
	err := s.db.View(func(tx *bolt.Tx) error {
		return getJSON(tx.Bucket(bucket), []byte(id), &v)
	})
	return v, err
}

// listRecords returns records from a bucket. When newestFirst is true it
// iterates in reverse key order (IDs are time-sortable, so this is newest
// first). filter may be nil. limit <= 0 means no limit.
func listRecords[T any](s *Store, bucket []byte, newestFirst bool, limit int, filter func(T) bool) ([]T, error) {
	var out []T
	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucket).Cursor()
		var k, v []byte
		next := c.Next
		if newestFirst {
			k, v = c.Last()
			next = c.Prev
		} else {
			k, v = c.First()
		}
		for ; k != nil; k, v = next() {
			var rec T
			if err := jsonUnmarshal(v, &rec); err != nil {
				continue // skip corrupt record rather than fail the whole list
			}
			if filter != nil && !filter(rec) {
				continue
			}
			out = append(out, rec)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
		return nil
	})
	return out, err
}

func countRecords(s *Store, bucket []byte) (int, error) {
	n := 0
	err := s.db.View(func(tx *bolt.Tx) error {
		n = tx.Bucket(bucket).Stats().KeyN
		return nil
	})
	return n, err
}

// --- Sessions ---

func (s *Store) PutSession(sess model.Session) error {
	return putRecord(s, bucketSessions, sess.ID, sess)
}

func (s *Store) GetSession(id string) (model.Session, error) {
	return getRecord[model.Session](s, bucketSessions, id)
}

func (s *Store) DeleteSession(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketSessions).Delete([]byte(id))
	})
}

func (s *Store) ListSessions() ([]model.Session, error) {
	return listRecords[model.Session](s, bucketSessions, true, 0, nil)
}

// DeleteSessionsExcept removes all sessions except keepID.
func (s *Store) DeleteSessionsExcept(keepID string) (int, error) {
	removed := 0
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		var toDelete [][]byte
		_ = b.ForEach(func(k, _ []byte) error {
			if string(k) != keepID {
				toDelete = append(toDelete, append([]byte(nil), k...))
			}
			return nil
		})
		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
			removed++
		}
		return nil
	})
	return removed, err
}

// PurgeExpiredSessions removes sessions past their absolute expiry.
func (s *Store) PurgeExpiredSessions(now time.Time) (int, error) {
	removed := 0
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		var toDelete [][]byte
		_ = b.ForEach(func(k, v []byte) error {
			var sess model.Session
			if jsonUnmarshal(v, &sess) == nil && now.After(sess.ExpiresAt) {
				toDelete = append(toDelete, append([]byte(nil), k...))
			}
			return nil
		})
		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
			removed++
		}
		return nil
	})
	return removed, err
}

// --- Incidents ---

func (s *Store) PutIncident(i model.Incident) error {
	return putRecord(s, bucketIncidents, i.ID, i)
}

func (s *Store) GetIncident(id string) (model.Incident, error) {
	return getRecord[model.Incident](s, bucketIncidents, id)
}

// ListIncidents returns incidents newest-first with an optional filter.
func (s *Store) ListIncidents(limit int, filter func(model.Incident) bool) ([]model.Incident, error) {
	return listRecords[model.Incident](s, bucketIncidents, true, limit, filter)
}

func (s *Store) CountIncidents() (int, error) { return countRecords(s, bucketIncidents) }

func (s *Store) DeleteIncident(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketIncidents).Delete([]byte(id))
	})
}

// --- Maintenance ---

func (s *Store) PutMaintenance(m model.Maintenance) error {
	return putRecord(s, bucketMaintenance, m.ID, m)
}

func (s *Store) GetMaintenance(id string) (model.Maintenance, error) {
	return getRecord[model.Maintenance](s, bucketMaintenance, id)
}

func (s *Store) ListMaintenance(limit int, filter func(model.Maintenance) bool) ([]model.Maintenance, error) {
	return listRecords[model.Maintenance](s, bucketMaintenance, true, limit, filter)
}

// --- Announcements ---

func (s *Store) PutAnnouncement(a model.Announcement) error {
	return putRecord(s, bucketAnnouncements, a.ID, a)
}

func (s *Store) ListAnnouncements(limit int, filter func(model.Announcement) bool) ([]model.Announcement, error) {
	return listRecords[model.Announcement](s, bucketAnnouncements, true, limit, filter)
}

func (s *Store) CountAnnouncements() (int, error) { return countRecords(s, bucketAnnouncements) }

// --- Notifications ---

func (s *Store) PutNotification(n model.Notification) error {
	return putRecord(s, bucketNotifications, n.ID, n)
}

func (s *Store) ListNotifications(limit int, filter func(model.Notification) bool) ([]model.Notification, error) {
	return listRecords[model.Notification](s, bucketNotifications, true, limit, filter)
}

// --- Audit ---

func (s *Store) PutAudit(a model.AuditEvent) error {
	return putRecord(s, bucketAudit, a.ID, a)
}

func (s *Store) ListAudit(limit int, filter func(model.AuditEvent) bool) ([]model.AuditEvent, error) {
	return listRecords[model.AuditEvent](s, bucketAudit, true, limit, filter)
}

// --- Idempotency ---

// ReserveIdempotency records key with the current time if absent and returns
// true (first use). If the key already exists it returns false. Old keys are
// pruned opportunistically.
func (s *Store) ReserveIdempotency(key string, now time.Time, ttl time.Duration) (bool, error) {
	fresh := false
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketIdempotency)
		if existing := b.Get([]byte(key)); existing != nil {
			return nil
		}
		fresh = true
		// prune expired keys
		cutoff := now.Add(-ttl)
		var stale [][]byte
		_ = b.ForEach(func(k, v []byte) error {
			var t time.Time
			if jsonUnmarshal(v, &t) == nil && t.Before(cutoff) {
				stale = append(stale, append([]byte(nil), k...))
			}
			return nil
		})
		for _, k := range stale {
			_ = b.Delete(k)
		}
		return putJSON(b, []byte(key), now)
	})
	return fresh, err
}
