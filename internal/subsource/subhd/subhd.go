// Package subhd 对接 SubHD 字幕站（https://subhd.tv）。
// 免 key。搜索页直接返回字幕条目；下载走站内预览接口
// GET /api/sub/preview/{sid}?manifest=1&file=N（与站内 JS 同源），
// 内容为 [HH:MM:SS] 简化时间轴，转成标准 SRT 落盘。
package subhd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

const (
	baseURL = "https://subhd.tv"
	ua      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
)

type Client struct {
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) Name() string  { return "subhd" }
func (c *Client) Enabled() bool { return true } // 免 key，恒启用

// throttle 轻节流，做个有礼貌的爬虫
func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := time.Since(c.last); d < 800*time.Millisecond {
		time.Sleep(800*time.Millisecond - d)
	}
	c.last = time.Now()
}

func (c *Client) get(ctx context.Context, url string, referer string) ([]byte, error) {
	c.throttle()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("subhd http %d %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

// buildQueries 构造搜索词：电影用"标题 年份"，剧集用"标题 SxxExx"，中英文各试一次
func buildQueries(m metadata.MediaInfo) []string {
	titles := []string{m.Title}
	if m.OriginalTitle != "" && m.OriginalTitle != m.Title {
		titles = append(titles, m.OriginalTitle)
	}
	var out []string
	for _, t := range titles {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		switch {
		case m.Type == metadata.Episode && m.Season > 0 && m.Episode > 0:
			out = append(out, fmt.Sprintf("%s S%02dE%02d", t, m.Season, m.Episode))
		case m.Year > 0:
			out = append(out, fmt.Sprintf("%s %d", t, m.Year))
		default:
			out = append(out, t)
		}
	}
	return out
}

type entry struct {
	sid       string
	name      string // 条目标题（中文名）
	detail    string // 副标题行（语言/英文名/年份）
	lang      string // 归一化语言
	format    string
	downloads int
	uploader  string
}

var (
	chunkMarker = `<div class="bg-white shadow-sm rounded-3 mb-4">`
	reSid       = regexp.MustCompile(`href=['"]/a/([A-Za-z0-9]+)['"]`)
	reLinkTxt   = regexp.MustCompile(`(?s)href=['"]/a/[A-Za-z0-9]+['"][^>]*>([^<]{1,120})<`)
	reDetail    = regexp.MustCompile(`(?s)view-text text-secondary.*?<a[^>]*>([^<]{1,160})<`)
	reStat      = regexp.MustCompile(`<span class=['"]align-text-top me-3['"]>([^<]{1,12})</span>`)
	reUploader  = regexp.MustCompile(`发布人\s*[|｜]?\s*([^<|\s]{1,40})`)
	reWordASS   = regexp.MustCompile(`\bass\b`)
	reWordSSA   = regexp.MustCompile(`\bssa\b`)
)

func (c *Client) Search(ctx context.Context, m metadata.MediaInfo) ([]subsource.Candidate, error) {
	queries := buildQueries(m)
	seen := map[string]bool{}
	var out []subsource.Candidate
	for _, q := range queries {
		if len([]rune(strings.TrimSpace(q))) < 2 {
			continue
		}
		body, err := c.get(ctx, baseURL+"/search/"+url.PathEscape(q), baseURL+"/")
		if err != nil {
			log.Printf("[subhd] 搜索失败 %q: %v", q, err)
			continue
		}
		for _, e := range parseEntries(string(body)) {
			if seen[e.sid] {
				continue
			}
			seen[e.sid] = true
			if !relevant(e, m) {
				continue
			}
			e := e
			name := strings.TrimSpace(e.name + " " + e.detail)
			out = append(out, subsource.Candidate{
				Source: "subhd",
				RefID:  e.sid,
				Name:   name,
				Lang:   e.lang,
				Format: e.format,
				Votes:  e.downloads,
				Detail: fmt.Sprintf("SubHD · %s · 下载%d · %s", e.detail, e.downloads, e.uploader),
				Download: func(ctx context.Context) (string, []byte, error) {
					return c.download(ctx, e.sid)
				},
			})
		}
		if len(out) >= 10 {
			break
		}
	}
	return out, nil
}

// relevant 条目标题/副标题是否包含媒体标题（中/英文），防串片
func relevant(e entry, m metadata.MediaInfo) bool {
	hay := strings.ToLower(e.name + " " + e.detail)
	for _, t := range []string{m.Title, m.OriginalTitle} {
		t = strings.ToLower(strings.TrimSpace(t))
		if len([]rune(t)) >= 2 && strings.Contains(hay, t) {
			return true
		}
	}
	return false
}

func parseEntries(html string) []entry {
	var out []entry
	for _, chunk := range strings.Split(html, chunkMarker)[1:] {
		sm := reSid.FindStringSubmatch(chunk)
		if sm == nil {
			continue
		}
		e := entry{sid: sm[1], format: "srt"}
		if lm := reLinkTxt.FindStringSubmatch(chunk); lm != nil {
			e.name = strings.TrimSpace(lm[1])
		}
		if dm := reDetail.FindStringSubmatch(chunk); dm != nil {
			e.detail = strings.TrimSpace(dm[1])
		}
		e.lang = detectLang(chunk)
		if e.lang == subsource.LangOther {
			continue
		}
		lower := strings.ToLower(chunk)
		switch {
		case reWordASS.MatchString(lower):
			e.format = "ass"
		case reWordSSA.MatchString(lower):
			e.format = "ssa"
		}
		stats := reStat.FindAllStringSubmatch(chunk, -1)
		if len(stats) >= 2 {
			e.downloads = parseCount(stats[1][1]) // 第一个是浏览，第二个是下载
		} else if len(stats) == 1 {
			e.downloads = parseCount(stats[0][1])
		}
		if um := reUploader.FindStringSubmatch(chunk); um != nil {
			e.uploader = strings.TrimSpace(um[1])
		}
		out = append(out, e)
	}
	return out
}

// detectLang 从条目文本识别语言
func detectLang(chunk string) string {
	has := func(s string) bool { return strings.Contains(chunk, s) }
	simp, trad := has("简体"), has("繁体")
	switch {
	case simp && trad:
		return subsource.LangBilingual
	case has("双语") || has("简英"):
		return subsource.LangBilingual
	case simp:
		return subsource.LangHans
	case trad:
		return subsource.LangHant
	default:
		return subsource.LangOther
	}
}

func parseCount(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	mult := 1
	if strings.HasSuffix(s, "k") {
		mult = 1000
		s = strings.TrimSuffix(s, "k")
	} else if strings.HasSuffix(s, "w") {
		mult = 10000
		s = strings.TrimSuffix(s, "w")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int(f * float64(mult))
}

// ---- 下载：预览接口 ----

type previewFile struct {
	Index       int    `json:"index"`
	Filename    string `json:"filename"`
	Filesize    int    `json:"filesize"`
	Previewable bool   `json:"previewable"`
	Content     string `json:"content"`
}

type manifestResp struct {
	Success bool          `json:"success"`
	Files   []previewFile `json:"files"`
	File    *previewFile  `json:"file"`
}

// DownloadRef 按 RefID（subhd sid）下载
func (c *Client) DownloadRef(ctx context.Context, refID string) (string, []byte, error) {
	if refID == "" {
		return "", nil, fmt.Errorf("subhd refID 为空")
	}
	return c.download(ctx, refID)
}

func (c *Client) download(ctx context.Context, sid string) (string, []byte, error) {
	referer := baseURL + "/a/" + sid
	body, err := c.get(ctx, fmt.Sprintf("%s/api/sub/preview/%s?manifest=1", baseURL, sid), referer)
	if err != nil {
		return "", nil, err
	}
	var mr manifestResp
	if err := json.Unmarshal(body, &mr); err != nil {
		return "", nil, err
	}
	files := mr.Files
	if len(files) == 0 && mr.File != nil {
		files = []previewFile{*mr.File}
	}
	var srt strings.Builder
	fname := "subhd_" + sid + ".srt"
	seq := 0
	got := false
	for _, f := range files {
		if !f.Previewable && f.Content == "" {
			continue
		}
		content := f.Content
		if content == "" {
			// manifest 里没带内容，单独取
			b, err := c.get(ctx, fmt.Sprintf("%s/api/sub/preview/%s?file=%d", baseURL, sid, f.Index), referer)
			if err != nil {
				continue
			}
			var fr manifestResp
			if err := json.Unmarshal(b, &fr); err != nil || fr.File == nil || fr.File.Content == "" {
				continue
			}
			content = fr.File.Content
			f.Filename = fr.File.Filename
		}
		conv, n := toSRT(content, seq)
		if n == 0 {
			continue
		}
		got = true
		seq += n
		if f.Filename != "" {
			fname = f.Filename
		}
		srt.WriteString(conv)
	}
	if !got {
		return "", nil, fmt.Errorf("subhd 预览无可用内容 sid=%s", sid)
	}
	return fname, []byte(srt.String()), nil
}

var reCue = regexp.MustCompile(`(?m)^\[(\d{1,2}):(\d{2}):(\d{2})\]\s*$`)

// toSRT 把 [HH:MM:SS] 简化时间轴转成标准 SRT；已是标准 SRT 则原样返回
func toSRT(content string, startSeq int) (string, int) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if strings.Contains(content, "-->") {
		n := strings.Count(content, "-->")
		return content, n
	}
	locs := reCue.FindAllStringSubmatchIndex(content, -1)
	if len(locs) == 0 {
		return "", 0
	}
	type cue struct {
		start int // 秒
		text  string
	}
	var cues []cue
	for i, loc := range locs {
		h, _ := strconv.Atoi(content[loc[2]:loc[3]])
		mi, _ := strconv.Atoi(content[loc[4]:loc[5]])
		se, _ := strconv.Atoi(content[loc[6]:loc[7]])
		end := len(content)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		text := strings.TrimSpace(content[loc[1]:end])
		if text == "" {
			continue
		}
		cues = append(cues, cue{h*3600 + mi*60 + se, text})
	}
	var b strings.Builder
	for i, cu := range cues {
		end := cu.start + 2
		if i+1 < len(cues) {
			end = cues[i+1].start
			if end <= cu.start {
				end = cu.start + 2
			}
		}
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", startSeq+i+1, fmtTS(cu.start), fmtTS(end), cu.text)
	}
	return b.String(), len(cues)
}

func fmtTS(sec int) string {
	h, m, s := sec/3600, (sec%3600)/60, sec%60
	return fmt.Sprintf("%02d:%02d:%02d,000", h, m, s)
}
