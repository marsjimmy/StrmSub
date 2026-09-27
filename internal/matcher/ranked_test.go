package matcher

import (
	"testing"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

func TestPickRankedOrder(t *testing.T) {
	m := metadata.MediaInfo{Title: "金装律师", Type: metadata.Episode, Season: 1, Episode: 5}
	cands := []subsource.Candidate{
		{Name: "金装律师 S01E04 中英双语", Lang: subsource.LangBilingual},
		{Name: "金装律师 S01E05 简体中文", Lang: subsource.LangHans},
		{Name: "金装律师 S01E05 繁体中文", Lang: subsource.LangHant}, // 目标简体，繁体被语言门槛过滤
	}
	ranked := PickRanked(cands, m, subsource.LangHans)
	if len(ranked) != 2 {
		t.Fatalf("len=%d", len(ranked))
	}
	// 简体 S01E05 应该排第一（语言 100 + 季集 50）
	if ranked[0].Candidate.Name != "金装律师 S01E05 简体中文" {
		t.Fatalf("top=%q", ranked[0].Candidate.Name)
	}
	for i := 1; i < len(ranked); i++ {
		if ranked[i].Score > ranked[i-1].Score {
			t.Fatalf("未按分数降序: %v", ranked)
		}
	}
	// PickBest 行为不变：阈值 30
	if best := PickBest(cands, m, subsource.LangHans); best == nil || best.Name != "金装律师 S01E05 简体中文" {
		t.Fatalf("best=%v", best)
	}
}
