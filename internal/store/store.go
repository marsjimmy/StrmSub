// Package store 用 SQLite 持久化全部运行时数据：设置 kv、标题正则、
// 媒体增量索引、搜索缓存、下载历史。
package store

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(dataDir string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dataDir, "strmsub.db")+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS kv(
		key TEXT PRIMARY KEY, value TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS title_rules(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL, pattern TEXT NOT NULL,
		ord INT NOT NULL DEFAULT 0, enabled INT NOT NULL DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS media_index(
		id TEXT PRIMARY KEY,
		file_path TEXT UNIQUE NOT NULL,
		file_size INT NOT NULL, mod_time INT NOT NULL,
		title TEXT NOT NULL, year INT DEFAULT 0,
		season INT DEFAULT 0, episode INT DEFAULT 0,
		rule_id INT DEFAULT 0, rule_sig TEXT DEFAULT '',
		cover_path TEXT DEFAULT '',
		sub_status TEXT DEFAULT '', sub_path TEXT DEFAULT '', sub_source TEXT DEFAULT '',
		updated_at TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_media_title ON media_index(title);
	CREATE TABLE IF NOT EXISTS search_cache(
		key TEXT PRIMARY KEY,
		result_json TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL
	);
	CREATE TABLE IF NOT EXISTS download_history(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL, filename TEXT NOT NULL, save_path TEXT NOT NULL,
		media_title TEXT DEFAULT '', status TEXT NOT NULL, detail TEXT DEFAULT '',
		created_at TIMESTAMP NOT NULL
	);`)
	if err != nil {
		return err
	}
	// v1.x 老表不再使用，直接清理
	for _, t := range []string{"media", "sub_status"} {
		_, _ = s.db.Exec("DROP TABLE IF EXISTS " + t)
	}
	return nil
}

// ---------------- kv ----------------

func (s *Store) GetKV(key, def string) string {
	var v string
	if err := s.db.QueryRow(`SELECT value FROM kv WHERE key=?`, key).Scan(&v); err != nil {
		return def
	}
	return v
}

func (s *Store) SetKV(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO kv(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// ---------------- 标题正则 ----------------

type TitleRule struct {
	ID      int64
	Name    string
	Pattern string
	Ord     int
	Enabled bool
}

func (s *Store) ListTitleRules() ([]TitleRule, error) {
	rows, err := s.db.Query(`SELECT id,name,pattern,ord,enabled FROM title_rules ORDER BY ord,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TitleRule
	for rows.Next() {
		var r TitleRule
		var en int
		if err := rows.Scan(&r.ID, &r.Name, &r.Pattern, &r.Ord, &en); err != nil {
			continue
		}
		r.Enabled = en != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) AddTitleRule(name, pattern string, ord int) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO title_rules(name,pattern,ord,enabled) VALUES(?,?,?,1)`, name, pattern, ord)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateTitleRule(id int64, name, pattern string, ord int, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	_, err := s.db.Exec(`UPDATE title_rules SET name=?,pattern=?,ord=?,enabled=? WHERE id=?`,
		name, pattern, ord, en, id)
	return err
}

func (s *Store) DeleteTitleRule(id int64) error {
	_, err := s.db.Exec(`DELETE FROM title_rules WHERE id=?`, id)
	return err
}

// RulesSignature 已启用规则的签名；扫描时比对，变了就重新识别标题
func (s *Store) RulesSignature() string {
	rules, err := s.ListTitleRules()
	if err != nil {
		return ""
	}
	var parts []string
	for _, r := range rules {
		if r.Enabled {
			parts = append(parts, fmt.Sprintf("%d:%s", r.Ord, r.Pattern))
		}
	}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(fmt.Sprintf("%v", parts)))
	return hex.EncodeToString(sum[:])[:16]
}

// ---------------- 媒体增量索引 ----------------

type MediaEntry struct {
	ID        string
	FilePath  string
	Size      int64
	ModTime   int64
	Title     string
	Year      int
	Season    int
	Episode   int
	RuleID    int64
	RuleSig   string
	CoverPath string
	SubStatus string
	SubPath   string
	SubSource string
	UpdatedAt time.Time
}

func MediaID(path string) string {
	sum := sha1.Sum([]byte(path))
	return hex.EncodeToString(sum[:])[:16]
}

func (s *Store) UpsertMediaEntry(e *MediaEntry) error {
	e.ID = MediaID(e.FilePath)
	_, err := s.db.Exec(`INSERT INTO media_index(id,file_path,file_size,mod_time,title,year,season,episode,
		rule_id,rule_sig,cover_path,sub_status,sub_path,sub_source,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET file_path=excluded.file_path,file_size=excluded.file_size,
		mod_time=excluded.mod_time,title=excluded.title,year=excluded.year,
		season=excluded.season,episode=excluded.episode,rule_id=excluded.rule_id,
		rule_sig=excluded.rule_sig,cover_path=excluded.cover_path,updated_at=excluded.updated_at`,
		e.ID, e.FilePath, e.Size, e.ModTime, e.Title, e.Year, e.Season, e.Episode,
		e.RuleID, e.RuleSig, e.CoverPath, e.SubStatus, e.SubPath, e.SubSource, time.Now())
	return err
}

// SetMediaSub 更新某媒体的字幕状态（保留识别信息）
func (s *Store) SetMediaSub(id, status, subPath, subSource string) error {
	_, err := s.db.Exec(`UPDATE media_index SET sub_status=?,sub_path=?,sub_source=?,updated_at=?
		WHERE id=?`, status, subPath, subSource, time.Now(), id)
	return err
}

func (s *Store) GetMediaEntry(id string) (*MediaEntry, error) {
	list, err := s.queryMedia(`SELECT id,file_path,file_size,mod_time,title,year,season,episode,
		rule_id,rule_sig,cover_path,sub_status,sub_path,sub_source,updated_at
		FROM media_index WHERE id=?`, id)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

// MapMediaIndex 全量索引映射（增量扫描用）：path -> entry
func (s *Store) MapMediaIndex() (map[string]MediaEntry, error) {
	list, err := s.queryMedia(`SELECT id,file_path,file_size,mod_time,title,year,season,episode,
		rule_id,rule_sig,cover_path,sub_status,sub_path,sub_source,updated_at FROM media_index`)
	if err != nil {
		return nil, err
	}
	m := make(map[string]MediaEntry, len(list))
	for _, e := range list {
		m[e.FilePath] = e
	}
	return m, nil
}

// ListMedia 媒体库列表（支持标题模糊搜索）
func (s *Store) ListMedia(q string, limit int) ([]MediaEntry, error) {
	if q = trimQ(q); q != "" {
		return s.queryMedia(`SELECT id,file_path,file_size,mod_time,title,year,season,episode,
			rule_id,rule_sig,cover_path,sub_status,sub_path,sub_source,updated_at
			FROM media_index WHERE title LIKE ? ORDER BY title LIMIT ?`, "%"+q+"%", limit)
	}
	return s.queryMedia(`SELECT id,file_path,file_size,mod_time,title,year,season,episode,
		rule_id,rule_sig,cover_path,sub_status,sub_path,sub_source,updated_at
		FROM media_index ORDER BY title LIMIT ?`, limit)
}

func (s *Store) DeleteMediaEntry(id string) error {
	_, err := s.db.Exec(`DELETE FROM media_index WHERE id=?`, id)
	return err
}

// MediaStats 统计
func (s *Store) MediaStats() (total, withSub, missing int) {
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM media_index`).Scan(&total)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM media_index WHERE sub_status='ok'`).Scan(&withSub)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM media_index WHERE sub_status='' OR sub_status='missing'`).Scan(&missing)
	return
}

func (s *Store) queryMedia(q string, args ...any) ([]MediaEntry, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MediaEntry
	for rows.Next() {
		var e MediaEntry
		var updated sql.NullString
		if err := rows.Scan(&e.ID, &e.FilePath, &e.Size, &e.ModTime, &e.Title, &e.Year,
			&e.Season, &e.Episode, &e.RuleID, &e.RuleSig, &e.CoverPath,
			&e.SubStatus, &e.SubPath, &e.SubSource, &updated); err != nil {
			continue
		}
		if updated.Valid {
			e.UpdatedAt = parseDBTime(updated.String)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func trimQ(q string) string {
	if len(q) > 60 {
		q = q[:60]
	}
	out := ""
	for _, r := range q {
		if r == '%' || r == '_' || r == '\\' {
			out += "\\"
		}
		out += string(r)
	}
	return out
}

// ---------------- 搜索缓存 ----------------

func (s *Store) GetSearchCache(key string, ttl time.Duration) ([]byte, bool) {
	var data string
	var created sql.NullString
	err := s.db.QueryRow(`SELECT result_json,created_at FROM search_cache WHERE key=?`, key).Scan(&data, &created)
	if err != nil {
		return nil, false
	}
	if created.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", created.String); err == nil {
			if time.Since(t) > ttl {
				return nil, false
			}
		}
	}
	return []byte(data), true
}

func (s *Store) SetSearchCache(key string, data []byte) error {
	_, err := s.db.Exec(`INSERT INTO search_cache(key,result_json,created_at) VALUES(?,?,?)
		ON CONFLICT(key) DO UPDATE SET result_json=excluded.result_json,created_at=excluded.created_at`,
		key, string(data), time.Now())
	return err
}

func (s *Store) CleanSearchCache(ttl time.Duration) {
	cutoff := time.Now().Add(-ttl)
	_, _ = s.db.Exec(`DELETE FROM search_cache WHERE created_at < ?`, cutoff)
}

// ---------------- 下载历史 ----------------

type DownloadRecord struct {
	ID         int64     `json:"id"`
	Source     string    `json:"source"`
	Filename   string    `json:"filename"`
	SavePath   string    `json:"save_path"`
	MediaTitle string    `json:"media_title"`
	Status     string    `json:"status"` // ok / failed
	Detail     string    `json:"detail"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Store) AddDownloadRecord(r *DownloadRecord) error {
	_, err := s.db.Exec(`INSERT INTO download_history(source,filename,save_path,media_title,status,detail,created_at)
		VALUES(?,?,?,?,?,?,?)`, r.Source, r.Filename, r.SavePath, r.MediaTitle, r.Status, r.Detail,
		time.Now().Format("2006-01-02 15:04:05.999999999-07:00"))
	return err
}

// parseDBTime 解析库里的时间字符串，兼容新旧两种写入格式
func parseDBTime(s string) time.Time {
	if i := strings.Index(s, " m="); i > 0 {
		s = s[:i] // 去掉 time.Time.String() 的单调时钟后缀
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (s *Store) ListDownloadHistory(limit int) ([]DownloadRecord, error) {
	rows, err := s.db.Query(`SELECT id,source,filename,save_path,media_title,status,detail,created_at
		FROM download_history ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DownloadRecord
	for rows.Next() {
		var r DownloadRecord
		var created sql.NullString
		if err := rows.Scan(&r.ID, &r.Source, &r.Filename, &r.SavePath, &r.MediaTitle, &r.Status, &r.Detail, &created); err != nil {
			continue
		}
		if created.Valid {
			r.CreatedAt = parseDBTime(created.String)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
