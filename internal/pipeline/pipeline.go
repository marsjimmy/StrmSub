// Package pipeline 串起整条链路：
// 元数据(飞牛/NFO) → 找缺字幕的 .strm → 各源搜索 → 打分选最优 → 下载落盘 → 入库
package pipeline

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/marsjimmy/strmsub/internal/config"
	"github.com/marsjimmy/strmsub/internal/downloader"
	"github.com/marsjimmy/strmsub/internal/matcher"
	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/metadata/fnos"
	"github.com/marsjimmy/strmsub/internal/store"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

type Pipeline struct {
	cfg       *config.Config
	providers []metadata.Provider
	sources   []subsource.Source
	store     *store.Store

	mu       sync.Mutex
	provMu   sync.RWMutex
	srcMu    sync.RWMutex
	mediaMu  sync.RWMutex
	lastRun  time.Time
	lastStat Result
	lastMedia []metadata.MediaInfo // 上次扫描识别到的媒体，供单条搜索与展示
}

type Result struct {
	Total      int
	Skipped    int // 已有字幕
	Downloaded int
	Missing    int // 无合适字幕
	Failed     int
	At         time.Time
}

func New(cfg *config.Config, providers []metadata.Provider, sources []subsource.Source, st *store.Store) *Pipeline {
	return &Pipeline{cfg: cfg, providers: providers, sources: sources, store: st}
}

// PosterFetcher 元数据提供方可实现：按引用下载海报字节
type PosterFetcher interface {
	FetchPoster(ctx context.Context, ref string) ([]byte, error)
}

// ensurePoster 确保海报落盘到 data/posters/，设置 m.PosterPath。
// 引用是本地已存在文件则拷贝，否则找实现 PosterFetcher 的 provider 下载。
// 已下载过（按引用哈希判重）则直接复用，失败静默跳过。
func (p *Pipeline) ensurePoster(ctx context.Context, m *metadata.MediaInfo) {
	ref := strings.TrimSpace(m.PosterURL)
	if ref == "" {
		return
	}
	dir := filepath.Join(p.cfg.DataDir, "posters")
	sum := sha1.Sum([]byte(ref))
	name := hex.EncodeToString(sum[:])
	for _, ext := range []string{".jpg", ".png", ".webp", ".gif"} {
		if _, err := os.Stat(filepath.Join(dir, name+ext)); err == nil {
			m.PosterPath = "posters/" + name + ext
			return
		}
	}
	var data []byte
	if st, err := os.Stat(ref); err == nil && !st.IsDir() && st.Size() < 5<<20 {
		// 本地文件（NFO thumb 解析出的绝对路径）
		if b, err := os.ReadFile(ref); err == nil {
			data = b
		}
	} else {
		for _, pr := range p.getProviders() {
			if f, ok := pr.(PosterFetcher); ok {
				if b, err := f.FetchPoster(ctx, ref); err == nil {
					data = b
					break
				}
			}
		}
	}
	if len(data) == 0 || len(data) > 5<<20 {
		return
	}
	ext := sniffImageExt(data)
	if ext == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	rel := "posters/" + name + ext
	if err := os.WriteFile(filepath.Join(p.cfg.DataDir, rel), data, 0o644); err != nil {
		return
	}
	m.PosterPath = rel
}

func sniffImageExt(b []byte) string {
	if len(b) < 12 {
		return ""
	}
	if b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return ".jpg"
	}
	if b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G' {
		return ".png"
	}
	if b[0] == 'R' && b[1] == 'I' && b[2] == 'F' && b[3] == 'F' &&
		b[8] == 'W' && b[9] == 'E' && b[10] == 'B' && b[11] == 'P' {
		return ".webp"
	}
	if b[0] == 'G' && b[1] == 'I' && b[2] == 'F' {
		return ".gif"
	}
	return ""
}

func (p *Pipeline) LastResult() Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastStat
}

// SetProviders 运行时热替换元数据源（设置页保存后调用），下次扫描即生效
func (p *Pipeline) SetProviders(ps []metadata.Provider) {
	p.provMu.Lock()
	defer p.provMu.Unlock()
	for _, old := range p.providers {
		old.Close()
	}
	p.providers = ps
}

func (p *Pipeline) getProviders() []metadata.Provider {
	p.provMu.RLock()
	defer p.provMu.RUnlock()
	out := make([]metadata.Provider, len(p.providers))
	copy(out, p.providers)
	return out
}

// ProviderNames 当前生效的元数据源名字（供 Web 展示）
func (p *Pipeline) ProviderNames() []string {
	var out []string
	for _, pr := range p.getProviders() {
		out = append(out, pr.Name())
	}
	return out
}

// SetSources 运行时热替换字幕源（设置页保存后调用）
func (p *Pipeline) SetSources(ss []subsource.Source) {
	p.srcMu.Lock()
	defer p.srcMu.Unlock()
	p.sources = ss
}

func (p *Pipeline) getSources() []subsource.Source {
	p.srcMu.RLock()
	defer p.srcMu.RUnlock()
	out := make([]subsource.Source, len(p.sources))
	copy(out, p.sources)
	return out
}

// SourceStatus 字幕源启用状态（供 Web 展示）
func (p *Pipeline) SourceStatus() []map[string]any {
	var out []map[string]any
	for _, s := range p.getSources() {
		out = append(out, map[string]any{"name": s.Name(), "enabled": s.Enabled()})
	}
	return out
}

// RunOnce 跑一轮全量
func (p *Pipeline) RunOnce(ctx context.Context) Result {
	res := Result{At: time.Now()}
	seen := map[string]bool{}
	var allMedia []metadata.MediaInfo
	for _, pr := range p.getProviders() {
		medias, err := pr.ListMedia(ctx)
		if err != nil {
			log.Printf("[pipeline] 元数据源 %s 失败: %v", pr.Name(), err)
			continue
		}
		log.Printf("[pipeline] 元数据源 %s 返回 %d 条媒体", pr.Name(), len(medias))
		for _, m := range medias {
			if seen[m.ID+m.FilePath] {
				continue
			}
			seen[m.ID+m.FilePath] = true
			allMedia = append(allMedia, m)
			if m.Type == metadata.Series || m.Type == metadata.Season {
				// 剧/季条目只入库（供媒体库展示），不参与字幕搜索
				p.ensurePoster(ctx, &m)
				_ = p.store.UpsertMedia(m)
				continue
			}
			res.Total++
			p.processOne(ctx, m, &res)
		}
	}
	p.mu.Lock()
	p.lastRun = time.Now()
	p.lastStat = res
	p.mu.Unlock()
	p.mediaMu.Lock()
	p.lastMedia = allMedia
	p.mediaMu.Unlock()
	log.Printf("[pipeline] 本轮完成: 共%d 缺字幕检查, 下载%d, 已有%d, 无合适%d, 失败%d",
		res.Total, res.Downloaded, res.Skipped, res.Missing, res.Failed)
	return res
}

// 搜索单条结果
const (
	SearchHasSub    = "has_sub"    // 已有字幕，跳过
	SearchDownloaded = "downloaded" // 已下载
	SearchMissing   = "missing"    // 无合适字幕
	SearchFailed    = "failed"     // 失败
	SearchSkipped   = "skipped"    // 无文件，跳过
)

// SearchOne 单独为一部媒体搜索字幕（Web 媒体库的"搜字幕"按钮）
func (p *Pipeline) SearchOne(ctx context.Context, id string) (string, error) {
	find := func(list []metadata.MediaInfo) *metadata.MediaInfo {
		for i := range list {
			if list[i].ID == id {
				return &list[i]
			}
		}
		return nil
	}
	p.mediaMu.RLock()
	cached := make([]metadata.MediaInfo, len(p.lastMedia))
	copy(cached, p.lastMedia)
	p.mediaMu.RUnlock()
	if m := find(cached); m != nil {
		res := Result{}
		return p.processOne(ctx, *m, &res), nil
	}
	// 缓存里没有（比如刚保存设置还没扫描），实时拉一次
	for _, pr := range p.getProviders() {
		medias, err := pr.ListMedia(ctx)
		if err != nil {
			continue
		}
		if m := find(medias); m != nil {
			res := Result{}
			return p.processOne(ctx, *m, &res), nil
		}
	}
	return "", fmt.Errorf("找不到该媒体（ID=%s），请先点\"立即扫描\"", id)
}

func (p *Pipeline) processOne(ctx context.Context, m metadata.MediaInfo, res *Result) string {
	strm := m.FilePath
	if strm == "" || !strings.HasSuffix(strings.ToLower(strm), ".strm") {
		// 飞牛库里的路径未必是 strm，尝试同名 strm 配对
		if s := fnos.StrmFor(m, p.cfg.MediaDirs); s != "" {
			strm = s
			m.FilePath = s
		}
	}
	if strm == "" {
		res.Skipped++
		return SearchSkipped
	}
	p.ensurePoster(ctx, &m)
	_ = p.store.UpsertMedia(m)

	if downloader.HasSubtitle(strm) {
		_ = p.store.SetStatus(m.ID, p.cfg.TargetLang, "", "", "ok")
		res.Skipped++
		return SearchHasSub
	}

	var all []subsource.Candidate
	for _, src := range p.getSources() {
		if !src.Enabled() {
			continue
		}
		cands, err := src.Search(ctx, m)
		if err != nil {
			log.Printf("[pipeline] %s 搜索失败 %s: %v", src.Name(), m.Title, err)
			continue
		}
		all = append(all, cands...)
	}
	best := matcher.PickBest(all, m, p.cfg.TargetLang)
	if best == nil {
		_ = p.store.SetStatus(m.ID, p.cfg.TargetLang, "", "", "missing")
		res.Missing++
		log.Printf("[pipeline] 无合适字幕 [%s]: %s", m.Source, describe(m))
		return SearchMissing
	}
	fname, data, err := best.Download(ctx)
	if err != nil {
		_ = p.store.SetStatus(m.ID, p.cfg.TargetLang, "", best.Source, "failed")
		res.Failed++
		log.Printf("[pipeline] 下载失败 [%s] %s (%s): %v", m.Source, describe(m), best.Source, err)
		return SearchFailed
	}
	saved, err := downloader.Save(strm, p.cfg.TargetLang, fname, data)
	if err != nil {
		res.Failed++
		log.Printf("[pipeline] 落盘失败 [%s] %s: %v", m.Source, describe(m), err)
		return SearchFailed
	}
	_ = p.store.SetStatus(m.ID, p.cfg.TargetLang, saved, best.Source, "ok")
	res.Downloaded++
	log.Printf("[pipeline] 已下载 [%s]: %s ← %s [%s]", m.Source, describe(m), best.Name, best.Source)
	return SearchDownloaded
}

func describe(m metadata.MediaInfo) string {
	if m.Type == metadata.Episode {
		return m.Title + " " + epLabel(m.Season, m.Episode)
	}
	if m.Year > 0 {
		return m.Title + " " + itoa(m.Year)
	}
	return m.Title
}

func epLabel(s, e int) string {
	return "S" + pad2(s) + "E" + pad2(e)
}
func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
func itoa(n int) string { return strconv.Itoa(n) }
