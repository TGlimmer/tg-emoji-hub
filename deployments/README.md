# tg-emoji-hub Docker 部署指南

> This guide is in Chinese. For an English overview and quick start, see the [README](../README.md).

本文介绍如何用 Docker Compose 部署 tg-emoji-hub。如果要在服务器上直接运行（不用 Docker），请看 [bare-metal/README.md](bare-metal/README.md)（命令行 + systemd）或 [baota/README.md](baota/README.md)（宝塔面板）。

## 前置要求

- Docker 与 Docker Compose v2（使用 `docker compose` 命令，而不是旧版的 `docker-compose`）
- `make`（可选）。每个 `make` 目标都是对 `docker compose -f deployments/docker-compose.yml ...` 的简单封装，没有 `make` 时可直接执行 [Makefile](../Makefile) 中对应的命令
- 爬取数据需要一个 Telegram 用户账号，以及在 [my.telegram.org](https://my.telegram.org) 申请的 API ID 和 API Hash

以下命令均在仓库根目录执行。

---

## 服务组成

| 服务 | 说明 | 对宿主机开放端口 |
|------|------|------------------|
| `mysql` | MySQL 8.4，数据保存在 `mysql_data` 数据卷 | 否 |
| `server` | Go 主服务：JSON API、MCP、缩略图和原文件 | 否 |
| `web` | Nginx：托管前端页面，并把 `/api/`、`/healthz`、`/thumbs/`、`/emoji/`、`/mcp` 转发给 `server` | 是，`HTTP_PORT`（默认 8080） |
| `crawler` | Python 爬虫，属于 `crawler` profile，`make up` 不会启动它 | 否 |

浏览器和 MCP 客户端只访问 `web` 的端口。`server` 与 `crawler` 通过共享数据卷协作：

| 数据卷 | 用途 |
|--------|------|
| `mysql_data` | 数据库文件 |
| `signals` | `server` 写入「添加表情包」信号文件，`crawler` 读取并处理 |
| `thumbs` | 缩略图，`crawler` 写入，`server` 提供访问 |
| `media` | 表情原文件（webp / webm / tgs），`crawler` 写入，`server` 提供访问 |
| `tg_session` | Telegram 登录会话，丢失后需要重新登录 |

Compose 项目名为 `tg-emoji-hub`，所以数据卷在宿主机上的完整名称形如 `tg-emoji-hub_tg_session`。

---

## 配置：`deployments/.env`

Docker 部署的全部配置只写在 `deployments/.env` 一个文件里。不需要再准备 `configs/config.yaml` 和 `crawler/.env`，这两个文件只在不使用 Docker 时才会用到。`server` 容器启动时会根据环境变量自动生成自己的配置文件（`server.mode` 固定为 `prod`）。

生成配置文件：

```bash
make env
# 等价于：cp deployments/.env.example deployments/.env（文件已存在时不会覆盖）
```

| 变量 | 必填 | 说明 |
|------|------|------|
| `MYSQL_ROOT_PASSWORD` | 是 | MySQL root 密码。只能使用字母和数字 |
| `MYSQL_PASSWORD` | 是 | 应用数据库账号 `tgemoji` 的密码。只能使用字母和数字，因为它会被写入 YAML 配置和数据库连接串 |
| `HTTP_PORT` | 否 | 对外端口，默认 `8080` |
| `PUBLIC_URL` | 否 | 用户在浏览器中访问的地址，接口返回的缩略图和原文件链接以它开头。默认 `http://localhost:8080`，绑定域名后改为 `https://你的域名` |
| `LOG_LEVEL` | 否 | 日志级别，默认 `info` |
| `TG_API_ID` / `TG_API_HASH` | 爬虫必填 | 在 my.telegram.org 申请 |
| `TG_PHONE` | 爬虫必填 | Telegram 账号手机号（带国际区号，如 `+8613800000000`） |
| `DOWNLOAD_ORIGINALS` | 否 | 默认 `1`。设为 `0` 时只保存元数据和缩略图，动态表情只显示静态图 |
| `AI_PROVIDER` | 否 | AI 打标服务商：`gemini`（默认）或 `zhipu` |
| `GEMINI_API_KEY` / `ZAI_API_KEY` | 否 | AI 打标所用服务商的 API Key，只需填写所选服务商的一个 |
| `HTTPS_PROXY` | 否 | AI 打标访问外部 API 时使用的代理，例如国内服务器访问 Gemini。必须填容器能访问到的地址，不能写 `127.0.0.1`，详见「[可选：AI 关键词与分类](#可选ai-关键词与分类)」 |

`deployments/.env` 已被 `.gitignore` 排除，其中包含数据库密码和 Telegram 凭据，切勿提交或分享。

---

## 首次部署

### 第 1 步：填写配置

```bash
make env
nano deployments/.env    # 至少填写 MYSQL_ROOT_PASSWORD 和 MYSQL_PASSWORD
```

### 第 2 步：构建并启动

```bash
make up
# 等价于：docker compose -f deployments/docker-compose.yml up -d --build
```

首次启动时，MySQL 会自动执行 `migrations/` 下所有 `*.up.sql` 建表脚本（只在 `mysql_data` 数据卷为空时执行一次）。

### 第 3 步：确认服务正常

```bash
make ps
curl http://localhost:8080/healthz
```

`mysql`、`server`、`web` 三个服务都显示 `healthy` 后，`/healthz` 应返回：

```json
{"status":"ok","db":"ok","version":"0.1.0"}
```

浏览器打开 `http://localhost:8080` 即可看到页面。此时数据库还是空的，需要继续部署爬虫。

---

## 部署爬虫

### 第 4 步：填写 Telegram 凭据

在 `deployments/.env` 中填写 `TG_API_ID`、`TG_API_HASH`、`TG_PHONE`。

### 第 5 步：登录 Telegram（只需一次）

```bash
make crawler-login
```

按提示输入 Telegram 发送的验证码（若账号开启了两步验证，还需要输入密码）。登录会话保存在 `tg_session` 数据卷中，之后重启无需重新登录。

### 第 6 步：首次全量爬取

```bash
make crawler-full
```

爬取所有官方推荐（Featured）表情包，以及数据库中通过网页添加的表情包，耗时视包数量而定。

### 第 7 步：让爬虫常驻运行

```bash
make crawler-up
```

常驻的爬虫会每 2 秒检查一次信号目录，处理网页上提交的「添加表情包」请求。不启动它，添加的表情包会一直停留在「待爬取」状态。

**常驻爬虫和全量爬取不能同时运行。** 两者使用同一个 Telegram 登录会话文件，而 Telethon 的会话文件同一时间只能被一个客户端使用，同时运行会出现 `database is locked` 等错误。首次部署按上面的顺序先全量、再常驻即可；以后需要再次全量爬取时，先停掉常驻爬虫：

```bash
make crawler-down && make crawler-full && make crawler-up
```

停止期间网页上提交的「添加表情包」请求会作为信号文件留在信号目录里，常驻爬虫恢复后会继续处理，不会丢失。

自检：`make crawler-dry` 会用模拟数据离线跑一遍爬取流程，不联网、不写入数据库。

---

## 可选：AI 关键词与分类

Telegram 自带的关键词通常较少，分类筛选也完全依赖 AI 打标。在 `deployments/.env` 中填写 `GEMINI_API_KEY`（或设置 `AI_PROVIDER=zhipu` 并填写 `ZAI_API_KEY`），国内服务器访问 Gemini 还需要填写 `HTTPS_PROXY`。

配置代理时注意两点：

- **代理地址必须是容器能访问到的地址，不能写 `127.0.0.1`**：容器里的 `127.0.0.1` 指的是容器自己。Docker Desktop（Windows / macOS）可以写 `http://host.docker.internal:端口`；Linux 请填宿主机的内网 IP，并确认代理没有只监听 `127.0.0.1`。
- **宿主机终端里的同名变量会覆盖 `.env`**：Docker Compose 替换变量时，当前终端的环境变量优先于 `.env` 文件。如果终端里已经设置了 `HTTPS_PROXY`（常见于为了让终端走代理而设置的 `127.0.0.1` 地址），它会被带进容器。执行下面的 compose 命令前先运行 `unset HTTPS_PROXY`。

然后运行（打标脚本不使用 Telegram 登录会话，可以在常驻爬虫运行时执行）：

```bash
docker compose -f deployments/docker-compose.yml --profile crawler \
  run --rm --entrypoint python crawler backfill_keywords_ai.py --limit 20   # 先小批量试跑

docker compose -f deployments/docker-compose.yml --profile crawler \
  run --rm --entrypoint python crawler backfill_keywords_ai.py
```

默认只处理还没有英文关键词的表情；加 `--all` 会全部重新处理，`--concurrency 1` 可降低请求频率。

---

## 日常运维

| 操作 | 命令 |
|------|------|
| 查看服务状态 | `make ps` |
| 查看日志 | `make logs` |
| 重启单个服务 | `docker compose -f deployments/docker-compose.yml restart server` |
| 手动触发全量爬取 | `make crawler-down && make crawler-full && make crawler-up`（必须先停常驻爬虫） |
| 停止常驻爬虫 | `make crawler-down` |
| 停止所有服务 | `make down` |
| 停止并删除所有数据卷 | `make clean`（数据库、缩略图、登录会话都会被删除，请谨慎） |

定期全量爬取（例如每周日 03:00）可以加到宿主机的 crontab。同样要先停常驻爬虫，全量完成后再恢复：

```cron
0 3 * * 0  cd /path/to/tg-emoji-hub && { make crawler-down && make crawler-full; make crawler-up; } >> /var/log/tg-emoji-crawler-full.log 2>&1
```

`make crawler-up` 前面用的是 `;` 而不是 `&&`，这样即使全量爬取失败，常驻爬虫也会恢复运行。停止期间提交的「添加表情包」请求会留在信号目录里，恢复后继续处理。

---

## 升级

```bash
git pull
make up            # 重新构建并启动 mysql、server、web
make crawler-up    # 如果在运行常驻爬虫，同样需要重新构建
```

建表脚本只在 MySQL 数据卷首次创建时自动执行。如果升级后 `migrations/` 中出现了新的 `*.up.sql`，需要按编号顺序手动执行。命令中的 `< migrations/...` 是宿主机上的输入重定向，路径相对于当前目录，因此**请在仓库根目录执行**，例如：

```bash
docker compose -f deployments/docker-compose.yml exec -T mysql \
  sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" tg_emoji_hub' < migrations/003_add_category.up.sql
```

执行前建议先备份数据库。

---

## 正式上线

- 把 `PUBLIC_URL` 改成对外地址（如 `https://emoji.example.com`），然后 `make up` 使其生效。
- `web` 容器只提供 HTTP。请在它前面加一层 HTTPS 反向代理（例如宿主机上的 Nginx 反代到 `127.0.0.1:${HTTP_PORT}`），可参考 [bare-metal/nginx/tg-emoji-hub.conf](bare-metal/nginx/tg-emoji-hub.conf) 中的 SSL 和 `/mcp` 配置。
- 服务没有登录认证，任何能访问端口的人都可以搜索和添加表情包。不想公开时，请在反向代理上加访问控制，并用防火墙限制 `HTTP_PORT`。
- 添加表情包时，`server` 会请求 `https://t.me/addemoji/<包名>` 校验表情包是否存在，因此需要能访问外网。请求失败时会在超时（最长 10 秒）后跳过校验。

---

## 故障排查

### MySQL 一直不健康

```bash
docker compose -f deployments/docker-compose.yml logs mysql
```

常见原因：

- `deployments/.env` 中没有设置 `MYSQL_PASSWORD` 或 `MYSQL_ROOT_PASSWORD`：Compose 会直接报错退出
- 修改了密码，但 `mysql_data` 数据卷是用旧密码初始化的：MySQL 只在数据卷首次创建时读取这两个变量。改回旧密码，或在确认数据可以丢弃后执行 `make clean` 重新初始化

### 打开页面报 502

`web` 已经启动，但 `server` 还没就绪或启动失败。用 `make ps` 查看状态，`docker compose -f deployments/docker-compose.yml logs server` 查看原因，最常见的是数据库连接失败。

### 爬虫提示「session 未授权」

登录会话不存在或已失效。重新执行 `make crawler-login`。如果需要彻底清除旧会话：

```bash
make down
docker volume rm tg-emoji-hub_tg_session
make up
make crawler-login
make crawler-up
```

### 添加表情包后一直搜不到

- 确认常驻爬虫在运行：`make ps` 中应能看到 `crawler`，否则执行 `make crawler-up`
- 查看爬虫日志：`docker compose -f deployments/docker-compose.yml --profile crawler logs -f crawler`
- 同一个包连续失败 5 次后，信号文件会被改名为 `<包名>.failed` 并停止重试；该包仍会在下一次全量爬取时重新爬取（全量爬取前要先停常驻爬虫，见「日常运维」）

### 动态表情不播放，只显示静态图

`DOWNLOAD_ORIGINALS=0` 时不会下载原文件；或者该表情包是在开启下载之前爬取的。设为 `1` 后执行 `make crawler-down && make crawler-full && make crawler-up` 重新全量爬取即可补齐。

### AI 打标连不上代理

代理地址写成了 `127.0.0.1`，或者宿主机终端里的 `HTTPS_PROXY` 覆盖了 `.env` 中的值。处理方法见「[可选：AI 关键词与分类](#可选ai-关键词与分类)」。

---

## 架构示意

```
               浏览器 / MCP 客户端
                       │
                       ▼  HTTP_PORT (默认 8080)
               ┌───────────────┐
               │  web (Nginx)  │  前端静态文件
               └───────┬───────┘
     /api /healthz /thumbs /emoji /mcp
                       ▼
               ┌───────────────┐        数据卷：signals / thumbs / media
               │ server (Go)   │◀──────────────────────────────┐
               └───────┬───────┘                               │
                       │                                       │
               ┌───────▼───────┐                       ┌───────┴───────┐
               │  mysql 8.4    │◀──────────────────────│ crawler (Py)  │
               └───────────────┘                       └───────────────┘
                                                       数据卷：tg_session
```

各组件的详细说明见 [docs/architecture.md](../docs/architecture.md)（英文）。

---

## MCP 集成

MCP 地址为 `http://localhost:8080/mcp`（Streamable HTTP，经 `web` 容器转发）。Claude Code 可直接执行：

```bash
claude mcp add --transport http tg-emoji-hub http://localhost:8080/mcp
```

Claude Desktop 等只支持 stdio 的客户端，可以通过 `docker compose exec` 在 `server` 容器内启动 stdio 模式：

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

Cursor、Codex 等客户端的配置方法和全部工具说明见 [docs/mcp.md](../docs/mcp.md)（英文）。
