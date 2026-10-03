# tg-emoji-hub

**English** | [简体中文](README.zh-CN.md)

A self-hosted search engine for Telegram Premium custom emoji. Find an emoji by keyword or by the standard emoji it represents, and copy its `custom_emoji_id` in the format your bot code needs. It includes a web UI, a JSON API, and an MCP server so AI coding assistants can look up IDs for you.

## Why

Telegram bots send custom emoji with the HTML tag

```html
<tg-emoji emoji-id="5404674261681232568">🔥</tg-emoji>
```

or the equivalent MarkdownV2 syntax. The ID is not visible anywhere in the Telegram apps. Getting one usually means opening the client, finding the pack, sending the emoji to a helper bot to read its `custom_emoji_id`, and pasting it into a lookup table in your code. Doing that for every emoji gets tedious quickly.

tg-emoji-hub crawls custom emoji packs into a database once, and then lets you:

- search in a browser and copy the ID or a ready-to-paste snippet with one click, or
- let Claude Code, Cursor, Codex or another MCP client call `search_emoji("fire")` while it writes your bot code.

## Features

- **Search** by keyword (`fire`, `cat`, `火`) or by emoji character (`🔥`), with relevance ranking that puts exact and whole-word matches first
- **Four copy formats**: raw ID, `<tg-emoji>` HTML tag, Go map entry, JSON object
- **Filters**: by pack, by one of 11 categories (emotion, animal, person, food, crypto, object, symbol, gesture, meme, vehicle, nature), animated only
- **Animated previews** for video (`.webm`) and Lottie (`.tgs`) emoji
- **Chinese and English interface**, switchable from the header
- **Add any pack** by its short name; the crawler fetches it in the background
- **MCP server** with five tools, over Streamable HTTP or stdio
- **JSON API** for your own tooling

What gets indexed: Telegram's featured custom emoji packs, plus every pack you add. Keyword search uses the keywords Telegram provides; an optional AI tagging step adds Chinese and English keywords and assigns the category used by the category filter.

## How it works

| Part | Technology | Role |
|------|------------|------|
| Server | Go, Gin, GORM, [mcp-go](https://github.com/mark3labs/mcp-go) | JSON API, MCP server, thumbnails and original files |
| Crawler | Python, [Telethon](https://github.com/LonamiWebs/Telethon) | Logs in as a Telegram user, downloads packs into MySQL and onto disk |
| Web UI | React 19, Vite, Tailwind CSS, shadcn/ui | Search page (static files, served by Nginx) |
| Database | MySQL 8 | Packs and emoji |

The server and crawler communicate through the database and a shared directory of signal files. See [docs/architecture.md](docs/architecture.md) for details.

## Before you start

- **Filling the index needs a Telegram user account.** The crawler uses the MTProto API as a normal user, because the Bot API cannot list featured packs or return emoji keywords. Create an API ID and hash at [my.telegram.org](https://my.telegram.org) under "API development tools".
- **The crawler's session is a login credential.** Anyone who has your API credentials and the Telethon `.session` file can use your Telegram account. Never commit or share them. See [SECURITY.md](SECURITY.md).
- **There is no authentication.** Anyone who can reach the web port can search and add packs. Keep it on a private network, or put a reverse proxy with HTTPS and access control in front of it.
- **Categories need AI tagging.** The category filter only works after running the optional tagging step, which needs a Google Gemini or Zhipu API key.
- **Sending custom emoji from a bot has its own requirements.** According to the Bot API documentation, a bot can use custom emoji if it has purchased additional usernames on Fragment, or in messages it sends directly to private chats, groups and supergroups when the bot's owner has Telegram Premium. tg-emoji-hub only helps you find the IDs.

## Quick start with Docker

Requirements: Docker with the Compose plugin, and `make`. Each `make` target is a short wrapper around `docker compose -f deployments/docker-compose.yml ...`; without `make`, run the commands from the [Makefile](Makefile) directly. For example, `make up` is `docker compose -f deployments/docker-compose.yml up -d --build`.

### 1. Start the stack

```bash
git clone https://github.com/TGlimmer/tg-emoji-hub.git
cd tg-emoji-hub
make env
```

`make env` creates `deployments/.env` from the template. This one file holds all Docker settings. Set at least:

| Variable | Description |
|----------|-------------|
| `MYSQL_ROOT_PASSWORD`, `MYSQL_PASSWORD` | Required. Use letters and digits only; the password is inserted into a YAML file and a database URL. |
| `HTTP_PORT` | Port for the browser and MCP clients. Default `8080`. |
| `PUBLIC_URL` | The address users open in the browser. Thumbnail and media URLs returned by the API start with it. Default `http://localhost:8080`. |

Then build and start MySQL, the server and the web front end:

```bash
make up
make ps          # wait until mysql, server and web are all healthy
curl http://localhost:8080/healthz
```

Open <http://localhost:8080>. The page works, but the index is still empty.

### 2. Fill the index with the crawler

The crawler is optional in Compose terms (it is in the `crawler` profile and `make up` does not start it), but without it there is nothing to search.

Add your Telegram credentials to `deployments/.env`: `TG_API_ID`, `TG_API_HASH` and `TG_PHONE` (international format). Then:

```bash
make crawler-login   # interactive: enter the login code Telegram sends you
make crawler-full    # first full crawl of the featured packs; this takes a while
make crawler-up      # keep the crawler running so packs added in the web UI are fetched
```

The login session is stored in the `tg_session` Docker volume and reused after restarts. `make crawler-dry` runs the crawler against fake data without network access, as a self-check.

The resident crawler and a full crawl cannot run at the same time, because they use the same Telegram session file and Telethon allows only one client per session file. For later full crawls, stop the resident crawler first:

```bash
make crawler-down && make crawler-full && make crawler-up
```

Packs submitted in the web UI while the crawler is stopped stay queued in the signals directory and are processed after it restarts.

### 3. Optional: AI keywords and categories

Set `GEMINI_API_KEY` (or `AI_PROVIDER=zhipu` and `ZAI_API_KEY`) in `deployments/.env`. If your machine cannot reach the Gemini API directly, also set `HTTPS_PROXY`:

- The proxy address must be reachable from inside the container, so do not use `127.0.0.1` (inside a container that is the container itself). On Docker Desktop (Windows, macOS) use `http://host.docker.internal:<port>`; on Linux use the host's LAN IP and make sure the proxy does not listen only on `127.0.0.1`.
- Docker Compose gives variables from your shell priority over `deployments/.env`. If `HTTPS_PROXY` is already set in your terminal, run `unset HTTPS_PROXY` before the compose command.

Then run the tagging script in a one-off crawler container (it does not use the Telegram session, so it can run while the resident crawler is running):

```bash
docker compose -f deployments/docker-compose.yml --profile crawler \
  run --rm --entrypoint python crawler backfill_keywords_ai.py
```

### Stop, remove, upgrade

```bash
make down    # stop all services
make clean   # stop and delete all volumes, including the database
```

The schema in `migrations/` is applied automatically only when the MySQL volume is created for the first time. When upgrading an existing installation, apply any new `*.up.sql` files yourself; see [deployments/README.md](deployments/README.md).

### Going to production

- Set `PUBLIC_URL` to your public address, for example `https://emoji.example.com`.
- The web container listens on plain HTTP. Put an HTTPS reverse proxy in front of it; [deployments/bare-metal/nginx/](deployments/bare-metal/nginx/) contains an Nginx configuration you can adapt.
- Adding a pack makes the server request `https://t.me/addemoji/<name>` to check that the pack exists, so the server needs outbound internet access for that check. If the request fails, the check is skipped after a timeout of up to 10 seconds.

The full Docker guide, including troubleshooting, is in [deployments/README.md](deployments/README.md) (Chinese).

## Connect an AI assistant (MCP)

With the stack running, the MCP endpoint is `http://localhost:8080/mcp` (Streamable HTTP).

**Claude Code**

```bash
claude mcp add --transport http tg-emoji-hub http://localhost:8080/mcp
```

**Cursor** (`~/.cursor/mcp.json` or `.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "tg-emoji-hub": { "url": "http://localhost:8080/mcp" }
  }
}
```

**Codex CLI** (`~/.codex/config.toml`)

```toml
[mcp_servers.tg-emoji-hub]
url = "http://localhost:8080/mcp"
```

**Claude Desktop** launches local commands, so it uses stdio mode. With the Docker stack running:

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

Available tools: `search_emoji`, `get_emoji_by_id`, `list_packs`, `list_pack_stickers`, `search_in_pack`. Full reference, stdio setup without Docker and troubleshooting: [docs/mcp.md](docs/mcp.md).

## Deploying without Docker

Two step-by-step guides (in Chinese) cover running the server and crawler directly on a Linux server with Nginx and HTTPS, including scheduled full crawls and AI tagging:

| Guide | For |
|-------|-----|
| [deployments/bare-metal/README.md](deployments/bare-metal/README.md) | Command line, systemd and Nginx |
| [deployments/baota/README.md](deployments/baota/README.md) | The BaoTa (BT) server control panel |

Both are written as moving an existing local installation to a server, and both include a section on a fresh installation without local data.

## Local development

Requirements: Go 1.26+, Node.js 22, Python 3.11+, MySQL 8.

**Database.** Use any MySQL 8 instance, or start only the database from the Compose files with port 3306 published on `127.0.0.1` (needs `deployments/.env`, see the quick start):

```bash
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.dev.yml up -d mysql
```

**Server**

```bash
cp configs/config.yaml.example configs/config.yaml   # set database.* for your MySQL
go run ./cmd/server                                   # http://localhost:8080
```

With `server.mode: dev` (the default in the template), the server creates or updates the tables at startup.

**Web UI**

```bash
cd web
npm ci
npm run dev    # http://127.0.0.1:5173
```

The dev server proxies `/api`, `/healthz` and `/mcp` to `http://localhost:8080`. Thumbnails and animations are loaded from the URLs the API returns, which are built from `server.base_url`, so keep it at `http://localhost:8080` during development.

**Crawler**

```bash
cd crawler
python -m venv .venv
source .venv/bin/activate          # Windows: .venv\Scripts\activate
pip install -r requirements.txt
cp .env.example .env
```

For a local setup next to `go run`, set these in `crawler/.env` so both processes use the same directories:

```dotenv
DB_DSN=mysql+pymysql://tgemoji:<password>@127.0.0.1:3306/tg_emoji_hub?charset=utf8mb4
CRAWLER_SIGNALS_DIR=../var/signals
THUMBNAIL_DIR=../web/public/thumbs
MEDIA_DIR=../web/public/emoji
SESSION_DIR=./.session
```

Then log in once and crawl:

```bash
python -c "import asyncio; from lib.client import login_interactive; from lib.config import load_config; asyncio.run(login_interactive(load_config()))"
python crawler.py full     # featured packs + packs added through the UI
python crawler.py watch    # keep running to process packs added through the UI
```

`full` and `watch` use the same session file and cannot run at the same time. Stop `watch` before running `full` again.

Optional AI tagging (needs `GEMINI_API_KEY` or `ZAI_API_KEY` in `crawler/.env`):

```bash
python backfill_keywords_ai.py --limit 20   # try a small batch first
python backfill_keywords_ai.py
```

More crawler options are in [crawler/README.md](crawler/README.md) (Chinese). How to run the tests and submit changes: [CONTRIBUTING.md](CONTRIBUTING.md).

## Documentation

| Document | Content |
|----------|---------|
| [docs/architecture.md](docs/architecture.md) | Components, data model, search scoring, crawl flow, configuration reference |
| [docs/api.md](docs/api.md) | HTTP API reference |
| [docs/mcp.md](docs/mcp.md) | MCP tools and client setup |
| [deployments/README.md](deployments/README.md) | Docker deployment (Chinese) |
| [deployments/bare-metal/README.md](deployments/bare-metal/README.md) | Linux + systemd + Nginx deployment (Chinese) |
| [deployments/baota/README.md](deployments/baota/README.md) | BaoTa panel deployment (Chinese) |
| [crawler/README.md](crawler/README.md) | Crawler usage (Chinese) |
| [web/README.md](web/README.md) | Web UI development (Chinese) |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Development and pull requests |
| [SECURITY.md](SECURITY.md) | Reporting vulnerabilities, protecting credentials |

## Community

Questions and discussion: [Telegram group](https://t.me/devqun). Bug reports and feature requests: GitHub Issues.

## License

[MIT](LICENSE)
