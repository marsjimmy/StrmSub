// Package subtitlecat 对接 SubtitleCat（https://www.subtitlecat.com），
// 成人影片（番号）中文字幕源。无需 key，HTML 抓取。
// 仅当查询词像番号（ABC-123 / SONE028）时才搜索，避免无效请求。
// 站点有反爬时可经 FlareSolverr 抓取（配置后自动使用）。
package subtitlecat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/marsjimmy/strmsub/internal/flare"
	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

const baseURL = "https://www.subtitlecat.com"

var (
	reCode      = regexp.MustCompile(`^[A-Za-z]{2,6}[\-_]?\d{2,5}$`)
	reDetail    = regexp.MustCompile(`href="(subs/\d+/[^"]*)"`)
	reSRT       = regexp.MustCompile(`href\s*=\s*["']([^"']*\.srt)["']`)
	reChinese   = regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)
	reDetailAlt = regexp.MustCompile(`(?i)href="([^"]*\.srt[^"]*)"`)
)

type Client struct {
	http  *http.Client
	flare *flare.Client
}

func New(f *flare.Client) *Client {
	return &Client{
		http:  &http.Client{Timeout: 15 * time.Second},
		flare: f,
	}
}

func (c *Client) Name() string  { return "subtitlecat" }
func (c *Client) Enabled() bool { return true }

func (c *Client) get(ctx context.Context, pageURL, referer string) (string, error) {
	if c.flare.Enabled() {
		return c.flare.Get(ctx, pageURL)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("subtitlecat http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// looksLikeCode 查询词是否像番号
func looksLikeCode(q string) bool {
	q = strings.TrimSpace(q)
	return reCode.MatchString(q)
}

type detailLink struct {
	path string
	text string
}

func (c *Client) Search(ctx context.Context, m metadata.MediaInfo) ([]subsource.Candidate, error) {
	q := strings.TrimSpace(m.Title)
	if !looksLikeCode(q) {
		return nil, nil // 非番号不查
	}
	searchURL := baseURL + "/index.php?search=" + url.QueryEscape(q) + "&show=1000"
	html, err := c.get(ctx, searchURL, baseURL+"/")
	if err != nil {
		return nil, err
	}
	// 收集详情页链接，优先锚文本含番号的
	var links []detailLink
	for _, sm := range reDetail.FindAllStringSubmatch(html, -1) {
		links = append(links, detailLink{path: sm[1]})
	}
	if len(links) == 0 {
		return nil, nil
	}
	// 取前 3 个详情页找中文字幕
	var out []subsource.Candidate
	seen := map[string]bool{}
	for i, l := range links {
		if i >= 3 {
			break
		}
		detailURL := baseURL + "/" + strings.TrimPrefix(l.path, "/")
		dhtml, err := c.get(ctx, detailURL, searchURL)
		if err != nil {
			continue
		}
		for _, srtPath := range pickSRTs(dhtml) {
			if seen[srtPath] {
				continue
			}
			seen[srtPath] = true
			lang := langOf(srtPath)
			if lang == subsource.LangOther {
				continue
			}
			refID := detailURL + "|" + url.QueryEscape(srtPath)
			out = append(out, subsource.Candidate{
				Source: "subtitlecat",
				RefID:  refID,
				Name:   srtBase(srtPath),
				Lang:   lang,
				Format: "srt",
				Detail: fmt.Sprintf("SubtitleCat · %s", q),
				Download: func(ctx context.Context) (string, []byte, error) {
					return c.downloadRef(ctx, detailURL, srtPath)
				},
			})
		}
		if len(out) >= 6 {
			break
		}
	}
	return out, nil
}

// pickSRTs 按 zh-CN > zh-TW > zh 优先级挑 SRT 链接
func pickSRTs(html string) []string {
	var zhCN, zhTW, zh, rest []string
	for _, sm := range reSRT.FindAllStringSubmatch(html, -1) {
		p := sm[1]
		l := strings.ToLower(p)
		switch {
		case strings.Contains(l, "zh-cn"):
			zhCN = append(zhCN, p)
		case strings.Contains(l, "zh-tw"):
			zhTW = append(zhTW, p)
		case strings.Contains(l, "-zh.") || strings.Contains(l, "_zh.") || strings.Contains(l, "zh.srt"):
			zh = append(zh, p)
		default:
			rest = append(rest, p)
		}
	}
	// 兜底：大小写不敏感的 .srt 链接
	if len(zhCN)+len(zhTW)+len(zh)+len(rest) == 0 {
		for _, sm := range reDetailAlt.FindAllStringSubmatch(html, -1) {
			rest = append(rest, sm[1])
		}
	}
	return append(append(append(zhCN, zhTW...), zh...), rest...)
}

func langOf(p string) string {
	l := strings.ToLower(p)
	switch {
	case strings.Contains(l, "zh-cn"):
		return subsource.LangHans
	case strings.Contains(l, "zh-tw"):
		return subsource.LangHant
	case strings.Contains(l, "chinese"), strings.Contains(l, "chs"), strings.Contains(l, "cht"):
		if strings.Contains(l, "cht") {
			return subsource.LangHant
		}
		return subsource.LangHans
	default:
		return subsource.LangOther
	}
}

func srtBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		p = p[i+1:]
	}
	if u, err := url.QueryUnescape(p); err == nil {
		p = u
	}
	return p
}

// DownloadRef 按 RefID（detailURL|urlencode(srtPath)）下载
func (c *Client) DownloadRef(ctx context.Context, refID string) (string, []byte, error) {
	parts := strings.SplitN(refID, "|", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("subtitlecat refID 非法")
	}
	srtPath, err := url.QueryUnescape(parts[1])
	if err != nil || srtPath == "" {
		return "", nil, fmt.Errorf("subtitlecat refID 非法")
	}
	return c.downloadRef(ctx, parts[0], srtPath)
}

func (c *Client) downloadRef(ctx context.Context, detailURL, srtPath string) (string, []byte, error) {
	// SRT 路径必须 URL 编码（含 [ ]、中文、特殊撇号等）
	var full string
	if strings.HasPrefix(srtPath, "http://") || strings.HasPrefix(srtPath, "https://") {
		full = srtPath
	} else {
		full = baseURL + "/" + strings.TrimPrefix(urlEncodePath(srtPath), "/")
	}
	var data []byte
	if c.flare.Enabled() {
		// FlareSolverr 返回的是页面文本；SRT 走直接下载
		html, err := c.flare.Get(ctx, full)
		if err != nil {
			return "", nil, err
		}
		data = []byte(html)
	} else {
		req, err := http.NewRequestWithContext(ctx, "GET", full, nil)
		if err != nil {
			return "", nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
		req.Header.Set("Referer", detailURL)
		resp, err := c.http.Do(req)
		if err != nil {
			return "", nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return "", nil, fmt.Errorf("subtitlecat 下载 http %d", resp.StatusCode)
		}
		data, err = io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		if err != nil {
			return "", nil, err
		}
	}
	// 校验：中文字符 > 100 才算拿到有效字幕
	if len(reChinese.FindAll(data, -1)) < 100 {
		return "", nil, fmt.Errorf("subtitlecat 下载内容无效（中文字符过少）")
	}
	return srtBase(srtPath), data, nil
}

// urlEncodePath 对路径逐段编码，保留 /
func urlEncodePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}
