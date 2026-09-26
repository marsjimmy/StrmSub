package subhd

import (
	"context"
	"strings"
	"testing"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

const sampleHTML = `
<div class="bg-white shadow-sm rounded-3 mb-4">
  <div class="row">
    <div class="col-2 d-none d-lg-block">
        <a href='/d/34780991'>
          <div class="pics"><img class="rounded-start" src="https://img.subhd.me/poster/x.webp" alt="test "></div>
        </a>
    </div>
    <div class="col-lg-10">
      <div class="pt-3 pe-3 pb-2 ps-3 ps-lg-0 position-relative">
        <div class="clearfix">
          <div class="float-start f16 fw-bold">
              <a class="link-dark align-middle" href='/a/8gG7bR' target="_blank" rel="noopener noreferrer">哪吒之魔童闹海</a>
          </div>
          <div class="view-text text-secondary">
            <a href='/a/8gG7bR' class='link-dark' target="_blank" rel="noopener noreferrer">
              港繁 | Ne Zha 2 (2025)
            </a>
          </div>
          <div class="text-truncate py-2 f11">
            <span class="rounded p-1 me-1 text-white" style="background-color:#00a3ff!important">官方字幕</span>
            <span class="p-1 fw-bold">繁体</span>
            <span class="p-1 text-secondary">SRT</span>
          </div>
          <div class="pt-2 text-secondary f12">
            <span class='align-text-top me-3'>45k</span>
            <span class="align-text-top me-3">167</span>
            <span>04-15 10:32</span>
            发布人 |Coritz|
          </div>
        </div>
      </div>
    </div>
  </div>
</div>
<div class="bg-white shadow-sm rounded-3 mb-4">
  <div class="row">
          <div class="float-start f16 fw-bold">
              <a class="link-dark align-middle" href="/a/EngOnly1" target="_blank">Ne Zha 2 English</a>
          </div>
          <div class="view-text text-secondary"><a href="/a/EngOnly1">英文 | Ne Zha 2 (2025)</a></div>
          <div class="text-truncate py-2 f11">
            <span class="p-1 fw-bold">英文</span>
            <span class="p-1 text-secondary">SRT</span>
          </div>
  </div>
</div>`

func TestParseEntries(t *testing.T) {
	es := parseEntries(sampleHTML)
	if len(es) != 1 {
		t.Fatalf("应解析出 1 条中文条目（英文条目应被过滤），got %d", len(es))
	}
	e := es[0]
	if e.sid != "8gG7bR" {
		t.Errorf("sid=%q", e.sid)
	}
	if e.name != "哪吒之魔童闹海" {
		t.Errorf("name=%q", e.name)
	}
	if !strings.Contains(e.detail, "Ne Zha 2 (2025)") {
		t.Errorf("detail=%q", e.detail)
	}
	if e.lang != subsource.LangHant {
		t.Errorf("lang=%q", e.lang)
	}
	if e.format != "srt" {
		t.Errorf("format=%q", e.format)
	}
	if e.downloads != 167 {
		t.Errorf("downloads=%d", e.downloads)
	}
}

func TestRelevant(t *testing.T) {
	es := parseEntries(sampleHTML)
	m := metadata.MediaInfo{Title: "哪吒之魔童闹海", OriginalTitle: "Ne Zha 2", Year: 2025}
	if !relevant(es[0], m) {
		t.Fatal("应判定相关")
	}
	m2 := metadata.MediaInfo{Title: "流浪地球", OriginalTitle: "The Wandering Earth"}
	if relevant(es[0], m2) {
		t.Fatal("不应判定相关")
	}
}

func TestToSRT(t *testing.T) {
	in := "[00:01:23]\n第一句\n第二句\n\n[00:01:29]\n第三句\n"
	out, n := toSRT(in, 0)
	if n != 2 {
		t.Fatalf("n=%d", n)
	}
	want := "1\n00:01:23,000 --> 00:01:29,000\n第一句\n第二句\n\n2\n00:01:29,000 --> 00:01:31,000\n第三句\n\n"
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	// 已是标准 SRT 则原样
	srt := "1\n00:00:01,000 --> 00:00:02,000\nHi\n"
	out2, n2 := toSRT(srt, 0)
	if out2 != srt || n2 != 1 {
		t.Fatal("标准 SRT 不应被改写")
	}
	// 无时间轴
	if _, n := toSRT("hello world", 0); n != 0 {
		t.Fatal("无时间轴应返回 0")
	}
}

func TestDetectLang(t *testing.T) {
	cases := map[string]string{
		"xxx简体xxx": subsource.LangHans,
		"xxx繁体xxx": subsource.LangHant,
		"简体繁体":     subsource.LangBilingual,
		"xxx英文xxx": subsource.LangOther,
		"双语字幕":     subsource.LangBilingual,
		"nothing":  subsource.LangOther,
	}
	for in, want := range cases {
		if got := detectLang(in); got != want {
			t.Errorf("detectLang(%q)=%q want %q", in, got, want)
		}
	}
}

func TestBuildQueries(t *testing.T) {
	m := metadata.MediaInfo{Title: "沙丘", OriginalTitle: "Dune", Year: 2021, Type: metadata.Movie}
	qs := buildQueries(m)
	if len(qs) != 2 || qs[0] != "沙丘 2021" || qs[1] != "Dune 2021" {
		t.Fatalf("qs=%v", qs)
	}
	e := metadata.MediaInfo{Title: "绝命毒师", Type: metadata.Episode, Season: 1, Episode: 2}
	qs = buildQueries(e)
	if len(qs) != 1 || qs[0] != "绝命毒师 S01E02" {
		t.Fatalf("qs=%v", qs)
	}
}

func TestParseCount(t *testing.T) {
	if parseCount("167") != 167 || parseCount("45k") != 45000 || parseCount("1.2w") != 12000 {
		t.Fatal("parseCount 错误")
	}
}

func TestClientImplements(t *testing.T) {
	var _ subsource.Source = New()
	if New().Name() != "subhd" || !New().Enabled() {
		t.Fatal("Name/Enabled 错误")
	}
	// 无网络时 Search 不应 panic（错误可忽略，这里只验证签名）
	_ = context.Background
}
