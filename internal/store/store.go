// Package store 用 SQLite 记录媒体与字幕状态（自用库，与飞牛库无关）。
package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/marsjimmy/strmsub/internal/metadata"
)

type Store struct{ db *sql.DB }

func Open(dataDir string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dataDir, "strmsub.db")+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS media (
		id TEXT PRIMARY KEY, type TEXT, title TEXT, year INT,
		season INT, episode INT, file_path TEXT, imdb TEXT, tmdb TEXT,
		source TEXT, updated_at TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS sub_status (
		media_id TEXT PRIMARY KEY, lang TEXT, path TEXT, source TEXT,
		status TEXT, downloaded_at TIMESTAMP
	);`)
	if err != nil {
		return err
	}
	// v1.0.4 新增列：老库原地 ALTER，列已存在则忽略
	for _, col := range []string{
		"ALTER TABLE media ADD COLUMN overview TEXT",
		"ALTER TABLE media ADD COLUMN poster_path TEXT",
		"ALTER TABLE media ADD COLUMN series_id TEXT",
	} {
		if _, err := s.db.Exec(col); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}

// UpsertMedia 记录媒体
func (s *Store) UpsertMedia(m metadata.MediaInfo) error {
	_, err := s.db.Exec(`INSERT INTO media(id,type,title,year,season,episode,file_path,imdb,tmdb,source,overview,poster_path,series_id,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		type=excluded.type,title=excluded.title,year=excluded.year,season=excluded.season,
		episode=excluded.episode,file_path=excluded.file_path,imdb=excluded.imdb,
		tmdb=excluded.tmdb,source=excluded.source,overview=excluded.overview,
		poster_path=excluded.poster_path,series_id=excluded.series_id,updated_at=excluded.updated_at`,
		m.ID, string(m.Type), m.Title, m.Year, m.Season, m.Episode,
		m.FilePath, m.ImdbID, m.TmdbID, m.Source, m.Overview, m.PosterPath, m.SeriesID, time.Now())
	return err
}

// SetStatus 记录字幕状态：ok / missing / failed
func (s *Store) SetStatus(mediaID, lang, path, source, status string) error {
	_, err := s.db.Exec(`INSERT INTO sub_status(media_id,lang,path,source,status,downloaded_at)
		VALUES(?,?,?,?,?,?) ON CONFLICT(media_id) DO UPDATE SET
		lang=excluded.lang,path=excluded.path,source=excluded.source,
		status=excluded.status,downloaded_at=excluded.downloaded_at`,
		mediaID, lang, path, source, status, time.Now())
	return err
}

// MediaWithStatus 供 Web 界面展示
type MediaWithStatus struct {
	metadata.MediaInfo
	SubLang   string
	SubPath   string
	SubSource string
	SubStatus string
}

func (s *Store) ListMediaWithStatus() ([]MediaWithStatus, error) {
	return s.queryMedia(`SELECT m.id,m.type,m.title,m.year,m.season,m.episode,m.file_path,
		m.imdb,m.tmdb,m.source,m.overview,m.poster_path,m.series_id, s.lang,s.path,s.source,s.status
		FROM media m LEFT JOIN sub_status s ON s.media_id=m.id ORDER BY m.title`)
}

// ListByType 按类型列出（媒体库用）
func (s *Store) ListByType(typ metadata.MediaType) ([]MediaWithStatus, error) {
	return s.queryMedia(`SELECT m.id,m.type,m.title,m.year,m.season,m.episode,m.file_path,
		m.imdb,m.tmdb,m.source,m.overview,m.poster_path,m.series_id, s.lang,s.path,s.source,s.status
		FROM media m LEFT JOIN sub_status s ON s.media_id=m.id WHERE m.type=? ORDER BY m.title`, string(typ))
}

// ListSeasons 列出某部剧的季
func (s *Store) ListSeasons(seriesID string) ([]MediaWithStatus, error) {
	return s.queryMedia(`SELECT m.id,m.type,m.title,m.year,m.season,m.episode,m.file_path,
		m.imdb,m.tmdb,m.source,m.overview,m.poster_path,m.series_id, s.lang,s.path,s.source,s.status
		FROM media m LEFT JOIN sub_status s ON s.media_id=m.id
		WHERE m.type='season' AND m.series_id=? ORDER BY m.season`, seriesID)
}

// ListEpisodes 列出某部剧某季的集
func (s *Store) ListEpisodes(seriesID string, season int) ([]MediaWithStatus, error) {
	return s.queryMedia(`SELECT m.id,m.type,m.title,m.year,m.season,m.episode,m.file_path,
		m.imdb,m.tmdb,m.source,m.overview,m.poster_path,m.series_id, s.lang,s.path,s.source,s.status
		FROM media m LEFT JOIN sub_status s ON s.media_id=m.id
		WHERE m.type='episode' AND m.series_id=? AND m.season=? ORDER BY m.episode`, seriesID, season)
}

// GetMedia 按 ID 取一条
func (s *Store) GetMedia(id string) (*MediaWithStatus, error) {
	list, err := s.queryMedia(`SELECT m.id,m.type,m.title,m.year,m.season,m.episode,m.file_path,
		m.imdb,m.tmdb,m.source,m.overview,m.poster_path,m.series_id, s.lang,s.path,s.source,s.status
		FROM media m LEFT JOIN sub_status s ON s.media_id=m.id WHERE m.id=?`, id)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

// ListOrphanEpisodes 没有归属剧 ID 的散集（NFO 等来源），供媒体库按剧名合成
func (s *Store) ListOrphanEpisodes() ([]MediaWithStatus, error) {
	return s.queryMedia(`SELECT m.id,m.type,m.title,m.year,m.season,m.episode,m.file_path,
		m.imdb,m.tmdb,m.source,m.overview,m.poster_path,m.series_id, s.lang,s.path,s.source,s.status
		FROM media m LEFT JOIN sub_status s ON s.media_id=m.id
		WHERE m.type='episode' AND (m.series_id IS NULL OR m.series_id='')
		ORDER BY m.title,m.season,m.episode`)
}

// SeriesStats 某部剧的季数和集数
func (s *Store) SeriesStats(seriesID string) (seasons, episodes int) {
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM media WHERE type='season' AND series_id=?`, seriesID).Scan(&seasons)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM media WHERE type='episode' AND series_id=?`, seriesID).Scan(&episodes)
	return
}

func (s *Store) queryMedia(q string, args ...any) ([]MediaWithStatus, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MediaWithStatus
	for rows.Next() {
		var mw MediaWithStatus
		var typ string
		var lang, spath, ssource, sstatus sql.NullString
		var overview, posterPath, seriesID sql.NullString
		if err := rows.Scan(&mw.ID, &typ, &mw.Title, &mw.Year, &mw.Season, &mw.Episode,
			&mw.FilePath, &mw.ImdbID, &mw.TmdbID, &mw.Source,
			&overview, &posterPath, &seriesID,
			&lang, &spath, &ssource, &sstatus); err != nil {
			continue
		}
		mw.Type = metadata.MediaType(typ)
		mw.Overview, mw.PosterPath, mw.SeriesID = overview.String, posterPath.String, seriesID.String
		mw.SubLang, mw.SubPath, mw.SubSource, mw.SubStatus = lang.String, spath.String, ssource.String, sstatus.String
		out = append(out, mw)
	}
	return out, rows.Err()
}
