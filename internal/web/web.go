// Package web 提供 HTTP API 与内置 Web 界面（侧边栏 + 主页/媒体/字幕/设置）。
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/marsjimmy/strmsub/internal/config"
	"github.com/marsjimmy/strmsub/internal/pipeline"
	"github.com/marsjimmy/strmsub/internal/scheduler"
	"github.com/marsjimmy/strmsub/internal/store"
	"github.com/marsjimmy/strmsub/internal/title"
)

type Server struct {
	cfg     *config.Config
	store   *store.Store
	pipe    *pipeline.Pipeline
	sched   *scheduler.Scheduler
	rebuild func()
}

func New(cfg *config.Config, st *store.Store, pipe *pipeline.Pipeline, sched *scheduler.Scheduler, rebuild func()) *Server {
	return &Server{cfg: cfg, store: st, pipe: pipe, sched: sched, rebuild: rebuild}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	// API 全部走令牌鉴权
	api := func(pattern string, h http.HandlerFunc) {
		mux.HandleFunc(pattern, s.requireToken(h))
	}
	api("GET /api/status", s.handleStatus)
	api("POST /api/scan", s.handleScan)
	api("GET /api/media", s.handleMediaList)
	api("GET /api/media/{id}", s.handleMediaOne)
	api("GET /api/cover", s.handleCover)
	api("POST /api/search", s.handleSearch)
	api("POST /api/search-media", s.handleSearchMedia)
	api("POST /api/download", s.handleDownload)
	api("POST /api/download-best", s.handleDownloadBest)
	api("GET /api/history", s.handleHistory)
	api("GET /api/settings", s.handleGetSettings)
	api("POST /api/settings", s.handleSaveSettings)
	api("GET /api/rules", s.handleListRules)
	api("POST /api/rules", s.handleAddRule)
	api("PUT /api/rules/{id}", s.handleUpdateRule)
	api("DELETE /api/rules/{id}", s.handleDeleteRule)
	api("POST /api/rules/test", s.handleTestRule)
	api("POST /api/token/regenerate", s.handleRegenToken)
	return mux
}

// requireToken API 令牌鉴权：X-API-Token 头或 ?token= 参数
func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		want := s.store.APIToken()
		got := r.Header.Get("X-API-Token")
		if got == "" {
			got = r.URL.Query().Get("token")
		}
		if want == "" || got != want {
			http.Error(w, "未授权（API 令牌无效）", 401)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := strings.ReplaceAll(dashboardHTML, "__API_TOKEN__", s.store.APIToken())
	w.Write([]byte(html))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	total, withSub, missing := s.store.MediaStats()
	lastScan, scanRes := s.pipe.LastScan()
	writeJSON(w, map[string]any{
		"mediaTotal": total, "withSub": withSub, "missing": missing,
		"sources":      s.pipe.Searcher().SourceStatus(),
		"lastScan":     lastScan,
		"lastAdded":    scanRes.Added,
		"lastUpdated":  scanRes.Updated,
		"lastRemoved":  scanRes.Removed,
		"targetLang":   s.store.TargetLang(),
		"scanInterval": s.store.ScanInterval().String(),
		"autoDownload": s.store.AutoDownload(),
		"mediaDirs":    s.cfg.MediaDirs,
		"subtitleDir":  s.store.SubtitleDir(),
	})
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	s.sched.Trigger()
	writeJSON(w, map[string]string{"status": "triggered"})
}

type mediaJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
	Season    int    `json:"season"`
	Episode   int    `json:"episode"`
	FilePath  string `json:"file_path"`
	HasCover  bool   `json:"has_cover"`
	SubStatus string `json:"sub_status"`
	SubSource string `json:"sub_source"`
}

func toMediaJSON(e store.MediaEntry) mediaJSON {
	return mediaJSON{
		ID: e.ID, Title: e.Title, Year: e.Year, Season: e.Season, Episode: e.Episode,
		FilePath: e.FilePath, HasCover: e.CoverPath != "",
		SubStatus: e.SubStatus, SubSource: e.SubSource,
	}
}

func (s *Server) handleMediaList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 200
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 1000 {
		limit = n
	}
	list, err := s.store.ListMedia(q, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	out := make([]mediaJSON, 0, len(list))
	for _, e := range list {
		out = append(out, toMediaJSON(e))
	}
	writeJSON(w, out)
}

func (s *Server) handleMediaOne(w http.ResponseWriter, r *http.Request) {
	e, err := s.store.GetMediaEntry(r.PathValue("id"))
	if err != nil || e == nil {
		http.Error(w, "找不到该媒体", 404)
		return
	}
	writeJSON(w, toMediaJSON(*e))
}

func (s *Server) handleCover(w http.ResponseWriter, r *http.Request) {
	e, err := s.store.GetMediaEntry(r.URL.Query().Get("id"))
	if err != nil || e == nil || e.CoverPath == "" {
		http.NotFound(w, r)
		return
	}
	fi, err := os.Stat(e.CoverPath)
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}
	// 路径穿越防护：封面必须在媒体目录下（扫描时写入，可信，但仍校验）
	abs, _ := filepath.Abs(e.CoverPath)
	ok := false
	for _, d := range s.cfg.MediaDirs {
		ad, _ := filepath.Abs(d)
		if abs == ad || strings.HasPrefix(abs, ad+string(os.PathSeparator)) {
			ok = true
			break
		}
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	default:
		w.Header().Set("Content-Type", "image/jpeg")
	}
	http.ServeFile(w, r, abs)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var f struct {
		Keyword string `json:"keyword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || strings.TrimSpace(f.Keyword) == "" {
		http.Error(w, "缺少关键词", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	writeJSON(w, s.pipe.SearchKeyword(ctx, strings.TrimSpace(f.Keyword)))
}

func (s *Server) handleSearchMedia(w http.ResponseWriter, r *http.Request) {
	var f struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || f.ID == "" {
		http.Error(w, "缺少媒体 ID", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	res, e, err := s.pipe.SearchMedia(ctx, f.ID)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "detail": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "media": toMediaJSON(e), "result": res})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	var f struct {
		Source  string `json:"source"`
		RefID   string `json:"ref_id"`
		MediaID string `json:"media_id"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || f.Source == "" || f.RefID == "" {
		http.Error(w, "缺少参数", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	saved, err := s.pipe.Download(ctx, f.Source, f.RefID, f.MediaID, f.Name)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "detail": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "save_path": saved})
}

func (s *Server) handleDownloadBest(w http.ResponseWriter, r *http.Request) {
	var f struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || f.ID == "" {
		http.Error(w, "缺少媒体 ID", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	saved, err := s.pipe.DownloadBest(ctx, f.ID)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "detail": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "save_path": saved})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 500 {
		limit = n
	}
	list, err := s.store.ListDownloadHistory(limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, list)
}

// ---------- 设置 ----------

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	srcs := s.pipe.Searcher().SourceStatus()
	toggles := map[string]bool{}
	for _, sc := range srcs {
		if name, ok := sc["name"].(string); ok {
			toggles[name] = s.store.SourceEnabled(name, true)
		}
	}
	writeJSON(w, map[string]any{
		"sources":      srcs,
		"toggles":      toggles,
		"assrtToken":   s.store.Cred(store.KCredAssrt),
		"osApiKey":     s.store.Cred(store.KCredOSKey),
		"osUser":       s.store.Cred(store.KCredOSUser),
		"hasOsPass":    s.store.Cred(store.KCredOSPass) != "",
		"subdlKey":     s.store.Cred(store.KCredSubDL),
		"scanInterval": int(s.store.ScanInterval() / time.Minute),
		"targetLang":   s.store.TargetLang(),
		"autoDownload": s.store.AutoDownload(),
		"subtitleDir":  s.store.SubtitleDir(),
		"flaresolverr": s.store.FlareSolverrURL(),
		"apiToken":     s.store.APIToken(),
		"mediaDirs":    s.cfg.MediaDirs,
	})
}

type settingsForm struct {
	Toggles map[string]bool `json:"toggles"`

	AssrtToken string `json:"assrtToken"`
	OSAPIKey   string `json:"osApiKey"`
	OSUser     string `json:"osUser"`
	OSPass     string `json:"osPass"` // 空=保持原密码
	SubDLKey   string `json:"subdlKey"`

	ScanInterval int    `json:"scanInterval"`
	TargetLang   string `json:"targetLang"`
	AutoDownload *bool  `json:"autoDownload"`
	SubtitleDir  string `json:"subtitleDir"`
	FlareSolverr string `json:"flaresolverr"`
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var f settingsForm
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		http.Error(w, "请求解析失败", 400)
		return
	}
	for name, en := range f.Toggles {
		_ = s.store.SetSourceEnabled(name, en)
	}
	_ = s.store.SetCred(store.KCredAssrt, strings.TrimSpace(f.AssrtToken))
	_ = s.store.SetCred(store.KCredOSKey, strings.TrimSpace(f.OSAPIKey))
	_ = s.store.SetCred(store.KCredOSUser, strings.TrimSpace(f.OSUser))
	if f.OSPass != "" {
		_ = s.store.SetCred(store.KCredOSPass, f.OSPass)
	}
	_ = s.store.SetCred(store.KCredSubDL, strings.TrimSpace(f.SubDLKey))
	if f.ScanInterval >= 5 {
		_ = s.store.SetKV(store.KScanInterval, strconv.Itoa(f.ScanInterval))
	}
	if f.TargetLang != "" {
		_ = s.store.SetKV(store.KTargetLang, f.TargetLang)
	}
	if f.AutoDownload != nil {
		_ = s.store.SetAutoDownload(*f.AutoDownload)
	}
	_ = s.store.SetKV(store.KSubtitleDir, strings.TrimSpace(f.SubtitleDir))
	_ = s.store.SetKV(store.KFlareSolverr, strings.TrimSpace(f.FlareSolverr))
	s.rebuild()
	writeJSON(w, map[string]any{"ok": true})
}

// ---------- 标题正则 ----------

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.ListTitleRules()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"rules": rules, "defaults": title.DefaultRuleHint()})
}

func (s *Server) handleAddRule(w http.ResponseWriter, r *http.Request) {
	var f struct {
		Name    string `json:"name"`
		Pattern string `json:"pattern"`
	}
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || strings.TrimSpace(f.Pattern) == "" {
		http.Error(w, "缺少正则", 400)
		return
	}
	if _, err := title.Test(f.Pattern, "test.mkv"); err != nil {
		if se, ok := err.(interface{ Error() string }); ok && se.Error() != "正则未匹配到标题" {
			http.Error(w, "正则非法: "+err.Error(), 400)
			return
		}
	}
	id, err := s.store.AddTitleRule(strings.TrimSpace(f.Name), strings.TrimSpace(f.Pattern), 0)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "ID 非法", 400)
		return
	}
	var f struct {
		Name    string `json:"name"`
		Pattern string `json:"pattern"`
		Ord     int    `json:"ord"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || strings.TrimSpace(f.Pattern) == "" {
		http.Error(w, "缺少正则", 400)
		return
	}
	if err := s.store.UpdateTitleRule(id, strings.TrimSpace(f.Name), strings.TrimSpace(f.Pattern), f.Ord, f.Enabled); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "ID 非法", 400)
		return
	}
	if err := s.store.DeleteTitleRule(id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleTestRule(w http.ResponseWriter, r *http.Request) {
	var f struct {
		Pattern  string `json:"pattern"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || f.Pattern == "" || f.Filename == "" {
		http.Error(w, "缺少参数", 400)
		return
	}
	res, err := title.Test(f.Pattern, f.Filename)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "detail": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "title": res.Title, "year": res.Year,
		"season": res.Season, "episode": res.Episode})
}

func (s *Server) handleRegenToken(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true, "token": s.store.RegenerateAPIToken()})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
