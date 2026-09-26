// Package subdl 对接 SubDL API（https://subdl.com/api）。
// 免费 API key，接口细节待联调，先占位实现接口。
package subdl

import (
	"context"
	"fmt"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

type Client struct{ apiKey string }

func New(apiKey string) *Client { return &Client{apiKey: apiKey} }

func (c *Client) Name() string  { return "subdl" }
func (c *Client) Enabled() bool { return c.apiKey != "" }

func (c *Client) Search(ctx context.Context, m metadata.MediaInfo) ([]subsource.Candidate, error) {
	// TODO: 按 https://subdl.com/api 文档实现
	// 搜索支持 imdb_id 参数，返回含中文的字幕
	return nil, fmt.Errorf("subdl 尚未实现")
}
