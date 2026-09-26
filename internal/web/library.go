// Package web 媒体库分级接口：电影/电视剧分区、季/集钻取、海报服务。
package web

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/store"
)

// libraryItem 媒体库列表项（电影 / 剧）
type libraryItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
	Poster    string `json:"poster"`
	Overview  string `json:"overview"`
	SubStatus string `json:"subStatus"`
	Seasons   int    `json:"seasons,omitempty"`
	Episodes  int    `json:"episodes,omitempty"`
}

type seasonItem struct {
	ID       string `json:"id"`
	SeriesID string `json:"seriesId"`
	Season   int    `json:"season"`
	Title    string `json:"title"`
	Poster   string `json:"poster"`
	Overview string `json:"overview"`
}

type episodeItem struct {
	ID        string `json:"id"`
	Season    int    `json:"season"`
	Episode   int    `json:"episode"`
	Title     string `json:"title"`
	Overview  string `json:"overview"`
	SubStatus string `json:"subStatus"`
	HasFile   bool   `json:"hasFile"`
}

func posterURL(id string) string {
	if id == "" {
		return ""
	}
	return "/api/poster?id=" + id
}

func toLibraryItem(m store.MediaWithStatus) libraryItem {
	return libraryItem{
		ID: m.ID, Title: m.Title, Year: m.Year,
		Poster: posterURL(m.ID), Overview: m.Overview, SubStatus: m.SubStatus,
	}
}

// handleLibraryMovies 电影列表
func (s *Server) handleLibraryMovies(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListByType(metadata.Movie)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	out := make([]libraryItem, 0, len(list))
	for _, m := range list {
		out = append(out, toLibraryItem(m))
	}
	writeJSON(w, out)
}

// handleLibrarySeries 电视剧列表：库里的 series 行 + 无 series_id 的散集按剧名合成
func (s *Server) handleLibrarySeries(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ListByType(metadata.Series)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	out := make([]libraryItem, 0, len(rows)+4)
	seen := map[string]bool{}
	for _, m := range rows {
		it := toLibraryItem(m)
		it.Seasons, it.Episodes = s.store.SeriesStats(m.ID)
		out = append(out, it)
		seen[m.ID] = true
		seen[strings.ToLower(m.Title)] = true
	}
	// NFO 等来源的散集：按剧名合成剧条目
	orphans, err := s.store.ListOrphanEpisodes()
	if err == nil {
		groups := map[string][]store.MediaWithStatus{}
		var order []string
		for _, m := range orphans {
			key := strings.ToLower(strings.TrimSpace(m.Title))
			if seen[key] {
				continue
			}
			if _, ok := groups[key]; !ok {
				order = append(order, key)
			}
			groups[key] = append(groups[key], m)
		}
		for _, key := range order {
			g := groups[key]
			seasons := map[int]bool{}
			for _, m := range g {
				seasons[m.Season] = true
			}
			poster := ""
			if g[0].PosterPath != "" {
				poster = posterURL(g[0].ID)
			}
			out = append(out, libraryItem{
				ID: "syn:" + g[0].Title, Title: g[0].Title, Year: g[0].Year,
				Poster: poster, Overview: g[0].Overview,
				Seasons: len(seasons), Episodes: len(g),
			})
		}
	}
	writeJSON(w, out)
}

// handleLibrarySeasons 某部剧的季列表
func (s *Server) handleLibrarySeasons(w http.ResponseWriter, r *http.Request) {
	seriesID := r.URL.Query().Get("series")
	if seriesID == "" {
		http.Error(w, "缺少 series 参数", 400)
		return
	}
	var rows []store.MediaWithStatus
	if strings.HasPrefix(seriesID, "syn:") {
		// 合成剧：从散集里 distinct 季
		orphans, err := s.store.ListOrphanEpisodes()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		title := strings.TrimPrefix(seriesID, "syn:")
		seenS := map[int]bool{}
		for _, m := range orphans {
			if m.Title != title || seenS[m.Season] {
				continue
			}
			seenS[m.Season] = true
			rows = append(rows, store.MediaWithStatus{
				MediaInfo: metadata.MediaInfo{
					ID: seriesID + "/S" + strconv.Itoa(m.Season),
					Type: metadata.Season, Title: m.Title,
					Year: m.Year, Season: m.Season, SeriesID: seriesID,
					Overview: m.Overview, PosterPath: m.PosterPath,
				},
			})
		}
	} else {
		var err error
		rows, err = s.store.ListSeasons(seriesID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	out := make([]seasonItem, 0, len(rows))
	for _, m := range rows {
		out = append(out, seasonItem{
			ID: m.ID, SeriesID: seriesID, Season: m.Season, Title: m.Title,
			Poster: posterURL(m.ID), Overview: m.Overview,
		})
	}
	writeJSON(w, out)
}

// handleLibraryEpisodes 某部剧某季的集列表
func (s *Server) handleLibraryEpisodes(w http.ResponseWriter, r *http.Request) {
	seriesID := r.URL.Query().Get("series")
	season, _ := strconv.Atoi(r.URL.Query().Get("season"))
	if seriesID == "" {
		http.Error(w, "缺少 series 参数", 400)
		return
	}
	var rows []store.MediaWithStatus
	if strings.HasPrefix(seriesID, "syn:") {
		orphans, err := s.store.ListOrphanEpisodes()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		title := strings.TrimPrefix(seriesID, "syn:")
		for _, m := range orphans {
			if m.Title == title && m.Season == season {
				rows = append(rows, m)
			}
		}
	} else {
		var err error
		rows, err = s.store.ListEpisodes(seriesID, season)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	out := make([]episodeItem, 0, len(rows))
	for _, m := range rows {
		out = append(out, episodeItem{
			ID: m.ID, Season: m.Season, Episode: m.Episode, Title: m.Title,
			Overview: m.Overview, SubStatus: m.SubStatus, HasFile: m.FilePath != "",
		})
	}
	writeJSON(w, out)
}

// handlePoster 海报服务：data/posters/ 下的文件；单集没有海报时回退到所属剧的海报
func (s *Server) handlePoster(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	m, err := s.store.GetMedia(id)
	if err != nil || m == nil {
		http.NotFound(w, r)
		return
	}
	rel := m.PosterPath
	if rel == "" && m.SeriesID != "" {
		if series, err := s.store.GetMedia(m.SeriesID); err == nil && series != nil {
			rel = series.PosterPath
		}
	}
	if rel == "" || strings.Contains(rel, "..") || !strings.HasPrefix(rel, "posters/") {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.dataDir, rel))
}
