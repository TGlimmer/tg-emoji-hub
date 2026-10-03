# tg-emoji-hub 爬虫（Python + Telethon）

> This guide is in Chinese. For an English overview, see the [README](../README.md) and [docs/architecture.md](../docs/architecture.md).

Telegram Premium 自定义表情爬虫。把官方推荐（Featured）表情包和手动添加的表情包写入 MySQL，并下载缩略图和原文件，供 Go 主服务搜索和展示。

爬虫以 Telegram **用户账号**身份调用 MTProto API（不是 Bot），需要在 [my.telegram.org](https://my.telegram.org) 的 “API development tools” 申请 API ID 和 API Hash。

## 目录结构

```
crawler/
├── crawler.py               # 主入口：full / watch 子命令
├── single_pack.py           # 爬取单个表情包
├── backfill_keywords_ai.py  # 可选：AI 补全关键词与分类
├── lib/
│   ├── config.py            # 读取 .env / 环境变量
│   ├── client.py            # Telethon 客户端与首次登录
│   ├── fetcher.py           # getFeaturedEmojiStickers / getStickerSet 封装
│   ├── extractor.py         # 从 Document 中提取表情字段与关键词
│   ├── thumbnail.py         # 下载缩略图（文件名白名单校验）
│   ├── media.py             # 下载原文件 webp / webm / tgs（文件名白名单校验）
│   ├── db.py                # SQLAlchemy 模型与 upsert
│   ├── signals.py           # 信号目录监听
│   ├── pipeline.py          # 串联 client、fetcher、extractor、db
│   └── mocks.py             # 测试与 --dry-run 使用的模拟客户端
├── tests/                   # pytest 单元测试
├── requirements.txt
└── .env.example
```

## 本地运行

需要 Python 3.11 及以上版本。

```bash
cd crawler

# 1. 安装依赖
python -m venv .venv
source .venv/bin/activate    # Windows：.venv\Scripts\activate
pip install -r requirements.txt

# 2. 填写配置
cp .env.example .env
# 编辑 .env：至少填写 TG_API_ID / TG_API_HASH / TG_PHONE / DB_DSN
```

`.env` 中的相对路径以**启动爬虫时所在的目录**为基准。与本地 `go run ./cmd/server`（在仓库根目录运行，使用默认路径）配合时，建议这样设置：

```dotenv
DB_DSN=mysql+pymysql://tgemoji:<密码>@127.0.0.1:3306/tg_emoji_hub?charset=utf8mb4
CRAWLER_SIGNALS_DIR=../var/signals
THUMBNAIL_DIR=../web/public/thumbs
MEDIA_DIR=../web/public/emoji
SESSION_DIR=./.session
```

三个目录必须与 Server 配置一一对应：

| 用途 | Server（`config.yaml`） | 爬虫（`.env`） |
|------|-------------------------|----------------|
| 信号目录 | `paths.signals_dir` | `CRAWLER_SIGNALS_DIR` |
| 缩略图目录 | `paths.thumbnail_dir` | `THUMBNAIL_DIR` |
| 原文件目录 | `paths.media_dir` | `MEDIA_DIR` |

全部配置项说明见 [docs/architecture.md](../docs/architecture.md#configuration-reference)（英文）。

## 首次登录（生成 .session）

```bash
python -c "import asyncio; from lib.client import login_interactive; from lib.config import load_config; asyncio.run(login_interactive(load_config()))"
```

按提示输入 Telegram 发送的验证码（开启了两步验证的账号还需要输入密码）。成功后会在 `SESSION_DIR` 下生成 `.session` 文件，之后自动复用。

**警告**：`.session` 文件等同于已登录的 Telegram 账号，拿到它的人无需密码和验证码即可操作你的账号。切勿提交到 Git 或分享给他人（已在 `.gitignore` 中排除）。详见 [SECURITY.md](../SECURITY.md)。

## 命令

```bash
# 全量爬取：官方推荐表情包 + 数据库中所有手动添加的表情包
python crawler.py full
python crawler.py full --no-thumbs      # 跳过缩略图，只写入元数据
python crawler.py full --no-originals   # 跳过原文件，动态表情只显示静态图

# 常驻监听信号目录：Server 收到「添加表情包」请求后写入信号，这里自动爬取对应的包
python crawler.py watch

# 爬取指定表情包（以 manual 来源入库）
python single_pack.py AnimatedEmojies

# 离线自检：使用模拟客户端和内存 SQLite，不联网、不写入数据库，不需要 Telegram 凭据
python crawler.py full --dry-run
python crawler.py watch --dry-run
python single_pack.py AnimatedEmojies --dry-run
```

**同一个会话文件同一时间只能被一个进程使用。** `full`、`watch`、`single_pack.py`（不带 `--dry-run` 时）以及首次登录命令都会打开 `SESSION_DIR` 下的同一个 `.session` 文件；Telethon 的会话文件同一时间只能被一个客户端使用，同时运行会出现 `database is locked` 等错误。首次部署时先 `full`、再启动 `watch`；以后需要再次全量爬取或单独爬某个包时，先停掉常驻的 `watch`，执行完再启动。停止期间写入信号目录的「添加表情包」请求会保留，`watch` 启动后继续处理。

爬虫本身没有定时功能。需要定期刷新官方推荐表情包时，请用 cron 等外部工具定时执行 `python crawler.py full`，并在执行前停止、执行后恢复常驻的 `watch`。

## AI 关键词与分类（可选）

`backfill_keywords_ai.py` 会把每个表情的缩略图发给视觉模型，生成中英文关键词并从 11 个分类中选出一个，写回数据库。网页上的分类筛选依赖这一步。

在 `.env` 中配置：

| 变量 | 说明 |
|------|------|
| `AI_PROVIDER` | `gemini`（默认）或 `zhipu` |
| `GEMINI_API_KEY` | Google Gemini API Key，在 <https://aistudio.google.com/apikey> 申请 |
| `GEMINI_MODEL` | 可选，默认 `gemini-3.5-flash` |
| `ZAI_API_KEY` | 智谱 API Key，在 <https://bigmodel.cn/usercenter/apikeys> 申请 |
| `HTTPS_PROXY` | 可选，所在网络无法直接访问服务商 API 时使用，例如 `http://127.0.0.1:7890`（直接在本机运行时）。Docker 部署见下方「Docker 部署」中的说明 |

```bash
python backfill_keywords_ai.py --limit 20          # 先小批量试跑，检查效果
python backfill_keywords_ai.py                     # 增量处理
python backfill_keywords_ai.py --provider zhipu    # 临时切换服务商
python backfill_keywords_ai.py --all               # 全部重新处理
python backfill_keywords_ai.py --concurrency 1     # 降低并发（默认 3）
```

- 增量模式只处理关键词中还没有英文单词（连续 3 个及以上英文字母）的表情。
- 被服务商内容审核拒绝的图片，会被写入 `ai_filtered` 标记并在之后跳过。
- 重新爬取某个表情包时，`keywords` 会被 Telegram 原始关键词覆盖（`category` 保留）。如需恢复 AI 关键词，请使用 `--all` 重新处理。

## 测试

```bash
python -m pytest -q
```

测试覆盖 extractor、数据库 upsert、缩略图路径白名单、信号重试、pipeline 端到端（dry-run），不需要数据库和 Telegram 凭据。

## 与 Go 服务的字段对应

爬虫写入的字段与 Go 侧 `internal/domain/entity/*.go` 保持一致：

| 表 | 字段 |
|----|------|
| `sticker_packs` | id / short_name / title / tg_pack_id / is_featured / sticker_count / source / last_crawled_at / created_at / updated_at |
| `emoji_stickers` | id / custom_emoji_id / alt / keywords / category / pack_id / thumbnail_path / mime_type / is_free / is_animated / created_at / updated_at |

`source` 只接受 `featured` / `manual`，`mime_type` 只接受 `image/webp` / `video/webm` / `application/x-tgsticker`，非法值会按与 Go 侧相同的规则回退为默认值。修改表结构时，两边的模型需要同步修改。

## 故障排查

| 现象 | 原因 | 处理 |
|------|------|------|
| `FLOOD_WAIT` 报错 | Telegram 限流 | 60 秒以内的等待由 Telethon 自动处理；更长的等待会让该包本次失败，下次爬取时会重试。频繁出现时请降低爬取频率 |
| `FILE_REFERENCE_EXPIRED` | 文件引用过期 | 重新爬取该包即可，每次爬取都会重新获取表情包信息 |
| `session 未授权` | `.session` 文件丢失、路径不对或已失效 | 检查 `SESSION_DIR` 和 `TG_SESSION_NAME`，然后重新执行首次登录命令 |
| `TG_API_ID / TG_API_HASH 未配置` | `.env` 未填写或未被读取 | 确认 `.env` 位于 `crawler/` 目录；或加 `--dry-run` 离线运行 |
| 某个包入库 0 条表情 | 包内没有自定义表情 | 正常现象，普通贴纸会被跳过 |
| 添加的包一直没有爬取 | `watch` 未运行，或该包已连续失败 5 次 | 确认 `watch` 在运行；失败 5 次的信号文件会被改名为 `<包名>.failed`，下次 `full` 时仍会重新爬取 |

## Docker 部署

Docker 部署时不使用 `crawler/.env`。所有配置写在 `deployments/.env` 中，由 Compose 以环境变量形式传给爬虫容器，容器内的目录和数据库连接已自动设置好。爬虫属于 Compose 的 `crawler` profile，默认不随 `make up` 启动。

```bash
make crawler-login   # 首次登录，会话保存在 tg_session 数据卷
make crawler-full    # 全量爬取
make crawler-up      # 常驻运行 watch，处理网页上提交的添加请求
make crawler-dry     # 离线自检
make crawler-down    # 停止常驻爬虫
```

首次部署按 `crawler-login` → `crawler-full` → `crawler-up` 的顺序执行。以后需要再次全量爬取时，先停掉常驻爬虫：`make crawler-down && make crawler-full && make crawler-up`。

Docker 部署中配置 `HTTPS_PROXY` 时：代理地址必须是容器能访问到的地址，不能写 `127.0.0.1`（Docker Desktop 可用 `host.docker.internal`，Linux 填宿主机内网 IP 并确认代理没有只监听 `127.0.0.1`）；另外，宿主机终端里已设置的 `HTTPS_PROXY` 会覆盖 `deployments/.env` 中的值，执行 compose 命令前先 `unset HTTPS_PROXY`。

完整步骤见 [deployments/README.md](../deployments/README.md)。不使用 Docker 的服务器部署见 [deployments/bare-metal/README.md](../deployments/bare-metal/README.md) 和 [deployments/baota/README.md](../deployments/baota/README.md)。
