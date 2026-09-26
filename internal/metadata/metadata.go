// Package metadata 定义"媒体是谁"的抽象。
// 识别不靠猜文件名，而是靠已经刮削好的元数据：
//   - 飞牛影视：自有 SQLite（trimmedia.db），只读挂载
//   - NFO：Kodi/Jellyfin/TMM 等写入的 .nfo（兼容备用）
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
	ID            string // provider 内唯一键（飞牛 guid / nfo 路径）
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
	PosterURL     string // 海报原始引用（URL/相对路径/本地路径，provider 填）
	PosterPath    string // 已下载到 data/posters/ 的相对路径（pipeline 填）
	FilePath      string // 对应的 .strm 文件路径
	Source        string // "fnos" / "nfo"
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
