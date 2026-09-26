// Package web 提供 HTTP API 与内置仪表盘。
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/marsjimmy/strmsub/internal/config"
	"github.com/marsjimmy/strmsub/internal/metadata/fnosapi"
	"github.com/marsjimmy/strmsub/internal/pipeline"
	"github.com/marsjimmy/strmsub/internal/scheduler"
	"github.com/marsjimmy/strmsub/internal/store"
)

type Server struct {
	store   *store.Store
	pipe    *pipeline.Pipeline
	sched   *scheduler.Scheduler
	stg     *config.Settings
	rebuild func() // 设置变更后热重载 provider
}

func New(st *store.Store, pipe *pipeline.Pipeline, sched *scheduler.Scheduler, stg *config.Settings, rebuild func()) *Server {
	return &Server{store: st, pipe: pipe, sched: sched, stg: stg, rebuild: rebuild}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/media", s.handleMedia)
	mux.HandleFunc("POST /api/scan", s.handleScan)
	mux.HandleFunc("POST /api/search-one", s.handleSearchOne)
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("POST /api/settings", s.handleSaveSettings)
	mux.HandleFunc("POST /api/fnos/test", s.handleTestFnos)
	mux.HandleFunc("GET /", s.handleIndex)
	return mux
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	res := s.pipe.LastResult()
	writeJSON(w, map[string]any{
		"lastScan":     res.At,
		"total":        res.Total,
		"downloaded":   res.Downloaded,
		"skipped":      res.Skipped,
		"missing":      res.Missing,
		"failed":       res.Failed,
		"providers":    s.pipe.ProviderNames(),
		"sources":      s.pipe.SourceStatus(),
		"targetLang":   s.stg.LangEffective(),
		"scanInterval": s.stg.IntervalEffective().String(),
	})
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListMediaWithStatus()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, list)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	s.sched.Trigger()
	writeJSON(w, map[string]string{"status": "triggered"})
}

// handleSearchOne 单独为一部媒体搜索字幕
func (s *Server) handleSearchOne(w http.ResponseWriter, r *http.Request) {
	var f struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || f.ID == "" {
		http.Error(w, "缺少媒体 ID", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	result, err := s.pipe.SearchOne(ctx, f.ID)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "detail": err.Error()})
		return
	}
	msgs := map[string]string{
		pipeline.SearchHasSub:     "已有字幕，跳过",
		pipeline.SearchDownloaded: "已找到并下载字幕 ✓",
		pipeline.SearchMissing:    "无合适字幕",
		pipeline.SearchFailed:     "搜索/下载失败，请看容器日志",
		pipeline.SearchSkipped:    "找不到对应的 .strm 文件，跳过",
	}
	msg, ok := msgs[result]
	if !ok {
		msg = result
	}
	writeJSON(w, map[string]any{"ok": true, "result": result, "message": msg})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

// handleGetSettings 返回全部设置（密码永不返回，只给 hasPass）
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	f := s.stg.FnosAPI
	sub := s.stg.Sub
	writeJSON(w, map[string]any{
		"enabled":  f.Enabled,
		"url":      f.URL,
		"user":     f.User,
		"hasPass":  f.Pass != "",
		"pathMap":  f.PathMap,
		"assrtToken":      sub.AssrtToken,
		"osApiKey":        sub.OSAPIKey,
		"osUser":          sub.OSUser,
		"hasOsPass":       sub.OSPass != "",
		"subdlKey":        sub.SubDLKey,
		"scanIntervalMin": s.stg.ScanIntervalMinutes,
		"targetLang":      s.stg.LangEffective(),
	})
}

type settingsForm struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
	User    string `json:"user"`
	Pass    string `json:"pass"` // 空=保持原密码
	PathMap string `json:"pathMap"`

	AssrtToken string `json:"assrtToken"`
	OSAPIKey   string `json:"osApiKey"`
	OSUser     string `json:"osUser"`
	OSPass     string `json:"osPass"` // 空=保持原密码
	SubDLKey   string `json:"subdlKey"`

	ScanIntervalMinutes int    `json:"scanIntervalMinutes"`
	TargetLang          string `json:"targetLang"`
}

// handleSaveSettings 保存设置并热重载 provider + 字幕源
func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var f settingsForm
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		http.Error(w, "请求解析失败", 400)
		return
	}
	pass := s.stg.FnosAPI.Pass
	if f.Pass != "" {
		pass = f.Pass
	}
	s.stg.FnosAPI = config.FnosAPISettings{
		Enabled: f.Enabled,
		URL:     strings.TrimRight(strings.TrimSpace(f.URL), "/"),
		User:    strings.TrimSpace(f.User),
		Pass:    pass,
		PathMap: strings.TrimSpace(f.PathMap),
	}
	osPass := s.stg.Sub.OSPass
	if f.OSPass != "" {
		osPass = f.OSPass
	}
	s.stg.Sub = config.SubSourceSettings{
		AssrtToken: strings.TrimSpace(f.AssrtToken),
		OSAPIKey:   strings.TrimSpace(f.OSAPIKey),
		OSUser:     strings.TrimSpace(f.OSUser),
		OSPass:     osPass,
		SubDLKey:   strings.TrimSpace(f.SubDLKey),
	}
	if f.ScanIntervalMinutes > 0 {
		s.stg.ScanIntervalMinutes = f.ScanIntervalMinutes
	}
	if f.TargetLang != "" {
		s.stg.TargetLang = f.TargetLang
	}
	if err := s.stg.Save(); err != nil {
		http.Error(w, "保存失败: "+err.Error(), 500)
		return
	}
	s.rebuild()
	writeJSON(w, map[string]any{"ok": true})
}

type testForm struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
	User    string `json:"user"`
	Pass    string `json:"pass"`
	PathMap string `json:"pathMap"`
}

// handleTestFnos 检测飞牛服务：登录 + 拉一页条目，返回诊断摘要。
// 检测成功后自动保存表单里的连接设置并热重载（免得"检测通了但没生效"）。
func (s *Server) handleTestFnos(w http.ResponseWriter, r *http.Request) {
	var f testForm
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		http.Error(w, "请求解析失败", 400)
		return
	}
	url := strings.TrimRight(strings.TrimSpace(f.URL), "/")
	user := strings.TrimSpace(f.User)
	pass := f.Pass
	if pass == "" && url == s.stg.FnosAPI.URL && user == s.stg.FnosAPI.User {
		pass = s.stg.FnosAPI.Pass // 用已保存的密码
	}
	if url == "" || user == "" {
		writeJSON(w, map[string]any{"ok": false, "detail": "请填写服务器 URL 和用户名"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	// 用 Probe 拿诊断（含样本 JSON），截断后返回
	prov := fnosapi.New(url, user, pass, func(s string) string { return s })
	out, err := prov.Probe(ctx)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "detail": "连接失败: " + err.Error()})
		return
	}
	// 检测成功：自动保存并热重载
	s.stg.FnosAPI = config.FnosAPISettings{
		Enabled: f.Enabled,
		URL:     url,
		User:    user,
		Pass:    pass,
		PathMap: strings.TrimSpace(f.PathMap),
	}
	saved := ""
	if err := s.stg.Save(); err != nil {
		saved = "（但自动保存失败，请手动点保存： " + err.Error() + "）"
	} else {
		s.rebuild()
		saved = "，已自动保存并启用"
	}
	// 只返回前 2000 字符的摘要，避免页面过长
	runes := []rune(out)
	if len(runes) > 2000 {
		out = string(runes[:2000]) + "\n…（已截断）"
	}
	writeJSON(w, map[string]any{"ok": true, "detail": out, "saved": saved})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
