.PHONY: env build up down logs ps clean crawler-login crawler-up crawler-down crawler-full crawler-dry test test-py

COMPOSE = docker compose -f deployments/docker-compose.yml
# 爬虫服务挂在 crawler profile 下，默认 make up 不会启动它
COMPOSE_CRAWLER = $(COMPOSE) --profile crawler

# ===== 容器编排 =====

# 生成 deployments/.env（已存在则不动），然后手动填两个数据库密码
env:
	@test -f deployments/.env && echo "deployments/.env 已存在，跳过" || (cp deployments/.env.example deployments/.env && echo "已生成 deployments/.env，请至少填 MYSQL_ROOT_PASSWORD 和 MYSQL_PASSWORD")

build:
	$(COMPOSE_CRAWLER) build

# mysql + server + web，不含爬虫
up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE_CRAWLER) down

ps:
	$(COMPOSE_CRAWLER) ps

logs:
	$(COMPOSE_CRAWLER) logs -f --tail=100

# 连数据卷一起删，数据库内容会丢
clean:
	$(COMPOSE_CRAWLER) down -v

# ===== 爬虫 =====

# 首次必做：交互输入手机号验证码，session 存进 tg_session 卷。需要先在 deployments/.env 填 TG_API_ID / TG_API_HASH / TG_PHONE
crawler-login:
	$(COMPOSE_CRAWLER) run --rm --entrypoint python crawler -c \
		"import asyncio; from lib.client import login_interactive; from lib.config import load_config; asyncio.run(login_interactive(load_config()))"

# 起常驻爬虫，监听 server 写入的 signals
crawler-up:
	$(COMPOSE_CRAWLER) up -d --build crawler

crawler-down:
	$(COMPOSE_CRAWLER) stop crawler

# 全量爬 Featured 包 + 库里 manual 包，登录后手动跑一次
crawler-full:
	$(COMPOSE_CRAWLER) run --rm crawler full

# 干跑验证：FakeClient + 内存 sqlite，不联网不入库；输出目录指到 /tmp，不污染共享卷
crawler-dry:
	$(COMPOSE_CRAWLER) run --rm -e THUMBNAIL_DIR=/tmp/t -e MEDIA_DIR=/tmp/m -e CRAWLER_SIGNALS_DIR=/tmp/s crawler full --dry-run

# ===== 本地开发 =====

test:
	go test ./... -count=1 -timeout 120s

test-py:
	cd crawler && python -m pytest -q
