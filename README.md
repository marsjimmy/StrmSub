# StrmSub 🎬

基于**已刮削元数据**的 STRM 字幕工具。不猜文件名，不调 TMDB，直接吃现成的识别结果。

## 为什么做这个

传统字幕工具（ChineseSubFinder 等）靠解析文件名识别媒体，`S01E01` 写成 `1x01` 就全乱套，
而且普遍不支持 `.strm`。StrmSub 换条路：

- **飞牛影视**：两种接入方式（二选一）
  - **HTTP API（推荐，Emby 式）**：直连飞牛影视服务 `http://<飞牛IP>:8005`，用户名密码登录，
    分页拉取媒体库、自动展开剧集季/集。配了 `STRMSUB_FNOS_API_URL` 就启用，不再需要挂载 db，
    StrmSub 可以跑在任何能连上飞牛的机器上
  - **SQLite 直读（备选）**：只读读取它的 SQLite 库（`trimmedia.db`），刮削好的标题/年份/季集/IMDb ID 直接用
- **NFO**：兼容 Kodi/Jellyfin/TMM 写入的 `.nfo`，可做补充
- **STRM 原生**：扫描 `.strm`，字幕落到 `.strm` 旁边，命名兼容主流播放器（`<basename>.zh.srt`）

## 架构

```
┌─────────────┐   ┌──────────┐   ┌───────────┐   ┌─────────┐   ┌──────────┐
│ 飞牛 SQLite │──▶│ 扫描 .strm│──▶│ 字幕源搜索 │──▶│ 打分匹配 │──▶│ 下载落盘 │
│ / NFO       │   │          │   │ASSRT/OS/SubDL│ │         │   │ .zh.srt  │
└─────────────┘   └──────────┘   └───────────┘   └─────────┘   └──────────┘
                        │ 定时调度（默认 30min）+ Web 仪表盘 :8099
```

- `internal/metadata/fnos`：飞牛库只读读取，**启动时 PRAGMA 探测列名**，表结构变化不崩
- `internal/metadata/fnosapi`：飞牛影视 HTTP API 接入（登录 + Authx 签名 + item/season/episode 拉取），
  字段解析走候选键防御式匹配；`doctor` 可打印样本原始 JSON 确认映射
- `internal/metadata/nfo`：Kodi NFO 解析（movie / episodedetails）
- `internal/subsource`：统一 `Source` 接口；ASSRT（真实实现，官方 API，20次/分钟节流）、
  OpenSubtitles（搜索真实实现，下载需账号）、SubDL（占位）
- `internal/matcher`：语言硬门槛 + 季集/年份/标题/热度打分，低于阈值宁缺毋滥
- `internal/store`：自用 SQLite 记录媒体与字幕状态
- `internal/web`：REST API + 内置深色仪表盘（手机友好）

## 快速开始（飞牛 fnOS）

1. 复制环境变量模板并填写：
   ```bash
   cp .env.example .env
   # ASSRT_TOKEN 去 https://assrt.net/usercp.php 拿（免费）
   ```
2. 修改 `docker-compose.yml` 里你的媒体目录路径
3. 启动：
   ```bash
   docker compose up -d --build
   ```
4. 打开 `http://<飞牛IP>:8099`

## 本地运行

```bash
export PATH=$HOME/workspace/toolchain/go/bin:$PATH
go run ./cmd/strmsub doctor   # 诊断飞牛库结构（贴回给开发者确认字段映射）
go run ./cmd/strmsub scan     # 单次扫描
go run ./cmd/strmsub          # 常驻服务 + 定时扫描
```

环境变量见 `internal/config/config.go`（`STRMSUB_*`）。

## 飞牛 API 接入说明

**推荐：在仪表盘里配置。** 打开 `http://<host>:8099` → 右上角 **⚙️ 设置**：

- **是否开启飞牛影视 API**：开关，打开后即用飞牛刮削好的元数据识别媒体、找字幕
- **飞牛服务器 URL**：如 `http://192.168.1.10:8005`
- **用户名 / 密码**：飞牛账号，密码只存本机 `settings.json`（0600），不会回传
- **路径映射**：飞牛侧绝对路径 → StrmSub 容器内路径，如 `/vol1/media:/media`
- **检测服务**：登录并拉一页条目，返回诊断摘要
- **保存**：立即生效，无需重启

也可以用环境变量（容器场景）达到同样效果，设置页里的值会覆盖环境变量：

```bash
# .env
STRMSUB_FNOS_API_URL=http://192.168.1.10:8005
STRMSUB_FNOS_API_USER=你的飞牛用户名
STRMSUB_FNOS_API_PASS=你的飞牛密码
STRMSUB_PATH_MAP=/vol1/media:/media
```

配好后先跑诊断，确认能登录、能拉到条目，并把样本 JSON 贴回确认字段映射：

```bash
go run ./cmd/strmsub doctor
```

## 待办

- [ ] 用真实飞牛 API 联调确认字段映射（跑 `doctor` 贴回样本 JSON）
- [ ] 用真实飞牛库确认 `item` 表字段（跑 `doctor` 贴回输出）
- [ ] SubDL 适配器实现
- [ ] 字幕时间轴偏移微调（STRM 远端流对不上时）
- [ ] React 前端替换内置仪表盘（可选）
