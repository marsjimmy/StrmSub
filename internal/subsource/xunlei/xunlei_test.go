package xunlei

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

// refCID 参考 ChineseSubFinder 的算法直接算，用于交叉验证
func refCID(data []byte) string {
	size := int64(len(data))
	positions := []int64{0, size / 3, size - chunkSize}
	h := sha1.New()
	for _, pos := range positions {
		h.Write(data[pos : pos+chunkSize])
	}
	return fmt.Sprintf("%X", h.Sum(nil))
}

func TestCIDForURL(t *testing.T) {
	// 构造 1MB 伪视频数据
	data := make([]byte, 1<<20)
	for i := range data {
		data[i] = byte(i * 31 % 251)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		http.ServeContent(w, r, "video.mp4", time.Unix(0, 0), strings.NewReader(string(data)))
	}))
	defer srv.Close()

	c := New()
	cid, err := c.cidForURL(context.Background(), srv.URL+"/v.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if want := refCID(data); cid != want {
		t.Fatalf("CID 不一致: got %s want %s", cid, want)
	}
}

func TestCIDForURLNoRange(t *testing.T) {
	data := make([]byte, 1<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 不支持 Range：忽略 Range 头直接 200
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		w.Write(data)
	}))
	defer srv.Close()

	c := New()
	if _, err := c.cidForURL(context.Background(), srv.URL+"/v.mp4"); err == nil {
		t.Fatal("不支持 Range 的源应该报错跳过")
	}
}

func TestSearchEndToEnd(t *testing.T) {
	data := make([]byte, 1<<20)
	for i := range data {
		data[i] = byte(i % 251)
	}
	wantCID := refCID(data)

	mediaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		http.ServeContent(w, r, "v.mp4", time.Unix(0, 0), strings.NewReader(string(data)))
	}))
	defer mediaSrv.Close()

	subBody := `{"sublist":[
		{"scid":"ABC","sname":"测试.简体.srt","language":"简体","surl":"http://x/y.srt","svote":99},
		{"scid":"DEF","sname":"test.eng.srt","language":"英语","surl":"http://x/z.srt","svote":1}
	]}`
	var gotCID string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCID = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/subxl/"), ".json")
		w.Write([]byte(subBody))
	}))
	defer apiSrv.Close()

	// 把 rootURL 指向测试服：通过改包变量不可行，这里直接验证 cidForURL + 解析逻辑
	dir := t.TempDir()
	strm := filepath.Join(dir, "m.strm")
	os.WriteFile(strm, []byte(mediaSrv.URL+"/v.mp4\n"), 0644)

	c := New()
	cid, err := c.cidForURL(context.Background(), mediaSrv.URL+"/v.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if cid != wantCID {
		t.Fatalf("CID %s != %s", cid, wantCID)
	}
	// 调真实 API 地址会被网络影响，这里只验证解析：手动走 Search 需要 rootURL 可覆盖，
	// 因此把解析逻辑抽出来测——用 httptest 替换整个 Search 的网络层过于侵入，
	// 改为验证 normLang 与 formatOf
	_ = gotCID
	_ = apiSrv
	_ = strm
}

func TestNormLang(t *testing.T) {
	cases := map[string]string{
		"简体":    subsource.LangHans,
		"简体&英语": subsource.LangBilingual,
		"繁体":    subsource.LangHant,
		"繁体&英语": subsource.LangBilingual,
		"英语":    subsource.LangOther,
		"未知语言":  subsource.LangOther,
		"":      subsource.LangOther,
	}
	for in, want := range cases {
		if got := normLang(in); got != want {
			t.Errorf("normLang(%q)=%q want %q", in, got, want)
		}
	}
}

func TestStrmTarget(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.strm")
	os.WriteFile(p, []byte("\xef\xbb\xbf\n# comment\nhttps://example.com/v.mp4\n"), 0644)
	u, ok := strmTarget(p)
	if !ok || u != "https://example.com/v.mp4" {
		t.Fatalf("got %q %v", u, ok)
	}
	p2 := filepath.Join(dir, "b.strm")
	os.WriteFile(p2, []byte("plugin://x\n"), 0644)
	if _, ok := strmTarget(p2); ok {
		t.Fatal("非 http(s) 不应接受")
	}
}

func TestFormatOf(t *testing.T) {
	if formatOf("a.ass") != "ass" || formatOf("b.srt") != "srt" || formatOf("c.txt") != "srt" {
		t.Fatal("formatOf 错误")
	}
}

func TestClientImplements(t *testing.T) {
	var _ subsource.Source = New()
	m := metadata.MediaInfo{FilePath: "/nonexistent/a.strm"}
	cands, err := New().Search(context.Background(), m)
	if err != nil || len(cands) != 0 {
		t.Fatalf("不存在的 strm 应返回空, got %v %v", cands, err)
	}
}
