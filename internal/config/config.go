// Package config 从环境变量加载基础配置；运行时设置统一持久化在 SQLite（store kv 表），
// 环境变量仅作为首次启动的默认值。
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// HTTP 服务监听地址
	Addr string
	// 本地数据目录（sqlite、封面缓存等）
	DataDir string
	// 媒体目录，逗号分隔；递归扫描其中的视频文件
	MediaDirs []string
	// 字幕保存目录：为空则保存到视频文件旁边
	SubDir string
	// FlareSolverr 地址（可选），用于过 Cloudflare 的字幕站
	FlareSolverrURL string

	// 扫描间隔
	ScanInterval time.Duration
	// 目标语言：zh-Hans / zh-Hant
	TargetLang string

	// 字幕源凭证（首次启动写入 SQLite，之后以 SQLite 为准）
	AssrtToken string
	OSApiKey   string
	OSUser     string
	OSPass     string
	SubDLKey   string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() *Config {
	dirs := []string{}
	for _, d := range strings.Split(getenv("STRMSUB_MEDIA_DIRS", "/media"), ",") {
		if d = strings.TrimSpace(d); d != "" {
			dirs = append(dirs, d)
		}
	}
	interval := 30 * time.Minute
	if v := os.Getenv("STRMSUB_SCAN_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		} else if n, err := strconv.Atoi(v); err == nil {
			interval = time.Duration(n) * time.Minute
		}
	}
	return &Config{
		Addr:            getenv("STRMSUB_ADDR", ":8099"),
		DataDir:         getenv("STRMSUB_DATA_DIR", "./data"),
		MediaDirs:       dirs,
		SubDir:          os.Getenv("STRMSUB_SUB_DIR"),
		FlareSolverrURL: os.Getenv("STRMSUB_FLARESOLVERR_URL"),
		ScanInterval:    interval,
		TargetLang:      getenv("STRMSUB_TARGET_LANG", "zh-Hans"),
		AssrtToken:      os.Getenv("STRMSUB_ASSRT_TOKEN"),
		OSApiKey:        os.Getenv("STRMSUB_OS_API_KEY"),
		OSUser:          os.Getenv("STRMSUB_OS_USER"),
		OSPass:          os.Getenv("STRMSUB_OS_PASS"),
		SubDLKey:        os.Getenv("STRMSUB_SUBDL_KEY"),
	}
}
