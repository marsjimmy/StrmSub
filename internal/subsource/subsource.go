// Package subsource 定义字幕源的统一抽象。
// 每个源实现 Search（按媒体找候选）即可，Download 延迟到真正选中时才执行
// （ASSRT 等源的下载链接有时效，不能提前拿）。
//
// DownloadRef 支持从缓存的搜索结果下载：缓存只存可序列化的 Hit，
// 下载时凭 RefID 重新定位到源内资源。
package subsource

import (
	"context"

	"github.com/marsjimmy/strmsub/internal/metadata"
)

// 语言归一化值
const (
	LangHans      = "zh-Hans"      // 简体
	LangHant      = "zh-Hant"      // 繁体
	LangBilingual = "zh-Bilingual" // 双语（中英）
	LangOther     = "other"
)

// Candidate 一条候选字幕
type Candidate struct {
	Source string // assrt / opensubtitles / subdl / subhd / xunlei / subtitlecat
	RefID  string // 源内定位符，DownloadRef 可用
	Name   string // 展示名
	Lang   string // 归一化语言
	Format string // srt / ass
	Votes  int    // 源内评分/下载量，供打分参考
	Detail string // 备注（字幕组、上传时间等）

	// Download 选中后才调用，返回文件名与内容
	Download func(ctx context.Context) (filename string, data []byte, err error)
}

// Hit 可序列化的候选（搜索缓存用）
type Hit struct {
	Source string `json:"source"`
	RefID  string `json:"ref_id"`
	Name   string `json:"name"`
	Lang   string `json:"lang"`
	Format string `json:"format"`
	Votes  int    `json:"votes"`
	Detail string `json:"detail"`
}

func (c Candidate) ToHit() Hit {
	return Hit{Source: c.Source, RefID: c.RefID, Name: c.Name, Lang: c.Lang,
		Format: c.Format, Votes: c.Votes, Detail: c.Detail}
}

// Source 字幕源
type Source interface {
	Name() string
	Enabled() bool
	Search(ctx context.Context, m metadata.MediaInfo) ([]Candidate, error)
	// DownloadRef 按 RefID 下载（缓存命中后走这里）
	DownloadRef(ctx context.Context, refID string) (filename string, data []byte, err error)
}

// IsChinese 目标语言是否命中
func IsChinese(lang, target string) bool {
	switch target {
	case LangHans:
		return lang == LangHans || lang == LangBilingual
	case LangHant:
		return lang == LangHant || lang == LangBilingual
	default:
		return lang == LangHans || lang == LangHant || lang == LangBilingual
	}
}
