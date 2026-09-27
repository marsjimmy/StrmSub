// Package downloader 把字幕落盘到 .strm 旁边，命名兼容主流播放器。
package downloader

import (
	"os"
	"path/filepath"
	"strings"
)

// Save 把字幕写到 strmPath 旁边：<basename>.zh.srt / <basename>.zh-Hant.ass
// 已存在则跳过。返回最终文件路径。
func Save(strmPath, lang string, filename string, data []byte) (string, error) {
	return SaveWithDir(strmPath, "", lang, filename, data)
}

// SaveWithDir 保存字幕：subDir 为空则保存在视频旁边，否则保存在 subDir 下
// （文件名仍为 <视频basename>.<lang>.<ext>）。已存在则跳过不覆盖。
func SaveWithDir(videoPath, subDir, lang string, filename string, data []byte) (string, error) {
	ext := subExt(filename)
	suffix := "zh"
	if lang == "zh-Hant" {
		suffix = "zh-Hant"
	}
	dir := filepath.Dir(videoPath)
	if subDir != "" {
		dir = subDir
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", err
		}
	}
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	target := filepath.Join(dir, base+"."+suffix+ext)
	if _, err := os.Stat(target); err == nil {
		return target, nil // 已有，不覆盖
	}
	if !looksLikeSubtitle(data) {
		return "", os.ErrInvalid
	}
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", err
	}
	return target, nil
}

// SaveManual 手动关键词下载的字幕（无关联视频）：保存到 dir 下，文件名清洗，
// 重名自动加序号。返回最终文件路径。
func SaveManual(dir, filename string, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	base := sanitizeFilename(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if base == "" {
		base = "subtitle"
	}
	ext := subExt(filename)
	target := filepath.Join(dir, base+ext)
	for i := 2; ; i++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		target = filepath.Join(dir, base+"_"+itoa(i)+ext)
		if i > 100 {
			return "", os.ErrExist
		}
	}
	if !looksLikeSubtitle(data) {
		return "", os.ErrInvalid
	}
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", err
	}
	return target, nil
}

func subExt(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".srt" && ext != ".ass" && ext != ".ssa" {
		ext = ".srt"
	}
	return ext
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), " .")
	if len(out) > 120 {
		out = string([]rune(out)[:120])
	}
	return out
}

func itoa(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// HasSubtitle 检查视频旁边（或 subDir 下）是否已有中文字幕
func HasSubtitle(strmPath string) bool {
	return HasSubtitleIn(strmPath, "")
}

// HasSubtitleIn 检查视频旁边或指定字幕目录下是否已有中文字幕
func HasSubtitleIn(videoPath, subDir string) bool {
	dirs := []string{filepath.Dir(videoPath)}
	if subDir != "" {
		dirs = append(dirs, subDir)
	}
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	for _, dir := range dirs {
		for _, suffix := range []string{".zh.srt", ".zh-Hant.srt", ".zh.ass", ".zh-Hant.ass", ".zh.ssa", ".chs.srt", ".cht.srt"} {
			if _, err := os.Stat(filepath.Join(dir, base+suffix)); err == nil {
				return true
			}
		}
	}
	return false
}

func looksLikeSubtitle(data []byte) bool {
	if len(data) < 20 {
		return false
	}
	s := string(data[:min(len(data), 4096)])
	return strings.Contains(s, "-->") // srt/ass/vtt 都有时间轴箭头
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
