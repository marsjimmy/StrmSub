// Package fnosapi 通过飞牛影视的 HTTP API（Emby 式）接入媒体库。
//
// 飞牛影视（trim.media）自带一套 Web API，官方前端也在用，前缀 /v/api/v1，
// 已被开源项目 fly_player 完整逆向。本包实现：
//   - POST /v/api/v1/login 用户名密码登录拿 token
//   - 后续请求头带 Authorization + Trim-MC-token + Authx（新版后端强制签名）
//   - POST /v/api/v1/item/list 分页拉取全库条目
//   - GET /v/api/v1/season/list/:guid + GET /v/api/v1/episode/list/:guid 展开剧集
//   - GET /v/api/v1/item/:guid 拿文件路径等详情
//
// 相比直读 SQLite：不需要挂载 db 文件，可跑在任意能连上飞牛的机器上；
// 代价是需要一个飞牛账号（只读权限即可）且接口为逆向所得，字段解析全部走
// 候选键防御式匹配，未知字段忽略不崩。
package fnosapi

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Authx 签名用的公开 key/secret，来自官方前端打包代码（fly_player 逆向）。
const (
	authxKey    = "NDzZTVxnRKP8Z0jXg1VAMonaG8akvh"
	authxSecret = "16CCEB3D-AB42-077D-36A1-F355324E4237"
	apiPrefix   = "/v/api/v1"
	loginPath   = apiPrefix + "/login"
)

// Client 飞牛影视 API 客户端：管登录、token、Authx 签名。
type Client struct {
	base string // 如 http://192.168.1.10:8005
	user string
	pass string

	http *http.Client

	mu    sync.Mutex
	token string
}

func NewClient(baseURL, user, pass string) *Client {
	return &Client{
		base: strings.TrimRight(baseURL, "/"),
		user: user,
		pass: pass,
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

// envelope 飞牛统一响应包裹
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (c *Client) login(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{"userName": c.user, "password": c.pass})
	req, err := http.NewRequestWithContext(ctx, "POST", c.base+loginPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("飞牛登录请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("飞牛登录响应解析失败: %w", err)
	}
	if env.Code != 0 {
		return fmt.Errorf("飞牛登录失败(code=%d): %s", env.Code, env.Msg)
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil || data.Token == "" {
		return fmt.Errorf("飞牛登录未返回 token: %s", string(env.Data))
	}
	c.mu.Lock()
	c.token = data.Token
	c.mu.Unlock()
	log.Printf("[fnosapi] 登录成功 user=%s", c.user)
	return nil
}

func (c *Client) ensureToken(ctx context.Context) error {
	c.mu.Lock()
	t := c.token
	c.mu.Unlock()
	if t != "" {
		return nil
	}
	return c.login(ctx)
}

func md5hex(b []byte) string {
	h := md5.Sum(b)
	return hex.EncodeToString(h[:])
}

// authxHeader 构造 Authx 签名头。新版后端严格校验：
// sign = md5("KEY_path_nonce_timestamp_payloadMd5_SECRET")，各段用 _ 连接。
// GET 的 payloadMd5 取排序后的 query 串 k=v&k2=v2；非 GET 取请求体原始字节。
// 头格式: nonce=<nonce>&timestamp=<ts>&sign=<sign>（timestamp 为毫秒）。
func authxHeader(method, path string, query map[string]string, bodyBytes []byte) string {
	nonce := strconv.Itoa(100000 + rand.IntN(900000))
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)

	var payloadMD5 string
	if strings.ToUpper(method) == "GET" {
		keys := make([]string, 0, len(query))
		for k := range query {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var sb strings.Builder
		for i, k := range keys {
			if i > 0 {
				sb.WriteByte('&')
			}
			sb.WriteString(k)
			sb.WriteByte('=')
			sb.WriteString(query[k])
		}
		payloadMD5 = md5hex([]byte(sb.String()))
	} else {
		if bodyBytes == nil {
			bodyBytes = []byte{}
		}
		payloadMD5 = md5hex(bodyBytes)
	}
	signBase := strings.Join([]string{authxKey, path, nonce, ts, payloadMD5, authxSecret}, "_")
	sign := md5hex([]byte(signBase))
	return "nonce=" + nonce + "&timestamp=" + ts + "&sign=" + sign
}

// do 发起一次 API 调用，返回 data 段的原始 JSON。token 失效时自动重登一次。
func (c *Client) do(ctx context.Context, method, path string, query map[string]string, body any) (json.RawMessage, error) {
	return c.doWithRetry(ctx, method, path, query, body, true)
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, query map[string]string, body any, allowRelogin bool) (json.RawMessage, error) {
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}

	u := c.base + path
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	if len(query) > 0 {
		q := url.Values{}
		for k, v := range query {
			q.Set(k, v)
		}
		u += "?" + q.Encode()
	}
	var rdr io.Reader
	if bodyBytes != nil {
		rdr = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	req.Header.Set("Authorization", token)
	req.Header.Set("Trim-MC-token", token)
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// 登录接口不加 Authx，其余 /v/api/v1/* 都加
	if path != loginPath {
		req.Header.Set("Authx", authxHeader(method, path, query, bodyBytes))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("飞牛 API %s %s 失败: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

	// token 过期：HTTP 401 时重登一次再试
	if resp.StatusCode == http.StatusUnauthorized && allowRelogin {
		c.mu.Lock()
		c.token = ""
		c.mu.Unlock()
		log.Printf("[fnosapi] token 疑似过期，重新登录后重试 %s %s", method, path)
		return c.doWithRetry(ctx, method, path, query, body, false)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("飞牛 API %s %s 响应解析失败: %w", method, path, err)
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("飞牛 API %s %s 返回 code=%d: %s", method, path, env.Code, env.Msg)
	}
	return env.Data, nil
}

// Get / Post 便捷封装
func (c *Client) Get(ctx context.Context, path string, query map[string]string) (json.RawMessage, error) {
	return c.do(ctx, "GET", path, query, nil)
}

func (c *Client) Post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.do(ctx, "POST", path, nil, body)
}

// GetBytes 下载二进制（海报用）：相对路径拼到 base 上，绝对 URL 直接用，均带认证头。
// 校验魔数，只接受 JPEG/PNG/WebP/GIF。
func (c *Client) GetBytes(ctx context.Context, ref string) ([]byte, error) {
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}
	var u, signPath string
	switch {
	case strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://"):
		u = ref
		if strings.HasPrefix(ref, c.base) {
			signPath = strings.TrimPrefix(ref, c.base)
		}
	case strings.HasPrefix(ref, "/"):
		u = c.base + ref
		signPath = ref
	default:
		return nil, fmt.Errorf("不支持的海报引用: %s", ref)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	req.Header.Set("Authorization", token)
	req.Header.Set("Trim-MC-token", token)
	if signPath != "" && strings.HasPrefix(signPath, apiPrefix+"/") && signPath != loginPath {
		req.Header.Set("Authx", authxHeader("GET", signPath, nil, nil))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("海报下载失败 %s: %w", ref, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("海报下载 %s: HTTP %d", ref, resp.StatusCode)
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if !isImageBytes(raw) {
		return nil, fmt.Errorf("海报 %s 不是图片", ref)
	}
	return raw, nil
}

func isImageBytes(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	// JPEG
	if b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return true
	}
	// PNG
	if b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G' {
		return true
	}
	// GIF
	if b[0] == 'G' && b[1] == 'I' && b[2] == 'F' {
		return true
	}
	// WebP: RIFF....WEBP
	if len(b) >= 12 && b[0] == 'R' && b[1] == 'I' && b[2] == 'F' && b[3] == 'F' &&
		b[8] == 'W' && b[9] == 'E' && b[10] == 'B' && b[11] == 'P' {
		return true
	}
	return false
}
