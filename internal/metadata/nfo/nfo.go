// Package nfo 解析 Kodi/Jellyfin/TMM 等写入的 .nfo 文件。
// 标题识别时作为自定义正则之后的第二优先级：同名 .nfo 里的 <title>/<year>。
package nfo

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/marsjimmy/strmsub/internal/metadata"
)

type uniqueID struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type nfoMovie struct {
	XMLName       xml.Name   `xml:"movie"`
	Title         string     `xml:"title"`
	OriginalTitle string     `xml:"originaltitle"`
	Year          int        `xml:"year"`
	Premiered     string     `xml:"premiered"`
	Plot          string     `xml:"plot"`
	Thumb         string     `xml:"thumb"`
	UniqueIDs     []uniqueID `xml:"uniqueid"`
}

type nfoEpisode struct {
	XMLName   xml.Name   `xml:"episodedetails"`
	Title     string     `xml:"title"`
	ShowTitle string     `xml:"showtitle"`
	Season    int        `xml:"season"`
	Episode   int        `xml:"episode"`
	Aired     string     `xml:"aired"`
	Plot      string     `xml:"plot"`
	Thumb     string     `xml:"thumb"`
	UniqueIDs []uniqueID `xml:"uniqueid"`
}

type nfoTvshow struct {
	XMLName       xml.Name   `xml:"tvshow"`
	Title         string     `xml:"title"`
	OriginalTitle string     `xml:"originaltitle"`
	Year          int        `xml:"year"`
	Premiered     string     `xml:"premiered"`
	Plot          string     `xml:"plot"`
	Thumb         string     `xml:"thumb"`
	UniqueIDs     []uniqueID `xml:"uniqueid"`
}

type Provider struct {
	mediaDirs []string
}

func New(mediaDirs []string) *Provider { return &Provider{mediaDirs: mediaDirs} }

func (p *Provider) Name() string { return "nfo" }
func (p *Provider) Close() error { return nil }

func ids(uds []uniqueID) (imdb, tmdb string) {
	for _, u := range uds {
		v := strings.TrimSpace(u.Value)
		switch strings.ToLower(u.Type) {
		case "imdb":
			imdb = v
			if !strings.HasPrefix(imdb, "tt") {
				imdb = "tt" + imdb
			}
		case "tmdb":
			tmdb = v
		}
	}
	return
}

// ParseFile 解析单个 .nfo，返回媒体信息（不含 FilePath，由调用方配对）
func ParseFile(path string) (metadata.MediaInfo, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return metadata.MediaInfo{}, false
	}
	// 先看根节点名做分派
	type root struct {
		XMLName xml.Name
	}
	var r root
	if err := xml.Unmarshal(data, &r); err != nil {
		return metadata.MediaInfo{}, false
	}
	var mi metadata.MediaInfo
	mi.Source = "nfo"
	mi.ID = path
	switch r.XMLName.Local {
	case "movie":
		var m nfoMovie
		if err := xml.Unmarshal(data, &m); err != nil {
			return metadata.MediaInfo{}, false
		}
		mi.Type = metadata.Movie
		mi.Title = strings.TrimSpace(m.Title)
		mi.OriginalTitle = strings.TrimSpace(m.OriginalTitle)
		mi.Year = m.Year
		if mi.Year == 0 && len(m.Premiered) >= 4 {
			mi.Year, _ = strconv.Atoi(m.Premiered[:4])
		}
		mi.ImdbID, mi.TmdbID = ids(m.UniqueIDs)
		mi.Overview = strings.TrimSpace(m.Plot)
		mi.PosterURL = resolveNFOThumb(path, strings.TrimSpace(m.Thumb))
	case "episodedetails":
		var e nfoEpisode
		if err := xml.Unmarshal(data, &e); err != nil {
			return metadata.MediaInfo{}, false
		}
		mi.Type = metadata.Episode
		// 注意：没有 <showtitle> 时不再回退到单集 <title>（分集名做搜索关键词无意义，
		// 且会导致“识别成 Bail Out”这类误识别）；调用方会用目录名兜底。
		mi.Title = strings.TrimSpace(e.ShowTitle)
		mi.Season, mi.Episode = e.Season, e.Episode
		if len(e.Aired) >= 4 {
			mi.Year, _ = strconv.Atoi(e.Aired[:4])
		}
		mi.ImdbID, mi.TmdbID = ids(e.UniqueIDs)
		mi.Overview = strings.TrimSpace(e.Plot)
		mi.PosterURL = resolveNFOThumb(path, strings.TrimSpace(e.Thumb))
	case "tvshow":
		var t nfoTvshow
		if err := xml.Unmarshal(data, &t); err != nil {
			return metadata.MediaInfo{}, false
		}
		mi.Type = metadata.Series
		mi.Title = strings.TrimSpace(t.Title)
		mi.OriginalTitle = strings.TrimSpace(t.OriginalTitle)
		mi.Year = t.Year
		if mi.Year == 0 && len(t.Premiered) >= 4 {
			mi.Year, _ = strconv.Atoi(t.Premiered[:4])
		}
		mi.ImdbID, mi.TmdbID = ids(t.UniqueIDs)
		mi.Overview = strings.TrimSpace(t.Plot)
		mi.PosterURL = resolveNFOThumb(path, strings.TrimSpace(t.Thumb))
	default:
		return metadata.MediaInfo{}, false // tvshow/season 等暂不需要
	}
	if mi.Title == "" {
		return metadata.MediaInfo{}, false
	}
	return mi, true
}

// resolveNFOThumb 把 NFO 里的 thumb 解析成可用引用：URL 原样返回，
// 相对文件名按 NFO 同目录解析成绝对路径（pipeline 会拷贝到 data/posters/）。
func resolveNFOThumb(nfoPath, thumb string) string {
	if thumb == "" {
		return ""
	}
	if strings.HasPrefix(thumb, "http://") || strings.HasPrefix(thumb, "https://") {
		return thumb
	}
	if filepath.IsAbs(thumb) {
		return thumb
	}
	return filepath.Join(filepath.Dir(nfoPath), thumb)
}

// ListMedia 扫描媒体目录，为每个 .strm 找同名 .nfo 配对
func (p *Provider) ListMedia(ctx context.Context) ([]metadata.MediaInfo, error) {
	var out []metadata.MediaInfo
	for _, dir := range p.mediaDirs {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".strm") {
				return nil
			}
			nfoPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".nfo"
			mi, ok := ParseFile(nfoPath)
			if !ok {
				return nil
			}
			mi.FilePath = path
			out = append(out, mi)
			return nil
		})
	}
	return out, nil
}
