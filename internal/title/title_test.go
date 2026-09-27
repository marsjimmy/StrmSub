package title

import "testing"

func TestRecognizeDefaults(t *testing.T) {
	cases := []struct {
		file                  string
		title                 string
		year, season, episode int
	}{
		{"Ne.Zha.2.2025.1080p.WEB-DL.mkv", "Ne Zha 2", 2025, 0, 0},
		{"哪吒之魔童闹海 (2025).strm", "哪吒之魔童闹海", 2025, 0, 0},
		{"Breaking.Bad.S01E02.720p.mkv", "Breaking Bad", 0, 1, 2},
		{"绝命毒师 S02E05.mkv", "绝命毒师", 0, 2, 5},
		{"Show.EP03.mp4", "Show", 0, 0, 3},
		{"某某剧第12集.mp4", "某某剧", 0, 0, 12},
		{"ABC-123.mp4", "ABC-123", 0, 0, 0},
		{"SONE028.mp4", "SONE028", 0, 0, 0},
		{"random video file.mkv", "random video file", 0, 0, 0},
	}
	r := New(nil)
	for _, c := range cases {
		res := r.Recognize("/media/" + c.file)
		if res.Title != c.title || res.Year != c.year || res.Season != c.season || res.Episode != c.episode {
			t.Errorf("%s => %+v, want title=%q year=%d s=%d e=%d",
				c.file, res, c.title, c.year, c.season, c.episode)
		}
	}
}

func TestCustomRule(t *testing.T) {
	r := New([]Rule{{ID: 7, Name: "test", Pattern: `^(?P<title>.+?)\.(?P<year>\d{4})\..+$`}})
	res := r.Recognize("/x/Foo.Bar.2021.1080p.mkv")
	if res.Title != "Foo Bar" || res.Year != 2021 || res.RuleID != 7 {
		t.Fatalf("got %+v", res)
	}
}

func TestPatternTest(t *testing.T) {
	res, err := Test(`^(?P<title>.+?)\.(?P<year>\d{4})`, "Foo.Bar.2021.mkv")
	if err != nil || res.Title != "Foo Bar" || res.Year != 2021 {
		t.Fatalf("got %+v err=%v", res, err)
	}
	if _, err := Test(`[unclosed`, "x.mkv"); err == nil {
		t.Fatal("非法正则应报错")
	}
	if _, err := Test(`^nomatch\d+$`, "abc.mkv"); err == nil {
		t.Fatal("未匹配应报错")
	}
}
