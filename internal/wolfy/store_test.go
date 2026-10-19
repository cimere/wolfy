package wolfy

import (
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func testJob(id, user string, now time.Time) Job {
	return Job{ID: id, User: user, Text: "Quando?", Kind: "text", Received: now, MessageTime: now}
}

func TestDurableQueueDeduplicatesAndRecovers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.db")
	now := time.Now().UTC()
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	job := testJob("first", "399000000001", now)
	if err := s.Enqueue([]Job{job, job}); err != nil {
		t.Fatal(err)
	}
	claimed, err := s.Claim(now)
	if err != nil || claimed == nil || claimed.Attempts != 1 {
		t.Fatalf("claim: %v %v", claimed, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Recover(); err != nil {
		t.Fatal(err)
	}
	claimed, err = s.Claim(now)
	if err != nil || claimed == nil || claimed.Attempts != 2 {
		t.Fatalf("recovery: %v %v", claimed, err)
	}
	claimed.State = Sending
	if err := s.SaveJob(*claimed); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != Uncertain || stored.Text != "" || stored.User != "" {
		t.Fatalf("unsafe recovered send: %+v", stored)
	}
	if err := s.Enqueue([]Job{job}); err != nil {
		t.Fatal(err)
	}
	next, err := s.Claim(now)
	if err != nil || next != nil {
		t.Fatalf("duplicate replayed: %+v %v", next, err)
	}
}

func TestHistoryIsolationExpiryAndDeletion(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	for _, user := range []string{"one", "two"} {
		j := testJob(user, user, now)
		j.Reply = "Risposta per " + user
		j.Remember = true
		if err := s.Finish(j, Sent, now); err != nil {
			t.Fatal(err)
		}
	}
	turns, err := s.History("one", now)
	if err != nil || len(turns) != 2 || turns[1].Text != "Risposta per one" {
		t.Fatalf("history: %v %v", turns, err)
	}
	if err := s.Forget("one"); err != nil {
		t.Fatal(err)
	}
	turns, _ = s.History("one", now)
	if len(turns) != 0 {
		t.Fatal("forget failed")
	}
	turns, _ = s.History("two", now)
	if len(turns) != 2 {
		t.Fatal("other user erased")
	}
	turns, _ = s.History("two", now.Add(Retention))
	if len(turns) != 0 {
		t.Fatal("expired memory still readable")
	}
	if err := s.Cleanup(now.Add(Retention)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetJob("two"); err == nil {
		t.Fatal("expired job not deleted")
	}
}

func TestBudgetReservationsSurviveUnknownResults(t *testing.T) {
	s := testStore(t)
	ok, err := s.Reserve("a", "2026-10", 60, 100)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	ok, err = s.Reserve("b", "2026-10", 60, 100)
	if err != nil || ok {
		t.Fatal("overspend", ok, err)
	}
	if err := s.Recover(); err != nil {
		t.Fatal(err)
	}
	used, _ := s.Used("2026-10")
	if used != 60 {
		t.Fatal("reservation lost", used)
	}
	if err := s.Reconcile("a", 20); err != nil {
		t.Fatal(err)
	}
	ok, err = s.Reserve("b", "2026-10", 60, 100)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := s.Reconcile("b", 80); err != nil {
		t.Fatal(err)
	}
	used, _ = s.Used("2026-10")
	if used != 100 {
		t.Fatal(used)
	}
	ok, err = s.Reserve("c", "2026-10", 1, 100)
	if err != nil || ok {
		t.Fatal("limit failed")
	}
	ok, err = s.Reserve("d", "2026-11", 1, 100)
	if err != nil || !ok {
		t.Fatal("new month blocked")
	}
}

func TestBackupRestoresQueueAndMemory(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	if err := s.Enqueue([]Job{testJob("pending", "one", now)}); err != nil {
		t.Fatal(err)
	}
	j := testJob("sent", "two", now)
	j.Reply = "official"
	j.Remember = true
	if err := s.Finish(j, Sent, now); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "backup.db")
	if err := s.Backup(backup); err != nil {
		t.Fatal(err)
	}
	if err := s.Backup(backup); err == nil {
		t.Fatal("backup overwritten")
	}
	restored, err := OpenStore(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err := restored.Check(); err != nil {
		t.Fatal(err)
	}
	pending, err := restored.Claim(now)
	if err != nil || pending == nil || pending.ID != "pending" {
		t.Fatal(pending, err)
	}
	history, err := restored.History("two", now)
	if err != nil || len(history) != 2 {
		t.Fatal(history, err)
	}
}

func TestQueuePreservesPerUserOrderDuringBackoff(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	a := testJob("a", "one", now)
	b := testJob("b", "one", now.Add(time.Second))
	c := testJob("c", "two", now.Add(2*time.Second))
	if err := s.Enqueue([]Job{a, b, c}); err != nil {
		t.Fatal(err)
	}
	a.State = Pending
	a.Due = now.Add(time.Minute)
	if err := s.SaveJob(a); err != nil {
		t.Fatal(err)
	}
	j, err := s.Claim(now.Add(3 * time.Second))
	if err != nil || j == nil || j.ID != "c" {
		t.Fatalf("out-of-order processing: %+v %v", j, err)
	}
}

func TestSafeRestoreDiscardsPotentialReplaysAndRequiresBudgetReconciliation(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	month := now.Format("2006-01")
	if err := s.Enqueue([]Job{testJob("pending", "one", now)}); err != nil {
		t.Fatal(err)
	}
	j := testJob("sent", "two", now)
	j.Reply = "private"
	j.Remember = true
	if err := s.Finish(j, Sent, now); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Reserve("pre-backup", month, 50, 100); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := s.PrepareRestore(now); err != nil {
		t.Fatal(err)
	}
	if job, err := s.Claim(now); err != nil || job != nil {
		t.Fatal("restored send replayed", job, err)
	}
	if turns, err := s.History("two", now); err != nil || len(turns) != 0 {
		t.Fatal("restored memory resurrected")
	}
	if ok, err := s.Reserve("after-restore", month, 1, 100); err != nil || ok {
		t.Fatal("AI not paused")
	}
	if err := s.ResumeAI(month, 80); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Reserve("after-restore", month, 20, 100); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := s.Reserve("over-limit", month, 1, 100); err != nil || ok {
		t.Fatal("reconciled limit ignored")
	}
	// This message was sent after the snapshot and its ID is absent from it.
	oldRetry := testJob("absent-from-backup", "one", now.Add(time.Second))
	oldRetry.MessageTime = now.Add(-time.Minute)
	newQuestion := testJob("new-question", "one", now.Add(2*time.Second))
	if err := s.Enqueue([]Job{oldRetry, newQuestion}); err != nil {
		t.Fatal(err)
	}
	job, err := s.Claim(now.Add(3 * time.Second))
	if err != nil || job == nil || job.ID != newQuestion.ID {
		t.Fatal("old webhook replayed after restore", job, err)
	}
}

func TestCleanupKeepsPreviousMonthAtMonthEnd(t *testing.T) {
	s := testStore(t)
	for _, month := range []string{"2026-02", "2026-03", "2026-04"} {
		if ok, err := s.Reserve(month, month, 50, 100); err != nil || !ok {
			t.Fatal(ok, err)
		}
	}
	if err := s.Cleanup(time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	for month, want := range map[string]int64{"2026-02": 0, "2026-03": 50, "2026-04": 50} {
		if used, err := s.Used(month); err != nil || used != want {
			t.Fatalf("month %s: got %d, want %d, err %v", month, used, want, err)
		}
	}
}

func TestSameBatchPreservesInputOrder(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	if err := s.Enqueue([]Job{testJob("z", "one", now), testJob("a", "one", now)}); err != nil {
		t.Fatal(err)
	}
	job, err := s.Claim(now)
	if err != nil || job == nil || job.ID != "z" {
		t.Fatal(job, err)
	}
}
