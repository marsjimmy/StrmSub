package title

import (
	"os"
	"path/filepath"
	"testing"
)

// 复现用户场景：/media/电视剧/金装律师 1-9季/S01/S01E05.HD1080P.中英双字.霸王龙压制组T-Rex.(mp4).strm
// 同名 NFO 只有单集 <title>Bail Out</title>，没有 <showtitle>
func TestReproSuits(t *testing.T) {
	dir := t.TempDir()
	seriesDir := filepath.Join(dir, "电视剧", "金装律师 1-9季", "S01")
	if err := os.MkdirAll(seriesDir, 0755); err != nil {
		t.Fatal(err)
	}
	base := "S01E05.HD1080P.中英双字.霸王龙压制组T-Rex.(mp4)"
	video := filepath.Join(seriesDir, base+".strm")
	os.WriteFile(video, []byte("http://x"), 0644)
	nfoContent := `<?xml version="1.0" encoding="utf-8"?><episodedetails><title>Bail Out</title><season>1</season><episode>5</episode><aired>2011-07-21</aired></episodedetails>`
	os.WriteFile(filepath.Join(seriesDir, base+".nfo"), []byte(nfoContent), 0644)

	r := New(nil)
	res := r.Recognize(video)
	t.Logf("有NFO(无showtitle): title=%q S%dE%d rule=%s", res.Title, res.Season, res.Episode, res.RuleName)
	if res.Title == "Bail Out" {
		t.Errorf("BUG复现: 仍识别成单集标题 Bail Out")
	}
	if res.Title != "金装律师" || res.Season != 1 || res.Episode != 5 {
		t.Errorf("期望 金装律师 S01E05, 得到 %q S%dE%d", res.Title, res.Season, res.Episode)
	}

	// 场景2: 无 NFO，纯目录兜底
	os.Remove(filepath.Join(seriesDir, base+".nfo"))
	res2 := r.Recognize(video)
	t.Logf("无NFO: title=%q S%dE%d rule=%s", res2.Title, res2.Season, res2.Episode, res2.RuleName)
	if res2.Title != "金装律师" || res2.Season != 1 || res2.Episode != 5 {
		t.Errorf("期望 金装律师 S01E05, 得到 %q S%dE%d", res2.Title, res2.Season, res2.Episode)
	}

	// 场景3: tvshow.nfo 在剧目录
	tvshow := `<?xml version="1.0" encoding="utf-8"?><tvshow><title>金装律师</title><originaltitle>Suits</originaltitle><year>2011</year></tvshow>`
	os.WriteFile(filepath.Join(dir, "电视剧", "金装律师 1-9季", "tvshow.nfo"), []byte(tvshow), 0644)
	res3 := r.Recognize(video)
	t.Logf("tvshow.nfo: title=%q year=%d rule=%s", res3.Title, res3.Year, res3.RuleName)
	if res3.Title != "金装律师" || res3.Year != 2011 {
		t.Errorf("期望 金装律师/2011, 得到 %q/%d", res3.Title, res3.Year)
	}
}

// 目录名清洗单测
func TestDirSeriesTitle(t *testing.T) {
	cases := map[string]string{
		"/media/电视剧/金装律师 1-9季/S01/x.strm":         "金装律师",
		"/media/电视剧/Breaking Bad/Season 1/x.strm": "Breaking Bad",
		"/media/电视剧/权力的游戏/第2季/x.strm":             "权力的游戏",
		"/media/电影/星际穿越/x.strm":                   "星际穿越",
		"/media/电视剧/24/第1季/x.strm":                "24",
	}
	for path, want := range cases {
		if got := dirSeriesTitle(path); got != want {
			t.Errorf("%s: 期望 %q, 得到 %q", path, want, got)
		}
	}
}
