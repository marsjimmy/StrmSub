// 设置的类型化访问：全部落在 kv 表；环境变量仅作首次启动的种子。
package store

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/marsjimmy/strmsub/internal/config"
)

const (
	KAPIToken      = "api_token"
	KScanInterval  = "scan_interval_min"
	KTargetLang    = "target_lang"
	KSubtitleDir   = "subtitle_dir"
	KFlareSolverr  = "flaresolverr_url"
	KCredAssrt     = "cred_assrt_token"
	KCredOSKey     = "cred_os_api_key"
	KCredOSUser    = "cred_os_user"
	KCredOSPass    = "cred_os_pass"
	KCredSubDL     = "cred_subdl_key"
	sourceEnPrefix = "source_enabled_"
)

// SeedFromEnv 首次启动时用环境变量播种（已有值不覆盖）
func (s *Store) SeedFromEnv(cfg *config.Config) {
	seed := map[string]string{
		KScanInterval: strconv.Itoa(int(cfg.ScanInterval / time.Minute)),
		KTargetLang:   cfg.TargetLang,
		KSubtitleDir:  cfg.SubDir,
		KFlareSolverr: cfg.FlareSolverrURL,
		KCredAssrt:    cfg.AssrtToken,
		KCredOSKey:    cfg.OSApiKey,
		KCredOSUser:   cfg.OSUser,
		KCredOSPass:   cfg.OSPass,
		KCredSubDL:    cfg.SubDLKey,
	}
	for k, v := range seed {
		if s.GetKV(k, "") == "" && v != "" {
			_ = s.SetKV(k, v)
		}
	}
	if s.GetKV(KAPIToken, "") == "" {
		_ = s.SetKV(KAPIToken, newAPIToken())
	}
}

func newAPIToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// APIToken 取自动令牌
func (s *Store) APIToken() string { return s.GetKV(KAPIToken, "") }

// RegenerateAPIToken 重新生成
func (s *Store) RegenerateAPIToken() string {
	t := newAPIToken()
	_ = s.SetKV(KAPIToken, t)
	return t
}

func (s *Store) ScanInterval() time.Duration {
	if n, err := strconv.Atoi(s.GetKV(KScanInterval, "30")); err == nil && n >= 5 {
		return time.Duration(n) * time.Minute
	}
	return 30 * time.Minute
}

func (s *Store) TargetLang() string {
	if v := s.GetKV(KTargetLang, ""); v != "" {
		return v
	}
	return "zh-Hans"
}

func (s *Store) SubtitleDir() string     { return s.GetKV(KSubtitleDir, "") }
func (s *Store) FlareSolverrURL() string { return s.GetKV(KFlareSolverr, "") }

// SourceEnabled 字幕源开关；def 为该源的默认状态
func (s *Store) SourceEnabled(name string, def bool) bool {
	v := s.GetKV(sourceEnPrefix+name, "")
	if v == "" {
		return def
	}
	return v == "1"
}

func (s *Store) SetSourceEnabled(name string, enabled bool) error {
	v := "0"
	if enabled {
		v = "1"
	}
	return s.SetKV(sourceEnPrefix+name, v)
}

// 凭证读写
func (s *Store) Cred(key string) string      { return s.GetKV(key, "") }
func (s *Store) SetCred(key, v string) error { return s.SetKV(key, v) }
