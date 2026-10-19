package wolfy

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var ErrQueueFull = errors.New("queue capacity reached")
var buckets = []string{"jobs", "memory", "budget", "reservations", "limits", "meta"}

type Store struct{ db *bolt.DB }

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	err = db.Update(func(tx *bolt.Tx) error {
		for _, name := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return err
			}
		}
		meta := tx.Bucket([]byte("meta"))
		v := meta.Get([]byte("schema"))
		if v != nil && string(v) != "1" {
			return errors.New("unsupported database schema")
		}
		return meta.Put([]byte("schema"), []byte("1"))
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }
func putJSON(b *bolt.Bucket, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return b.Put([]byte(key), data)
}
func readJob(b *bolt.Bucket, id string) (Job, error) {
	var j Job
	data := b.Get([]byte(id))
	if data == nil {
		return j, errors.New("job not found")
	}
	err := json.Unmarshal(data, &j)
	return j, err
}

func (s *Store) Enqueue(jobs []Job) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("jobs"))
		restoredAt := integer(tx.Bucket([]byte("meta")), "restored_at_unix")
		active := 0
		if err := b.ForEach(func(_, v []byte) error {
			var j Job
			if err := json.Unmarshal(v, &j); err != nil {
				return err
			}
			if j.State == Pending || j.State == Preparing || j.State == Ready || j.State == Sending {
				active++
			}
			return nil
		}); err != nil {
			return err
		}
		for _, j := range jobs {
			// A restored snapshot may lack deduplication records for messages
			// already handled after the backup. Discard retries from that period.
			if restoredAt != 0 && j.MessageTime.Unix() <= restoredAt {
				continue
			}
			if b.Get([]byte(j.ID)) != nil {
				continue
			}
			if active >= 1000 {
				return ErrQueueFull
			}
			j.State = Pending
			j.Due = j.Received
			sequence, err := b.NextSequence()
			if err != nil {
				return err
			}
			j.Sequence = sequence
			if err := putJSON(b, j.ID, j); err != nil {
				return err
			}
			active++
		}
		return nil
	})
}

// Recover must run once before starting the single worker. A crash during a send
// cannot establish whether Meta accepted it; never automatically replay it.
func (s *Store) Recover() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("jobs"))
		var changed []Job
		if err := b.ForEach(func(_, v []byte) error {
			var j Job
			if err := json.Unmarshal(v, &j); err != nil {
				return err
			}
			switch j.State {
			case Preparing:
				j.State = Pending
				changed = append(changed, j)
			case Ready:
				if j.Remember {
					j.State = Pending
					j.Reply = ""
					changed = append(changed, j)
				}
			case Sending:
				j.State = Uncertain
				j.Text = ""
				j.Reply = ""
				j.User = ""
				changed = append(changed, j)
			}
			return nil
		}); err != nil {
			return err
		}
		for _, j := range changed {
			if err := putJSON(b, j.ID, j); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) Claim(now time.Time) (*Job, error) {
	var found *Job
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("jobs"))
		// First select each user's oldest outstanding job, preserving conversation order.
		first := map[string]Job{}
		if err := b.ForEach(func(_, v []byte) error {
			var j Job
			if err := json.Unmarshal(v, &j); err != nil {
				return err
			}
			if j.State != Pending && j.State != Ready {
				return nil
			}
			old, ok := first[j.User]
			if !ok || earlier(j, old) {
				first[j.User] = j
			}
			return nil
		}); err != nil {
			return err
		}
		for _, j := range first {
			if j.Due.After(now) {
				continue
			}
			if found == nil || earlier(j, *found) {
				copy := j
				found = &copy
			}
		}
		if found == nil {
			return nil
		}
		if found.State == Pending {
			found.State = Preparing
			found.Attempts++
		}
		return putJSON(b, found.ID, found)
	})
	return found, err
}

func earlier(a, b Job) bool {
	if !a.Received.Equal(b.Received) {
		return a.Received.Before(b.Received)
	}
	if a.Sequence != b.Sequence {
		return a.Sequence < b.Sequence
	}
	return a.ID < b.ID
}

func (s *Store) GetJob(id string) (Job, error) {
	var j Job
	err := s.db.View(func(tx *bolt.Tx) error { var e error; j, e = readJob(tx.Bucket([]byte("jobs")), id); return e })
	return j, err
}
func (s *Store) SaveJob(j Job) error {
	return s.db.Update(func(tx *bolt.Tx) error { return putJSON(tx.Bucket([]byte("jobs")), j.ID, j) })
}

func (s *Store) Finish(j Job, state string, now time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		if state == Sent && j.Remember {
			b := tx.Bucket([]byte("memory"))
			var turns []Turn
			if data := b.Get([]byte(j.User)); data != nil {
				if err := json.Unmarshal(data, &turns); err != nil {
					return err
				}
			}
			turns = recent(turns, now)
			turns = append(turns, Turn{Role: "user", Text: j.Text, At: now}, Turn{Role: "assistant", Text: j.Reply, At: now})
			if len(turns) > 12 {
				turns = turns[len(turns)-12:]
			}
			if err := putJSON(b, j.User, turns); err != nil {
				return err
			}
		}
		j.State = state
		j.Text = ""
		j.Reply = ""
		j.User = ""
		return putJSON(tx.Bucket([]byte("jobs")), j.ID, j)
	})
}

func recent(turns []Turn, now time.Time) []Turn {
	out := make([]Turn, 0, len(turns))
	for _, t := range turns {
		if t.At.After(now.Add(-Retention)) {
			out = append(out, t)
		}
	}
	return out
}
func (s *Store) History(user string, now time.Time) ([]Turn, error) {
	var turns []Turn
	err := s.db.View(func(tx *bolt.Tx) error {
		if data := tx.Bucket([]byte("memory")).Get([]byte(user)); data != nil {
			return json.Unmarshal(data, &turns)
		}
		return nil
	})
	return recent(turns, now), err
}
func (s *Store) Forget(user string) error {
	return s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket([]byte("memory")).Delete([]byte(user)) })
}

func integer(b *bolt.Bucket, key string) int64 {
	v := b.Get([]byte(key))
	if len(v) != 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(v))
}
func putInteger(b *bolt.Bucket, key string, value int64) error {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], uint64(value))
	return b.Put([]byte(key), data[:])
}

// Reservations are charged before the HTTP request. Unknown outcomes keep the
// entire reservation, including after a crash; retrying must reserve again.
func (s *Store) Reserve(key string, month string, amount, limit int64) (bool, error) {
	allowed := false
	err := s.db.Update(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte("meta")).Get([]byte("ai_paused")) != nil {
			return nil
		}
		b := tx.Bucket([]byte("budget"))
		r := tx.Bucket([]byte("reservations"))
		if r.Get([]byte(key)) != nil {
			return errors.New("reservation already exists")
		}
		used := integer(b, month)
		if amount < 0 || amount > limit || used > limit-amount {
			return nil
		}
		if err := putInteger(b, month, used+amount); err != nil {
			return err
		}
		if err := putJSON(r, key, reservation{Month: month, Amount: amount}); err != nil {
			return err
		}
		allowed = true
		return nil
	})
	return allowed, err
}

type reservation struct {
	Month  string
	Amount int64
}

func (s *Store) Reconcile(key string, actual int64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		r := tx.Bucket([]byte("reservations"))
		data := r.Get([]byte(key))
		if data == nil {
			return errors.New("reservation not found")
		}
		var v reservation
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		if actual < 0 {
			return errors.New("negative usage")
		}
		b := tx.Bucket([]byte("budget"))
		used := integer(b, v.Month)
		if err := putInteger(b, v.Month, used-v.Amount+actual); err != nil {
			return err
		}
		return r.Delete([]byte(key))
	})
}
func (s *Store) Used(month string) (int64, error) {
	var value int64
	err := s.db.View(func(tx *bolt.Tx) error { value = integer(tx.Bucket([]byte("budget")), month); return nil })
	return value, err
}
func (s *Store) AllowDaily(user string, now time.Time, limit int) (bool, error) {
	ok := false
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("limits"))
		key := now.UTC().Format("2006-01-02") + "/" + user
		n := integer(b, key)
		if n >= int64(limit) {
			return nil
		}
		ok = true
		return putInteger(b, key, n+1)
	})
	return ok, err
}

func (s *Store) Cleanup(now time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		jobs := tx.Bucket([]byte("jobs"))
		var remove [][]byte
		if err := jobs.ForEach(func(k, v []byte) error {
			var j Job
			if err := json.Unmarshal(v, &j); err != nil {
				return err
			}
			if !j.Received.After(now.Add(-Retention)) {
				remove = append(remove, append([]byte(nil), k...))
			}
			return nil
		}); err != nil {
			return err
		}
		for _, k := range remove {
			if err := jobs.Delete(k); err != nil {
				return err
			}
		}
		memory := tx.Bucket([]byte("memory"))
		values := map[string][]Turn{}
		if err := memory.ForEach(func(k, v []byte) error {
			var turns []Turn
			if err := json.Unmarshal(v, &turns); err != nil {
				return err
			}
			values[string(k)] = recent(turns, now)
			return nil
		}); err != nil {
			return err
		}
		for key, turns := range values {
			if len(turns) == 0 {
				if err := memory.Delete([]byte(key)); err != nil {
					return err
				}
			} else if err := putJSON(memory, key, turns); err != nil {
				return err
			}
		}
		limits := tx.Bucket([]byte("limits"))
		remove = nil
		today := now.UTC().Format("2006-01-02")
		if err := limits.ForEach(func(k, _ []byte) error {
			if len(k) < 10 || string(k[:10]) < today {
				remove = append(remove, append([]byte(nil), k...))
			}
			return nil
		}); err != nil {
			return err
		}
		for _, k := range remove {
			if err := limits.Delete(k); err != nil {
				return err
			}
		}
		// Keep only the current and previous UTC month, without user identifiers.
		utc := now.UTC()
		monthStart := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
		cutoff := monthStart.AddDate(0, -2, 0).Format("2006-01")
		for _, name := range []string{"budget", "reservations"} {
			b := tx.Bucket([]byte(name))
			remove = nil
			if err := b.ForEach(func(k, v []byte) error {
				month := string(k)
				if name == "reservations" {
					var r reservation
					if err := json.Unmarshal(v, &r); err != nil {
						return err
					}
					month = r.Month
				}
				if month <= cutoff {
					remove = append(remove, append([]byte(nil), k...))
				}
				return nil
			}); err != nil {
				return err
			}
			for _, k := range remove {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// Backup compacts a consistent snapshot, omitting freed pages containing deleted
// records. The destination must not exist; maintenance commands run offline.
func (s *Store) Backup(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(path)
		}
	}()
	destination, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return err
	}
	defer destination.Close()
	if err := bolt.Compact(destination, s.db, 0); err != nil {
		return err
	}
	if err := destination.Sync(); err != nil {
		return err
	}
	if err := destination.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}
func (s *Store) Check() error {
	return s.db.View(func(tx *bolt.Tx) error {
		var first error
		for err := range tx.Check() {
			if err != nil && first == nil {
				first = fmt.Errorf("database integrity: %w", err)
			}
		}
		return first
	})
}

// PrepareRestore discards potentially stale conversations and deliveries: a
// snapshot cannot know what was sent or deleted after it was taken.
func (s *Store) PrepareRestore(now time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte("memory")); err != nil {
			return err
		}
		if _, err := tx.CreateBucket([]byte("memory")); err != nil {
			return err
		}
		b := tx.Bucket([]byte("jobs"))
		var jobs []Job
		if err := b.ForEach(func(_, v []byte) error {
			var j Job
			if err := json.Unmarshal(v, &j); err != nil {
				return err
			}
			if j.State == Pending || j.State == Preparing || j.State == Ready || j.State == Sending {
				j.State = Ignored
			}
			j.User = ""
			j.Text = ""
			j.Reply = ""
			jobs = append(jobs, j)
			return nil
		}); err != nil {
			return err
		}
		for _, j := range jobs {
			if err := putJSON(b, j.ID, j); err != nil {
				return err
			}
		}
		meta := tx.Bucket([]byte("meta"))
		if err := putInteger(meta, "restored_at_unix", now.UTC().Unix()); err != nil {
			return err
		}
		return meta.Put([]byte("ai_paused"), []byte("true"))
	})
}

// ResumeAI requires the operator's reconciled current-month usage, including
// all expenditure since the snapshot. It never lowers the restored ledger.
func (s *Store) ResumeAI(month string, spent int64) error {
	if spent < 0 || spent > 100_000_000_000 {
		return errors.New("invalid reconciled usage")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("budget"))
		if current := integer(b, month); spent < current {
			spent = current
		}
		if err := putInteger(b, month, spent); err != nil {
			return err
		}
		return tx.Bucket([]byte("meta")).Delete([]byte("ai_paused"))
	})
}
