package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/store"
)

func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	srv := &Server{store: st, dataDir: dir}
	// 一部电影
	_ = st.UpsertMedia(metadata.MediaInfo{ID: "m1", Type: metadata.Movie, Title: "沙丘", Year: 2021,
		Overview: "沙漠史诗", PosterPath: "posters/m1.jpg", Source: "fnosapi"})
	// 一部剧：剧 + 1 季 + 2 集
	_ = st.UpsertMedia(metadata.MediaInfo{ID: "t1", Type: metadata.Series, Title: "权力的游戏", Year: 2011,
		Overview: "铁王座", PosterPath: "posters/t1.jpg", Source: "fnosapi"})
	_ = st.UpsertMedia(metadata.MediaInfo{ID: "s1", Type: metadata.Season, Title: "权力的游戏",
		Year: 2011, Season: 1, SeriesID: "t1", Overview: "第一季", Source: "fnosapi"})
	_ = st.UpsertMedia(metadata.MediaInfo{ID: "e1", Type: metadata.Episode, Title: "权力的游戏",
		Year: 2011, Season: 1, Episode: 1, SeriesID: "t1", FilePath: "/media/s1e1.strm", Source: "fnosapi"})
	_ = st.UpsertMedia(metadata.MediaInfo{ID: "e2", Type: metadata.Episode, Title: "权力的游戏",
		Year: 2011, Season: 1, Episode: 2, SeriesID: "t1", FilePath: "/media/s1e2.strm", Source: "fnosapi"})
	_ = st.SetStatus("e1", "zh-Hans", "", "assrt", "ok")
	// 放一张假海报
	_ = os.MkdirAll(filepath.Join(dir, "posters"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "posters", "m1.jpg"), []byte{0xFF, 0xD8, 0xFF, 0x00}, 0o644)
	return srv, dir
}

func getJSON(t *testing.T, srv *Server, path string, v any) int {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, r)
	if v != nil && w.Code == 200 {
		if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
			t.Fatalf("GET %s 解析失败: %v", path, err)
		}
	}
	return w.Code
}

func TestLibraryEndpoints(t *testing.T) {
	srv, _ := testServer(t)

	var movies []libraryItem
	if code := getJSON(t, srv, "/api/library/movies", &movies); code != 200 || len(movies) != 1 {
		t.Fatalf("movies: code=%d len=%d", code, len(movies))
	}
	if movies[0].Title != "沙丘" || movies[0].Poster != "/api/poster?id=m1" || movies[0].Overview != "沙漠史诗" {
		t.Errorf("movie 字段错误: %+v", movies[0])
	}

	var series []libraryItem
	if code := getJSON(t, srv, "/api/library/series", &series); code != 200 || len(series) != 1 {
		t.Fatalf("series: code=%d len=%d", code, len(series))
	}
	if series[0].Seasons != 1 || series[0].Episodes != 2 {
		t.Errorf("series 统计错误: %+v", series[0])
	}

	var seasons []seasonItem
	if code := getJSON(t, srv, "/api/library/seasons?series=t1", &seasons); code != 200 || len(seasons) != 1 {
		t.Fatalf("seasons: code=%d len=%d", code, len(seasons))
	}
	if seasons[0].Season != 1 || seasons[0].Overview != "第一季" {
		t.Errorf("season 字段错误: %+v", seasons[0])
	}

	var eps []episodeItem
	if code := getJSON(t, srv, "/api/library/episodes?series=t1&season=1", &eps); code != 200 || len(eps) != 2 {
		t.Fatalf("episodes: code=%d len=%d", code, len(eps))
	}
	if eps[0].Episode != 1 || eps[0].SubStatus != "ok" || !eps[0].HasFile {
		t.Errorf("episode 字段错误: %+v", eps[0])
	}

	// 海报：电影直取
	r := httptest.NewRequest("GET", "/api/poster?id=m1", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Errorf("poster m1 期望 200，实际 %d", w.Code)
	}
	// 海报：单集回退到剧海报（t1.jpg 不存在 → 404）
	r = httptest.NewRequest("GET", "/api/poster?id=e1", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf("poster e1 回退文件不存在期望 404，实际 %d", w.Code)
	}
	// 海报：不存在的 ID
	r = httptest.NewRequest("GET", "/api/poster?id=xxx", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf("poster xxx 期望 404，实际 %d", w.Code)
	}
	// 路径穿越防护
	r = httptest.NewRequest("GET", "/api/poster?id=..%2Fsecret", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf("poster 穿越期望 404，实际 %d", w.Code)
	}
}
