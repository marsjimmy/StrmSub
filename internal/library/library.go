// Package library 递归扫描媒体目录，维护 SQLite 增量索引。
// 增量策略：文件大小+修改时间未变且正则签名未变则跳过；
// 新增文件入库，消失的文件删索引；正则改了则全量重新识别标题。
package library

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/marsjimmy/strmsub/internal/store"
	"github.com/marsjimmy/strmsub/internal/title"
)

// 常见视频扩展名
var videoExts = map[string]bool{
	".strm": true, ".mp4": true, ".mkv": true, ".avi": true,
	".ts": true, ".m2ts": true, ".mts": true, ".wmv": true,
	".flv": true, ".rmvb": true, ".rm": true, ".mov": true,
	".m4v": true, ".mpg": true, ".mpeg": true, ".webm": true,
}

var imageExts = []string{".jpg", ".jpeg", ".png", ".webp", ".gif"}

// 目录级常见封面名
var coverNames = []string{"poster", "folder", "cover", "movie", "default", "backdrop", "landscape"}

type Scanner struct {
	dirs  []string
	store *store.Store
}

func New(dirs []string, st *store.Store) *Scanner {
	return &Scanner{dirs: dirs, store: st}
}

type ScanResult struct {
	Added   int
	Updated int
	Removed int
	Total   int
}

// Scan 执行一轮增量扫描
func (s *Scanner) Scan(ctx context.Context) (ScanResult, error) {
	var res ScanResult
	// 当前正则签名 + 识别器
	rules, err := s.store.ListTitleRules()
	if err != nil {
		return res, err
	}
	ruleSig := s.store.RulesSignature()
	var trules []title.Rule
	for _, r := range rules {
		if r.Enabled {
			trules = append(trules, title.Rule{ID: r.ID, Name: r.Name, Pattern: r.Pattern})
		}
	}
	rec := title.New(trules)

	// 递归收集视频文件
	found := map[string]os.FileInfo{}
	for _, dir := range s.dirs {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, "@") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "#") {
					return filepath.SkipDir
				}
				return nil
			}
			if videoExts[strings.ToLower(filepath.Ext(path))] {
				if fi, err := d.Info(); err == nil {
					found[path] = fi
				}
			}
			return nil
		})
	}

	indexed, err := s.store.MapMediaIndex()
	if err != nil {
		return res, err
	}

	for path, fi := range found {
		size, mtime := fi.Size(), fi.ModTime().Unix()
		if e, ok := indexed[path]; ok {
			delete(indexed, path) // 剩下的是已消失的文件
			if e.Size == size && e.ModTime == mtime && e.RuleSig == ruleSig {
				res.Total++
				continue // 无变化，跳过
			}
			// 文件变了或正则变了：重新识别（保留字幕状态）
			recognized := rec.Recognize(path)
			e.Size, e.ModTime = size, mtime
			e.Title, e.Year, e.Season, e.Episode = recognized.Title, recognized.Year, recognized.Season, recognized.Episode
			e.RuleID, e.RuleSig = recognized.RuleID, ruleSig
			e.CoverPath = FindCover(path)
			if err := s.store.UpsertMediaEntry(&e); err == nil {
				res.Updated++
			}
			res.Total++
			continue
		}
		recognized := rec.Recognize(path)
		e := store.MediaEntry{
			FilePath:  path,
			Size:      size,
			ModTime:   mtime,
			Title:     recognized.Title,
			Year:      recognized.Year,
			Season:    recognized.Season,
			Episode:   recognized.Episode,
			RuleID:    recognized.RuleID,
			RuleSig:   ruleSig,
			CoverPath: FindCover(path),
		}
		if err := s.store.UpsertMediaEntry(&e); err == nil {
			res.Added++
		}
		res.Total++
	}
	// 清理已消失的文件
	for _, e := range indexed {
		if err := s.store.DeleteMediaEntry(e.ID); err == nil {
			res.Removed++
		}
	}
	return res, nil
}

// FindCover 找封面：同名图片优先，其次目录级常见封面名。返回绝对路径，找不到返回 ""。
func FindCover(videoPath string) string {
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	exists := func(p string) bool {
		fi, err := os.Stat(p)
		return err == nil && !fi.IsDir() && fi.Size() > 0 && fi.Size() < 20<<20
	}
	for _, ext := range imageExts {
		if p := filepath.Join(dir, base+ext); exists(p) {
			return p
		}
	}
	for _, name := range coverNames {
		for _, ext := range imageExts {
			if p := filepath.Join(dir, name+ext); exists(p) {
				return p
			}
		}
	}
	return ""
}
