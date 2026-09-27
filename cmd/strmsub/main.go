// StrmSub v2：目录扫描 + 正则标题识别 + 聚合字幕搜索的 STRM 字幕工具。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/marsjimmy/strmsub/internal/config"
	"github.com/marsjimmy/strmsub/internal/flare"
	"github.com/marsjimmy/strmsub/internal/library"
	"github.com/marsjimmy/strmsub/internal/pipeline"
	"github.com/marsjimmy/strmsub/internal/scheduler"
	"github.com/marsjimmy/strmsub/internal/search"
	"github.com/marsjimmy/strmsub/internal/store"
	"github.com/marsjimmy/strmsub/internal/subsource"
	"github.com/marsjimmy/strmsub/internal/subsource/assrt"
	"github.com/marsjimmy/strmsub/internal/subsource/opensubtitles"
	"github.com/marsjimmy/strmsub/internal/subsource/subdl"
	"github.com/marsjimmy/strmsub/internal/subsource/subhd"
	"github.com/marsjimmy/strmsub/internal/subsource/subtitlecat"
	"github.com/marsjimmy/strmsub/internal/subsource/xunlei"
	"github.com/marsjimmy/strmsub/internal/web"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	cfg := config.Load()

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatal(err)
	}
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("打不开数据库(%s): %v —— 检查该目录对容器用户是否可写", cfg.DataDir, err)
	}
	defer st.Close()

	// 首次启动用环境变量播种设置
	st.SeedFromEnv(cfg)

	fl := flare.New(st.FlareSolverrURL())
	sources := buildSources(st, fl)
	for _, s := range sources {
		log.Printf("[source] %s ready=%v toggle=%v", s.Name(), s.Enabled(), st.SourceEnabled(s.Name(), true))
	}

	scanner := library.New(cfg.MediaDirs, st)
	searcher := search.New(st, sources)
	pipe := pipeline.New(cfg, st, scanner, searcher)
	sched := scheduler.New(st.ScanInterval(), pipe)

	// 设置页保存后的热重载
	rebuild := func() {
		searcher.SetSources(buildSources(st, flare.New(st.FlareSolverrURL())))
		sched.SetInterval(st.ScanInterval())
		log.Printf("[config] 热重载完成")
	}

	// 单次扫描模式：strmsub scan
	if len(os.Args) > 1 && os.Args[1] == "scan" {
		pipe.RunOnce(context.Background())
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	go sched.Start(ctx)

	srv := &http.Server{Addr: cfg.Addr, Handler: web.New(cfg, st, pipe, sched, rebuild).Handler()}
	log.Printf("[strmsub] 启动，监听 %s，Web http://<host>%s", cfg.Addr, cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// buildSources 组装字幕源：5 个影视源 + 1 个成人源；凭证从 SQLite 读
func buildSources(st *store.Store, fl *flare.Client) []subsource.Source {
	return []subsource.Source{
		assrt.New(st.Cred(store.KCredAssrt)),
		opensubtitles.New(st.Cred(store.KCredOSKey), st.Cred(store.KCredOSUser), st.Cred(store.KCredOSPass)),
		subdl.New(st.Cred(store.KCredSubDL)),
		subhd.New(),         // 免 key
		xunlei.New(),        // 免 key，按视频 CID 查询
		subtitlecat.New(fl), // 成人源，仅番号查询
	}
}
