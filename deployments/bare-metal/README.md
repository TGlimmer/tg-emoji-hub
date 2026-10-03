# tg-emoji-hub 裸金属迁移手册

> This guide is in Chinese. For an English overview, see the [README](../../README.md).

把本地跑好的服务（含 AI 关键词数据）搬到 Linux 服务器，全程 copy-paste。

没有本地数据、想在服务器上从零开始？先看下方「[全新安装（没有本地数据）](#全新安装没有本地数据)」，再按其中列出的步骤执行。

## 总览（一图流）

```
本地
 ├─[Step 1] mysqldump ──── dump.sql ──scp──▶ 服务器 ──[Step 7] mysql 导入 ──▶ MySQL
 ├─[Step 2] tar 媒体 ───── media.tar.gz ─scp─▶ 服务器 ──[Step 8] 解压 ──▶ /opt/tg-emoji-hub/media/
 ├─[Step 3] go build ───── out/server ──scp──▶ 服务器 ──▶ /opt/tg-emoji-hub/bin/server
 ├─[Step 4] npm build ──── web-dist.tar.gz ─scp─▶ 服务器 ──▶ /opt/tg-emoji-hub/web/dist/
 └─[Step 5] config.yaml ──────────scp──▶ 服务器 ──▶ /opt/tg-emoji-hub/configs/config.yaml

服务器
 ├─[Step 6]  scp 上传所有产物（或用脚本批量传）
 ├─[Step 9]  systemd unit 安装 + 启动
 └─[Step 10] Nginx reload → HTTPS 反代 + 静态文件直出
```

一键导出脚本：`deployments/bare-metal/scripts/export-local.sh`（本地跑）
一键导入脚本：`deployments/bare-metal/scripts/import-server.sh`（服务器跑）

---

## 前置条件（服务器侧）

| 组件 | 要求 |
|------|------|
| MySQL | 5.7+ 在跑，有可创建数据库的账号 |
| Nginx | 已装，支持 `include /etc/nginx/conf.d/*.conf` |
| 域名 + SSL | 已解析到本机，证书在 `/etc/letsencrypt/live/<domain>/`（Let's Encrypt 或自签均可） |
| /opt 可写 | 或换其他路径，脚本里统一改 `INSTALL_DIR` 变量 |
| 系统用户 | 建议创建 `tg-emoji` 低权限用户跑服务（脚本会提示） |

---

## 全新安装（没有本地数据）

本手册的主线是「把本地已有数据迁移到服务器」。如果你是第一次部署、本地没有数据，按下面的顺序执行即可：

1. **跳过** Step 1（导出 MySQL）、Step 2（打包媒体）、Step 7 的数据导入、Step 8（解压媒体）。
2. 按 Step 3、Step 4、Step 5、Step 6 准备并上传 server、前端产物和 `config.yaml`（`dump.sql`、`media.tar.gz` 不需要）。另外把仓库中的 `migrations/` 目录一并上传到服务器，例如 `/tmp/tg-emoji-hub-deploy/migrations/`。
3. 在服务器上建库、建账号（Step 7 中的 `CREATE DATABASE` / `CREATE USER` / `GRANT` 部分），然后执行建表脚本：

   ```bash
   cat /tmp/tg-emoji-hub-deploy/migrations/*.up.sql | mysql -u tgemoji -p tg_emoji_hub
   ```

   只执行 `*.up.sql`，`*.down.sql` 是回滚脚本，不要执行。

4. 安装 server 二进制和前端，并创建运行目录：

   ```bash
   sudo mkdir -p /opt/tg-emoji-hub/bin /opt/tg-emoji-hub/web \
     /opt/tg-emoji-hub/media/emoji /opt/tg-emoji-hub/media/thumbs \
     /opt/tg-emoji-hub/var/signals /opt/tg-emoji-hub/logs
   sudo cp /tmp/tg-emoji-hub-deploy/server /opt/tg-emoji-hub/bin/server
   sudo chmod +x /opt/tg-emoji-hub/bin/server
   sudo tar xzf /tmp/tg-emoji-hub-deploy/web-dist.tar.gz -C /opt/tg-emoji-hub/web
   ```

5. 按 Step 9、Step 10 启动 server、配置 Nginx。
6. 按 Step 11 部署爬虫，但**不需要上传 `.session/`**，在 11.4 填好 `.env` 后，直接在服务器上登录并做首次全量爬取：

   ```bash
   cd /opt/tg-emoji-hub/crawler
   sudo chown -R tg-emoji:tg-emoji /opt/tg-emoji-hub/crawler
   sudo -u tg-emoji .venv/bin/python -c "import asyncio; from lib.client import login_interactive; from lib.config import load_config; asyncio.run(login_interactive(load_config()))"
   sudo -u tg-emoji .venv/bin/python crawler.py full
   ```

   登录时按提示输入 Telegram 发送的验证码。之后继续 11.5 及后续步骤。

---

## Step 1: 本地导出 MySQL（含 AI keywords）

```bash
# 在项目根目录执行，替换 -u / -p 参数
mysqldump \
  -u root -p \
  --default-character-set=utf8mb4 \
  --single-transaction \
  --no-tablespaces \
  --routines \
  --triggers \
  tg_emoji_hub \
  > deployments/bare-metal/out/dump.sql

echo "导出行数: $(grep -c '^INSERT' deployments/bare-metal/out/dump.sql)"
```

> `--single-transaction`：对 InnoDB 无锁热备  
> `--no-tablespaces`：目标库没有 PROCESS 权限时跳过表空间信息，避免报错  
> `--default-character-set=utf8mb4`：保证 emoji / 中文 keywords 不乱码

---

## Step 2: 本地打包媒体文件

```bash
# 在项目根目录执行（体积视数据量而定，可能较大）
tar czf deployments/bare-metal/out/media.tar.gz \
  web/public/emoji/ \
  web/public/thumbs/

# 验证
tar tzf deployments/bare-metal/out/media.tar.gz | wc -l
```

打包后目录结构保留 `web/public/emoji/` 和 `web/public/thumbs/`，import 脚本解压时会对应放到正确位置。

---

## Step 3: 本地交叉编译 Linux binary

```bash
# 项目根目录
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-s -w" \
  -o deployments/bare-metal/out/server \
  ./cmd/server/main.go

# 确认是 ELF 格式
file deployments/bare-metal/out/server
# 应输出：ELF 64-bit LSB executable, x86-64 ...
```

> `CGO_ENABLED=0`：Server 不依赖 CGO，关闭后得到静态二进制，不依赖服务器的 glibc 版本。

---

## Step 4: 本地 build 前端

```bash
cd web
npm ci          # 用 lockfile 精确复现依赖，不要用 npm install
npm run build
cd ..

tar czf deployments/bare-metal/out/web-dist.tar.gz \
  -C web \
  dist/

# 验证
tar tzf deployments/bare-metal/out/web-dist.tar.gz | head -5
```

---

## Step 5: 准备服务器侧 config.yaml

复制模板并填写服务器参数：

```bash
cp deployments/bare-metal/configs/config.production.yaml.example \
   deployments/bare-metal/out/config.yaml
```

然后编辑 `deployments/bare-metal/out/config.yaml`，**至少检查以下几项**：

```yaml
server:
  base_url: "https://your-domain.com"   # 改成你的域名
  mode: prod                             # 必须是 prod，关掉 AutoMigrate

database:
  host: localhost                        # 服务器上 MySQL 地址，通常 localhost
  user: tgemoji                          # 数据库账号
  password: "your-db-password"          # 数据库密码（和 dump 来源一致）
  name: tg_emoji_hub                     # 库名

paths:
  media_dir: /opt/tg-emoji-hub/media/emoji
  thumbnail_dir: /opt/tg-emoji-hub/media/thumbs
  signals_dir: /opt/tg-emoji-hub/var/signals
```

> `database.password` 填服务器上 MySQL 账号的密码即可，不要求与本地一致。

---

## Step 6: scp 上传所有产物

```bash
# 替换 user@your-server-ip 为实际 SSH 信息
SERVER="user@your-server-ip"
REMOTE_TMP="/tmp/tg-emoji-hub-deploy"

# 上传到服务器临时目录
ssh $SERVER "mkdir -p $REMOTE_TMP"

scp deployments/bare-metal/out/dump.sql       $SERVER:$REMOTE_TMP/
scp deployments/bare-metal/out/media.tar.gz   $SERVER:$REMOTE_TMP/
scp deployments/bare-metal/out/web-dist.tar.gz $SERVER:$REMOTE_TMP/
scp deployments/bare-metal/out/server         $SERVER:$REMOTE_TMP/
scp deployments/bare-metal/out/config.yaml    $SERVER:$REMOTE_TMP/
scp deployments/bare-metal/scripts/import-server.sh $SERVER:$REMOTE_TMP/
scp deployments/bare-metal/systemd/tg-emoji-crawler.service $SERVER:$REMOTE_TMP/
```

或者用 rsync（断点续传，适合媒体包大的情况）：

```bash
rsync -avz --progress \
  deployments/bare-metal/out/ \
  $SERVER:$REMOTE_TMP/
```

---

## Step 7: 服务器侧导入 MySQL

```bash
# 在服务器上执行（import 脚本会自动做这步，也可手动）

# 先创建数据库（如果还没有）
mysql -u root -p -e "
  CREATE DATABASE IF NOT EXISTS tg_emoji_hub
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;
  CREATE USER IF NOT EXISTS 'tgemoji'@'localhost'
    IDENTIFIED BY 'your-db-password';
  GRANT ALL PRIVILEGES ON tg_emoji_hub.* TO 'tgemoji'@'localhost';
  FLUSH PRIVILEGES;
"

# 导入数据（含 AI 关键词）
mysql -u tgemoji -p tg_emoji_hub < /tmp/tg-emoji-hub-deploy/dump.sql
```

---

## Step 8: 服务器侧解压媒体

```bash
sudo mkdir -p /opt/tg-emoji-hub/media/emoji /opt/tg-emoji-hub/media/thumbs

# 解压（保留目录结构 web/public/emoji/ → 手动 mv 到正确位置）
cd /tmp/tg-emoji-hub-deploy
tar xzf media.tar.gz

sudo mv web/public/emoji/*  /opt/tg-emoji-hub/media/emoji/
sudo mv web/public/thumbs/* /opt/tg-emoji-hub/media/thumbs/

# 确认文件数（应与本地目录中的文件数一致）
ls /opt/tg-emoji-hub/media/emoji/ | wc -l
ls /opt/tg-emoji-hub/media/thumbs/ | wc -l
```

或者直接跑 import 脚本（自动完成 Step 7、8、binary 安装）：

```bash
chmod +x /tmp/tg-emoji-hub-deploy/import-server.sh

SERVER_MYSQL_USER=tgemoji \
SERVER_MYSQL_PASS=your-db-password \
SERVER_MYSQL_DB=tg_emoji_hub \
  bash /tmp/tg-emoji-hub-deploy/import-server.sh \
    /tmp/tg-emoji-hub-deploy/dump.sql \
    /tmp/tg-emoji-hub-deploy/media.tar.gz \
    /tmp/tg-emoji-hub-deploy/web-dist.tar.gz \
    /tmp/tg-emoji-hub-deploy/server
```

---

## Step 9: 安装 systemd unit + 启动

```bash
# 1. 创建低权限用户（已存在则跳过）
sudo useradd --system --no-create-home --shell /sbin/nologin tg-emoji

# 2. 安装 config.yaml
sudo mkdir -p /opt/tg-emoji-hub/configs
sudo cp /tmp/tg-emoji-hub-deploy/config.yaml /opt/tg-emoji-hub/configs/config.yaml
sudo chown -R tg-emoji:tg-emoji /opt/tg-emoji-hub

# 3. 安装 systemd unit
sudo cp deployments/bare-metal/systemd/tg-emoji-hub.service \
        /etc/systemd/system/tg-emoji-hub.service

# 4. 启动
sudo systemctl daemon-reload
sudo systemctl enable tg-emoji-hub
sudo systemctl start tg-emoji-hub

# 5. 确认在跑
sudo systemctl status tg-emoji-hub
```

---

## Step 10: 配置 Nginx

```bash
# 1. 复制站点配置，替换域名
sudo cp deployments/bare-metal/nginx/tg-emoji-hub.conf \
        /etc/nginx/conf.d/tg-emoji-hub.conf

# 替换 __DOMAIN__ 占位符
sudo sed -i 's/__DOMAIN__/your-domain.com/g' /etc/nginx/conf.d/tg-emoji-hub.conf

# 2. 检查语法
sudo nginx -t

# 3. 生效
sudo systemctl reload nginx
```

---

## Step 11: 部署 crawler（Python 爬虫，让"添加表情包"功能工作）

用户在前端点「添加自定义表情包」时，Go server 往 `var/signals/` 写一个 `{short_name}.pending` 信号文件。**crawler 必须以 watch 模式常驻**，才能监听到并从 Telegram 拉取新包。不跑 crawler，添加表情包功能形同虚设。

### 11.1 安装 Python 3.11+

```bash
# 确认版本
python3 --version   # 需要 3.11+

# Ubuntu/Debian（版本不够时）
sudo apt install python3.11 python3.11-venv

# CentOS/RHEL
sudo yum install python3.11
```

### 11.2 上传 crawler 代码

```bash
# 本地打包（排除不必要文件），在本地仓库根目录执行
cd /path/to/tg-emoji-hub
tar czf /tmp/crawler-deploy.tar.gz \
  --exclude='crawler/.venv' \
  --exclude='crawler/__pycache__' \
  --exclude='crawler/.pytest_cache' \
  --exclude='crawler/.ruff_cache' \
  --exclude='crawler/tests' \
  --exclude='crawler/fixtures' \
  crawler/

# 上传到服务器
scp /tmp/crawler-deploy.tar.gz user@your-server-ip:/opt/tg-emoji-hub/

# 服务器侧解压
ssh user@your-server-ip
sudo mkdir -p /opt/tg-emoji-hub/crawler
sudo tar xzf /opt/tg-emoji-hub/crawler-deploy.tar.gz \
  -C /opt/tg-emoji-hub \
  --strip-components=0
```

`.session/` 目录是本地交互登录后生成的 Telegram 会话文件，随代码一起上传后可直接在服务器上使用，无需重新登录。也可以不上传，改为在服务器上登录（见「全新安装」一节中的登录命令）。

> **安全提示**：`.session` 文件等同于已登录的 Telegram 账号，拿到它的人无需密码和验证码即可操作你的账号。只通过 scp / rsync 等加密通道传输，上传后执行 `chmod 600`，不要放在网站目录或任何可被下载的位置。详见 [SECURITY.md](../../SECURITY.md)。

### 11.3 创建虚拟环境 + 安装依赖

```bash
cd /opt/tg-emoji-hub/crawler

# 建虚拟环境
python3 -m venv .venv

# 安装依赖（境内机器加清华镜像；境外机器去掉 -i 参数）
.venv/bin/pip install -i https://pypi.tuna.tsinghua.edu.cn/simple -r requirements.txt

# 确认安装正常
.venv/bin/python -c "import telethon; print('telethon', telethon.__version__)"
```

### 11.4 创建 crawler `.env`

```bash
cat > /opt/tg-emoji-hub/crawler/.env << 'EOF'
# Telegram MTProto 凭据（my.telegram.org → API development tools 申请）
TG_API_ID=你的API_ID
TG_API_HASH=你的API_HASH
TG_PHONE=+86你的手机号
TG_SESSION_NAME=tg-emoji-hub

# MySQL（与 Go server 共用同一个库）
DB_DSN=mysql+pymysql://tgemoji:数据库密码@127.0.0.1:3306/tg_emoji_hub?charset=utf8mb4

# ⚠️ 路径必须和 config.yaml 里 paths.* 完全一致
CRAWLER_SIGNALS_DIR=/opt/tg-emoji-hub/var/signals
THUMBNAIL_DIR=/opt/tg-emoji-hub/media/thumbs
MEDIA_DIR=/opt/tg-emoji-hub/media/emoji
SESSION_DIR=/opt/tg-emoji-hub/crawler/.session

DOWNLOAD_ORIGINALS=1
LOG_LEVEL=INFO

# AI 关键词补全（可选，不需要此功能可留空）
# AI_PROVIDER 默认 gemini，国内不便访问 Google 时改 zhipu
# AI_PROVIDER=gemini
# GEMINI_API_KEY=你的Gemini_API_Key
# ZAI_API_KEY=你的智谱API_Key
EOF
```

路径对齐检查——下表两列必须完全一致：

| 含义 | config.yaml（Go server） | crawler `.env` |
|------|--------------------------|----------------|
| 信号目录 | `paths.signals_dir` | `CRAWLER_SIGNALS_DIR` |
| 缩略图目录 | `paths.thumbnail_dir` | `THUMBNAIL_DIR` |
| 原文件目录 | `paths.media_dir` | `MEDIA_DIR` |

### 11.5 验证 watch 启动正常

```bash
cd /opt/tg-emoji-hub/crawler
.venv/bin/python crawler.py watch
# 看到「启动信号监听 dir=... interval=2.0s」日志 → OK，Ctrl+C 退出
```

如果报「session 未授权」，`.session/` 目录没上传或 `SESSION_DIR` 路径填错，重新检查 11.2。

### 11.6 安装 systemd unit + 启动

```bash
# 安装 service 文件（先把项目 deployments/ 目录整体 scp 上传到服务器，或手动创建）
sudo cp /tmp/tg-emoji-hub-deploy/tg-emoji-crawler.service \
        /etc/systemd/system/tg-emoji-crawler.service
# 如果没有 scp，也可以在服务器上直接 cat > /etc/systemd/system/tg-emoji-crawler.service
# 把 deployments/bare-metal/systemd/tg-emoji-crawler.service 的内容粘贴进去

# 调整属主（crawler 目录让 tg-emoji 用户可读写）
sudo chown -R tg-emoji:tg-emoji /opt/tg-emoji-hub/crawler

sudo systemctl daemon-reload
sudo systemctl enable tg-emoji-crawler
sudo systemctl start tg-emoji-crawler

# 确认在跑
sudo systemctl status tg-emoji-crawler
sudo journalctl -u tg-emoji-crawler -f
```

### 11.7 端到端验证

1. 浏览器打开 `https://your-domain.com` → 点「添加表情包」
2. 填一个真实的包 short_name（如 `SaitamaHero`）
3. 等 1-2 分钟，`journalctl -u tg-emoji-crawler -f` 查日志
4. 刷新首页能搜到该包内的 emoji = 全链路通

### 11.8 可选：定期全量更新

**常驻爬虫（`tg-emoji-crawler` 服务）和全量爬取不能同时运行。** 两者使用同一个 Telegram 登录会话文件，而 Telethon 的会话文件同一时间只能被一个客户端使用，同时运行会出现 `database is locked` 等错误。所以每次全量爬取前要先停掉常驻服务，完成后再启动。

手动执行：

```bash
sudo systemctl stop tg-emoji-crawler
cd /opt/tg-emoji-hub/crawler && sudo -u tg-emoji .venv/bin/python crawler.py full
sudo systemctl start tg-emoji-crawler
```

定时执行（每周日 03:00）。停启服务需要 root 权限，所以这条任务以 root 运行，爬取本身仍以 `tg-emoji` 用户执行：

```bash
# /etc/cron.d/tg-emoji-crawler-full
echo '0 3 * * 0  root  systemctl stop tg-emoji-crawler && cd /opt/tg-emoji-hub/crawler && sudo -u tg-emoji .venv/bin/python crawler.py full >> /opt/tg-emoji-hub/logs/crawler-full.log 2>&1; systemctl start tg-emoji-crawler' \
  | sudo tee /etc/cron.d/tg-emoji-crawler-full
sudo chmod 644 /etc/cron.d/tg-emoji-crawler-full
```

`systemctl start` 前面用的是 `;`，即使全量爬取失败，常驻服务也会恢复。停止期间网页上提交的「添加表情包」请求会留在信号目录里，服务恢复后继续处理。每周全量一次可以让 Featured 包列表保持最新。

### 11.9 推荐：AI 关键词每夜自动补全（cron）

> **背景**：crawler 拉新包时只写入 Telegram 原始关键词，数量通常较少，中文和同义词搜索命中率低；网页上的分类筛选也完全依赖 AI 打标。AI 看图打标由 `backfill_keywords_ai.py` 完成，增量模式会自动跳过已补全的 emoji，所以**每夜定时跑一次**即可让新加的包在 24 小时内补齐关键词和分类。
>
> **AI provider 选择**：
> - **默认 `gemini`**（Google Gemini，默认模型 `gemini-3.5-flash`，可用 `GEMINI_MODEL` 覆盖）— 能访问 Google 时首选；国内服务器需配置 `HTTPS_PROXY`。
> - AI 打标脚本不使用 Telegram 登录会话，可以在常驻爬虫运行时执行，不需要停服务。
> - **备选 `zhipu`**（智谱 GLM-4.6V-FlashX）— 内容审核较严格，部分图片会被拒绝（脚本会写入 `ai_filtered` 标记后跳过）。
> - 切换方式：`.env` 改 `AI_PROVIDER`，或命令行加 `--provider zhipu`。
> - 注意：重新全量爬取某个包时，`keywords` 会被 Telegram 原始关键词覆盖（分类保留），需要时用 `--all` 重新补全。

#### Step 1: 填 API key 到 `.env`

`/opt/tg-emoji-hub/crawler/.env` 加（**只需填用到的那个 provider 的 key**）：

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

⚠️ **确保依赖已按最新的 `requirements.txt` 安装**：

```bash
cd /opt/tg-emoji-hub/crawler
sudo -u tg-emoji .venv/bin/pip install -r requirements.txt
```

#### Step 2: 手动跑一次验证

```bash
cd /opt/tg-emoji-hub/crawler
sudo -u tg-emoji .venv/bin/python backfill_keywords_ai.py --limit 20
```

#### Step 3: 加 cron 每夜自动跑

```bash
# /etc/cron.d/tg-emoji-ai-backfill
echo '0 2 * * *  tg-emoji  cd /opt/tg-emoji-hub/crawler && .venv/bin/python backfill_keywords_ai.py --concurrency 1 >> /opt/tg-emoji-hub/logs/ai-backfill.log 2>&1' \
  | sudo tee /etc/cron.d/tg-emoji-ai-backfill
sudo chmod 644 /etc/cron.d/tg-emoji-ai-backfill
```

每天 02:00 跑一次（错开 03:00 的 full crawl）。`--concurrency 1` 用于避免触发服务商的请求频率限制，具体配额以服务商说明为准。

#### 耗时预估（仅供参考）

| 场景 | 处理量 | 耗时 |
|------|-------|------|
| 没新包的夜晚 | 0 | <1 秒退出 |
| 新加 1-3 个包 | 几十~一百多 emoji | 5-15 分钟 |
| 极端（手动 full 拉了 10+ 新包） | 千级 emoji | 1-3 小时 |

不阻塞 watch、不阻塞用户访问。日志在 `/opt/tg-emoji-hub/logs/ai-backfill.log`。

---

## 验证

```bash
# 健康检查（经 Nginx 转发）
curl https://your-domain.com/healthz

# 直接打后端（绕过 Nginx，确认 Go server 在跑）
curl http://localhost:8080/healthz

# API
curl "https://your-domain.com/api/v1/stats"

# 静态文件（Nginx 直接读磁盘，不经过 Go）。把 <id> 换成 media/thumbs/ 下真实存在的文件名
curl -I "https://your-domain.com/thumbs/<id>.webp"
# 应返回 200，并带有 Cache-Control: public, max-age=31536000, immutable

# MCP（Streamable HTTP）：发送 initialize 请求，响应中应包含 serverInfo
curl -X POST "https://your-domain.com/mcp" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"curl","version":"0.0.1"}}}'
```

---

## 后续运维

```bash
# 查实时日志
sudo journalctl -u tg-emoji-hub -f

# 查最近 200 行
sudo journalctl -u tg-emoji-hub -n 200

# 重启服务
sudo systemctl restart tg-emoji-hub

# 升级（只换二进制，不动数据）：
# 本地 Step 3 重新编译 → scp 上传 → 替换 → restart
scp out/server user@server:/opt/tg-emoji-hub/bin/server.new
ssh user@server "
  sudo mv /opt/tg-emoji-hub/bin/server.new /opt/tg-emoji-hub/bin/server
  sudo chown tg-emoji:tg-emoji /opt/tg-emoji-hub/bin/server
  sudo chmod +x /opt/tg-emoji-hub/bin/server
  sudo systemctl restart tg-emoji-hub
"

# 改配置后生效
sudo vim /opt/tg-emoji-hub/configs/config.yaml
sudo systemctl restart tg-emoji-hub
```

---

## 常见坑

**坑 1：CGO**  
Server 不依赖 CGO，`CGO_ENABLED=0` 交叉编译没问题。  
如果日后引入需要 CGO 的依赖，服务器必须装对应版本的 glibc，或改用 Docker。

**坑 2：MySQL 字符集**  
导出时必须加 `--default-character-set=utf8mb4`，导入的目标库也必须是 utf8mb4。  
中文 keywords 和 emoji 如果乱码，99% 是这里没配。

**坑 3：Nginx 静态文件权限**  
`/opt/tg-emoji-hub/media/` 下的文件要让 `www-data`（Nginx worker 用户）能读。  
做法：`sudo chmod -R o+r /opt/tg-emoji-hub/media/` 或把 Nginx worker 用户改成 `tg-emoji`。

**坑 4：media 路径与 config.yaml 不对齐**  
Go server 的 `paths.media_dir` 和 `paths.thumbnail_dir` 必须和实际文件放置位置一致。  
Nginx 的 `alias` 也必须和 Go server 的目录一样。  
三处路径配置不一致会导致 `/thumbs/*.webp` 有时 Nginx 直出、有时走 Go，行为混乱。

**坑 5：mode: prod 关掉了 AutoMigrate**  
生产用 `mode: prod`，Go server 不会自动建表。首次部署必须先导入 dump.sql（包含建表语句），或者在全新安装时执行 `migrations/` 下的 `*.up.sql`。  
如果表结构有变更，需要按编号顺序手动执行新增的 `*.up.sql`。

**坑 6：MCP 长连接被 Nginx 默认超时断掉**  
Nginx 默认 `proxy_read_timeout 60s`，MCP Streamable HTTP 是长连接，会被截断。  
本配置已改成 3600s，但如果你在外层还有其他 Nginx / CDN / 防火墙，也要同步调整。
