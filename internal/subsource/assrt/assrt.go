// Package assrt 对接射手网 ASSRT API（https://assrt.net/api/doc）。
// 免费 token，配额 20 次/分钟，本客户端内置节流。
package assrt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

const apiBase = "https://api.assrt.net"

type Client struct {
	token string
	http  *http.Client
	mu    sync.Mutex
	last  time.Time
}

func New(token string) *Client {
	return &Client{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) Name() string  { return "assrt" }
func (c *Client) Enabled() bool { return c.token != "" }

// throttle 配额 20/min，这里按 3.2 秒间隔保守节流
func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := time.Since(c.last); d < 3200*time.Millisecond {
		time.Sleep(3200*time.Millisecond - d)
	}
	c.last = time.Now()
}

type searchResp struct {
	Status int `json:"status"`
	Sub    struct {
		Subs []struct {
			ID          int    `json:"id"`
			NativeName  string `json:"native_name"`
			Videoname   string `json:"videoname"`
			Subtype     string `json:"subtype"`
			UploadTime  string `json:"upload_time"`
			VoteScore   int    `json:"vote_score"`
			ReleaseSite string `json:"release_site"`
			Lang        struct {
				Desc string `json:"desc"`
			} `json:"lang"`
		} `json:"subs"`
	} `json:"sub"`
}

type detailResp struct {
	Status int `json:"status"`
	Sub    struct {
		Subs []struct {
			URL      string `json:"url"`
			Filelist []struct {
				F   string `json:"f"`
				URL string `json:"url"`
			} `json:"filelist"`
		} `json:"subs"`
	} `json:"sub"`
}

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	c.throttle()
	params.Set("token", c.token)
	req, err := http.NewRequestWithContext(ctx, "GET", apiBase+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("assrt http %d", resp.StatusCode)
	}
	return body, nil
}

// buildQueries 构造搜索词列表。ASSRT 只支持关键词搜索，不确定服务端对
// "标题 SxxExx" 是否做 AND 匹配（subhd 已实测会返回 0 条），保险起见剧集
// 同时搜"标题 SxxExx"和纯"标题"，调用方合并去重。
func buildQueries(m metadata.MediaInfo) []string {
	title := m.Title
	if title == "" {
		title = m.OriginalTitle
	}
	var out []string
	if m.Type == metadata.Episode && m.Season > 0 && m.Episode > 0 {
		out = append(out, fmt.Sprintf("%s S%02dE%02d", title, m.Season, m.Episode))
	}
	if m.Year > 0 {
		out = append(out, fmt.Sprintf("%s %d", title, m.Year))
	}
	out = append(out, title)
	return out
}

// normLang 从 lang.desc 识别语言："简"/"中"→简体，"繁"→繁体，"双语"→双语
func normLang(desc string) string {
	d := desc
	has := func(s string) bool { return strings.Contains(d, s) }
	if has("双语") {
		return subsource.LangBilingual
	}
	if has("繁") {
		return subsource.LangHant
	}
	if has("简") || has("中字") || has("中文") || has("国语") {
		return subsource.LangHans
	}
	return subsource.LangOther
}

func (c *Client) Search(ctx context.Context, m metadata.MediaInfo) ([]subsource.Candidate, error) {
	// 先用标题搜；剧集/电影若有原名（中↔英）且不同，再用原名搜一次，合并候选
	queries := buildQueries(m)
	if m.OriginalTitle != "" && m.OriginalTitle != m.Title {
		m2 := m
		m2.Title, m2.OriginalTitle = m.OriginalTitle, m.Title
		for _, q := range buildQueries(m2) {
			dup := false
			for _, e := range queries {
				if e == q {
					dup = true
					break
				}
			}
			if !dup {
				queries = append(queries, q)
			}
		}
	}
	var out []subsource.Candidate
	seen := map[string]bool{}
	for _, q := range queries {
		cands, err := c.searchOne(ctx, m, q)
		if err != nil {
			return nil, err
		}
		for _, cd := range cands {
			key := cd.Source + ":" + cd.RefID
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, cd)
		}
	}
	return out, nil
}

func (c *Client) searchOne(ctx context.Context, m metadata.MediaInfo, q string) ([]subsource.Candidate, error) {
	if len([]rune(strings.TrimSpace(q))) < 3 {
		return nil, nil
	}
	body, err := c.get(ctx, "/v1/sub/search", url.Values{"q": {q}, "cnt": {"15"}})
	if err != nil {
		return nil, err
	}
	var sr searchResp
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, err
	}
	if sr.Status != 0 {
		return nil, fmt.Errorf("assrt api status=%d", sr.Status)
	}
	var out []subsource.Candidate
	for _, s := range sr.Sub.Subs {
		lang := normLang(s.Lang.Desc)
		if lang == subsource.LangOther {
			continue // 只要中文相关
		}
		id := s.ID
		out = append(out, subsource.Candidate{
			Source: "assrt",
			RefID:  fmt.Sprintf("%d", id),
			Name:   s.NativeName,
			Lang:   lang,
			Format: strings.ToLower(s.Subtype),
			Votes:  s.VoteScore,
			Detail: fmt.Sprintf("%s · %s · %s", s.ReleaseSite, s.Videoname, s.UploadTime),
			Download: func(ctx context.Context) (string, []byte, error) {
				return c.download(ctx, id)
			},
		})
	}
	return out, nil
}

// DownloadRef 按 RefID（字幕 id）下载
func (c *Client) DownloadRef(ctx context.Context, refID string) (string, []byte, error) {
	id, err := strconv.Atoi(refID)
	if err != nil {
		return "", nil, fmt.Errorf("assrt refID 非法: %s", refID)
	}
	return c.download(ctx, id)
}

// download 取详情→选最合适的单文件→下载（详情接口的 url 有时效，选中时才调）
func (c *Client) download(ctx context.Context, id int) (string, []byte, error) {
	body, err := c.get(ctx, "/v1/sub/detail", url.Values{"id": {fmt.Sprintf("%d", id)}})
	if err != nil {
		return "", nil, err
	}
	var dr detailResp
	if err := json.Unmarshal(body, &dr); err != nil {
		return "", nil, err
	}
	if dr.Status != 0 || len(dr.Sub.Subs) == 0 {
		return "", nil, fmt.Errorf("assrt detail 无结果 id=%d", id)
	}
	sub := dr.Sub.Subs[0]
	dlURL, fname := pickFile(sub)
	if dlURL == "" {
		return "", nil, fmt.Errorf("assrt detail 无可下载文件 id=%d", id)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", dlURL, nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", nil, err
	}
	return fname, data, nil
}

// pickFile 从 filelist 选最合适的一个：优先 .srt，其次 .ass；文件名含简/中优先
func pickFile(sub struct {
	URL      string `json:"url"`
	Filelist []struct {
		F   string `json:"f"`
		URL string `json:"url"`
	} `json:"filelist"`
}) (string, string) {
	type f struct{ url, name string }
	var srts, asss []f
	for _, fl := range sub.Filelist {
		lower := strings.ToLower(fl.F)
		switch {
		case strings.HasSuffix(lower, ".srt"):
			srts = append(srts, f{fl.URL, fl.F})
		case strings.HasSuffix(lower, ".ass") || strings.HasSuffix(lower, ".ssa"):
			asss = append(asss, f{fl.URL, fl.F})
		}
	}
	score := func(name string) int {
		s := 0
		if strings.Contains(name, "简") || strings.Contains(name, "chs") || strings.Contains(name, "中") {
			s += 2
		}
		if strings.Contains(name, "双语") {
			s++
		}
		return s
	}
	best := func(fs []f) (string, string) {
		bu, bn := "", ""
		bs := -1
		for _, x := range fs {
			if sc := score(x.name); sc > bs {
				bs, bu, bn = sc, x.url, x.name
			}
		}
		return bu, bn
	}
	if u, n := best(srts); u != "" {
		return u, n
	}
	if u, n := best(asss); u != "" {
		return u, n
	}
	return "", ""
}
