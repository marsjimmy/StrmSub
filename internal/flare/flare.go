// Package flare 封装 FlareSolverr（https://github.com/FlareSolverr/FlareSolverr），
// 用于绕过字幕站的 Cloudflare / 反爬验证。未配置地址时不启用。
package flare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	base string
	http *http.Client
}

func New(baseURL string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &Client{base: baseURL, http: &http.Client{Timeout: 90 * time.Second}}
}

// Enabled 是否配置了 FlareSolverr
func (c *Client) Enabled() bool { return c != nil && c.base != "" }

type v1req struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url,omitempty"`
	MaxTimeout int    `json:"maxTimeout,omitempty"`
}

type v1resp struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Solution *struct {
		URL      string `json:"url"`
		Status   int    `json:"status"`
		Response string `json:"response"`
	} `json:"solution"`
}

// Get 经 FlareSolverr 抓取页面，返回 HTML
func (c *Client) Get(ctx context.Context, url string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("未配置 FlareSolverr")
	}
	payload, _ := json.Marshal(v1req{Cmd: "request.get", URL: url, MaxTimeout: 60000})
	req, err := http.NewRequestWithContext(ctx, "POST", c.base+"/v1", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	var vr v1resp
	if err := json.Unmarshal(body, &vr); err != nil {
		return "", err
	}
	if vr.Status != "ok" || vr.Solution == nil {
		return "", fmt.Errorf("flaresolverr 失败: %s", vr.Message)
	}
	return vr.Solution.Response, nil
}
