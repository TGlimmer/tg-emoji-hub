# tg-emoji-hub

[English](README.md) | **简体中文**

一个可自行部署的 Telegram Premium 自定义表情（custom emoji）搜索工具。按关键词或对应的标准 emoji 查找表情，一键复制它的 `custom_emoji_id`，并直接得到 Bot 代码需要的格式。项目包含网页界面、JSON API，以及供 AI 编程助手直接查询表情 ID 的 MCP Server。

## 解决什么问题

Telegram Bot 发送自定义表情时，需要在 HTML 中写：

```html
<tg-emoji emoji-id="5404674261681232568">🔥</tg-emoji>
```

（或等价的 MarkdownV2 语法）。这个 ID 在 Telegram 客户端里看不到。通常的做法是：打开客户端找到表情包，把表情发给一个辅助 Bot 读出 `custom_emoji_id`，再手工填进代码里的映射表。每加一个表情都要重复一遍，非常繁琐。

tg-emoji-hub 先把表情包爬取入库，之后你可以：

- 在浏览器里搜索，一键复制 ID 或可直接粘贴的代码片段；
- 让 Claude Code、Cursor、Codex 等 MCP 客户端在写 Bot 代码时直接调用 `search_emoji("fire")` 取得 ID。

## 功能

- **搜索**：支持关键词（`fire`、`cat`、`火`）和 emoji 字符（`🔥`），按相关度排序，精确匹配和整词匹配优先
- **4 种复制格式**：纯 ID、`<tg-emoji>` HTML 标签、Go map 条目、JSON 对象
- **筛选**：按表情包、按 11 个分类（情绪、动物、人物、食物、加密货币、物品、符号、手势、梗图、交通工具、自然）、仅看动态表情
- **动态预览**：支持视频（`.webm`）和 Lottie（`.tgs`）表情
- **中英文界面**：页头一键切换
- **手动添加表情包**：输入包的 short_name，爬虫在后台抓取
- **MCP Server**：5 个工具，支持 Streamable HTTP 和 stdio
- **JSON API**：方便接入你自己的工具

收录范围：Telegram 官方推荐（Featured）的自定义表情包，以及你手动添加的表情包。关键词搜索使用 Telegram 自带的关键词；可选的 AI 打标步骤会补充中英文关键词，并给出分类筛选所需的分类。

## 工作原理

| 组成部分 | 技术栈 | 职责 |
|----------|--------|------|
| Server | Go、Gin、GORM、[mcp-go](https://github.com/mark3labs/mcp-go) | JSON API、MCP Server、缩略图和原文件 |
| 爬虫 | Python、[Telethon](https://github.com/LonamiWebs/Telethon) | 以 Telegram 用户身份登录，把表情包写入 MySQL 和磁盘 |
| 网页界面 | React 19、Vite、Tailwind CSS、shadcn/ui | 搜索页（静态文件，由 Nginx 提供） |
| 数据库 | MySQL 8 | 表情包与表情数据 |

Server 与爬虫通过数据库和一个共享的信号文件目录协作，详见 [docs/architecture.md](docs/architecture.md)（英文）。

## 开始之前

- **爬取数据需要 Telegram 用户账号。** 爬虫以普通用户身份调用 MTProto API，因为 Bot API 无法列出官方推荐的表情包，也拿不到表情关键词。请在 [my.telegram.org](https://my.telegram.org) 的 “API development tools” 中申请 API ID 和 API Hash。
- **爬虫的登录会话等同于账号凭据。** 任何人拿到你的 API 凭据和 Telethon `.session` 文件，都可以操作你的 Telegram 账号。切勿提交到代码仓库或分享给他人，详见 [SECURITY.md](SECURITY.md)。
- **服务没有登录认证。** 能访问网页端口的人都可以搜索和添加表情包。请部署在内网，或在前面加一层带 HTTPS 和访问控制的反向代理。
- **分类功能依赖 AI 打标。** 运行可选的 AI 打标步骤之后分类筛选才有结果，需要 Google Gemini 或智谱的 API Key。
- **Bot 能否发送自定义表情有官方限制。** 根据 Bot API 文档，Bot 需要在 Fragment 购买过额外用户名，或者 Bot 的所有者开通了 Telegram Premium 且消息由 Bot 直接发到私聊、群组或超级群组。tg-emoji-hub 只负责帮你找到 ID。

## Docker 快速上手

需要：Docker（含 Compose 插件）和 `make`。每个 `make` 目标都只是对 `docker compose -f deployments/docker-compose.yml ...` 的简单封装；没有 `make` 时可以直接执行 [Makefile](Makefile) 里对应的命令，例如 `make up` 等价于 `docker compose -f deployments/docker-compose.yml up -d --build`。

### 1. 启动服务

```bash
git clone https://github.com/TGlimmer/tg-emoji-hub.git
cd tg-emoji-hub
make env
```

`make env` 会从模板生成 `deployments/.env`，Docker 部署的全部配置都在这一个文件里。至少需要设置：

| 变量 | 说明 |
|------|------|
| `MYSQL_ROOT_PASSWORD`、`MYSQL_PASSWORD` | 必填。只能使用字母和数字，因为密码会被写入 YAML 配置和数据库连接串。 |
| `HTTP_PORT` | 浏览器和 MCP 客户端访问的端口，默认 `8080`。 |
| `PUBLIC_URL` | 用户在浏览器中访问的地址，接口返回的缩略图和原文件链接以它开头。默认 `http://localhost:8080`。 |

然后构建并启动 MySQL、Server 和网页前端：

```bash
make up
make ps          # 等待 mysql、server、web 三个服务都变为 healthy
curl http://localhost:8080/healthz
```

浏览器打开 <http://localhost:8080>。此时页面可以访问，但还没有任何表情数据。

### 2. 用爬虫填充数据

在 Compose 中爬虫是可选服务（属于 `crawler` profile，`make up` 不会启动它），但没有它就没有可搜索的数据。

先在 `deployments/.env` 中填写 Telegram 凭据：`TG_API_ID`、`TG_API_HASH`、`TG_PHONE`（带国际区号）。然后执行：

```bash
make crawler-login   # 交互式登录：输入 Telegram 发给你的验证码
make crawler-full    # 首次全量爬取官方推荐表情包，耗时较长
make crawler-up      # 让爬虫常驻运行，处理网页上提交的「添加表情包」请求
```

登录会话保存在 `tg_session` 数据卷中，重启后无需重新登录。`make crawler-dry` 会用模拟数据离线跑一遍爬虫流程，可用于自检。

常驻爬虫和全量爬取不能同时运行：两者使用同一个 Telegram 登录会话文件，而 Telethon 的会话文件同一时间只能被一个客户端使用。以后需要再次全量爬取时，先停掉常驻爬虫：

```bash
make crawler-down && make crawler-full && make crawler-up
```

停止期间网页上提交的「添加表情包」请求会留在信号目录里，常驻爬虫恢复后会继续处理。

### 3. 可选：AI 关键词与分类

在 `deployments/.env` 中填写 `GEMINI_API_KEY`（或设置 `AI_PROVIDER=zhipu` 并填写 `ZAI_API_KEY`）。如果所在网络无法直接访问 Gemini API，还需要设置 `HTTPS_PROXY`：

- 代理地址必须是容器能访问到的地址，不能写 `127.0.0.1`（容器里的 `127.0.0.1` 指的是容器自己）。Docker Desktop（Windows / macOS）可以写 `http://host.docker.internal:端口`；Linux 请填宿主机的内网 IP，并确认代理没有只监听 `127.0.0.1`。
- Docker Compose 替换变量时，当前终端的环境变量优先于 `deployments/.env`。如果终端里已经设置了 `HTTPS_PROXY`，执行 compose 命令前先运行 `unset HTTPS_PROXY`。

然后在一次性的爬虫容器中运行打标脚本（打标脚本不使用 Telegram 登录会话，可以在常驻爬虫运行时执行）：

```bash
docker compose -f deployments/docker-compose.yml --profile crawler \
  run --rm --entrypoint python crawler backfill_keywords_ai.py
```

### 停止、清理与升级

```bash
make down    # 停止所有服务
make clean   # 停止服务并删除所有数据卷，数据库内容会一并删除
```

`migrations/` 中的建表脚本只会在 MySQL 数据卷首次创建时自动执行。升级已有部署时，需要手动执行新增的 `*.up.sql`，方法见 [deployments/README.md](deployments/README.md)。

### 正式上线

- 把 `PUBLIC_URL` 改成对外地址，例如 `https://emoji.example.com`。
- web 容器只提供 HTTP。请在它前面再加一层 HTTPS 反向代理，可参考 [deployments/bare-metal/nginx/](deployments/bare-metal/nginx/) 中的 Nginx 配置。
- 添加表情包时，Server 会请求 `https://t.me/addemoji/<包名>` 校验表情包是否存在，因此 Server 需要能访问外网。请求失败时会在超时（最长 10 秒）后跳过校验。

完整的 Docker 部署说明和故障排查见 [deployments/README.md](deployments/README.md)。

## 接入 AI 编程助手（MCP）

服务启动后，MCP 地址为 `http://localhost:8080/mcp`（Streamable HTTP）。

**Claude Code**

```bash
claude mcp add --transport http tg-emoji-hub http://localhost:8080/mcp
```

**Cursor**（`~/.cursor/mcp.json` 或项目内 `.cursor/mcp.json`）

```json
{
  "mcpServers": {
    "tg-emoji-hub": { "url": "http://localhost:8080/mcp" }
  }
}
```

**Codex CLI**（`~/.codex/config.toml`）

```toml
[mcp_servers.tg-emoji-hub]
url = "http://localhost:8080/mcp"
```

**Claude Desktop** 通过启动本地命令接入，因此使用 stdio 模式。Docker 服务运行时可以这样配置：

```json
{
  "mcpServers": {
    "tg-emoji-hub": {
      "command": "docker",
      "args": [
        "compose", "-f", "/absolute/path/to/tg-emoji-hub/deployments/docker-compose.yml",
        "exec", "-T", "server", "server-entrypoint.sh", "--mcp-stdio"
      ]
    }
  }
}
```

可用工具：`search_emoji`、`get_emoji_by_id`、`list_packs`、`list_pack_stickers`、`search_in_pack`。完整参数、不使用 Docker 时的 stdio 配置和故障排查见 [docs/mcp.md](docs/mcp.md)（英文）。

## 不使用 Docker 部署

以下两份分步指南介绍如何在 Linux 服务器上直接运行 Server 和爬虫，并配置 Nginx 与 HTTPS，内容包括定期全量爬取和 AI 打标：

| 指南 | 适合 |
|------|------|
| [deployments/bare-metal/README.md](deployments/bare-metal/README.md) | 命令行 + systemd + Nginx |
| [deployments/baota/README.md](deployments/baota/README.md) | 宝塔面板 |

两份指南都以「把本地已有数据迁移到服务器」为主线编写，也都包含一节不带本地数据的全新安装说明。

## 本地开发

需要：Go 1.26+、Node.js 22、Python 3.11+、MySQL 8。

**数据库**：可以使用任意 MySQL 8 实例，或者只用 Compose 启动数据库，并把 3306 端口发布到 `127.0.0.1`（需要先准备好 `deployments/.env`，见快速上手）：

```bash
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.dev.yml up -d mysql
```

**Server**

```bash
cp configs/config.yaml.example configs/config.yaml   # 按你的 MySQL 修改 database.*
go run ./cmd/server                                   # http://localhost:8080
```

模板中默认 `server.mode: dev`，启动时会自动创建或更新数据表。

**网页前端**

```bash
cd web
npm ci
npm run dev    # http://127.0.0.1:5173
```

开发服务器会把 `/api`、`/healthz`、`/mcp` 转发到 `http://localhost:8080`。缩略图和动画通过接口返回的链接加载，链接以 `server.base_url` 开头，因此开发时请保持它为 `http://localhost:8080`。

**爬虫**

```bash
cd crawler
python -m venv .venv
source .venv/bin/activate          # Windows：.venv\Scripts\activate
pip install -r requirements.txt
cp .env.example .env
```

与本地 `go run` 的 Server 配合使用时，在 `crawler/.env` 中这样设置，让两个进程使用相同的目录：

```dotenv
DB_DSN=mysql+pymysql://tgemoji:<密码>@127.0.0.1:3306/tg_emoji_hub?charset=utf8mb4
CRAWLER_SIGNALS_DIR=../var/signals
THUMBNAIL_DIR=../web/public/thumbs
MEDIA_DIR=../web/public/emoji
SESSION_DIR=./.session
```

然后登录一次并开始爬取：

```bash
python -c "import asyncio; from lib.client import login_interactive; from lib.config import load_config; asyncio.run(login_interactive(load_config()))"
python crawler.py full     # 官方推荐表情包 + 网页上添加的表情包
python crawler.py watch    # 常驻运行，处理网页上添加的表情包
```

`full` 和 `watch` 使用同一个会话文件，不能同时运行。以后再次全量爬取前，请先停掉 `watch`。

可选的 AI 打标（需要在 `crawler/.env` 中填写 `GEMINI_API_KEY` 或 `ZAI_API_KEY`）：

```bash
python backfill_keywords_ai.py --limit 20   # 先小批量试跑
python backfill_keywords_ai.py
```

更多爬虫参数见 [crawler/README.md](crawler/README.md)。测试方法和提交流程见 [CONTRIBUTING.md](CONTRIBUTING.md)（英文）。

## 文档索引

| 文档 | 内容 |
|------|------|
| [docs/architecture.md](docs/architecture.md) | 组件划分、数据模型、搜索打分、爬取流程、配置项说明（英文） |
| [docs/api.md](docs/api.md) | HTTP API 参考（英文） |
| [docs/mcp.md](docs/mcp.md) | MCP 工具与客户端配置（英文） |
| [deployments/README.md](deployments/README.md) | Docker 部署 |
| [deployments/bare-metal/README.md](deployments/bare-metal/README.md) | Linux + systemd + Nginx 部署 |
| [deployments/baota/README.md](deployments/baota/README.md) | 宝塔面板部署 |
| [crawler/README.md](crawler/README.md) | 爬虫使用说明 |
| [web/README.md](web/README.md) | 前端开发说明 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 开发与提交 PR（英文） |
| [SECURITY.md](SECURITY.md) | 漏洞报告与凭据保护（英文） |

## 社区

交流讨论：[Telegram 交流群](https://t.me/devqun)。问题反馈与功能建议：GitHub Issues。

## 许可证

[MIT](LICENSE)
