# StrmSub 🎬 v2

给 `.strm` 媒体库自动找中文字幕的工具。按文件名正则识别标题，聚合 6 个字幕源搜索，字幕落到视频旁边。

## 特性

- **文件名正则识别**：内置规则（电影年份 / `SxxExx` / `EPxx` / 番号）+ 自定义多条正则（SQLite 持久化）+ 文件名测试；同名 `.nfo` 优先
- **目录增量索引**：递归扫描常见视频格式（含 `.strm`），按大小/mtime/正则签名判断新增/变化/删除
- **本地封面**：视频同名图片，或 `poster` / `folder` / `cover` / `movie` 等常见名称
- **6 字幕源聚合**：ASSRT、OpenSubtitles、SubDL、SubHD、迅雷 + 成人源 SubtitleCat（番号查询），搜索结果缓存 6 小时
- **扫描自动补字幕**：每次扫描后自动下载缺失的中文字幕，已有的跳过；失败/无结果 24 小时内不重复搜（设置页可开关）
- **FlareSolverr**：可选，用于过 Cloudflare 反爬的字幕站
- **设置全进库**：API 令牌（自动生成）、源开关/凭证、正则、扫描周期、字幕目录，环境变量只做首次播种
- **WebUI**：深色响应式，侧边栏可收起，主页 / 媒体 / 字幕 / 设置四页，`/api/*` 令牌鉴权

## 架构

```
媒体目录 ──▶ 扫描+增量索引 ──▶ 正则标题识别 ──▶ 6 字幕源聚合搜索 ──▶ 下载落盘
   │                │                    │              │               │
   │           media_index          title_rules    search_cache   download_history
   │                              (SQLite)         (SQLite)        (SQLite)
   └─────────────────── 定时调度 + Web 仪表盘 :8099 ───────────────────┘
```

- `internal/library`：目录扫描 + 增量索引 + 本地封面
- `internal/title`：正则标题识别引擎
- `internal/search`：聚合搜索 + SQLite 缓存
- `internal/subsource/*`：assrt / opensubtitles / subdl / subhd / xunlei / subtitlecat
- `internal/flare`：FlareSolverr 客户端
- `internal/store`：SQLite（kv / title_rules / media_index / search_cache / download_history）
- `internal/matcher`：语言硬门槛 + 打分选最佳
- `internal/web`：REST API + 内置 WebUI

## 快速开始（飞牛 fnOS）

1. 复制环境变量模板：`cp deploy/fnos/.env.example .env`，填字幕源 token
2. `cd deploy/fnos && docker compose up -d`
3. 打开 `http://<飞牛IP>:8099`，设置页配字幕源、正则规则
4. 点「扫描」建立媒体索引，然后在媒体页搜字幕、下载

镜像：`marsjimmyliu/strmsub:v2.0.2`（`latest` 同步），仅 `linux/amd64`，默认 root 运行。

## 环境变量（首次启动播种）

| 变量 | 说明 |
|---|---|
| `STRMSUB_MEDIA_DIRS` | 媒体目录，逗号分隔 |
| `STRMSUB_DATA_DIR` | 数据目录（SQLite） |
| `STRMSUB_SUB_DIR` | 字幕统一保存目录（空=视频同目录） |
| `STRMSUB_SCAN_INTERVAL` | 扫描周期，如 `30m` |
| `STRMSUB_TARGET_LANG` | `zh-Hans` / `zh-Hant` |
| `STRMSUB_FLARESOLVERR_URL` | FlareSolverr 地址，如 `http://192.168.1.10:8191` |
| `STRMSUB_ASSRT_TOKEN` | ASSRT token |
| `STRMSUB_OS_API_KEY/USER/PASS` | OpenSubtitles 凭证 |
| `STRMSUB_SUBDL_KEY` | SubDL API key |
| `STRMSUB_AUTO_DOWNLOAD` | 扫描后自动下载缺失中文字幕，`true`/`false`（默认 `true`） |

之后所有设置在 Web 设置页修改，存 SQLite。
