// Package matcher 从候选字幕中挑最适配的一条。
// 原则：错字幕不如无字幕。宁可标记"无合适字幕"，也不装一条对不上的。
package matcher

import (
	"fmt"
	"sort"
	"strings"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

// PickBest 返回得分最高的候选；若最高分低于阈值则返回 nil（视为无合适字幕）
func PickBest(cands []subsource.Candidate, m metadata.MediaInfo, targetLang string) *subsource.Candidate {
	ranked := PickRanked(cands, m, targetLang)
	if len(ranked) == 0 || ranked[0].Score < 30 {
		return nil
	}
	c := ranked[0].Candidate
	return &c
}

// RankedCandidate 带分数的候选，按分数从高到低排列
type RankedCandidate struct {
	Candidate subsource.Candidate
	Score     int
}

// PickRanked 返回所有通过语言门槛的候选（分数>0），按分数从高到低排序。
// 供下载时按顺序尝试：最佳候选下载失败就换下一个，而不是直接放弃。
func PickRanked(cands []subsource.Candidate, m metadata.MediaInfo, targetLang string) []RankedCandidate {
	var out []RankedCandidate
	for _, c := range cands {
		if s := score(c, m, targetLang); s > 0 {
			out = append(out, RankedCandidate{c, s})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func score(c subsource.Candidate, m metadata.MediaInfo, targetLang string) int {
	s := 0
	// 语言是硬门槛
	switch {
	case c.Lang == targetLang:
		s += 100
	case c.Lang == subsource.LangBilingual:
		s += 80 // 双语可接受
	case subsource.IsChinese(c.Lang, targetLang):
		s += 60
	default:
		return 0
	}
	name := strings.ToLower(c.Name)
	// 季集号对上是强信号
	if m.Type == metadata.Episode && m.Season > 0 && m.Episode > 0 {
		want := fmt.Sprintf("s%02de%02d", m.Season, m.Episode)
		alt := fmt.Sprintf("e%02d", m.Episode)
		if strings.Contains(name, want) {
			s += 50
		} else if strings.Contains(name, alt) {
			s += 15
		}
	}
	// 年份对上
	if m.Year > 0 && strings.Contains(name, fmt.Sprintf("%d", m.Year)) {
		s += 20
	}
	// 标题关键词对上
	for _, w := range strings.Fields(strings.ToLower(m.Title)) {
		if len([]rune(w)) >= 2 && strings.Contains(name, w) {
			s += 10
			break
		}
	}
	// 源内热度
	if c.Votes > 0 {
		s += min(c.Votes, 20)
	}
	// 格式偏好 srt
	if strings.Contains(strings.ToLower(c.Format), "srt") {
		s += 5
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
