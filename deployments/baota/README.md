# tg-emoji-hub 宝塔面板部署指南

> This guide is in Chinese. For an English overview, see the [README](../../README.md).

面向宝塔用户的部署方案。如果你更习惯命令行 + systemd，看 [bare-metal/README.md](../bare-metal/README.md)。

本指南的主线是把本地已有的项目和数据迁移到服务器。没有本地数据、想在服务器上从零开始，请先看「[全新安装（没有本地数据）](#全新安装没有本地数据)」。

## 前置条件

- 宝塔面板 7.x+ 已安装并可访问
- 域名 A 记录已解析到本服务器
- 服务器内存 ≥ 2GB
- 迁移方案：本地开发机已能正常运行项目并有数据要迁出（本文的打包命令以 Windows + Git Bash 为例）

---

## 第 1 步：软件商店装环境

📋 操作位置：宝塔左侧「软件商店」→「运行环境」

| 软件 | 推荐版本 | 备注 |
|------|---------|------|
| MySQL | 5.7 或 8.0 | 两者都行，8.0 默认 utf8mb4 更方便 |
| Nginx | 1.20+ | 宝塔通常已预装 |

不要用 `apt install` / `yum install` 装——会和宝塔管理的版本冲突。

---

## 第 2 步：创建数据库

📋 操作位置：宝塔「数据库」→「添加数据库」

| 字段 | 填写值 |
|------|--------|
| 数据库名 | `tg_emoji_hub` |
| 用户名 | `tgemoji` |
| 密码 | 点「随机」生成，**记下来**，后面 config.yaml 要填 |
| 字符集 | **utf8mb4**（必须改！默认 utf8 会导致 emoji 关键词乱码） |
| 排序规则 | `utf8mb4_unicode_ci` |

---

## 第 3 步：创建网站

📋 操作位置：宝塔「网站」→「添加站点」

| 字段 | 填写值 |
|------|--------|
| 域名 | `your-domain.com` |
| 根目录 | `/www/wwwroot/your-domain.com`（默认保持） |
| PHP 版本 | **纯静态**（不需要 PHP） |
| 数据库 | 跳过（第 2 步已单独建） |

SSL 放第 10 步申请，现在不用管。

---

## 全新安装（没有本地数据）

如果本地没有数据，第 4～6 步只需要准备程序文件，按下面的方式调整：

1. **第 4 步**：只执行 `build.bat backend` 和 `build.bat frontend`，然后打包前端产物（`tar czf deployments/bare-metal/out/web-dist.tar.gz -C web dist/`）。跳过 `mysqldump` 和媒体文件打包。
2. **第 5 步**：上传 `server-linux`（重命名为 `server`）和 `web-dist.tar.gz`，再把仓库中的 `migrations/` 目录上传到 `/www/wwwroot/your-domain.com/migrations/`。解压前端后，创建运行目录：

   ```bash
   SITE=/www/wwwroot/your-domain.com
   chmod +x $SITE/server
   cd $SITE && tar xzf web-dist.tar.gz
   mkdir -p $SITE/media/emoji $SITE/media/thumbs $SITE/var/signals $SITE/logs
   ```

3. **第 6 步**：不导入 dump.sql，改为在宝塔「终端」执行建表脚本：

   ```bash
   cat /www/wwwroot/your-domain.com/migrations/*.up.sql | mysql -u tgemoji -p tg_emoji_hub
   ```

   只执行 `*.up.sql`，`*.down.sql` 是回滚脚本，不要执行。

4. 第 7～10 步照常执行。
5. **第 11 步**：上传 crawler 代码时不需要 `.session/`。在 11.4 填好 `.env` 之后，在宝塔「终端」中登录并做首次全量爬取：

   ```bash
   cd /www/wwwroot/your-domain.com/crawler
   .venv/bin/python -c "import asyncio; from lib.client import login_interactive; from lib.config import load_config; asyncio.run(login_interactive(load_config()))"
   .venv/bin/python crawler.py full
   ```

   登录时按提示输入 Telegram 发送的验证码。之后继续 11.5 及后续步骤，并在第 12 步统一修复文件属主。

---

## 第 4 步：本地打包产物（Windows 开发机）

在项目根目录打开命令提示符：

```cmd
cd 你的本地仓库目录

REM 编译 Linux 二进制
build.bat backend
REM 输出：dist\server-linux

REM 构建前端
build.bat frontend
REM 输出：web\dist\
```

然后打开 Git Bash 导出数据：

```bash
cd /path/to/tg-emoji-hub   # 本地仓库目录
mkdir -p deployments/bare-metal/out

# 导出 MySQL（含 AI 关键词数据）
mysqldump -u root -p \
  --default-character-set=utf8mb4 --single-transaction --no-tablespaces \
  tg_emoji_hub > deployments/bare-metal/out/dump.sql

# 打包媒体文件（体积视数据量而定，可能较大）
tar czf deployments/bare-metal/out/media.tar.gz \
  web/public/emoji/ web/public/thumbs/

# 打包前端产物
tar czf deployments/bare-metal/out/web-dist.tar.gz -C web dist/
```

---

## 第 5 步：上传产物到服务器

📋 操作位置：宝塔「文件」→ 进入 `/www/wwwroot/your-domain.com/`→ 点「上传」

上传以下文件：

| 本地文件 | 上传到服务器 |
|---------|-------------|
| `dist\server-linux` | `/www/wwwroot/your-domain.com/server` |
| `deployments\bare-metal\out\web-dist.tar.gz` | `/www/wwwroot/your-domain.com/` |
| `deployments\bare-metal\out\media.tar.gz` | `/www/wwwroot/your-domain.com/` |
| `deployments\bare-metal\out\dump.sql` | `/www/wwwroot/your-domain.com/` |

上传完成后，在宝塔「终端」里执行：

```bash
SITE=/www/wwwroot/your-domain.com

# 给 server 加执行权限
chmod +x $SITE/server

# 解压前端（生成 dist/ 目录）
cd $SITE && tar xzf web-dist.tar.gz

# 解压媒体文件（保留 web/public/emoji 和 thumbs 两层目录）
tar xzf $SITE/media.tar.gz -C $SITE

# 移到正确位置
mkdir -p $SITE/media/emoji $SITE/media/thumbs
mv $SITE/web/public/emoji/* $SITE/media/emoji/
mv $SITE/web/public/thumbs/* $SITE/media/thumbs/
rm -rf $SITE/web

# 确认文件数（应与本地目录中的文件数一致）
ls $SITE/media/emoji/ | wc -l

# 创建运行时目录
mkdir -p $SITE/var/signals $SITE/logs
```

---

## 第 6 步：导入 MySQL 数据

📋 操作位置：宝塔「数据库」→ 找 `tg_emoji_hub` → 点「导入」

在弹出框里选 `dump.sql` 上传并导入。

如果文件超过宝塔上传限制（默认约 200MB），改在终端直接跑：

```bash
mysql -u tgemoji -p tg_emoji_hub < /www/wwwroot/your-domain.com/dump.sql
```

---

## 第 7 步：创建 config.yaml

📋 操作位置：宝塔「文件」→ `/www/wwwroot/your-domain.com/` → 点「新建文件」→ 文件名 `config.yaml`

```yaml
server:
  port: 8080
  base_url: "https://your-domain.com"
  mode: prod

database:
  host: 127.0.0.1
  port: 3306
  user: tgemoji
  password: "第2步创建数据库时填的密码"
  name: tg_emoji_hub

log:
  level: info
  format: json

mcp:
  enabled: true
  path: /mcp

paths:
  signals_dir: /www/wwwroot/your-domain.com/var/signals
  thumbnail_dir: /www/wwwroot/your-domain.com/media/thumbs
  media_dir: /www/wwwroot/your-domain.com/media/emoji
```

> `database.password` 填第 2 步在宝塔创建数据库时设置的密码，不要求与本地开发机一致。
>
> 注意：`config.yaml` 中的数据库密码、`crawler/.env` 和 `crawler/.session/` 都属于敏感文件，不要放到 Nginx 可以直接访问的目录（如 `dist/`）中。

---

## 第 8 步：配置进程守护

三种方式选其一，效果等价：

### 方式 A：Linux 工具箱进程守护（推荐）

📋 操作位置：宝塔「工具箱」（或「Linux 工具」）→「进程守护管理器」→「添加守护进程」
（不同版本入口名称略有差异，找带"进程守护"字样的菜单）

| 参数 | 填写值 |
|------|--------|
| 名称 | `tg-emoji-hub` |
| 启动命令 | `env CONFIG_PATH=/www/wwwroot/your-domain.com/config.yaml TZ=Asia/Shanghai /www/wwwroot/your-domain.com/server` |
| 工作目录 | `/www/wwwroot/your-domain.com` |
| 启动用户 | `www` |
| 进程数 | `1` |
| 开机自启 | 勾上 |

### 方式 B：Supervisor 插件

📋 操作位置：宝塔「软件商店」→ 搜索「Supervisor」→ 安装 → 打开 → 「添加守护进程」

参数同方式 A，安装后在生成的 ini 文件里手动加环境变量：

```ini
environment=CONFIG_PATH="/www/wwwroot/your-domain.com/config.yaml",TZ="Asia/Shanghai"
```

### 方式 C：systemd（命令行备选）

```bash
cat > /etc/systemd/system/tg-emoji-hub.service << 'EOF'
[Unit]
Description=tg-emoji-hub Go server
After=network.target

[Service]
Type=simple
User=www
WorkingDirectory=/www/wwwroot/your-domain.com
ExecStart=/www/wwwroot/your-domain.com/server
Restart=on-failure
RestartSec=5
Environment=CONFIG_PATH=/www/wwwroot/your-domain.com/config.yaml
Environment=TZ=Asia/Shanghai

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload && systemctl enable --now tg-emoji-hub
```

---

## 第 9 步：Nginx 反向代理 + 静态文件

📋 操作位置：宝塔「网站」→ `your-domain.com` 的「设置」→ 左侧「配置文件」

找到 443 端口的 `server` 块，把其中的 `location` 部分替换成：

```nginx
    location / {
        root /www/wwwroot/your-domain.com/dist;
        try_files $uri $uri/ /index.html;
        location ~* \.html$ {
            add_header Cache-Control "no-cache, must-revalidate";
        }
        location ~* \.(js|css|woff2?|ttf|eot|ico|svg)$ {
            add_header Cache-Control "public, max-age=31536000, immutable";
        }
    }

    # Nginx 直出缩略图，绕开 Go server
    location /thumbs/ {
        alias /www/wwwroot/your-domain.com/media/thumbs/;
        add_header Cache-Control "public, max-age=31536000, immutable";
        add_header X-Content-Type-Options nosniff;
        access_log off;
    }

    # 原始 emoji 文件（含 .tgs lottie 和 .webm 视频）
    location /emoji/ {
        alias /www/wwwroot/your-domain.com/media/emoji/;
        add_header Cache-Control "public, max-age=31536000, immutable";
        access_log off;
        location ~* \.tgs$ {
            alias /www/wwwroot/your-domain.com/media/emoji/;
            add_header Content-Type "application/x-tgsticker";
            add_header Cache-Control "public, max-age=31536000, immutable";
        }
        location ~* \.webm$ {
            alias /www/wwwroot/your-domain.com/media/emoji/;
            add_header Content-Type "video/webm";
            add_header Cache-Control "public, max-age=31536000, immutable";
        }
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
    }

    location /healthz {
        proxy_pass http://127.0.0.1:8080;
        proxy_read_timeout 5s;
        access_log off;
    }

    # MCP 长连接——不加这几行 SSE 事件会被 Nginx 攒着发，客户端等半天
    location /mcp {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
```

保存后宝塔自动 reload Nginx。

---

## 第 10 步：申请 SSL

📋 操作位置：宝塔「网站」→ `your-domain.com` 的「设置」→ 左侧「SSL」→「Let's Encrypt」标签

勾选域名，点「申请」，等 30-60 秒。宝塔自动把证书路径写进 Nginx 并开启 HTTP→HTTPS 跳转。

---

## 第 11 步：部署 crawler（Python 爬虫，让"添加表情包"功能工作）

用户在前端点「添加自定义表情包」时，Go server 往 `var/signals/` 写一个 `{short_name}.pending` 文件作为信号。**crawler 必须以 watch 模式常驻**，才能监听到并从 Telegram 拉取新包。不跑 crawler，添加表情包功能形同虚设。

### 11.1 服务器装 Python 3.11+

📋 操作位置：宝塔「软件商店」→ 搜索「Python 项目管理器」安装；或在「终端」确认系统已有合适版本：

```bash
python3 --version   # 需要 3.11+
# 如果版本太低，用宝塔软件商店装新版，或：
# Ubuntu/Debian:  apt install python3.11 python3.11-venv
# CentOS/RHEL:    yum install python3.11
```

### 11.2 上传 crawler 代码

📋 操作位置：宝塔「文件」→ 进入 `/www/wwwroot/your-domain.com/` → 新建目录 `crawler` → 上传

把项目本地 `crawler/` 目录整个上传到 `/www/wwwroot/your-domain.com/crawler/`。

**必须包含**：

| 文件/目录 | 说明 |
|-----------|------|
| `crawler.py` | 主入口 |
| `backfill_keywords_ai.py` | AI 关键词脚本 |
| `lib/` | 全部子模块 |
| `requirements.txt` | 依赖清单 |
| `.session/` | **核心**：本地交互登录后生成的 session 文件，直接迁移到服务器 |

**不需要包含**（排除可以省传输量）：

`.venv/`、`__pycache__/`、`.pytest_cache/`、`.ruff_cache/`、`tests/`、`fixtures/`

上传 `.session/` 目录后，服务器侧无需再次交互登录。也可以不上传，改为在服务器上登录（见「全新安装」一节中的登录命令）。

> **安全提示**：`.session` 文件等同于已登录的 Telegram 账号，拿到它的人无需密码和验证码即可操作你的账号。上传后请确认它不在网站可访问的目录中，权限设为仅运行用户可读（`chmod 600`）。详见 [SECURITY.md](../../SECURITY.md)。

### 11.3 创建虚拟环境 + 安装依赖

📋 操作位置：宝塔「终端」

```bash
cd /www/wwwroot/your-domain.com/crawler

# 建虚拟环境
python3 -m venv .venv

# 安装依赖（用清华镜像加速，境外机器去掉 -i 参数）
.venv/bin/pip install -i https://pypi.tuna.tsinghua.edu.cn/simple -r requirements.txt
```

安装完成后确认版本：

```bash
.venv/bin/python -c "import telethon; print('telethon', telethon.__version__)"
```

### 11.4 创建 crawler `.env`

📋 操作位置：宝塔「文件」→ `/www/wwwroot/your-domain.com/crawler/` → 新建文件 `.env`

```env
# Telegram MTProto 凭据（my.telegram.org → API development tools 申请）
TG_API_ID=你的API_ID
TG_API_HASH=你的API_HASH
TG_PHONE=+86你的手机号
TG_SESSION_NAME=tg-emoji-hub

# MySQL（与 Go server 共用同一个库）
DB_DSN=mysql+pymysql://tgemoji:数据库密码@127.0.0.1:3306/tg_emoji_hub?charset=utf8mb4

# ⚠️ 路径必须和 config.yaml 里 paths.* 完全一致
CRAWLER_SIGNALS_DIR=/www/wwwroot/your-domain.com/var/signals
THUMBNAIL_DIR=/www/wwwroot/your-domain.com/media/thumbs
MEDIA_DIR=/www/wwwroot/your-domain.com/media/emoji
SESSION_DIR=/www/wwwroot/your-domain.com/crawler/.session

DOWNLOAD_ORIGINALS=1
LOG_LEVEL=INFO

# AI 关键词补全（可选，不需要此功能可留空）
# AI_PROVIDER 默认 gemini，国内不便访问 Google 时改 zhipu
# AI_PROVIDER=gemini
# GEMINI_API_KEY=你的Gemini_API_Key
# ZAI_API_KEY=你的智谱API_Key
```

路径对齐检查——下表三列必须完全一致：

| 含义 | config.yaml（Go server） | crawler `.env` |
|------|--------------------------|----------------|
| 信号目录 | `paths.signals_dir` | `CRAWLER_SIGNALS_DIR` |
| 缩略图目录 | `paths.thumbnail_dir` | `THUMBNAIL_DIR` |
| 原文件目录 | `paths.media_dir` | `MEDIA_DIR` |

### 11.5 验证 watch 启动正常

📋 操作位置：宝塔「终端」

```bash
cd /www/wwwroot/your-domain.com/crawler
.venv/bin/python crawler.py watch
```

看到「启动信号监听 dir=... interval=2.0s」日志，说明 crawler 已正常连接 Telegram 并开始监听信号目录。`Ctrl+C` 停止。

**如果报「session 未授权」**：`.session/` 目录没上传或 `SESSION_DIR` 路径填错，重新检查第 11.2 步。

**如果报 `TG_API_ID 未配置`**：`.env` 文件没在 `crawler/` 目录下，或 API_ID 没填。

### 11.6 配置进程守护（让 watch 常驻）

📋 操作位置：宝塔「工具箱」（或「Linux 工具」）→「进程守护管理器」→「添加守护进程」
（不同版本入口名称略有差异，找带"进程守护"字样的菜单）

| 参数 | 填写值 |
|------|--------|
| 名称 | `tg-emoji-crawler` |
| 启动命令 | `/www/wwwroot/your-domain.com/crawler/.venv/bin/python /www/wwwroot/your-domain.com/crawler/crawler.py watch` |
| 工作目录 | `/www/wwwroot/your-domain.com/crawler` |
| 启动用户 | `www`（或与 Go server 守护进程一致的用户） |
| 进程数 | `1` |
| 开机自启 | 勾上 |

如果找不到进程守护管理器入口，用 Supervisor 插件替代（参考第 8 步方式 B），或用 systemd（方式 C，把 `ExecStart` 改成上面的启动命令）。

### 11.7 端到端验证

1. 浏览器打开 `https://your-domain.com` → 点「添加表情包」
2. 填一个真实的包 short_name（如 `SaitamaHero`）
3. 等 1-2 分钟，观察 crawler 进程日志
4. 刷新首页能搜到该包内的 emoji = 全链路通

### 11.8 可选：定期全量更新

**常驻爬虫（11.6 配置的 `tg-emoji-crawler` 守护进程）和全量爬取不能同时运行。** 两者使用同一个 Telegram 登录会话文件，而 Telethon 的会话文件同一时间只能被一个客户端使用，同时运行会出现 `database is locked` 等错误。所以每次全量爬取前要先停掉常驻爬虫，完成后再启动。停止期间网页上提交的「添加表情包」请求会留在信号目录里，恢复后继续处理。

**手动执行**：先在「进程守护管理器」（或 Supervisor 插件）中停止 `tg-emoji-crawler`，然后在宝塔「终端」执行下面的命令，完成后再在面板中启动它：

```bash
cd /www/wwwroot/your-domain.com/crawler && .venv/bin/python crawler.py full
```

**定时执行**：宝塔「计划任务」→「添加任务」，每周跑一次，让数据库跟上 Telegram 新上线的 Featured 包。脚本里需要用命令停止和启动常驻爬虫，写法取决于第 11.6 步用的守护方式：

```bash
# 任务类型：Shell 脚本
# 执行周期：每周日 03:00
# 以 systemd 守护（单元名 tg-emoji-crawler）为例
systemctl stop tg-emoji-crawler
cd /www/wwwroot/your-domain.com/crawler && \
  .venv/bin/python crawler.py full >> /www/wwwroot/your-domain.com/logs/crawler-full.log 2>&1
systemctl start tg-emoji-crawler
```

- 使用 Supervisor 插件时，把两行 `systemctl` 换成 `supervisorctl stop tg-emoji-crawler` 和 `supervisorctl start tg-emoji-crawler`（进程名以插件中填写的名称为准）。
- 使用宝塔「进程守护管理器」时，如果找不到对应的命令行停止方式，请改为手动执行，或把常驻爬虫改用 systemd / Supervisor 守护。
- 脚本中最后一行无论全量爬取是否成功都会执行，保证常驻爬虫恢复运行。

### 11.9 推荐：AI 关键词每夜自动补全（计划任务）

> **背景**：crawler 拉新包时只写入 Telegram 原始关键词，数量通常较少，中文和同义词搜索命中率低；网页上的分类筛选也完全依赖 AI 打标。AI 看图打标由 `backfill_keywords_ai.py` 完成，增量模式会自动跳过已补全的 emoji，所以**每夜定时跑一次**即可让新加的包在 24 小时内补齐关键词和分类。
>
> **AI provider 选择**：
> - **默认 `gemini`**（Google Gemini，默认模型 `gemini-3.5-flash`，可用 `GEMINI_MODEL` 覆盖）— 能访问 Google 时首选；国内服务器需在 `.env` 中配置 `HTTPS_PROXY`。
> - AI 打标脚本不使用 Telegram 登录会话，可以在常驻爬虫运行时执行，不需要停止守护进程。
> - **备选 `zhipu`**（智谱 GLM-4.6V-FlashX）— 内容审核较严格，部分图片会被拒绝（脚本会写入 `ai_filtered` 标记后跳过）。
> - 切换方式：`.env` 改 `AI_PROVIDER`，或命令行加 `--provider zhipu`。
> - 注意：重新全量爬取某个包时，`keywords` 会被 Telegram 原始关键词覆盖（分类保留），需要时用 `--all` 重新补全。

#### Step 1: 先填 API key 到 `.env`

`/www/wwwroot/your-domain.com/crawler/.env` 加（**只需填用到的那个 provider 的 key**）：

```env
# 用 Gemini（默认推荐）
AI_PROVIDER=gemini
GEMINI_API_KEY=你的 Gemini API key
# 拿 key：https://aistudio.google.com/apikey

# 或者用智谱
# AI_PROVIDER=zhipu
# ZAI_API_KEY=你的智谱 API key
# 拿 key：https://bigmodel.cn/usercenter/apikeys
```

⚠️ **确保依赖已按最新的 `requirements.txt` 安装**：宝塔「终端」跑

```bash
cd /www/wwwroot/your-domain.com/crawler
.venv/bin/pip install -r requirements.txt
```

#### Step 2: 手动跑一次验证（推荐）

先小批量验证质量没问题：

```bash
cd /www/wwwroot/your-domain.com/crawler
.venv/bin/python backfill_keywords_ai.py --limit 20
```

#### Step 3: 宝塔「计划任务」加 cron 每夜自动跑

📋 操作位置：宝塔面板 →「计划任务」→「添加任务」

- 任务类型: Shell 脚本
- 任务名称: tg-emoji-ai-backfill
- 执行周期: 每天 / 02:00（错开峰值，避开 full 任务）
- 脚本内容：

```bash
cd /www/wwwroot/your-domain.com/crawler && \
  .venv/bin/python backfill_keywords_ai.py --concurrency 1 \
  >> /www/wwwroot/your-domain.com/logs/ai-backfill.log 2>&1
```

`--concurrency 1` 用于避免触发服务商的请求频率限制，具体配额以服务商说明为准。

#### 触发频率与耗时预估（仅供参考）

| 场景 | 处理量 | 耗时 |
|------|-------|------|
| 首次全量（已跑过，跳过）| 0 | <1 秒 |
| 没新包的夜晚 | 0 | <1 秒就退 |
| 新加 1-3 个包的夜晚 | 几十~一百多 emoji | 5-15 分钟 |
| 极端情况（手动 full 拉了 10+ 新包） | 千级 emoji | 1-3 小时 |

不阻塞 watch、不阻塞用户访问，跑在后台日志里。

> 不想自动也可以每次加包后手动跑 `.venv/bin/python backfill_keywords_ai.py --concurrency 1`。

---

## 第 12 步：权限修复

📋 操作位置：宝塔「终端」

```bash
chown -R www:www /www/wwwroot/your-domain.com/
chmod -R 755 /www/wwwroot/your-domain.com/media/
chmod 640 /www/wwwroot/your-domain.com/config.yaml
chmod 755 /www/wwwroot/your-domain.com/server
chmod -R 755 /www/wwwroot/your-domain.com/var/
```

---

## 第 13 步：启动 + 验证

在进程守护管理器（或 Supervisor / systemctl）里启动 `tg-emoji-hub`，然后在宝塔终端验证：

```bash
# Go server 直连（绕 Nginx）
curl http://127.0.0.1:8080/healthz
# 期望：{"status":"ok","db":"ok","version":"0.1.0"}

# 经 Nginx（验证反代）
curl https://your-domain.com/healthz

# API
curl "https://your-domain.com/api/v1/stats"

# 静态文件（Nginx 直接读磁盘）。把 <id> 换成 media/thumbs/ 下真实存在的文件名
curl -I "https://your-domain.com/thumbs/<id>.webp"

# MCP（Streamable HTTP）：发送 initialize 请求，响应中应包含 serverInfo
curl -X POST "https://your-domain.com/mcp" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"curl","version":"0.0.1"}}}'
```

浏览器访问 `https://your-domain.com/` 看到前端首页 = 部署完成。

**防火墙**（宝塔「安全」→「防火墙」）：放行 80、443；8080 / 3306 **不对外开放**。

服务本身没有登录认证，任何能访问站点的人都可以搜索和添加表情包。如果不打算公开，请在宝塔站点设置中加访问限制（如密码访问或 IP 白名单）。

---

## 常见坑

| 现象 | 根因 | 解决 |
|------|------|------|
| 媒体文件 403 | `www` 用户没有读取权限 | 跑第 12 步权限修复 |
| 数据库 Access denied | config.yaml 密码和宝塔创建的不一致 | 宝塔「数据库」→ 查看密码，同步更新 config.yaml |
| /api/* 全 502/404 | Go server 没起来 | `curl http://127.0.0.1:8080/healthz` 排查进程 |
| MCP 长连接断开 | Nginx 缺 `proxy_buffering off` 或超时配置 | 确认第 9 步 `/mcp` 块完整 |
| 前端白屏 | `root` 路径错 / 缺 `try_files` fallback | 确认 `root` 指向 `dist/`，`try_files` 末尾有 `/index.html` |
| 表不存在 / API 报错 | `mode: prod` 关了 AutoMigrate，dump.sql 没导入 | 先导入 dump.sql（全新安装则执行 `migrations/*.up.sql`），再重启服务 |
| .tgs 文件被当下载 | Nginx 不认 tgs 格式，返回 octet-stream | 确认第 9 步 tgs location 块有正确的 Content-Type 头 |

---

## 升级流程

**只升级后端**：本地 `build.bat backend`（Linux / macOS 可参考裸金属指南 Step 3 直接 `go build`）→ 宝塔文件管理器上传 `dist\server-linux` 覆盖 `/www/wwwroot/your-domain.com/server` → 进程守护「重启」。

**只升级前端**：本地 `build.bat frontend` → 上传 `web-dist.tar.gz` → 解压覆盖 `dist/` → 不需重启 server，浏览器 Ctrl+F5 刷新。

**数据库变更**：在 `migrations/` 找新增的 `.up.sql` → 宝塔「数据库→管理（phpMyAdmin）→SQL」标签页粘贴执行。

---

## 与裸金属部署差异对照

| 项目 | 裸金属方案 | 宝塔方案 |
|------|-----------|---------|
| 文件路径 | `/opt/tg-emoji-hub/` | `/www/wwwroot/your-domain.com/` |
| 进程守护 | systemd unit 文件 | 宝塔进程守护管理器 / Supervisor 插件 |
| Nginx 配置 | `/etc/nginx/conf.d/` 手写文件 | 宝塔「网站→设置→配置文件」Web UI |
| SSL 申请 | certbot 命令行 | 宝塔「SSL→Let's Encrypt」一键 |
| 数据库操作 | mysql 命令行 | 宝塔「数据库」Web UI |
| 运行用户 | 自建 `tg-emoji` 低权限用户 | 宝塔默认 `www` 用户 |
| 日志查看 | `journalctl -u tg-emoji-hub -f` | 进程守护「查看日志」按钮 |
| 适合人群 | 熟悉 Linux 命令行 | 偏好 Web UI 操作 |

两种方案使用同一个 binary 和同一套 config.yaml 格式，可以随时互转。

---

## 注意事项

1. **进程守护管理器入口**：宝塔不同版本（7.x / 8.x / 专业版 / 开源版）该功能入口位置不同，有时在「工具箱」，有时在「Linux 工具」，有时在独立菜单。找不到就用方式 B（Supervisor 插件）或方式 C（systemd），功能完全等价。

2. **进程守护环境变量输入框**：部分宝塔版本没有该字段，此时把环境变量前置到启动命令里即可（第 8 步方式 A 的启动命令已用 `env KEY=VAL ...` 格式处理了这个情况）。
