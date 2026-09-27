package store

import (
	"testing"
	"time"
)

func TestUpdatedAtRoundtrip(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e := &MediaEntry{FilePath: "/a/b.strm", Title: "t"}
	if err := s.UpsertMediaEntry(e); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if err := s.SetMediaSub(e.ID, "failed", "", "subhd"); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListMedia("", 10)
	if err != nil {
		t.Fatal(err)
	}
	got := list[0].UpdatedAt
	t.Logf("updated_at parsed as: %v (zero=%v)", got, got.IsZero())
	var raw string
	_ = s.db.QueryRow(`SELECT updated_at FROM media_index WHERE id=?`, e.ID).Scan(&raw)
	t.Logf("raw in db: %q", raw)
	if got.IsZero() || got.Before(before.Add(-time.Minute)) || got.After(time.Now().Add(time.Minute)) {
		t.Errorf("UpdatedAt 解析不可靠: %v", got)
	}
}
