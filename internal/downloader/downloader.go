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
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".srt" && ext != ".ass" && ext != ".ssa" {
		ext = ".srt"
	}
	suffix := "zh"
	if lang == "zh-Hant" {
		suffix = "zh-Hant"
	}
	dir := filepath.Dir(strmPath)
	base := strings.TrimSuffix(filepath.Base(strmPath), filepath.Ext(strmPath))
	target := filepath.Join(dir, base+"."+suffix+ext)
	if _, err := os.Stat(target); err == nil {
		return target, nil // 已有，不覆盖
	}
	// 简单校验：srt/ass 文本应包含时间轴标记
	if !looksLikeSubtitle(data) {
		return "", os.ErrInvalid
	}
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", err
	}
	return target, nil
}

// HasSubtitle 检查 .strm 旁边是否已有中文字幕
func HasSubtitle(strmPath string) bool {
	dir := filepath.Dir(strmPath)
	base := strings.TrimSuffix(filepath.Base(strmPath), filepath.Ext(strmPath))
	for _, suffix := range []string{".zh.srt", ".zh-Hant.srt", ".zh.ass", ".zh-Hant.ass", ".zh.ssa", ".chs.srt", ".cht.srt"} {
		if _, err := os.Stat(filepath.Join(dir, base+suffix)); err == nil {
			return true
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
