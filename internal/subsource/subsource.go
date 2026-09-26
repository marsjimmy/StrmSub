// Package subsource 定义字幕源的统一抽象。
// 每个源实现 Search（按媒体找候选）即可，Download 延迟到真正选中时才执行
//（ASSRT 等源的下载链接有时效，不能提前拿）。
package subsource

import (
	"context"

	"github.com/marsjimmy/strmsub/internal/metadata"
)

// 语言归一化值
const (
	LangHans      = "zh-Hans"     // 简体
	LangHant      = "zh-Hant"     // 繁体
	LangBilingual = "zh-Bilingual" // 双语（中英）
	LangOther     = "other"
)

// Candidate 一条候选字幕
type Candidate struct {
	Source  string // assrt / opensubtitles / subdl
	RefID   string // 源内 ID
	Name    string // 展示名
	Lang    string // 归一化语言
	Format  string // srt / ass
	Votes   int    // 源内评分/下载量，供打分参考
	Detail  string // 备注（字幕组、上传时间等）

	// Download 选中后才调用，返回文件名与内容
	Download func(ctx context.Context) (filename string, data []byte, err error)
}

// Source 字幕源
type Source interface {
	Name() string
	Enabled() bool
	Search(ctx context.Context, m metadata.MediaInfo) ([]Candidate, error)
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
