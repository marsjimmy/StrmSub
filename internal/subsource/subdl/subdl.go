// Package subdl 对接 SubDL API（https://subdl.com/api-doc）。
// 免费 API key（subdl.com 注册后在账号面板获取）。
package subdl

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

const apiBase = "https://api.subdl.com/api/v1"

type Client struct {
	apiKey string
	http   *http.Client
}

func New(apiKey string) *Client {
	return &Client{apiKey: apiKey, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) Name() string  { return "subdl" }
func (c *Client) Enabled() bool { return c.apiKey != "" }

type subItem struct {
	Name          string `json:"name"`
	Language      string `json:"language"`
	URL           string `json:"url"`
	ReleaseName   string `json:"release_name"`
	Author        string `json:"author"`
	DownloadCount int    `json:"download_count"`
}

type searchResp struct {
	Status    bool      `json:"status"`
	Subtitles []subItem `json:"subtitles"`
}

func (c *Client) Search(ctx context.Context, m metadata.MediaInfo) ([]subsource.Candidate, error) {
	q := strings.TrimSpace(m.Title)
	if len([]rune(q)) < 2 {
		return nil, nil
	}
	v := url.Values{
		"api_key":       {c.apiKey},
		"film_name":     {q},
		"languages":     {"ZH"},
		"subs_per_page": {"20"},
	}
	if m.Year > 0 {
		v.Set("year", fmt.Sprintf("%d", m.Year))
	}
	if m.Season > 0 || m.Episode > 0 {
		v.Set("type", "tv")
		if m.Season > 0 {
			v.Set("season_number", fmt.Sprintf("%d", m.Season))
		}
		if m.Episode > 0 {
			v.Set("episode_number", fmt.Sprintf("%d", m.Episode))
		}
	} else {
		v.Set("type", "movie")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/subtitles?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "StrmSub/2.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var sr searchResp
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, err
	}
	var out []subsource.Candidate
	for _, s := range sr.Subtitles {
		lang := normLang(s.Language)
		if lang == subsource.LangOther {
			continue
		}
		if s.URL == "" {
			continue
		}
		dlURL := s.URL
		name := s.Name
		if name == "" {
			name = s.ReleaseName
		}
		out = append(out, subsource.Candidate{
			Source: "subdl",
			RefID:  dlURL, // 下载链接自带 api_key，直接可用
			Name:   name,
			Lang:   lang,
			Format: formatOf(name),
			Votes:  s.DownloadCount,
			Detail: fmt.Sprintf("SubDL · %s · %s", s.ReleaseName, s.Author),
			Download: func(ctx context.Context) (string, []byte, error) {
				return c.download(ctx, dlURL)
			},
		})
	}
	return out, nil
}

// DownloadRef 按 RefID（下载 URL）下载
func (c *Client) DownloadRef(ctx context.Context, refID string) (string, []byte, error) {
	if refID == "" || (!strings.HasPrefix(refID, "http://") && !strings.HasPrefix(refID, "https://")) {
		return "", nil, fmt.Errorf("subdl refID 非法")
	}
	return c.download(ctx, refID)
}

func (c *Client) download(ctx context.Context, dlURL string) (string, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", dlURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "StrmSub/2.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", nil, err
	}
	// 可能是 zip 包（PK 开头）：解压取第一个字幕文件
	if len(data) > 4 && data[0] == 'P' && data[1] == 'K' {
		if fname, inner, err := unzipFirst(data); err == nil {
			return fname, inner, nil
		}
	}
	return "subtitle.srt", data, nil
}

func unzipFirst(data []byte) (string, []byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, err
	}
	for _, f := range zr.File {
		lower := strings.ToLower(f.Name)
		if strings.HasSuffix(lower, ".srt") || strings.HasSuffix(lower, ".ass") || strings.HasSuffix(lower, ".ssa") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			inner, err := io.ReadAll(io.LimitReader(rc, 16<<20))
			rc.Close()
			if err != nil {
				continue
			}
			return f.Name, inner, nil
		}
	}
	return "", nil, fmt.Errorf("zip 内无字幕文件")
}

func normLang(s string) string {
	l := strings.ToLower(s)
	if strings.Contains(l, "zh") || strings.Contains(l, "chinese") || strings.Contains(l, "中文") {
		if strings.Contains(l, "tw") || strings.Contains(l, "traditional") || strings.Contains(l, "繁") {
			return subsource.LangHant
		}
		return subsource.LangHans
	}
	return subsource.LangOther
}

func formatOf(name string) string {
	l := strings.ToLower(name)
	switch {
	case strings.HasSuffix(l, ".ass") || strings.HasSuffix(l, ".ssa"):
		return "ass"
	default:
		return "srt"
	}
}
