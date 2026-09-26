// Package nfo 解析 Kodi/Jellyfin/TMM 等写入的 .nfo 文件。
// 作为飞牛 SQLite 的补充：当某条媒体在飞牛库里缺 ID 时，用同目录 .nfo 补全。
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
	UniqueIDs     []uniqueID `xml:"uniqueid"`
}

type nfoEpisode struct {
	XMLName   xml.Name   `xml:"episodedetails"`
	Title     string     `xml:"title"`
	ShowTitle string     `xml:"showtitle"`
	Season    int        `xml:"season"`
	Episode   int        `xml:"episode"`
	Aired     string     `xml:"aired"`
	UniqueIDs []uniqueID `xml:"uniqueid"`
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
	case "episodedetails":
		var e nfoEpisode
		if err := xml.Unmarshal(data, &e); err != nil {
			return metadata.MediaInfo{}, false
		}
		mi.Type = metadata.Episode
		mi.Title = strings.TrimSpace(e.ShowTitle)
		if mi.Title == "" {
			mi.Title = strings.TrimSpace(e.Title)
		}
		mi.Season, mi.Episode = e.Season, e.Episode
		if len(e.Aired) >= 4 {
			mi.Year, _ = strconv.Atoi(e.Aired[:4])
		}
		mi.ImdbID, mi.TmdbID = ids(e.UniqueIDs)
	default:
		return metadata.MediaInfo{}, false // tvshow/season 等暂不需要
	}
	if mi.Title == "" {
		return metadata.MediaInfo{}, false
	}
	return mi, true
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
