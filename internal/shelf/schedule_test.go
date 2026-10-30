package shelf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSchedulesPersistEditAndCompletion(t *testing.T) {
	s := newStore(t)
	entry, err := s.SaveSchedule("", ScheduleDraft{Title: "每周计划", StartDate: "2026-10-26", EndDate: "2026-11-01", AtTime: "09:30", Notes: "本地日程"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetScheduleDone(entry.ID, true); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	items := s2.Snapshot().Schedules
	if len(items) != 1 || !items[0].Done || items[0].EndDate != "2026-11-01" {
		t.Fatal("schedule lost on reopen")
	}
	updated, err := s2.SaveSchedule(entry.ID, ScheduleDraft{Title: "更新周计划", StartDate: "2026-10-26", EndDate: "2026-11-01"})
	if err != nil || !updated.Done || !updated.CreatedAt.Equal(entry.CreatedAt) {
		t.Fatal("edit lost schedule identity or done state")
	}
}
func TestScheduleValidationAndRollback(t *testing.T) {
	s := newStore(t)
	for _, d := range []ScheduleDraft{{Title: "", StartDate: "2026-10-26"}, {Title: "bad", StartDate: "2026-02-30"}, {Title: "bad", StartDate: "2026-10-26", EndDate: "2026-10-25"}, {Title: "bad", StartDate: "2026-10-26", AtTime: "25:61"}} {
		if _, err := s.SaveSchedule("", d); err == nil {
			t.Fatal("invalid schedule accepted")
		}
	}
	entry, err := s.SaveSchedule("", ScheduleDraft{Title: "one day", StartDate: "2026-10-26"})
	if err != nil || entry.EndDate != entry.StartDate {
		t.Fatal("single day normalization failed")
	}
	// An unwritable backup must fail the edit and preserve the in-memory original.
	backup := filepath.Join(s.dir, "shelf.json.bak")
	os.Rename(backup, backup+".saved")
	os.Mkdir(backup, 0700)
	if _, err = s.SaveSchedule(entry.ID, ScheduleDraft{Title: "unsaved edit", StartDate: "2026-10-26"}); err == nil {
		t.Fatal("expected persistence error")
	}
	if s.Snapshot().Schedules[0].Title != "one day" {
		t.Fatal("unsaved edit leaked")
	}
}
