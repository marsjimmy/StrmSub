package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s := LoadSettings(dir)
	if s.FnosAPI.Enabled {
		t.Fatalf("默认不应开启")
	}
	s.FnosAPI = FnosAPISettings{
		Enabled: true,
		URL:     "http://192.168.1.10:8005",
		User:    "admin",
		Pass:    "secret",
		PathMap: "/vol1/media:/media",
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	// 权限应为 0600
	fi, _ := os.Stat(filepath.Join(dir, "settings.json"))
	if fi.Mode().Perm() != 0600 {
		t.Errorf("settings.json 权限应为 0600，实际 %o", fi.Mode().Perm())
	}
	s2 := LoadSettings(dir)
	url, user, pass, ok := s2.Effective()
	if !ok || url != "http://192.168.1.10:8005" || user != "admin" || pass != "secret" {
		t.Errorf("重载后生效值不对: %q %q ok=%v", url, user, ok)
	}
	if got := s2.MapPath("/vol1/media/电影/a.strm"); got != "/media/电影/a.strm" {
		t.Errorf("路径映射不对: %q", got)
	}
	// 关闭开关后不生效
	s2.FnosAPI.Enabled = false
	if _, _, _, ok := s2.Effective(); ok {
		t.Errorf("关闭后仍生效")
	}
}
