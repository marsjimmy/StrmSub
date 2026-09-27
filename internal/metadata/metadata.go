// Package metadata 定义字幕搜索用的媒体信息抽象。
// 标题来自文件名正则识别（可自定义规则），无需依赖外部元数据 API。
package metadata

import "context"

type MediaType string

const (
	Movie   MediaType = "movie"
	Episode MediaType = "episode"
	Series  MediaType = "series" // 电视剧条目本身（剧级海报/简介）
	Season  MediaType = "season" // 季条目
)

// MediaInfo 一部电影、一部剧、一季或一集剧集。
type MediaInfo struct {
	ID            string // 媒体唯一键（sha1 路径哈希）
	Type          MediaType
	Title         string
	OriginalTitle string
	Year          int
	Season        int
	Episode       int
	SeriesID      string // 剧集/季所属的剧 ID（电影为空）
	ImdbID        string // tt1234567
	TmdbID        string
	Overview      string // 简介
	PosterURL     string // 海报原始引用（URL/相对路径/本地路径）
	PosterPath    string // 本地封面路径（library 扫描时识别）
	FilePath      string // 对应的视频文件路径
	Source        string // 标题来源: "rule" / "nfo" / "builtin"
}

// DisplayName 展示用名称
func (m MediaInfo) DisplayName() string {
	if m.Type == Episode {
		return m.Title
	}
	if m.Year > 0 {
		return m.Title
	}
	return m.Title
}

// Provider 元数据提供者
type Provider interface {
	Name() string
	// ListMedia 列出库中所有已识别媒体（含 .strm 文件路径）
	ListMedia(ctx context.Context) ([]MediaInfo, error)
	Close() error
}
