// Package store 用 SQLite 记录媒体与字幕状态（自用库，与飞牛库无关）。
package store

import (
	"database/sql"
	"path/filepath"
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
	return err
}

// UpsertMedia 记录媒体
func (s *Store) UpsertMedia(m metadata.MediaInfo) error {
	_, err := s.db.Exec(`INSERT INTO media(id,type,title,year,season,episode,file_path,imdb,tmdb,source,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		type=excluded.type,title=excluded.title,year=excluded.year,season=excluded.season,
		episode=excluded.episode,file_path=excluded.file_path,imdb=excluded.imdb,
		tmdb=excluded.tmdb,source=excluded.source,updated_at=excluded.updated_at`,
		m.ID, string(m.Type), m.Title, m.Year, m.Season, m.Episode,
		m.FilePath, m.ImdbID, m.TmdbID, m.Source, time.Now())
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
	rows, err := s.db.Query(`SELECT m.id,m.type,m.title,m.year,m.season,m.episode,m.file_path,
		m.imdb,m.tmdb,m.source, s.lang,s.path,s.source,s.status
		FROM media m LEFT JOIN sub_status s ON s.media_id=m.id ORDER BY m.title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MediaWithStatus
	for rows.Next() {
		var mw MediaWithStatus
		var typ string
		var lang, spath, ssource, sstatus sql.NullString
		if err := rows.Scan(&mw.ID, &typ, &mw.Title, &mw.Year, &mw.Season, &mw.Episode,
			&mw.FilePath, &mw.ImdbID, &mw.TmdbID, &mw.Source, &lang, &spath, &ssource, &sstatus); err != nil {
			continue
		}
		mw.Type = metadata.MediaType(typ)
		mw.SubLang, mw.SubPath, mw.SubSource, mw.SubStatus = lang.String, spath.String, ssource.String, sstatus.String
		out = append(out, mw)
	}
	return out, rows.Err()
}
