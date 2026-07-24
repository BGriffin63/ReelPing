package storage

import (
	"time"

	"github.com/BGriffin63/reelping/internal/config"
	"github.com/BGriffin63/reelping/internal/model"

	bolt "go.etcd.io/bbolt"
)

// RetentionReport summarises how many records a retention sweep removed.
type RetentionReport struct {
	Announcements int
	Audit         int
	Notifications int
	Incidents     int
}

// ApplyRetention prunes history according to the configured retention policy.
// Incidents are retained indefinitely unless MaxIncidents > 0. Time-based
// pruning uses record timestamps; because IDs are time-sortable we can prune
// from the oldest end efficiently.
func (s *Store) ApplyRetention(r config.Retention, now time.Time) (RetentionReport, error) {
	var rep RetentionReport

	pruneByAge := func(bucket []byte, days int, ts func([]byte) (time.Time, bool)) (int, error) {
		if days <= 0 {
			return 0, nil
		}
		cutoff := now.AddDate(0, 0, -days)
		removed := 0
		err := s.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucket)
			var stale [][]byte
			_ = b.ForEach(func(k, v []byte) error {
				if t, ok := ts(v); ok && t.Before(cutoff) {
					stale = append(stale, append([]byte(nil), k...))
				}
				return nil
			})
			for _, k := range stale {
				if err := b.Delete(k); err != nil {
					return err
				}
				removed++
			}
			return nil
		})
		return removed, err
	}

	var err error
	rep.Announcements, err = pruneByAge(bucketAnnouncements, r.AnnouncementsDays, func(v []byte) (time.Time, bool) {
		var a model.Announcement
		if jsonUnmarshal(v, &a) == nil {
			return a.Time, true
		}
		return time.Time{}, false
	})
	if err != nil {
		return rep, err
	}
	rep.Audit, err = pruneByAge(bucketAudit, r.AuditDays, func(v []byte) (time.Time, bool) {
		var a model.AuditEvent
		if jsonUnmarshal(v, &a) == nil {
			return a.Time, true
		}
		return time.Time{}, false
	})
	if err != nil {
		return rep, err
	}
	rep.Notifications, err = pruneByAge(bucketNotifications, r.NotificationsDays, func(v []byte) (time.Time, bool) {
		var n model.Notification
		if jsonUnmarshal(v, &n) == nil {
			return n.Time, true
		}
		return time.Time{}, false
	})
	if err != nil {
		return rep, err
	}

	// Incident count cap (oldest removed first).
	if r.MaxIncidents > 0 {
		err = s.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucketIncidents)
			n := b.Stats().KeyN
			if n <= r.MaxIncidents {
				return nil
			}
			toRemove := n - r.MaxIncidents
			c := b.Cursor()
			var keys [][]byte
			for k, _ := c.First(); k != nil && len(keys) < toRemove; k, _ = c.Next() {
				keys = append(keys, append([]byte(nil), k...))
			}
			for _, k := range keys {
				if err := b.Delete(k); err != nil {
					return err
				}
				rep.Incidents++
			}
			return nil
		})
		if err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// ClearBucket removes all records from a named history bucket. name is one of
// "announcements", "notifications", "audit", "incidents".
func (s *Store) ClearHistory(name string) error {
	var bucket []byte
	switch name {
	case "announcements":
		bucket = bucketAnnouncements
	case "notifications":
		bucket = bucketNotifications
	case "audit":
		bucket = bucketAudit
	case "incidents":
		bucket = bucketIncidents
	default:
		return ErrNotFound
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucket); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	})
}
