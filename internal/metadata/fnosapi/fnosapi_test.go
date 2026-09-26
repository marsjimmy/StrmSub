package fnosapi

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marsjimmy/strmsub/internal/metadata"
)

func md5s(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// mockFnos 模拟飞牛影视 API，并校验 Authx 签名
func mockFnos(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		write := func(code int, data any) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": "ok", "data": data})
		}

		if r.URL.Path == "/v/api/v1/login" {
			if r.Header.Get("Authx") != "" {
				t.Errorf("登录接口不应带 Authx")
			}
			var b map[string]string
			json.Unmarshal(body, &b)
			if b["userName"] != "admin" || b["password"] != "pw" {
				write(1, nil)
				return
			}
			write(0, map[string]string{"token": "tok123"})
			return
		}

		// 其余接口：校验鉴权头 + Authx 签名
		if r.Header.Get("Authorization") != "tok123" || r.Header.Get("Trim-MC-token") != "tok123" {
			t.Errorf("缺少 token 头")
		}
		ah := r.Header.Get("Authx")
		if !strings.HasPrefix(ah, "nonce=") || !strings.Contains(ah, "&timestamp=") || !strings.Contains(ah, "&sign=") {
			t.Errorf("Authx 头格式错误: %q", ah)
			write(1, nil)
			return
		}
		rest := strings.TrimPrefix(ah, "nonce=")
		nonce := rest[:strings.Index(rest, "&timestamp=")]
		rest = rest[strings.Index(rest, "&timestamp=")+len("&timestamp="):]
		ts := rest[:strings.Index(rest, "&sign=")]
		sign := rest[strings.Index(rest, "&sign=")+len("&sign="):]
		var payloadMD5 string
		if r.Method == "GET" {
			payloadMD5 = md5s("")
		} else {
			payloadMD5 = md5s(string(body))
		}
		expect := md5s(strings.Join([]string{authxKey, r.URL.Path, nonce, ts, payloadMD5, authxSecret}, "_"))
		if sign != expect {
			t.Errorf("Authx 签名错误 path=%s got=%s want=%s", r.URL.Path, sign, expect)
			write(1, nil)
			return
		}

		switch {
		case r.URL.Path == "/v/api/v1/item/list":
			write(0, map[string]any{"total": 2, "list": []any{
				map[string]any{"guid": "m1", "title": "沙丘", "category": "Movie",
					"original_title": "Dune", "year": 2021, "imdb_id": "tt1160419",
					"overview": "沙漠星球史诗", "poster": "/v/api/v1/poster/m1"},
				map[string]any{"guid": "t1", "title": "权力的游戏", "category": "TV",
					"year": 2011, "imdb_id": "tt0944947", "overview": "铁王座之争",
					"poster": "/v/api/v1/poster/t1"},
			}})
		case r.URL.Path == "/v/api/v1/season/list/t1":
			write(0, []any{map[string]any{"guid": "s1", "title": "第一季",
				"season": 1, "overview": "第一季简介", "poster": "/v/api/v1/poster/s1"}})
		case r.URL.Path == "/v/api/v1/episode/list/s1":
			write(0, []any{map[string]any{"guid": "e1", "title": "第一集",
				"meta": map[string]any{"season": 1, "episode": 1},
				"overview": "首集简介"}})
		case r.URL.Path == "/v/api/v1/item/m1":
			write(0, map[string]any{"guid": "m1", "file_path": "/vol1/media/电影/沙丘 (2021)/沙丘.strm"})
		case r.URL.Path == "/v/api/v1/item/e1":
			write(0, map[string]any{"guid": "e1", "file_path": "/vol1/media/剧集/权力的游戏/S01E01.strm"})
		default:
			t.Errorf("未预期的路径 %s", r.URL.Path)
			write(1, nil)
		}
	}))
}

func TestListMedia(t *testing.T) {
	srv := mockFnos(t)
	defer srv.Close()

	p := New(srv.URL, "admin", "pw", func(s string) string {
		return strings.Replace(s, "/vol1/media", "/media", 1)
	})
	got, err := p.ListMedia(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("期望 4 条（电影+剧+季+集），实际 %d: %+v", len(got), got)
	}
	byID := map[string]metadata.MediaInfo{}
	for _, m := range got {
		byID[m.ID] = m
	}
	movie := byID["m1"]
	if movie.Type != metadata.Movie || movie.Title != "沙丘" || movie.Year != 2021 ||
		movie.ImdbID != "tt1160419" || movie.OriginalTitle != "Dune" {
		t.Errorf("电影解析错误: %+v", movie)
	}
	if movie.Overview != "沙漠星球史诗" || movie.PosterURL != "/v/api/v1/poster/m1" {
		t.Errorf("电影简介/海报解析错误: %+v", movie)
	}
	if movie.FilePath != "/media/电影/沙丘 (2021)/沙丘.strm" {
		t.Errorf("路径映射错误: %q", movie.FilePath)
	}
	series := byID["t1"]
	if series.Type != metadata.Series || series.Title != "权力的游戏" || series.Year != 2011 {
		t.Errorf("剧条目解析错误: %+v", series)
	}
	if series.Overview != "铁王座之争" || series.PosterURL != "/v/api/v1/poster/t1" {
		t.Errorf("剧简介/海报解析错误: %+v", series)
	}
	if series.FilePath != "" {
		t.Errorf("剧条目不应有文件路径: %+v", series)
	}
	season := byID["s1"]
	if season.Type != metadata.Season || season.Season != 1 || season.SeriesID != "t1" {
		t.Errorf("季条目解析错误: %+v", season)
	}
	if season.Overview != "第一季简介" || season.PosterURL != "/v/api/v1/poster/s1" {
		t.Errorf("季简介/海报解析错误: %+v", season)
	}
	ep := byID["e1"]
	if ep.Type != metadata.Episode || ep.Title != "权力的游戏" || ep.Season != 1 || ep.Episode != 1 || ep.Year != 2011 {
		t.Errorf("剧集解析错误: %+v", ep)
	}
	if ep.SeriesID != "t1" {
		t.Errorf("剧集 SeriesID 错误: %+v", ep)
	}
	if ep.Overview != "首集简介" {
		t.Errorf("剧集简介解析错误: %+v", ep)
	}
	if ep.FilePath != "/media/剧集/权力的游戏/S01E01.strm" {
		t.Errorf("剧集路径映射错误: %q", ep.FilePath)
	}
	if movie.Source != "fnosapi" || ep.Source != "fnosapi" {
		t.Errorf("Source 错误")
	}
}

func TestProbe(t *testing.T) {
	srv := mockFnos(t)
	defer srv.Close()
	p := New(srv.URL, "admin", "pw", nil)
	out, err := p.Probe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "登录成功") || !strings.Contains(out, "item/list 成功") {
		t.Errorf("Probe 输出异常:\n%s", out)
	}
}
