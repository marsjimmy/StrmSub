// Package config 从环境变量加载全部配置，单二进制部署时无需配置文件。
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
	// 本地数据目录（自用 sqlite、日志）
	DataDir string

	// 飞牛影视元数据库（容器内只读挂载 trimmedia.db）
	FnosDBPath string
	// 飞牛影视 HTTP API（Emby 式接入）：配了 URL+账号就优先走 API，不再需要挂载 db
	FnosAPIURL  string
	FnosAPIUser string
	FnosAPIPass string
	// 路径映射：飞牛侧绝对路径 -> 本机/容器内路径，逗号分隔，形如 /vol1/media:/media
	// API 返回的是飞牛宿主机视角的路径，映射后才能定位 .strm
	PathMap map[string]string
	// 媒体目录，逗号分隔；字幕会写到 .strm 旁边，需要写权限
	MediaDirs []string

	// 扫描间隔
	ScanInterval time.Duration
	// 目标语言：zh-Hans / zh-Hant
	TargetLang string

	// 字幕源凭证
	AssrtToken string
	OSApiKey   string // OpenSubtitles api key
	OSUser     string // OpenSubtitles 用户名（下载需要）
	OSPass     string // OpenSubtitles 密码（下载需要）
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
		Addr:         getenv("STRMSUB_ADDR", ":8099"),
		DataDir:      getenv("STRMSUB_DATA_DIR", "./data"),
		FnosDBPath:   getenv("STRMSUB_FNOS_DB", "/data/trimmedia.db"),
		FnosAPIURL:   strings.TrimRight(getenv("STRMSUB_FNOS_API_URL", ""), "/"),
		FnosAPIUser:  os.Getenv("STRMSUB_FNOS_API_USER"),
		FnosAPIPass:  os.Getenv("STRMSUB_FNOS_API_PASS"),
		PathMap:      ParsePathMap(os.Getenv("STRMSUB_PATH_MAP")),
		MediaDirs:    dirs,
		ScanInterval: interval,
		TargetLang:   getenv("STRMSUB_TARGET_LANG", "zh-Hans"),
		AssrtToken:   os.Getenv("STRMSUB_ASSRT_TOKEN"),
		OSApiKey:     os.Getenv("STRMSUB_OS_API_KEY"),
		OSUser:       os.Getenv("STRMSUB_OS_USER"),
		OSPass:       os.Getenv("STRMSUB_OS_PASS"),
		SubDLKey:     os.Getenv("STRMSUB_SUBDL_KEY"),
	}
}

// ParsePathMap 解析 "/vol1/media:/media,/vol2/tv:/tv" 为映射表
func ParsePathMap(s string) map[string]string {
	m := map[string]string{}
	for _, kv := range strings.Split(s, ",") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, ":", 2)
		if len(parts) != 2 {
			continue
		}
		from := strings.TrimSpace(parts[0])
		to := strings.TrimSpace(parts[1])
		if from != "" && to != "" {
			m[from] = to
		}
	}
	return m
}

// MapPath 把飞牛侧路径映射为本机路径；无匹配规则时原样返回
func (c *Config) MapPath(p string) string {
	return MapPathWith(c.PathMap, p)
}

// MapPathWith 用给定的映射表做路径映射（最长前缀优先）
func MapPathWith(m map[string]string, p string) string {
	best := ""
	for from := range m {
		if p == from || strings.HasPrefix(p, from+"/") {
			if len(from) > len(best) {
				best = from
			}
		}
	}
	if best != "" {
		return m[best] + p[len(best):]
	}
	return p
}

// FnosAPIEnabled 是否配置了飞牛 API 接入
func (c *Config) FnosAPIEnabled() bool {
	return c.FnosAPIURL != "" && c.FnosAPIUser != ""
}
