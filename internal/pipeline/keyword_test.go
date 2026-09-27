package pipeline

import (
	"testing"

	"github.com/marsjimmy/strmsub/internal/store"
)

func TestSearchKeyword(t *testing.T) {
	cases := []struct {
		e    store.MediaEntry
		want string
	}{
		{store.MediaEntry{Title: "金装律师", Season: 1, Episode: 5}, "金装律师 S01E05"},
		{store.MediaEntry{Title: "Breaking Bad", Season: 5, Episode: 16}, "Breaking Bad S05E16"},
		{store.MediaEntry{Title: "某剧", Episode: 3}, "某剧 E03"},
		{store.MediaEntry{Title: "星际穿越", Year: 2014}, "星际穿越"},
	}
	for _, c := range cases {
		if got := SearchKeyword(c.e); got != c.want {
			t.Errorf("期望 %q, 得到 %q", c.want, got)
		}
	}
}
