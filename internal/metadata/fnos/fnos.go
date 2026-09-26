// Package fnos 从飞牛影视的 SQLite 数据库只读读取刮削好的元数据。
//
// 飞牛影视不写 NFO，元数据在其自有库中：
//  宿主机路径: /usr/local/apps/@appdata/trim.media/database/trimmedia.db
//  已知表: item(影片级), item_media(文件级)
//
// 因为飞牛是闭源应用，表结构可能随版本变化，本实现启动时用 PRAGMA
// 探测实际列名，按候选名模糊匹配，未知列会被忽略并打日志，
// 不会因为多一列/少一列就崩。
package fnos

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/marsjimmy/strmsub/internal/metadata"
)

type Provider struct {
	db           *sql.DB
	itemCols     map[string]string // 小写列名 -> 实际列名
	itemMediaCols map[string]string
	hasItemMedia bool
}

func New(dbPath string) (*Provider, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("飞牛数据库不存在: %s: %w", dbPath, err)
	}
	// 只读打开：不干扰飞牛写入，WAL 模式下也能安全并发读
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("打开飞牛数据库失败: %w", err)
	}
	p := &Provider{db: db}
	p.itemCols, err = tableColumns(db, "item")
	if err != nil {
		return nil, fmt.Errorf("读取 item 表结构失败: %w", err)
	}
	p.itemMediaCols, err = tableColumns(db, "item_media")
	if err == nil {
		p.hasItemMedia = true
	}
	return p, nil
}

func (p *Provider) Name() string { return "fnos" }

func (p *Provider) Close() error { return p.db.Close() }

// tableColumns 返回表的所有列：map[小写列名]实际列名
func tableColumns(db *sql.DB, table string) (map[string]string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]string{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[strings.ToLower(name)] = name
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("表 %s 不存在或无列", table)
	}
	return cols, rows.Err()
}

// pick 在候选名中找第一个实际存在的列
func pick(cols map[string]string, candidates ...string) string {
	for _, c := range candidates {
		if actual, ok := cols[strings.ToLower(c)]; ok {
			return actual
		}
	}
	return ""
}

// Inspect 打印库结构，用于 doctor 诊断（贴回给开发者确认字段映射）
func (p *Provider) Inspect() string {
	var sb strings.Builder
	sb.WriteString("== item 表列 ==\n")
	for lower, actual := range p.itemCols {
		if lower != strings.ToLower(actual) {
			fmt.Fprintf(&sb, "  %s\n", actual)
		} else {
			fmt.Fprintf(&sb, "  %s\n", actual)
		}
	}
	if p.hasItemMedia {
		sb.WriteString("== item_media 表列 ==\n")
		for _, actual := range p.itemMediaCols {
			fmt.Fprintf(&sb, "  %s\n", actual)
		}
		// 采样一行看看 path 长什么样
		var sample sql.NullString
		_ = p.db.QueryRow(`SELECT path FROM item_media LIMIT 1`).Scan(&sample)
		fmt.Fprintf(&sb, "== item_media.path 采样 ==\n  %s\n", sample.String)
	} else {
		sb.WriteString("== 无 item_media 表 ==\n")
	}
	return sb.String()
}

func (p *Provider) ListMedia(ctx context.Context) ([]metadata.MediaInfo, error) {
	cID := pick(p.itemCols, "guid", "id")
	cTitle := pick(p.itemCols, "title", "name")
	cOrig := pick(p.itemCols, "original_title", "originaltitle")
	cYear := pick(p.itemCols, "year", "release_year")
	cSeason := pick(p.itemCols, "season", "season_number")
	cEpisode := pick(p.itemCols, "episode", "episode_number")
	cImdb := pick(p.itemCols, "imdb_id", "imdbid")
	cTmdb := pick(p.itemCols, "tmdb_id", "tmdbid")
	cType := pick(p.itemCols, "media_type", "type", "kind")
	cPath := pick(p.itemCols, "path")

	if cTitle == "" {
		return nil, fmt.Errorf("item 表缺少标题列，实际列: %v（请运行 doctor 贴回结构）", keys(p.itemCols))
	}

	// 动态拼 SELECT，只取存在的列
	want := map[string]string{
		"id": cID, "title": cTitle, "orig": cOrig, "year": cYear,
		"season": cSeason, "episode": cEpisode, "imdb": cImdb,
		"tmdb": cTmdb, "type": cType, "path": cPath,
	}
	var sel []string
	var order []string // 与 sel 对应的 key
	for k, col := range want {
		if col != "" {
			sel = append(sel, fmt.Sprintf("%q", col))
			order = append(order, k)
		}
	}
	rows, err := p.db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM item", strings.Join(sel, ",")))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 文件级路径：guid -> []path
	filePaths := map[string][]string{}
	if p.hasItemMedia {
		fp := pick(p.itemMediaCols, "path")
		fg := pick(p.itemMediaCols, "item_guid", "guid")
		if fp != "" && fg != "" {
			mrows, err := p.db.QueryContext(ctx, fmt.Sprintf("SELECT %q, %q FROM item_media", fg, fp))
			if err == nil {
				defer mrows.Close()
				for mrows.Next() {
					var g, pa sql.NullString
					if err := mrows.Scan(&g, &pa); err == nil && g.Valid && pa.Valid {
						filePaths[g.String] = append(filePaths[g.String], pa.String)
					}
				}
			}
		}
	}

	var out []metadata.MediaInfo
	for rows.Next() {
		vals := make([]any, len(order))
		ptrs := make([]any, len(order))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		get := func(k string) string {
			for i, o := range order {
				if o == k {
					return asString(vals[i])
				}
			}
			return ""
		}
		mi := metadata.MediaInfo{
			ID:            get("id"),
			Title:         get("title"),
			OriginalTitle: get("orig"),
			Year:          asInt(get("year")),
			Season:        asInt(get("season")),
			Episode:       asInt(get("episode")),
			ImdbID:        normImdb(get("imdb")),
			TmdbID:        get("tmdb"),
			Source:        "fnos",
			Type:          metadata.Movie,
		}
		t := strings.ToLower(get("type"))
		if mi.Season > 0 || mi.Episode > 0 || strings.Contains(t, "episode") || strings.Contains(t, "tv") {
			mi.Type = metadata.Episode
		}
		// 文件路径：优先 item_media，其次 item.path
		paths := filePaths[mi.ID]
		if len(paths) == 0 && get("path") != "" {
			paths = []string{get("path")}
		}
		for _, pa := range paths {
			cp := mi
			cp.FilePath = pa
			// 只要 .strm（或同名视频）存在就收录；路径可能是 strm 也可能是实际视频
			if cp.FilePath != "" {
				out = append(out, cp)
			}
		}
		if len(paths) == 0 {
			// 没有文件行也保留一条（标题级），pipeline 会按标题在媒体目录反查 strm
			out = append(out, mi)
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	log.Printf("[fnos] 从飞牛库读到 %d 条媒体", len(out))
	return out, nil
}

// StrmFor 尝试把一条媒体记录对应到实际的 .strm 文件：
// 1) FilePath 本身就是 .strm 且存在；2) 同目录同名 .strm；3) 在媒体目录里按标题反查
func StrmFor(m metadata.MediaInfo, mediaDirs []string) string {
	if m.FilePath != "" {
		if strings.EqualFold(filepath.Ext(m.FilePath), ".strm") {
			if _, err := os.Stat(m.FilePath); err == nil {
				return m.FilePath
			}
		}
		base := strings.TrimSuffix(m.FilePath, filepath.Ext(m.FilePath)) + ".strm"
		if _, err := os.Stat(base); err == nil {
			return base
		}
	}
	return ""
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func asInt(s string) int {
	s = strings.TrimSpace(s)
	if len(s) >= 4 {
		if n, err := strconv.Atoi(s[:4]); err == nil && len(s) > 4 && strings.Contains(s, "-") {
			return n // "2021-03-05" 取年
		}
	}
	n, _ := strconv.Atoi(s)
	return n
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

func keys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for _, v := range m {
		ks = append(ks, v)
	}
	return ks
}
