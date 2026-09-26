package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// FnosAPISettings 飞牛影视 API 连接设置，可在 Web 设置页里修改，
// 保存在 DataDir/settings.json（0600，密码不进日志不进 API 返回）。
type FnosAPISettings struct {
	Enabled bool   `json:"enabled"`  // 是否开启
	URL     string `json:"url"`      // 飞牛影视服务地址，如 http://192.168.1.10:8005
	User    string `json:"user"`     // 飞牛用户名
	Pass    string `json:"pass"`     // 飞牛密码（保存时空=保持原值）
	PathMap string `json:"path_map"` // 逗号分隔，形如 /vol1/media:/media
}

// SubSourceSettings 字幕源凭证与参数，同样可在 Web 设置页里修改。
type SubSourceSettings struct {
	AssrtToken string `json:"assrt_token"` // 射手网 ASSRT token（https://assrt.net/usercp.php 免费获取）
	OSAPIKey   string `json:"os_api_key"`  // OpenSubtitles API Key
	OSUser     string `json:"os_user"`     // OpenSubtitles 用户名
	OSPass     string `json:"os_pass"`     // OpenSubtitles 密码（保存时空=保持原值）
	SubDLKey   string `json:"subdl_key"`   // SubDL API Key（预留）
}

type Settings struct {
	FnosAPI FnosAPISettings  `json:"fnos_api"`
	Sub     SubSourceSettings `json:"sub_source"`

	ScanIntervalMinutes int    `json:"scan_interval_minutes"` // 0 = 用默认值 30
	TargetLang          string `json:"target_lang"`           // 空 = zh-Hans

	dir string // settings.json 所在目录
}

func settingsPath(dir string) string { return filepath.Join(dir, "settings.json") }

// envMinutes 解析分钟数：支持 "30m"/"1h" 或纯数字（分钟）
func envMinutes(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return int(d / time.Minute)
		}
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// LoadSettings 读设置：先以环境变量为默认值，再用 settings.json 覆盖。
func LoadSettings(dir string) *Settings {
	s := &Settings{dir: dir}
	s.FnosAPI = FnosAPISettings{
		Enabled: os.Getenv("STRMSUB_FNOS_API_URL") != "",
		URL:     os.Getenv("STRMSUB_FNOS_API_URL"),
		User:    os.Getenv("STRMSUB_FNOS_API_USER"),
		Pass:    os.Getenv("STRMSUB_FNOS_API_PASS"),
		PathMap: os.Getenv("STRMSUB_PATH_MAP"),
	}
	s.Sub = SubSourceSettings{
		AssrtToken: os.Getenv("STRMSUB_ASSRT_TOKEN"),
		OSAPIKey:   os.Getenv("STRMSUB_OS_API_KEY"),
		OSUser:     os.Getenv("STRMSUB_OS_USER"),
		OSPass:     os.Getenv("STRMSUB_OS_PASS"),
		SubDLKey:   os.Getenv("STRMSUB_SUBDL_KEY"),
	}
	s.ScanIntervalMinutes = envMinutes("STRMSUB_SCAN_INTERVAL", 30)
	s.TargetLang = getenv("STRMSUB_TARGET_LANG", "zh-Hans")
	if raw, err := os.ReadFile(settingsPath(dir)); err == nil {
		var file Settings
		if json.Unmarshal(raw, &file) == nil {
			// 文件里有啥用啥（包括开关）
			*s = file
			s.dir = dir
		}
	}
	return s
}

// Save 持久化（0600）。调用方需先把 Pass 处理好：空表示保持原值。
func (s *Settings) Save() error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(s.dir), raw, 0600)
}

// Effective 生效的连接参数：开了开关且 URL+用户名齐了才返回 true
func (s *Settings) Effective() (url, user, pass string, ok bool) {
	f := s.FnosAPI
	if !f.Enabled || f.URL == "" || f.User == "" {
		return "", "", "", false
	}
	return f.URL, f.User, f.Pass, true
}

// IntervalEffective 生效的扫描间隔
func (s *Settings) IntervalEffective() time.Duration {
	if s.ScanIntervalMinutes > 0 {
		return time.Duration(s.ScanIntervalMinutes) * time.Minute
	}
	return 30 * time.Minute
}

// LangEffective 生效的目标语言
func (s *Settings) LangEffective() string {
	if s.TargetLang != "" {
		return s.TargetLang
	}
	return "zh-Hans"
}

// PathMapParsed 解析路径映射
func (s *Settings) PathMapParsed() map[string]string {
	return ParsePathMap(s.FnosAPI.PathMap)
}

// MapPath 飞牛侧路径 -> 本机路径
func (s *Settings) MapPath(p string) string {
	return MapPathWith(s.PathMapParsed(), p)
}
