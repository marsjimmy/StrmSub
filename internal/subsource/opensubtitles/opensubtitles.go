// Package opensubtitles 对接 OpenSubtitles.com REST API。
// 搜索只需免费 API key；下载需要账号登录（用户名+密码），
// 未配置账号时 Search 可用、Download 会明确报错。
package opensubtitles

import (
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

const apiBase = "https://api.opensubtitles.com/api/v1"

type Client struct {
	apiKey string
	user   string
	pass   string
	http   *http.Client
	token  string // 登录后缓存
}

func New(apiKey, user, pass string) *Client {
	return &Client{apiKey: apiKey, user: user, pass: pass, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) Name() string  { return "opensubtitles" }
func (c *Client) Enabled() bool { return c.apiKey != "" }

type searchResp struct {
	Data []struct {
		Attributes struct {
			SubtitleID  string `json:"subtitle_id"`
			Language    string `json:"language"`
			DownloadCount int  `json:"download_count"`
			Files []struct {
				FileID   int    `json:"file_id"`
				FileName string `json:"file_name"`
			} `json:"files"`
		} `json:"attributes"`
	} `json:"data"`
}

func (c *Client) Search(ctx context.Context, m metadata.MediaInfo) ([]subsource.Candidate, error) {
	q := m.Title
	if q == "" {
		q = m.OriginalTitle
	}
	params := url.Values{"query": {q}, "languages": {"zh-CN,zh-TW"}}
	if m.Year > 0 {
		params.Set("year", fmt.Sprintf("%d", m.Year))
	}
	if m.Type == metadata.Episode {
		if m.Season > 0 {
			params.Set("season_number", fmt.Sprintf("%d", m.Season))
		}
		if m.Episode > 0 {
			params.Set("episode_number", fmt.Sprintf("%d", m.Episode))
		}
	}
	// 有 imdb id 时更准
	if m.ImdbID != "" {
		params.Set("imdb_id", strings.TrimPrefix(m.ImdbID, "tt"))
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", apiBase+"/subtitles?"+params.Encode(), nil)
	req.Header.Set("Api-Key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var sr searchResp
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, err
	}
	var out []subsource.Candidate
	for _, d := range sr.Data {
		a := d.Attributes
		lang := subsource.LangOther
		switch strings.ToLower(a.Language) {
		case "zh-cn":
			lang = subsource.LangHans
		case "zh-tw":
			lang = subsource.LangHant
		}
		if lang == subsource.LangOther {
			continue
		}
		if len(a.Files) == 0 {
			continue
		}
		fileID := a.Files[0].FileID
		fname := a.Files[0].FileName
		out = append(out, subsource.Candidate{
			Source: "opensubtitles",
			RefID:  a.SubtitleID,
			Name:   fname,
			Lang:   lang,
			Format: "srt",
			Votes:  a.DownloadCount,
			Detail: fmt.Sprintf("下载 %d 次", a.DownloadCount),
			Download: func(ctx context.Context) (string, []byte, error) {
				return c.download(ctx, fileID, fname)
			},
		})
	}
	return out, nil
}

func (c *Client) login(ctx context.Context) error {
	if c.user == "" || c.pass == "" {
		return fmt.Errorf("opensubtitles 下载需要配置账号（STRMSUB_OS_USER/STRMSUB_OS_PASS）")
	}
	if c.token != "" {
		return nil
	}
	payload, _ := json.Marshal(map[string]string{"username": c.user, "password": c.pass})
	req, _ := http.NewRequestWithContext(ctx, "POST", apiBase+"/login", bytes.NewReader(payload))
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var lr struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &lr); err != nil || lr.Token == "" {
		return fmt.Errorf("opensubtitles 登录失败")
	}
	c.token = lr.Token
	return nil
}

func (c *Client) download(ctx context.Context, fileID int, fname string) (string, []byte, error) {
	if err := c.login(ctx); err != nil {
		return "", nil, err
	}
	payload, _ := json.Marshal(map[string]int{"file_id": fileID})
	req, _ := http.NewRequestWithContext(ctx, "POST", apiBase+"/download", bytes.NewReader(payload))
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var dr struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal(body, &dr); err != nil || dr.Link == "" {
		return "", nil, fmt.Errorf("opensubtitles 获取下载链接失败（可能超出每日免费额度）")
	}
	req2, _ := http.NewRequestWithContext(ctx, "GET", dr.Link, nil)
	resp2, err := c.http.Do(req2)
	if err != nil {
		return "", nil, err
	}
	defer resp2.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp2.Body, 32<<20))
	if err != nil {
		return "", nil, err
	}
	return fname, data, nil
}
