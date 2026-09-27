// Package pipeline 串起整条链路：
// 目录扫描（增量索引）→ 聚合搜索 → 下载落盘 → 历史记录。
package pipeline

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/marsjimmy/strmsub/internal/config"
	"github.com/marsjimmy/strmsub/internal/downloader"
	"github.com/marsjimmy/strmsub/internal/library"
	"github.com/marsjimmy/strmsub/internal/matcher"
	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/search"
	"github.com/marsjimmy/strmsub/internal/store"
)

type Pipeline struct {
	cfg      *config.Config
	store    *store.Store
	scanner  *library.Scanner
	searcher *search.Searcher

	mu         sync.Mutex
	runMu      sync.Mutex // RunOnce 串行锁：定时扫描与手动扫描重叠时只跑一轮
	lastScan   time.Time
	lastResult library.ScanResult
}

func New(cfg *config.Config, st *store.Store, scanner *library.Scanner, searcher *search.Searcher) *Pipeline {
	return &Pipeline{cfg: cfg, store: st, scanner: scanner, searcher: searcher}
}

// RunOnce 跑一轮增量扫描（scheduler 定时/手动触发）；扫描后自动补字幕
func (p *Pipeline) RunOnce(ctx context.Context) library.ScanResult {
	p.runMu.Lock()
	defer p.runMu.Unlock()
	res, err := p.scanner.Scan(ctx)
	if err != nil {
		log.Printf("[pipeline] 扫描失败: %v", err)
	}
	p.mu.Lock()
	p.lastScan = time.Now()
	p.lastResult = res
	p.mu.Unlock()
	log.Printf("[pipeline] 扫描完成: 新增%d 更新%d 删除%d 共%d", res.Added, res.Updated, res.Removed, res.Total)
	if p.store.AutoDownload() {
		p.autoDownload(ctx)
	}
	return res
}

// autoDownload 扫描后自动补字幕：
//   - 视频旁边（或字幕目录）已有中文字幕 → 跳过
//   - 缺失 → 下载最佳匹配；24 小时内下载失败过的不再重试
func (p *Pipeline) autoDownload(ctx context.Context) {
	entries, err := p.store.ListMedia("", 100000)
	if err != nil {
		log.Printf("[pipeline] 自动下载：读取媒体索引失败: %v", err)
		return
	}
	subDir := p.store.SubtitleDir()
	var done, skipped, failed int
	for _, e := range entries {
		if ctx.Err() != nil {
			break
		}
		if downloader.HasSubtitleIn(e.FilePath, subDir) {
			if e.SubStatus != "ok" {
				_ = p.store.SetMediaSub(e.ID, "ok", e.SubPath, e.SubSource)
			}
			skipped++
			continue
		}
		if e.SubStatus == "failed" || e.SubStatus == "missing" {
			if time.Since(e.UpdatedAt) < 24*time.Hour {
				skipped++
				continue
			}
		}
		saved, err := p.DownloadBest(ctx, e.ID)
		if err != nil {
			log.Printf("[pipeline] 自动下载失败: %s: %v", e.Title, err)
			failed++
			continue
		}
		log.Printf("[pipeline] 自动下载成功: %s → %s", e.Title, saved)
		done++
		time.Sleep(2 * time.Second) // 对源站温柔一点
	}
	log.Printf("[pipeline] 自动下载完成: 成功%d 跳过%d 失败%d", done, skipped, failed)
}

func (p *Pipeline) LastScan() (time.Time, library.ScanResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastScan, p.lastResult
}

// Searcher 暴露给 web 层
func (p *Pipeline) Searcher() *search.Searcher { return p.searcher }

// entryMedia 把索引条目转成字幕源查询用的 MediaInfo
func entryMedia(e store.MediaEntry) metadata.MediaInfo {
	typ := metadata.Movie
	if e.Season > 0 || e.Episode > 0 {
		typ = metadata.Episode
	}
	return metadata.MediaInfo{
		ID: e.ID, Type: typ, Title: e.Title, Year: e.Year,
		Season: e.Season, Episode: e.Episode, FilePath: e.FilePath,
		Source: "library",
	}
}

// SearchMedia 为一部媒体聚合搜索（关键词 = 识别标题）
func (p *Pipeline) SearchMedia(ctx context.Context, mediaID string) (*search.Result, store.MediaEntry, error) {
	e, err := p.store.GetMediaEntry(mediaID)
	if err != nil || e == nil {
		return nil, store.MediaEntry{}, fmt.Errorf("找不到该媒体")
	}
	return p.searcher.Search(ctx, SearchKeyword(*e), entryMedia(*e)), *e, nil
}

// SearchKeyword 搜索关键词：剧集带上季集（金装律师 S01E05），电影只用标题
func SearchKeyword(e store.MediaEntry) string {
	switch {
	case e.Season > 0 && e.Episode > 0:
		return fmt.Sprintf("%s S%02dE%02d", e.Title, e.Season, e.Episode)
	case e.Episode > 0:
		return fmt.Sprintf("%s E%02d", e.Title, e.Episode)
	default:
		return e.Title
	}
}

// SearchKeyword 手动关键词聚合搜索
func (p *Pipeline) SearchKeyword(ctx context.Context, keyword string) *search.Result {
	m := metadata.MediaInfo{Title: keyword, Type: metadata.Movie, Source: "manual"}
	return p.searcher.Search(ctx, keyword, m)
}

// Download 下载一条候选并落盘：mediaID 为空表示手动关键词下载。
// nameHint 是搜索命中的显示名，手动下载时优先用它命名文件。
// 成功/失败都会记入下载历史；关联媒体时更新其字幕状态。
func (p *Pipeline) Download(ctx context.Context, source, refID, mediaID, nameHint string) (string, error) {
	var mediaTitle, videoPath string
	if mediaID != "" {
		e, err := p.store.GetMediaEntry(mediaID)
		if err != nil || e == nil {
			return "", fmt.Errorf("找不到该媒体")
		}
		mediaTitle, videoPath = e.Title, e.FilePath
	}
	fname, data, err := p.searcher.DownloadRef(ctx, source, refID)
	if err != nil {
		p.record(source, fname, "", mediaTitle, "failed", err.Error())
		if mediaID != "" {
			_ = p.store.SetMediaSub(mediaID, "failed", "", source)
		}
		return "", err
	}
	subDir := p.store.SubtitleDir()
	targetLang := p.store.TargetLang()
	var saved string
	if videoPath != "" {
		saved, err = downloader.SaveWithDir(videoPath, subDir, targetLang, fname, data)
	} else {
		dir := subDir
		if dir == "" {
			dir = filepath.Join(p.cfg.DataDir, "downloads")
		}
		if nameHint != "" {
			fname = nameHint
		}
		saved, err = downloader.SaveManual(dir, fname, data)
	}
	if err != nil {
		p.record(source, fname, "", mediaTitle, "failed", err.Error())
		if mediaID != "" {
			_ = p.store.SetMediaSub(mediaID, "failed", "", source)
		}
		return "", fmt.Errorf("保存失败: %v", err)
	}
	p.record(source, fname, saved, mediaTitle, "ok", "")
	if mediaID != "" {
		_ = p.store.SetMediaSub(mediaID, "ok", saved, source)
	}
	log.Printf("[pipeline] 已下载: %s ← %s [%s]", mediaTitleOr(fname, mediaTitle), fname, source)
	return saved, nil
}

// DownloadBest 为一部媒体搜索并下载最佳匹配
func (p *Pipeline) DownloadBest(ctx context.Context, mediaID string) (string, error) {
	res, e, err := p.SearchMedia(ctx, mediaID)
	if err != nil {
		return "", err
	}
	cands := p.searcher.ToCandidates(res.Groups)
	ranked := matcher.PickRanked(cands, entryMedia(e), p.store.TargetLang())
	if len(ranked) == 0 || ranked[0].Score < 30 {
		_ = p.store.SetMediaSub(mediaID, "missing", "", "")
		return "", fmt.Errorf("无合适字幕")
	}
	// 按分数从高到低依次尝试下载：最佳候选下载失败就换下一个，
	// 而不是直接放弃（比如 subhd 某个预览无可用内容，换条候选往往能下到）
	var lastErr error
	tried := 0
	for _, rc := range ranked {
		if rc.Score < 30 || tried >= 3 {
			break
		}
		tried++
		best := rc.Candidate
		fname, data, err := best.Download(ctx)
		if err != nil {
			p.record(best.Source, best.Name, "", e.Title, "failed", err.Error())
			log.Printf("[pipeline] 候选下载失败，换下一个: %s [%s]: %v", best.Name, best.Source, err)
			lastErr = err
			continue
		}
		saved, err := downloader.SaveWithDir(e.FilePath, p.store.SubtitleDir(), p.store.TargetLang(), fname, data)
		if err != nil {
			p.record(best.Source, best.Name, "", e.Title, "failed", err.Error())
			_ = p.store.SetMediaSub(mediaID, "failed", "", best.Source)
			return "", fmt.Errorf("保存失败: %v", err)
		}
		p.record(best.Source, best.Name, saved, e.Title, "ok", "")
		_ = p.store.SetMediaSub(mediaID, "ok", saved, best.Source)
		log.Printf("[pipeline] 已下载最佳: %s ← %s [%s]", e.Title, best.Name, best.Source)
		return saved, nil
	}
	_ = p.store.SetMediaSub(mediaID, "failed", "", ranked[0].Candidate.Source)
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("无合适字幕")
}

func (p *Pipeline) record(source, filename, savePath, mediaTitle, status, detail string) {
	_ = p.store.AddDownloadRecord(&store.DownloadRecord{
		Source: source, Filename: filename, SavePath: savePath,
		MediaTitle: mediaTitle, Status: status, Detail: detail,
	})
}

func mediaTitleOr(fname, title string) string {
	if title != "" {
		return title
	}
	return fname
}
