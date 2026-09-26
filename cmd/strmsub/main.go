// StrmSub：基于已刮削元数据（飞牛影视 / NFO）的 STRM 字幕工具。
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/marsjimmy/strmsub/internal/config"
	"github.com/marsjimmy/strmsub/internal/metadata"
	"github.com/marsjimmy/strmsub/internal/metadata/fnos"
	"github.com/marsjimmy/strmsub/internal/metadata/fnosapi"
	"github.com/marsjimmy/strmsub/internal/metadata/nfo"
	"github.com/marsjimmy/strmsub/internal/pipeline"
	"github.com/marsjimmy/strmsub/internal/scheduler"
	"github.com/marsjimmy/strmsub/internal/store"
	"github.com/marsjimmy/strmsub/internal/subsource"
	"github.com/marsjimmy/strmsub/internal/subsource/assrt"
	"github.com/marsjimmy/strmsub/internal/subsource/opensubtitles"
	"github.com/marsjimmy/strmsub/internal/subsource/subdl"
	"github.com/marsjimmy/strmsub/internal/subsource/subhd"
	"github.com/marsjimmy/strmsub/internal/subsource/xunlei"
	"github.com/marsjimmy/strmsub/internal/web"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	cfg := config.Load()

	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		doctor(cfg)
		return
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatal(err)
	}
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("打不开数据库(%s): %v —— 检查该目录对容器用户是否可写", cfg.DataDir, err)
	}
	defer st.Close()

	stg := config.LoadSettings(cfg.DataDir)
	providers := buildProviders(cfg, stg)
	sources := buildSources(cfg, stg)

	pipe := pipeline.New(cfg, providers, sources, st)
	sched := scheduler.New(stg.IntervalEffective(), pipe)

	// 设置页保存后的热重载：按最新设置重建元数据源与字幕源，无需重启
	rebuild := func() {
		stg2 := config.LoadSettings(cfg.DataDir)
		pipe.SetProviders(buildProviders(cfg, stg2))
		pipe.SetSources(buildSources(cfg, stg2))
		cfg.TargetLang = stg2.LangEffective()
		sched.SetInterval(stg2.IntervalEffective())
		log.Printf("[config] 热重载完成: 扫描间隔=%v 目标语言=%s", stg2.IntervalEffective(), stg2.LangEffective())
	}

	// 单次扫描模式：strmsub scan
	if len(os.Args) > 1 && os.Args[1] == "scan" {
		pipe.RunOnce(context.Background())
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	go sched.Start(ctx)

	srv := &http.Server{Addr: cfg.Addr, Handler: web.New(st, pipe, sched, stg, cfg.DataDir, rebuild).Handler()}
	log.Printf("[strmsub] 启动，监听 %s，仪表盘 http://<host>%s", cfg.Addr, cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func buildProviders(cfg *config.Config, stg *config.Settings) []metadata.Provider {
	var ps []metadata.Provider
	// 优先走飞牛 HTTP API（Emby 式）：设置页开关打开才启用
	if url, user, pass, ok := stg.Effective(); ok {
		log.Printf("[metadata] 使用飞牛影视 API: %s", url)
		ps = append(ps, fnosapi.New(url, user, pass, stg.MapPath))
	} else if cfg.FnosAPIEnabled() {
		// 兼容：只配了环境变量没进过设置页
		log.Printf("[metadata] 使用飞牛影视 API（环境变量）: %s", cfg.FnosAPIURL)
		ps = append(ps, fnosapi.New(cfg.FnosAPIURL, cfg.FnosAPIUser, cfg.FnosAPIPass, cfg.MapPath))
	} else if fp, err := fnos.New(cfg.FnosDBPath); err == nil {
		log.Printf("[metadata] 飞牛影视库已挂载: %s", cfg.FnosDBPath)
		ps = append(ps, fp)
	} else {
		log.Printf("[metadata] 飞牛库不可用(%v)，将仅使用 NFO", err)
	}
	ps = append(ps, nfo.New(cfg.MediaDirs))
	return ps
}

// buildSources 组装字幕源：设置页的值优先（LoadSettings 已把环境变量作为默认值），
// 没进过设置页时行为与原来完全一致。
func buildSources(cfg *config.Config, stg *config.Settings) []subsource.Source {
	sub := stg.Sub
	srcs := []subsource.Source{
		assrt.New(sub.AssrtToken),
		opensubtitles.New(sub.OSAPIKey, sub.OSUser, sub.OSPass),
		subdl.New(sub.SubDLKey),
		subhd.New(),  // 免 key
		xunlei.New(), // 免 key，按视频 CID 查询
	}
	for _, s := range srcs {
		log.Printf("[source] %s enabled=%v", s.Name(), s.Enabled())
	}
	return srcs
}

// doctor 打印飞牛库结构诊断，贴回给开发者确认字段映射
func doctor(cfg *config.Config) {
	fmt.Println("== StrmSub doctor ==")
	stg := config.LoadSettings(cfg.DataDir)
	if url, user, pass, ok := stg.Effective(); ok {
		fmt.Printf("飞牛 API: %s（用户 %s）\n", url, user)
		fp := fnosapi.New(url, user, pass, stg.MapPath)
		out, err := fp.Probe(context.Background())
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}
		fmt.Println(out)
		return
	}
	if cfg.FnosAPIEnabled() {
		fmt.Printf("飞牛 API: %s（用户 %s）\n", cfg.FnosAPIURL, cfg.FnosAPIUser)
		fp := fnosapi.New(cfg.FnosAPIURL, cfg.FnosAPIUser, cfg.FnosAPIPass, cfg.MapPath)
		out, err := fp.Probe(context.Background())
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}
		fmt.Println(out)
		return
	}
	fmt.Printf("飞牛库路径: %s\n", cfg.FnosDBPath)
	fp, err := fnos.New(cfg.FnosDBPath)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	defer fp.Close()
	fmt.Println("✅ 飞牛库可读")
	fmt.Println(fp.Inspect())
}
