// Package search 聚合多个字幕源的搜索，带 SQLite 缓存。
// 缓存键 = 字幕源 + 关键词（+年/季/集），TTL 内直接返回，减少重复访问字幕站。
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/store"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

// CacheTTL 搜索缓存有效期
const CacheTTL = 6 * time.Hour

// querySchemaVersion 查询构造版本。改动各源的查询构造逻辑（关键词清洗、
// 季集/年份拼接等）时 +1，让旧缓存失效——缓存键本身感知不到查询逻辑变化。
const querySchemaVersion = 2

type Searcher struct {
	store   *store.Store
	ttl     time.Duration
	mu      sync.RWMutex
	sources []subsource.Source
}

func New(st *store.Store, sources []subsource.Source) *Searcher {
	return &Searcher{store: st, ttl: CacheTTL, sources: sources}
}

func (s *Searcher) SetSources(ss []subsource.Source) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources = ss
}

func (s *Searcher) getSources() []subsource.Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]subsource.Source, len(s.sources))
	copy(out, s.sources)
	return out
}

// SourceStatus 各源状态（展示用）
func (s *Searcher) SourceStatus() []map[string]any {
	var out []map[string]any
	for _, src := range s.getSources() {
		out = append(out, map[string]any{
			"name":    src.Name(),
			"enabled": s.EffectiveEnabled(src),
		})
	}
	return out
}

// EffectiveEnabled 凭证就绪且设置页开关打开
func (s *Searcher) EffectiveEnabled(src subsource.Source) bool {
	return src.Enabled() && s.store.SourceEnabled(src.Name(), true)
}

// Group 一个源的搜索结果分组
type Group struct {
	Source string          `json:"source"`
	Hits   []subsource.Hit `json:"hits"`
	Cached bool            `json:"cached"`
	Error  string          `json:"error,omitempty"`
}

type Result struct {
	Keyword string  `json:"keyword"`
	Groups  []Group `json:"groups"`
}

func cacheKey(source, keyword string, m metadata.MediaInfo) string {
	return fmt.Sprintf("search:v%d:%s|%s|%d|%d|%d", querySchemaVersion, source, keyword, m.Year, m.Season, m.Episode)
}

// Search 聚合搜索：各启用的源并行查询，带缓存
func (s *Searcher) Search(ctx context.Context, keyword string, m metadata.MediaInfo) *Result {
	keyword = trimKeyword(keyword)
	res := &Result{Keyword: keyword, Groups: []Group{}}
	if keyword == "" {
		return res
	}
	// 顺手清理过期缓存
	go s.store.CleanSearchCache(s.ttl)

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, src := range s.getSources() {
		if !s.EffectiveEnabled(src) {
			continue
		}
		wg.Add(1)
		go func(src subsource.Source) {
			defer wg.Done()
			g := s.searchOne(ctx, src, keyword, m)
			mu.Lock()
			res.Groups = append(res.Groups, g)
			mu.Unlock()
		}(src)
	}
	wg.Wait()
	return res
}

func (s *Searcher) searchOne(ctx context.Context, src subsource.Source, keyword string, m metadata.MediaInfo) Group {
	g := Group{Source: src.Name()}
	key := cacheKey(src.Name(), keyword, m)
	if raw, ok := s.store.GetSearchCache(key, s.ttl); ok {
		var hits []subsource.Hit
		if json.Unmarshal(raw, &hits) == nil {
			g.Hits, g.Cached = hits, true
			return g
		}
	}
	cands, err := src.Search(ctx, m)
	if err != nil {
		g.Error = err.Error()
		log.Printf("[search] %s 搜索失败 %q: %v", src.Name(), keyword, err)
		return g
	}
	for _, c := range cands {
		g.Hits = append(g.Hits, c.ToHit())
	}
	if raw, err := json.Marshal(g.Hits); err == nil {
		_ = s.store.SetSearchCache(key, raw)
	}
	return g
}

// ToCandidates 把 Hit 还原为 Candidate（Download 走 DownloadRef），供打分器使用
func (s *Searcher) ToCandidates(groups []Group) []subsource.Candidate {
	byName := map[string]subsource.Source{}
	for _, src := range s.getSources() {
		byName[src.Name()] = src
	}
	var out []subsource.Candidate
	for _, g := range groups {
		src, ok := byName[g.Source]
		if !ok {
			continue
		}
		for _, h := range g.Hits {
			h := h
			out = append(out, subsource.Candidate{
				Source: h.Source, RefID: h.RefID, Name: h.Name,
				Lang: h.Lang, Format: h.Format, Votes: h.Votes, Detail: h.Detail,
				Download: func(ctx context.Context) (string, []byte, error) {
					return src.DownloadRef(ctx, h.RefID)
				},
			})
		}
	}
	return out
}

// DownloadRef 按源名+RefID 下载
func (s *Searcher) DownloadRef(ctx context.Context, source, refID string) (string, []byte, error) {
	for _, src := range s.getSources() {
		if src.Name() == source {
			return src.DownloadRef(ctx, refID)
		}
	}
	return "", nil, fmt.Errorf("未知字幕源: %s", source)
}

func trimKeyword(q string) string {
	if len(q) > 120 {
		// 按 rune 截断
		r := []rune(q)
		if len(r) > 120 {
			q = string(r[:120])
		}
	}
	return q
}
