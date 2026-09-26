// Package xunlei 对接迅雷字幕库（http://sub.xmp.sandai.net:8000/subxl/{cid}.json）。
// 无需 key。按视频文件特征 CID（SHA1 取三段 0x5000 字节）查询。
// .strm 指向的通常是 http(s) 远程视频，用 HTTP Range 只取三段做哈希，
// 不下载整个文件；非 http(s) 或不支持 Range 的源自动跳过。
package xunlei

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/subsource"
)

const (
	rootURL   = "http://sub.xmp.sandai.net:8000/subxl/%s.json"
	chunkSize = 0x5000
	minSize   = 0xF000
)

type Client struct{ http *http.Client }

func New() *Client {
	return &Client{http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Name() string  { return "xunlei" }
func (c *Client) Enabled() bool { return true } // 免 key，恒启用

type subEntry struct {
	Scid     string `json:"scid"`
	Sname    string `json:"sname"`
	Language string `json:"language"`
	Rate     string `json:"rate"`
	Surl     string `json:"surl"`
	Svote    int64  `json:"svote"`
	Roffset  int64  `json:"roffset"`
}

type subResp struct {
	Sublist []subEntry `json:"sublist"`
}

func (c *Client) Search(ctx context.Context, m metadata.MediaInfo) ([]subsource.Candidate, error) {
	if m.FilePath == "" {
		return nil, nil
	}
	target, ok := strmTarget(m.FilePath)
	if !ok {
		return nil, nil
	}
	cid, err := c.cidForURL(ctx, target)
	if err != nil {
		log.Printf("[xunlei] 取 CID 失败 %s: %v", m.Title, err)
		return nil, nil // 取不到 CID 就跳过，不阻塞其他源
	}
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf(rootURL, cid), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("xunlei http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var sr subResp
	// 迅雷接口偶发坏字节（非合法 UTF-8），先清洗再解析，避免整个文档解析失败
	clean := strings.ToValidUTF8(string(body), "")
	if err := json.Unmarshal([]byte(clean), &sr); err != nil {
		return nil, err
	}
	var out []subsource.Candidate
	for _, s := range sr.Sublist {
		if s.Scid == "" || s.Surl == "" {
			continue
		}
		lang := normLang(s.Language)
		if lang == subsource.LangOther {
			lang = guessLangFromName(s.Sname) // language 缺失/未知时从文件名兜底
		}
		if lang == subsource.LangOther {
			continue // 只要中文相关
		}
		surl := s.Surl
		out = append(out, subsource.Candidate{
			Source: "xunlei",
			RefID:  s.Scid,
			Name:   s.Sname,
			Lang:   lang,
			Format: formatOf(s.Sname),
			Votes:  int(s.Svote),
			Detail: fmt.Sprintf("迅雷字幕库 · %s", s.Language),
			Download: func(ctx context.Context) (string, []byte, error) {
				return c.download(ctx, surl)
			},
		})
	}
	return out, nil
}

// normLang 映射迅雷的 language 字段："简体&英语"→双语，"简体"→简体，"繁体"→繁体，"未知语言"→other
func normLang(s string) string {
	has := func(sub string) bool { return strings.Contains(s, sub) }
	if has("简") && has("繁") {
		return subsource.LangBilingual
	}
	if has("简") && (has("英语") || has("英文") || has("双语")) {
		return subsource.LangBilingual
	}
	if has("繁") && (has("英语") || has("英文") || has("双语")) {
		return subsource.LangBilingual
	}
	if has("简") {
		return subsource.LangHans
	}
	if has("繁") {
		return subsource.LangHant
	}
	return subsource.LangOther
}

// guessLangFromName 从文件名猜语言：chs/简/中→简体，cht/繁→繁体
func guessLangFromName(name string) string {
	lower := strings.ToLower(name)
	has := func(s string) bool { return strings.Contains(lower, s) }
	if has("chs") || has("简") || has("中") {
		return subsource.LangHans
	}
	if has("cht") || has("繁") {
		return subsource.LangHant
	}
	return subsource.LangOther
}

func formatOf(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".ass") || strings.HasSuffix(lower, ".ssa"):
		return "ass"
	default:
		return "srt"
	}
}

func (c *Client) download(ctx context.Context, surl string) (string, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", surl, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("xunlei 下载 http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", nil, err
	}
	name := resp.Request.URL.Path
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		name = "subtitle.srt"
	}
	return name, data, nil
}

// strmTarget 读 .strm 文件取第一行非空 URL
func strmTarget(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 64*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		// 去掉 BOM
		line = strings.TrimPrefix(line, "\xef\xbb\xbf")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			return line, true
		}
		return "", false
	}
	return "", false
}

// cidForURL 用 Range 请求取视频三段内容算 CID，不下载整个文件
func (c *Client) cidForURL(ctx context.Context, target string) (string, error) {
	size, err := c.fileSize(ctx, target)
	if err != nil {
		return "", err
	}
	if size < minSize {
		return "", fmt.Errorf("文件太小 %d", size)
	}
	positions := []int64{0, size / 3, size - chunkSize}
	h := sha1.New()
	for _, pos := range positions {
		chunk, err := c.getRange(ctx, target, pos, chunkSize)
		if err != nil {
			return "", err
		}
		h.Write(chunk)
	}
	return fmt.Sprintf("%X", h.Sum(nil)), nil
}

// fileSize 先 HEAD 取 Content-Length，拿不到则用 Range: bytes=0-0 解析 Content-Range
func (c *Client) fileSize(ctx context.Context, target string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", target, nil)
	if err != nil {
		return 0, err
	}
	if resp, err := c.http.Do(req); err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			if n, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err == nil && n > 0 {
				return n, nil
			}
		}
	}
	// 回退：Range 0-0
	req2, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return 0, err
	}
	req2.Header.Set("Range", "bytes=0-0")
	resp2, err := c.http.Do(req2)
	if err != nil {
		return 0, err
	}
	defer resp2.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp2.Body, 64*1024))
	if cr := resp2.Header.Get("Content-Range"); cr != "" {
		// bytes 0-0/123456
		if i := strings.LastIndex(cr, "/"); i >= 0 {
			if n, err := strconv.ParseInt(strings.TrimSpace(cr[i+1:]), 10, 64); err == nil && n > 0 {
				return n, nil
			}
		}
	}
	return 0, fmt.Errorf("取不到文件长度")
}

// getRange 取 [pos, pos+n) 字节，要求服务端支持 Range（206）
func (c *Client) getRange(ctx context.Context, target string, pos, n int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", pos, pos+n-1))
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("服务端不支持 Range（http %d）", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, n))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) < n {
		return nil, fmt.Errorf("Range 数据不足")
	}
	return data, nil
}
