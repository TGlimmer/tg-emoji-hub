# Architecture

This document explains how tg-emoji-hub is put together: which process does what, how they talk to each other, how data is stored and searched, and why some of the less obvious design choices were made. It is written for people who deploy the project or want to contribute to it.

## Contents

- [Overview](#overview)
- [Components](#components)
- [Server-crawler communication: signal files](#server-crawler-communication-signal-files)
- [Data model](#data-model)
- [Search and relevance scoring](#search-and-relevance-scoring)
- [Crawling](#crawling)
- [AI keyword and category tagging](#ai-keyword-and-category-tagging)
- [Configuration reference](#configuration-reference)
- [Design decisions and trade-offs](#design-decisions-and-trade-offs)
- [Known limitations](#known-limitations)
- [Repository layout](#repository-layout)

## Overview

```
                 ┌────────────────────────────┐
                 │  Telegram (MTProto API)     │
                 └─────────────┬──────────────┘
                               │ Telethon, user account
                               ▼
┌──────────────────────────────────────────────┐        ┌──────────────────────┐
│ Crawler (Python)                              │        │ AI provider          │
│  full   : featured packs + manual packs       │◀──────▶│ Gemini or Zhipu      │
│  watch  : polls signals dir for new packs     │ optional (backfill script)    │
│  backfill_keywords_ai.py (optional)           │        └──────────────────────┘
└───────┬───────────────────────┬──────────────┘
        │ upsert rows           │ writes thumbnails / original files
        ▼                       ▼
┌──────────────┐      ┌─────────────────────────────┐
│ MySQL        │      │ Shared directories           │
│ sticker_packs│      │  thumbnails  originals       │
│ emoji_stickers      │  signals (*.pending)         │
└──────┬───────┘      └──────────────┬──────────────┘
       │ GORM                         │ reads files / writes *.pending
       ▼                              ▼
┌──────────────────────────────────────────────┐
│ Server (Go, single binary)                    │
│  /api/v1/*      JSON API (Gin)                │
│  /mcp           MCP, Streamable HTTP          │
│  --mcp-stdio    MCP over stdio (separate run) │
│  /thumbs, /emoji static media                 │
└───────┬──────────────────────┬───────────────┘
        │                      │
  Web UI (React, static)   AI coding assistants
  in a browser             (Claude Code, Cursor, ...)
```

There are three moving parts plus MySQL:

1. **Crawler** (Python): logs in to Telegram as a user, downloads custom emoji packs and writes them to MySQL and to disk.
2. **Server** (Go): read-mostly API over MySQL, the MCP server, and static file serving for thumbnails and original files.
3. **Web UI** (React): a static single-page app that calls the server's API.

The server and the crawler never call each other over the network. They share the database and a few directories, and the server asks the crawler to fetch a new pack by dropping a file into a shared directory.

## Components

### Server (`cmd/server`, `internal/`, `pkg/`)

A single Go binary built with Gin, GORM (MySQL driver) and [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go). The code follows a ports-and-adapters layout:

| Package | Responsibility |
|---------|----------------|
| `internal/domain/entity` | `StickerPack`, `EmojiSticker`, and the allowed values for `source`, `mime_type` and `category` |
| `internal/domain/port` | Repository interfaces and filter types |
| `internal/app` | Use cases: search, pack listing and adding, stats |
| `internal/adapter/handler` | HTTP handlers and route registration |
| `internal/adapter/mcp` | MCP tool definitions; reuses the same use cases as HTTP |
| `internal/adapter/repository/mysql` | GORM implementations of the repositories |
| `pkg/formats` | Builds the four copy formats (`id`, `html`, `go_map`, `json`) |
| `pkg/errors`, `pkg/response` | Error codes and the `{code, data, message}` envelope |
| `cmd/server/bootstrap` | Configuration, database, HTTP server, MCP wiring, graceful shutdown |

Two run modes:

- **Default**: HTTP server with the API, the MCP Streamable HTTP endpoint (unless `mcp.enabled: false`), `/thumbs/*` and `/emoji/*`.
- **`--mcp-stdio`**: MCP over stdin/stdout only. No HTTP listener; logs go to stderr so they do not corrupt the JSON-RPC stream.

Both modes connect to MySQL at startup and exit if it is unreachable. The HTTP API and MCP tools are rendered from the same use cases, so their emoji objects are identical (see [api.md](api.md) and [mcp.md](mcp.md)).

The server does not serve the web UI files. The static build (`web/dist`) is served by Nginx, which also proxies the server's paths:

- **Docker**: the `web` container (Nginx) is the only published port. It serves the UI and proxies `/api/`, `/healthz`, `/thumbs/`, `/emoji/` and `/mcp` to the `server` container. MySQL and the server are not reachable from the host.
- **Bare metal / BaoTa panel**: Nginx serves the UI and reads `/thumbs/` and `/emoji/` directly from disk, and proxies `/api/`, `/healthz` and `/mcp`; see the guides under `deployments/`.
- **Development**: the Vite dev server proxies `/api`, `/healthz` and `/mcp`.

### Crawler (`crawler/`)

Python with [Telethon](https://github.com/LonamiWebs/Telethon) and SQLAlchemy. It logs in with a Telegram **user** account (API ID and hash from [my.telegram.org](https://my.telegram.org)) rather than a bot. The Bot API can fetch a sticker set by name, but it has no method for listing Telegram's featured custom emoji packs, and its sticker objects do not carry the keyword lists that MTProto returns with a set. Both are needed here.

| Entry point | What it does |
|-------------|--------------|
| `crawler.py full` | Crawl all featured custom emoji packs, then every pack in the database with `source = manual` |
| `crawler.py watch` | Long-running: poll the signals directory and crawl each requested pack |
| `single_pack.py <short_name>` | Crawl one pack by short name (stored as `manual`) |
| `backfill_keywords_ai.py` | Optional: ask a vision model for keywords and a category for each emoji |

`full`, `watch` and `single_pack.py` accept `--dry-run`, which uses a fake Telegram client and an in-memory SQLite database, so the pipeline can be exercised without credentials. `full` also accepts `--no-thumbs` and `--no-originals`.

The Telethon session file (`SESSION_DIR/TG_SESSION_NAME.session`) is created once by an interactive login and reused afterwards. It grants full access to the Telegram account; see [SECURITY.md](../SECURITY.md).

### Web UI (`web/`)

React 19 + TypeScript, built with Vite, styled with Tailwind CSS and shadcn/ui-style components on Radix primitives, state in Zustand, HTTP via Axios. Features:

- Search as you type (300 ms debounce), pack filter, category chips, animated-only toggle
- Animated previews: `.webm` with `<video>`, `.tgs` with `lottie-web` (loaded as a separate chunk)
- Copy menu with the four formats; `Enter` on a focused card copies the HTML format
- Add-a-pack dialog (`POST /api/v1/packs`) and an MCP setup dialog
- Chinese and English interface; Chinese is the default, and the choice is stored in `localStorage`

## Server-crawler communication: signal files

Adding a pack from the web UI or API works without any message broker:

```
POST /api/v1/packs {"short_name": "X"}
  └─ server: create sticker_packs row (source=manual, last_crawled_at=NULL)
  └─ server: write  <signals_dir>/X.pending   {"created_at": <unix>, "retry_count": 0}
                                  │
crawler watch (polls every 2 s) ──┘
  └─ crawl pack X → upsert rows, download files
  └─ success: delete X.pending
  └─ failure: rewrite X.pending with retry_count + 1
              after 5 failures: rename to X.failed (no further retries)
```

Details:

- Both sides validate the short name against `^[A-Za-z][A-Za-z0-9_]{0,63}$` and refuse paths outside the signals directory, so a signal file name cannot be used for path traversal.
- The watcher polls instead of using filesystem notifications, which behave inconsistently across platforms and inside containers. The cost is up to two seconds of latency.
- Pending files are processed one at a time in name order. A failed pack is retried on later polls until it reaches the retry limit.
- `.failed` files are left in place for an operator to inspect. Packs that never crawled successfully are still picked up by the next `full` run, because `full` re-crawls every `manual` pack in the database.

For this to work, three directories must be the same physical directory for both processes (a shared Docker volume or the same host path):

| Purpose | Server (`config.yaml`) | Crawler (`.env`) |
|---------|------------------------|------------------|
| Signals | `paths.signals_dir` | `CRAWLER_SIGNALS_DIR` |
| Thumbnails | `paths.thumbnail_dir` | `THUMBNAIL_DIR` |
| Original files | `paths.media_dir` | `MEDIA_DIR` |

## Data model

Two tables, created by `migrations/*.up.sql` (or by GORM AutoMigrate when the server runs with `server.mode: dev`).

### `sticker_packs`

| Column | Type | Notes |
|--------|------|-------|
| `id` | BIGINT UNSIGNED, PK | |
| `short_name` | VARCHAR(64), unique | Telegram short name, the last segment of `t.me/addemoji/...` |
| `title` | VARCHAR(255) | Set to the short name on manual creation, replaced by the real title when crawled |
| `tg_pack_id` | VARCHAR(32) | Telegram sticker set ID |
| `is_featured` | TINYINT(1) | |
| `sticker_count` | INT | As reported by Telegram |
| `source` | ENUM(`featured`, `manual`) | How the pack entered the index |
| `last_crawled_at` | TIMESTAMP NULL | `NULL` until the first successful crawl |
| `created_at`, `updated_at` | TIMESTAMP | |

### `emoji_stickers`

| Column | Type | Notes |
|--------|------|-------|
| `id` | BIGINT UNSIGNED, PK | |
| `custom_emoji_id` | VARCHAR(32), unique | Telegram `Document.id`; the idempotency key for upserts |
| `alt` | VARCHAR(16) | Standard emoji the custom emoji represents |
| `keywords` | TEXT | Comma-separated: Telegram emoticons and keywords, plus AI keywords if tagging was run |
| `category` | VARCHAR(20) | One of 11 values or `''` (not classified) |
| `pack_id` | BIGINT UNSIGNED, FK → `sticker_packs.id` (ON DELETE CASCADE) | |
| `thumbnail_path` | VARCHAR(255) | Relative path such as `thumbs/<id>.webp`, or `''` if the download failed |
| `mime_type` | VARCHAR(64) | `image/webp`, `video/webm` or `application/x-tgsticker` |
| `is_free` | TINYINT(1) | Telegram's `free` flag |
| `is_animated` | TINYINT(1) | `true` for webm and tgs |
| `created_at`, `updated_at` | TIMESTAMP | |

Indexes on `emoji_stickers`: `alt`, `(pack_id, alt)`, `category`, and a FULLTEXT index `ft_search (keywords, alt)`.

The categories are `emotion`, `animal`, `person`, `food`, `crypto`, `object`, `symbol`, `gesture`, `meme`, `vehicle`, `nature`. The same list exists in the Go entity package, the AI tagging script and the web UI.

Files on disk are named by ID: thumbnails `<thumbnail_dir>/<custom_emoji_id>.webp`, originals `<media_dir>/<custom_emoji_id>.<webp|webm|tgs>`.

## Search and relevance scoring

`GET /api/v1/search` and the MCP search tools share one repository method. Filters (`pack`, `category`, `animated_only`) are plain `AND` conditions. With a query `q`, each candidate row gets a score, and the highest applicable layer wins:

| Score | Condition | Typical case |
|-------|-----------|--------------|
| 100 | `alt = q` | Searching with the emoji itself, e.g. `🔥` |
| 80 | `q` is one whole entry in the comma-separated `keywords` | `fire` matches `fire,flame` but not `campfire` |
| 50 | `q` is a substring of `keywords` (`LIKE`, wildcards escaped) | Prefixes and Chinese text |
| 0–30 | MySQL `MATCH(keywords, alt) AGAINST(q IN NATURAL LANGUAGE MODE)`, capped at 30 | Loose full-text matches |

A row is returned if any layer matches. Results are sorted by score, then animated before static, then newest first. Without `q`, rows matching the filters are sorted animated first, then newest.

Why layers instead of plain full-text relevance: raw full-text scores reward common words and do not understand that an exact emoji or whole keyword is a better answer than a partial match. Capping the full-text score keeps it as a fallback that can never outrank an exact or whole-word match.

Notes:

- The whole-keyword check uses `LOCATE(',q,', CONCAT(',', keywords, ','))` rather than a regular expression, so user input cannot trigger expensive pattern matching.
- Migration `001` creates the FULLTEXT index with MySQL's default parser, which does not split Chinese text into words. Chinese queries are therefore served mainly by the substring layer. When the server runs in `dev` mode and finds the index missing, it creates it with the `ngram` parser instead.
- The repository has a SQLite variant of the query (without the full-text layer). It is used by the Go test suite, which runs against in-memory SQLite.

## Crawling

### Full crawl (`crawler.py full`)

1. Call `messages.getFeaturedEmojiStickers` to get the featured custom emoji packs.
2. For each pack, call `messages.getStickerSet` and process it (below).
3. Read every `source = manual` pack from the database and process it by short name.
4. Sleep one second between packs. Telethon waits out short `FLOOD_WAIT` responses automatically (below 60 seconds by default); a longer wait makes that pack fail and it is retried on the next run.

A failure in one pack is logged and does not stop the run. There is no built-in scheduler; run `full` from cron (weekly is a reasonable default) to pick up newly featured packs. Stop a running `watch` process first and start it again afterwards, because both use the same Telegram session (see [Known limitations](#known-limitations)).

### Processing one pack

1. Build a keyword index for the pack from two parts of the `getStickerSet` result: the emoticon groups (emoji → document IDs) and the keyword lists (document ID → keywords).
2. For every document that has a `DocumentAttributeCustomEmoji` attribute (other documents are skipped), record `custom_emoji_id`, `alt`, `free`, `mime_type`, and the sorted keywords as a CSV string.
3. Download the largest thumbnail to `<thumbnail_dir>/<id>.webp`, and, unless disabled, the original file to `<media_dir>/<id>.<ext>`. Existing non-empty files are not downloaded again. Download failures are logged and do not drop the emoji.
4. In one transaction, upsert the pack by `short_name` (setting `last_crawled_at` to now) and upsert all emoji by `custom_emoji_id`.

The emoji upsert overwrites `alt`, `keywords`, `pack_id`, `thumbnail_path`, `mime_type`, `is_free` and `is_animated`. It never touches `category`.

## AI keyword and category tagging

Telegram's own keyword lists are often short. `crawler/backfill_keywords_ai.py` improves search by showing each thumbnail to a vision model and asking for 3–8 Chinese and 3–8 English tags plus one of the 11 categories.

- Providers: `gemini` (default; model `gemini-3.5-flash`, override with `GEMINI_MODEL`) or `zhipu` (`GLM-4.6V-FlashX`). Select with `AI_PROVIDER` or `--provider`.
- The new tags are merged with the existing keywords (plus `alt`), and `category` is written.
- If the provider's content filter rejects an image, the emoji gets an `ai_filtered` marker keyword and is skipped in later runs.
- By default the script only processes emoji whose `keywords` contain no run of three or more ASCII letters, i.e. emoji without English keywords yet. `--all` processes everything; `--limit N` and `--concurrency N` (default 3) control the batch.
- `HTTPS_PROXY` is honoured for regions where the provider's API is not directly reachable. In Docker, it must be an address reachable from inside the container (not `127.0.0.1`), and a value set in the shell that runs `docker compose` overrides the one in `deployments/.env`.
- The script uses only the database and the thumbnail files, not the Telegram session, so it can run while `watch` is running.

**Categories only come from this script.** Without it, every emoji has an empty category and the category filter in the web UI returns nothing.

## Configuration reference

In the Docker deployment you do not edit either file below. Everything is set in `deployments/.env`: the server container generates its configuration from it at startup (with `server.mode: prod`, `log.format: json` and fixed paths under `/var/`), and the crawler container receives its variables from Compose. See [deployments/README.md](../deployments/README.md).

### Server: `configs/config.yaml`

The file is read from `CONFIG_PATH` if set, otherwise from `./configs/config.yaml` or `./config.yaml` relative to the working directory. If no file is found, built-in defaults are used, but the `database` section has no defaults, so in practice a file is required. Template: `configs/config.yaml.example`.

| Key | Default | Description |
|-----|---------|-------------|
| `server.port` | `8080` | HTTP listen port |
| `server.base_url` | `http://localhost:8080` | Prefix for `thumbnail_url` / `media_url`. Use the public URL in production, or `""` for root-relative URLs. |
| `server.mode` | `dev` | `dev` runs GORM AutoMigrate and creates the FULLTEXT index at startup, and enables Gin debug mode. Any other value (use `prod`) skips both. |
| `database.host` / `port` / `user` / `password` / `name` | | MySQL connection |
| `log.level` | `info` | `debug`, `info`, `warn`, `error` |
| `log.format` | `console` | `console` or `json` |
| `mcp.enabled` | `true` | Mount the MCP Streamable HTTP endpoint |
| `mcp.path` | `/mcp` | Mount path of that endpoint |
| `paths.signals_dir` | `./var/signals` | Where `*.pending` files are written |
| `paths.thumbnail_dir` | `./web/public/thumbs` | Served at `/thumbs/` |
| `paths.media_dir` | `./web/public/emoji` | Served at `/emoji/` |

### Crawler: `crawler/.env`

Template: `crawler/.env.example`. Relative paths are resolved against the directory the crawler is started from.

| Variable | Default | Description |
|----------|---------|-------------|
| `TG_API_ID`, `TG_API_HASH` | | Required (except with `--dry-run`). From my.telegram.org. |
| `TG_PHONE` | | Phone number of the account, needed for the first login |
| `TG_SESSION_NAME` | `tg-emoji-hub` | Session file name |
| `SESSION_DIR` | `./.session` | Directory of the session file |
| `DB_DSN` | | SQLAlchemy URL, e.g. `mysql+pymysql://user:pass@host:3306/tg_emoji_hub?charset=utf8mb4` |
| `CRAWLER_SIGNALS_DIR` | `./var/signals` | Must match the server's `paths.signals_dir` |
| `THUMBNAIL_DIR` | `./var/thumbnails` | Must match the server's `paths.thumbnail_dir` |
| `MEDIA_DIR` | `./web/public/emoji` | Must match the server's `paths.media_dir` |
| `DOWNLOAD_ORIGINALS` | `1` | `0` to skip original files (animated previews then fall back to static thumbnails) |
| `LOG_LEVEL` | `INFO` | |
| `AI_PROVIDER` | `gemini` | AI tagging only: `gemini` or `zhipu` |
| `GEMINI_API_KEY`, `GEMINI_MODEL` | | AI tagging with Gemini |
| `ZAI_API_KEY` | | AI tagging with Zhipu |
| `HTTPS_PROXY` | | Optional proxy for the AI provider |

## Design decisions and trade-offs

| Decision | Reasoning |
|----------|-----------|
| Go server plus a separate Python crawler | Telethon already handles the hard parts of MTProto (login and session storage, short rate-limit waits, media download). The serving side stays a single static Go binary, and a crawler crash never affects search. |
| MySQL with layered SQL scoring instead of a search engine | The expected index size is in the tens of thousands of emoji. A few `LIKE`/`LOCATE` checks plus a capped full-text score give predictable ranking without running Elasticsearch or Meilisearch. |
| Files in a shared directory instead of a queue | Keeps the deployment to MySQL plus two processes. Signals survive restarts of either side because they are plain files. |
| Download thumbnails and originals | Telegram does not provide stable public URLs for sticker files, so the crawler stores them and they are served from disk. |
| Static thumbnails in pack previews | A page of pack cards would otherwise start dozens of video and Lottie animations at once. |
| No authentication | The project is meant to be self-hosted as a developer tool. Put a reverse proxy with access control in front of it if it is reachable from the internet. |
| Stats computed per request | Four `COUNT(*)` queries are cheap at this data size; there is no cache to invalidate. |

## Known limitations

- **A re-crawl replaces AI keywords.** The crawler upsert overwrites `keywords` with Telegram's keywords. `category` is preserved. Because the AI script's default mode skips emoji that already have English keywords, emoji whose Telegram keywords include English words are not re-tagged automatically; run the script with `--all` if you need the AI keywords back.
- **One Telegram client at a time.** `full`, `watch`, `single_pack.py` and the login command all open the same Telethon session file, and Telethon allows only one client per session file at a time; running two at once fails with errors such as `database is locked`. Stop `watch` before a full crawl and start it again afterwards. Signal files written in the meantime stay in the signals directory and are processed when `watch` restarts.
- **Single watcher.** The watcher has no locking. Run one `watch` process per signals directory.
- **No search pagination.** Search returns up to `limit` results (200 over HTTP, 100 over MCP).
- **Error messages are in Chinese.** Clients should rely on numeric `code` values.

## Repository layout

```
cmd/server/            Server entry point and bootstrap (config, DB, HTTP, MCP)
internal/              Domain, use cases and adapters (see table above)
pkg/                   Error codes, response envelope, copy-format builder
migrations/            MySQL schema migrations (*.up.sql / *.down.sql)
configs/               Server configuration template
crawler/               Python crawler, AI tagging script and tests
web/                   React web UI
deployments/           Docker Compose files, Dockerfiles, bare-metal and BaoTa panel guides
docs/                  This document, API and MCP references
```
