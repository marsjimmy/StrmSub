package fnosapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/marsjimmy/strmsub/internal/metadata"
)

// Provider 通过飞牛 HTTP API 实现 metadata.Provider。
type Provider struct {
	client  *Client
	mapPath func(string) string // 飞牛侧路径 -> 本机路径
}

func New(baseURL, user, pass string, mapPath func(string) string) *Provider {
	if mapPath == nil {
		mapPath = func(s string) string { return s }
	}
	return &Provider{client: NewClient(baseURL, user, pass), mapPath: mapPath}
}

func (p *Provider) Name() string { return "fnosapi" }
func (p *Provider) Close() error { return nil }

// apiItem 条目原始结构（字段名以官方前端为准，缺失则忽略）
type apiItem struct {
	GUID     string
	Title    string
	Category string
	Path     string
	Overview string // 简介
	Poster   string // 海报引用（URL/相对路径）
	Meta     map[string]any
	raw      map[string]any
}

func parseItem(m map[string]any) apiItem {
	it := apiItem{raw: m, Meta: map[string]any{}}
	it.GUID = strOf(m, "guid", "id")
	it.Title = strOf(m, "title", "name")
	it.Category = strOf(m, "category", "type")
	// 注意：列表里的 path 常是海报路径，真实文件路径从详情接口拿
	it.Path = strOf(m, "path")
	it.Overview = strOf(m, "overview", "plot", "description", "summary", "intro", "storyline")
	it.Poster = extractPosterRef(m)
	if it.Poster == "" && looksLikeImage(it.Path) {
		it.Poster = it.Path // path 常是海报路径
	}
	if meta, ok := m["meta"].(map[string]any); ok {
		it.Meta = meta
		if it.Overview == "" {
			it.Overview = strOf(meta, "overview", "plot", "description", "summary")
		}
		if it.Poster == "" {
			it.Poster = extractPosterRef(meta)
		}
	}
	return it
}

// extractPosterRef 从条目里抠海报引用（字段名防御式匹配）
func extractPosterRef(m map[string]any) string {
	for _, k := range []string{"poster", "poster_path", "posterpath", "thumb", "thumbnail",
		"cover", "cover_path", "artwork", "image", "img", "pic", "poster_url"} {
		if s := strOf(m, k); s != "" {
			return s
		}
	}
	// 嵌套 images: {"poster": "..."} / [{"type":"poster","url":"..."}]
	if im, ok := m["images"].(map[string]any); ok {
		for _, k := range []string{"poster", "thumb", "cover"} {
			if s := strOf(im, k); s != "" {
				return s
			}
		}
	}
	if arr, ok := m["images"].([]any); ok {
		for _, e := range arr {
			if em, ok := e.(map[string]any); ok {
				t := strings.ToLower(strOf(em, "type", "kind"))
				if strings.Contains(t, "poster") || strings.Contains(t, "thumb") {
					if s := strOf(em, "url", "path", "src"); s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

// ListMedia 拉取全库：电影直接收录，电视剧展开季/集。
func (p *Provider) ListMedia(ctx context.Context) ([]metadata.MediaInfo, error) {
	items, err := p.fetchAllItems(ctx)
	if err != nil {
		return nil, err
	}
	log.Printf("[fnosapi] 条目 %d 个，开始展开详情", len(items))

	var (
		mu  sync.Mutex
		out []metadata.MediaInfo
		sem = make(chan struct{}, 8) // 详情并发上限
		wg  sync.WaitGroup
	)
	for _, it := range items {
		it := it
		if isDirectory(it) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			got, err := p.expandItem(ctx, it)
			if err != nil {
				log.Printf("[fnosapi] 展开 %s 失败: %v", it.Title, err)
				return
			}
			mu.Lock()
			out = append(out, got...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	log.Printf("[fnosapi] 从飞牛 API 读到 %d 条媒体", len(out))
	return out, nil
}

// fetchAllItems 分页拉 POST /v/api/v1/item/list
func (p *Provider) fetchAllItems(ctx context.Context) ([]apiItem, error) {
	var items []apiItem
	page := 1
	for {
		body := map[string]any{
			"exclude_grouped_video": 1,
			"page":                  page,
			"page_size":             200,
			"sort_column":           "create_time",
			"sort_type":             "DESC",
			"tags":                  map[string]any{"type": []string{"Movie", "TV"}},
		}
		data, err := p.client.Post(ctx, apiPrefix+"/item/list", body)
		if err != nil {
			return nil, err
		}
		var pg struct {
			List  []map[string]any `json:"list"`
			Total int              `json:"total"`
		}
		if err := json.Unmarshal(data, &pg); err != nil {
			return nil, fmt.Errorf("item/list 解析失败: %w", err)
		}
		for _, m := range pg.List {
			items = append(items, parseItem(m))
		}
		if len(items) >= pg.Total || len(pg.List) == 0 {
			break
		}
		page++
	}
	return items, nil
}

func isDirectory(it apiItem) bool {
	c := strings.ToLower(it.Category)
	return strings.Contains(c, "director")
}

func isTV(it apiItem) bool {
	c := strings.ToLower(it.Category)
	return strings.Contains(c, "tv") || strings.Contains(c, "show") || strings.Contains(c, "series")
}

// expandItem 把一个库条目展开成一条或多条 MediaInfo
func (p *Provider) expandItem(ctx context.Context, it apiItem) ([]metadata.MediaInfo, error) {
	if !isTV(it) {
		mi := p.movieInfo(it)
		p.fillPath(ctx, &mi, it.GUID, it)
		return []metadata.MediaInfo{mi}, nil
	}
	// 电视剧：先收录剧条目本身（剧级海报/简介），再展开季/集
	seriesMi := metadata.MediaInfo{
		ID:          it.GUID,
		Type:        metadata.Series,
		Title:       it.Title,
		Year:        numOf([]string{"year", "release_year", "air_year"}, it.raw, it.Meta),
		ImdbID:      normImdb(strOf(it.raw, "imdb_id", "imdbid")),
		TmdbID:      strOf(it.raw, "tmdb_id", "tmdbid"),
		Overview:    it.Overview,
		PosterURL:   it.Poster,
		Source:      "fnosapi",
	}
	if seriesMi.Year == 0 {
		seriesMi.Year = yearOf(strOf(it.raw, "release_date", "first_air_date", "premiered"))
	}
	out := []metadata.MediaInfo{seriesMi}

	// 电视剧：季列表 -> 集列表
	seasons, err := p.getList(ctx, apiPrefix+"/season/list/"+it.GUID)
	if err != nil {
		return out, nil // 季拉不到也别丢了剧条目
	}
	for i, s := range seasons {
		sn := numOf([]string{"season", "season_number"}, s.raw, s.Meta)
		if sn == 0 {
			sn = i + 1
		}
		seasonID := s.GUID
		if seasonID == "" {
			seasonID = it.GUID + "/S" + pad2(sn)
		}
		out = append(out, metadata.MediaInfo{
			ID:         seasonID,
			Type:       metadata.Season,
			Title:      it.Title,
			Year:       seriesMi.Year,
			Season:     sn,
			SeriesID:   it.GUID,
			Overview:   s.Overview,
			PosterURL:  s.Poster,
			Source:     "fnosapi",
		})
		episodes, err := p.getList(ctx, apiPrefix+"/episode/list/"+s.GUID)
		if err != nil {
			log.Printf("[fnosapi] 集列表失败 %s: %v", s.Title, err)
			continue
		}
		for _, ep := range episodes {
			mi := metadata.MediaInfo{
				ID:            ep.GUID,
				Type:          metadata.Episode,
				Title:         it.Title, // 用剧名做标题，单集名不参与字幕搜索
				OriginalTitle: strOf(it.raw, "original_title", "originaltitle"),
				Year:          seriesMi.Year,
				Season:        sn,
				Episode:       numOf([]string{"episode", "episode_number", "index"}, ep.raw, ep.Meta),
				SeriesID:      it.GUID,
				ImdbID:        seriesMi.ImdbID,
				TmdbID:        seriesMi.TmdbID,
				Overview:      ep.Overview,
				Source:        "fnosapi",
			}
			p.fillPath(ctx, &mi, ep.GUID, ep)
			out = append(out, mi)
		}
	}
	return out, nil
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func (p *Provider) movieInfo(it apiItem) metadata.MediaInfo {
	mi := metadata.MediaInfo{
		ID:            it.GUID,
		Type:          metadata.Movie,
		Title:         it.Title,
		OriginalTitle: strOf(it.raw, "original_title", "originaltitle"),
		Year:          numOf([]string{"year", "release_year"}, it.raw, it.Meta),
		ImdbID:        normImdb(strOf(it.raw, "imdb_id", "imdbid")),
		TmdbID:        strOf(it.raw, "tmdb_id", "tmdbid"),
		Overview:      it.Overview,
		PosterURL:     it.Poster,
		Source:        "fnosapi",
	}
	if mi.Year == 0 {
		mi.Year = yearOf(strOf(it.raw, "release_date", "premiered"))
	}
	return mi
}

// FetchPoster 下载海报字节（pipeline 在扫描时调用，结果存 data/posters/）
func (p *Provider) FetchPoster(ctx context.Context, ref string) ([]byte, error) {
	return p.client.GetBytes(ctx, ref)
}

// getList 拿返回 {"code":0,"data":[...]} 的列表型接口
func (p *Provider) getList(ctx context.Context, path string) ([]apiItem, error) {
	data, err := p.client.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("%s 解析失败: %w", path, err)
	}
	out := make([]apiItem, 0, len(list))
	for _, m := range list {
		out = append(out, parseItem(m))
	}
	return out, nil
}

// fillPath 补文件路径：先看条目自带 meta，没有再调详情接口，最后做路径映射
func (p *Provider) fillPath(ctx context.Context, mi *metadata.MediaInfo, guid string, it apiItem) {
	if pa := extractPath(it.raw); pa != "" {
		mi.FilePath = p.mapPath(pickMediaPath(pa))
		return
	}
	if pa := extractPath(it.Meta); pa != "" {
		mi.FilePath = p.mapPath(pickMediaPath(pa))
		return
	}
	data, err := p.client.Get(ctx, apiPrefix+"/item/"+guid, nil)
	if err != nil {
		return
	}
	var detail map[string]any
	if err := json.Unmarshal(data, &detail); err != nil {
		return
	}
	if pa := extractPath(detail); pa != "" {
		mi.FilePath = p.mapPath(pickMediaPath(pa))
	}
}

// extractPath 从详情/条目里抠文件路径，候选键防御式匹配
func extractPath(m map[string]any) string {
	for _, k := range []string{"file_path", "filepath", "src", "file", "path"} {
		if s := strOf(m, k); s != "" && looksLikeMedia(s) {
			return s
		}
	}
	// 嵌套的文件列表
	for _, k := range []string{"files", "medias", "file_list", "streams"} {
		if arr, ok := m[k].([]any); ok {
			for _, e := range arr {
				if em, ok := e.(map[string]any); ok {
					if s := extractPath(em); s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

// looksLikeMedia 粗判是不是媒体文件路径（排除海报/缩略图）
func looksLikeMedia(s string) bool {
	ls := strings.ToLower(s)
	if strings.Contains(ls, "poster") || strings.Contains(ls, "thumb") || strings.Contains(ls, "backdrop") ||
		strings.Contains(ls, "logo") || strings.Contains(ls, ".jpg") || strings.Contains(ls, ".png") || strings.Contains(ls, ".webp") {
		return false
	}
	return strings.HasPrefix(s, "/") || strings.HasPrefix(ls, "http")
}

// looksLikeImage 粗判是不是图片引用
func looksLikeImage(s string) bool {
	ls := strings.ToLower(s)
	return strings.Contains(ls, ".jpg") || strings.Contains(ls, ".jpeg") ||
		strings.Contains(ls, ".png") || strings.Contains(ls, ".webp") ||
		strings.Contains(ls, "poster") || strings.Contains(ls, "thumb") ||
		strings.Contains(ls, "cover") || strings.Contains(ls, "artwork")
}

// pickMediaPath 列表里 path 可能是海报，这里只做透传，looksLikeMedia 已过滤
func pickMediaPath(s string) string { return s }

// Probe 诊断：登录并抓一条样本的原始 JSON，贴回确认字段映射
func (p *Provider) Probe(ctx context.Context) (string, error) {
	var sb strings.Builder
	sb.WriteString("== 飞牛 API 连通性 ==\n")
	if err := p.client.login(ctx); err != nil {
		return "", err
	}
	sb.WriteString("✅ 登录成功\n")

	data, err := p.client.Post(ctx, apiPrefix+"/item/list", map[string]any{
		"exclude_grouped_video": 1,
		"page":                  1,
		"page_size":             5,
		"sort_column":           "create_time",
		"sort_type":             "DESC",
		"tags":                  map[string]any{"type": []string{"Movie", "TV"}},
	})
	if err != nil {
		return sb.String(), err
	}
	var pg struct {
		List  []map[string]any `json:"list"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(data, &pg); err != nil {
		return sb.String(), err
	}
	fmt.Fprintf(&sb, "✅ item/list 成功，共 %d 条\n", pg.Total)
	for i, m := range pg.List {
		pretty, _ := json.MarshalIndent(m, "", "  ")
		fmt.Fprintf(&sb, "== 样本条目 %d 原始 JSON ==\n%s\n", i+1, pretty)
		it := parseItem(m)
		if isTV(it) {
			seasons, err := p.getList(ctx, apiPrefix+"/season/list/"+it.GUID)
			if err != nil || len(seasons) == 0 {
				continue
			}
			eps, err := p.getList(ctx, apiPrefix+"/episode/list/"+seasons[0].GUID)
			if err != nil || len(eps) == 0 {
				continue
			}
			epPretty, _ := json.MarshalIndent(eps[0].raw, "", "  ")
			fmt.Fprintf(&sb, "== 剧集样本 episode 原始 JSON ==\n%s\n", epPretty)
			// 再抓一条详情看文件路径字段
			d, err := p.client.Get(ctx, apiPrefix+"/item/"+eps[0].GUID, nil)
			if err == nil {
				var dm map[string]any
				if json.Unmarshal(d, &dm) == nil {
					dp, _ := json.MarshalIndent(dm, "", "  ")
					fmt.Fprintf(&sb, "== 单集详情原始 JSON ==\n%s\n", dp)
				}
			}
			break
		}
		if i >= 1 {
			break
		}
	}
	return sb.String(), nil
}

// ---- 小工具 ----

func strOf(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return strings.TrimSpace(t)
				}
			case float64:
				return strconv.FormatFloat(t, 'f', -1, 64)
			}
		}
	}
	return ""
}

func numOf(keys []string, maps ...map[string]any) int {
	for _, m := range maps {
		if m == nil {
			continue
		}
		for _, k := range keys {
			v, ok := m[k]
			if !ok {
				continue
			}
			switch t := v.(type) {
			case float64:
				if int(t) != 0 {
					return int(t)
				}
			case int:
				if t != 0 {
					return t
				}
			case string:
				s := strings.TrimSpace(t)
				if n, err := strconv.Atoi(s); err == nil && n != 0 {
					return n
				}
				if len(s) >= 4 { // "2021-03-05" 取年
					if n, err := strconv.Atoi(s[:4]); err == nil && n != 0 {
						return n
					}
				}
			}
		}
	}
	return 0
}

func yearOf(s string) int {
	s = strings.TrimSpace(s)
	if len(s) >= 4 {
		if n, err := strconv.Atoi(s[:4]); err == nil {
			return n
		}
	}
	return 0
}

func normImdb(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "tt") {
		return s
	}
	if _, err := strconv.Atoi(s); err == nil {
		return "tt" + s
	}
	return s
}
