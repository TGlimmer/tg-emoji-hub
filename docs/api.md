# HTTP API Reference

The Go server exposes a small read-mostly JSON API under `/api/v1`, plus a health check and two static file routes. Everything on this page is served by the same process on `server.port` (default `8080`).

- Base URL: `http://localhost:8080` by default (in Docker, the `web` container proxies these paths on `HTTP_PORT`)
- Authentication: none. Every endpoint, including `POST /api/v1/packs`, is open. If you expose the server to the internet, put it behind a reverse proxy with access control.
- CORS: the server sends no CORS headers. Call the API from server-side code, from tools such as `curl`, or from a page served on the same origin.
- Field names are `snake_case`. Error `message` strings are human-readable and currently in Chinese; branch on `code`, not on `message`.

For the AI-assistant interface, see [mcp.md](mcp.md).

## Contents

- [Response envelope and error codes](#response-envelope-and-error-codes)
- [The emoji object](#the-emoji-object)
- [GET /api/v1/search](#get-apiv1search)
- [GET /api/v1/emoji/{custom_emoji_id}](#get-apiv1emojicustom_emoji_id)
- [GET /api/v1/emoji/random](#get-apiv1emojirandom)
- [GET /api/v1/packs](#get-apiv1packs)
- [POST /api/v1/packs](#post-apiv1packs)
- [GET /api/v1/stats](#get-apiv1stats)
- [GET /healthz](#get-healthz)
- [Static files: /thumbs and /emoji](#static-files-thumbs-and-emoji)

## Response envelope and error codes

All `/api/v1/*` endpoints wrap their payload in the same envelope:

```json
{
  "code": 0,
  "data": { },
  "message": "ok"
}
```

On error, `data` is `null` (except for `409` on `POST /api/v1/packs`, see below):

```json
{
  "code": 4001,
  "data": null,
  "message": "limit 不能超过 200"
}
```

| `code` | HTTP status | Meaning |
|--------|-------------|---------|
| `0` | 200 / 202 | Success |
| `4001` | 400 | Invalid parameter |
| `4004` | 404 | Resource not found |
| `4009` | 409 | Resource already exists |
| `4222` | 422 | Business rule violation (reserved, not returned by any current endpoint) |
| `5000` | 500 | Internal error |

`/healthz` and the static file routes do not use this envelope.

## The emoji object

Search, lookup and random endpoints all return emoji in this shape:

```json
{
  "custom_emoji_id": "5404674261681232568",
  "alt": "🔥",
  "keywords": "fire,flame,火",
  "category": "emotion",
  "pack": {
    "short_name": "AnimatedEmojies",
    "title": "Animated Emojies"
  },
  "thumbnail_url": "http://localhost:8080/thumbs/5404674261681232568.webp",
  "media_url": "http://localhost:8080/emoji/5404674261681232568.webm",
  "mime_type": "video/webm",
  "is_free": false,
  "is_animated": true,
  "formats": {
    "id": "5404674261681232568",
    "html": "<tg-emoji emoji-id=\"5404674261681232568\">🔥</tg-emoji>",
    "go_map": "\"fire\": \"5404674261681232568\",",
    "json": {
      "id": "5404674261681232568",
      "alt": "🔥",
      "pack": "AnimatedEmojies"
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `custom_emoji_id` | string | Telegram `Document.id` as a decimal string. This is the value Bot API messages need. |
| `alt` | string | The standard emoji the custom emoji stands for. |
| `keywords` | string | Comma-separated search keywords (from Telegram, optionally extended by AI tagging). |
| `category` | string | One of `emotion`, `animal`, `person`, `food`, `crypto`, `object`, `symbol`, `gesture`, `meme`, `vehicle`, `nature`, or an empty string when the emoji has not been classified yet. |
| `pack.short_name` / `pack.title` | string | The sticker set the emoji belongs to. |
| `thumbnail_url` | string | Static preview image. See [URL fields](#url-fields). |
| `media_url` | string | Original file (`.webp`, `.webm` or `.tgs`, chosen by `mime_type`). See [URL fields](#url-fields). |
| `mime_type` | string | `image/webp`, `video/webm` or `application/x-tgsticker`. Unknown values are reported as `image/webp`. |
| `is_free` | bool | Telegram's `free` flag for this custom emoji. |
| `is_animated` | bool | `true` for `video/webm` and `application/x-tgsticker` emoji. |
| `formats` | object | Ready-to-paste snippets, see below. |

### Copy formats

| Key | Example | Notes |
|-----|---------|-------|
| `id` | `5404674261681232568` | The raw ID. |
| `html` | `<tg-emoji emoji-id="5404674261681232568">🔥</tg-emoji>` | For messages sent with `parse_mode=HTML`. The ID and `alt` are HTML-escaped. |
| `go_map` | `"fire": "5404674261681232568",` | A Go map entry. The key is derived from the first keyword that contains ASCII letters or digits (lower-cased, other characters replaced with `_`); if there is none, `alt` is used. |
| `json` | `{"id": "...", "alt": "🔥", "pack": "AnimatedEmojies"}` | A JSON object. |

### URL fields

`thumbnail_url` and `media_url` are built as `server.base_url` + path:

- `thumbnail_url`: the stored thumbnail path, or `/thumbs/{custom_emoji_id}.webp` when none was recorded.
- `media_url`: `/emoji/{custom_emoji_id}.{webp|webm|tgs}`.

With the default `base_url` (`http://localhost:8080`) the URLs are absolute. In the Docker deployment, `base_url` is taken from `PUBLIC_URL` in `deployments/.env`. If `base_url` is set to an empty string, they are root-relative paths such as `/thumbs/5404674261681232568.webp`. A URL is only reachable if the crawler has actually downloaded that file.

## GET /api/v1/search

Search emoji by keyword or by standard emoji character, optionally restricted to one pack and/or one category.

| Query parameter | Type | Default | Description |
|-----------------|------|---------|-------------|
| `q` | string | | Keyword (`fire`) or standard emoji (`🔥`). |
| `pack` | string | | Only return emoji from the pack with this `short_name`. |
| `category` | string | | Only return emoji in this category. Values are listed under [The emoji object](#the-emoji-object). The value is not validated: an unknown category simply matches nothing. |
| `limit` | int | `50` | `1`–`200`. |
| `animated_only` | bool | `false` | Only return animated emoji. Accepts the usual boolean spellings (`true`, `false`, `1`, `0`). |

At least one of `q`, `pack` or `category` is required.

**Ordering**

- When `q` is given, results are ranked by a relevance score (see [architecture.md](architecture.md#search-and-relevance-scoring)): exact `alt` match first, then whole-keyword matches, then substring matches, then full-text matches. Ties are broken by animated first, then newest.
- When `q` is empty (browsing a pack or category), results are ordered animated first, then newest.

**Response** `200`

```json
{
  "code": 0,
  "data": {
    "items": [ { "custom_emoji_id": "5404674261681232568", "alt": "🔥", "...": "..." } ],
    "total": 1
  },
  "message": "ok"
}
```

`total` is the number of items in this response, not the number of all possible matches. There is no pagination; raise `limit` to get more.

**Errors**

| `code` | HTTP | When |
|--------|------|------|
| `4001` | 400 | `q`, `pack` and `category` are all empty; `limit` is not a positive integer or exceeds 200; `animated_only` is not a boolean |

```bash
curl "http://localhost:8080/api/v1/search?q=fire&limit=5"
curl "http://localhost:8080/api/v1/search?category=crypto&animated_only=true"
curl "http://localhost:8080/api/v1/search?pack=AnimatedEmojies&q=heart"
```

## GET /api/v1/emoji/{custom_emoji_id}

Look up a single emoji by ID.

**Response** `200`: the [emoji object](#the-emoji-object) directly in `data`.

**Errors**

| `code` | HTTP | When |
|--------|------|------|
| `4004` | 404 | No emoji with this ID has been indexed |

```bash
curl "http://localhost:8080/api/v1/emoji/5404674261681232568"
```

## GET /api/v1/emoji/random

Return a random sample of emoji (used by the home page preview).

| Query parameter | Type | Default | Description |
|-----------------|------|---------|-------------|
| `limit` | int | `48` | `1`–`100`. |

**Response** `200`: `data` is `{ "items": [emoji...], "total": N }`, same as search.

**Errors**

| `code` | HTTP | When |
|--------|------|------|
| `4001` | 400 | `limit` is not a positive integer or exceeds 100 |

## GET /api/v1/packs

List indexed sticker packs with pagination.

| Query parameter | Type | Default | Description |
|-----------------|------|---------|-------------|
| `source` | string | `all` | `all`, `featured` (Telegram's featured emoji packs) or `manual` (added by users). |
| `sort` | string | `newest` | `newest`: featured packs first, then most recently added. `hot`: featured first, then by `sticker_count` descending, then most recently crawled. `name`: by `short_name` ascending. |
| `with_preview` | bool | `false` | When `true`, each pack includes `preview_emojis`. |
| `preview_size` | int | `4` | Number of preview emoji per pack, `1`–`20`. Only used when `with_preview=true`, but always validated. |
| `page` | int | `1` | Page number, starting at 1. |
| `page_size` | int | `20` | `1`–`100`. |

**Response** `200`

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": 1,
        "short_name": "AnimatedEmojies",
        "title": "Animated Emojies",
        "is_featured": true,
        "sticker_count": 120,
        "source": "featured",
        "last_crawled_at": "2026-05-16T03:00:00Z",
        "created_at": "2026-05-16T00:00:00Z",
        "preview_emojis": [
          {
            "custom_emoji_id": "5404674261681232568",
            "alt": "🔥",
            "thumbnail_url": "http://localhost:8080/thumbs/5404674261681232568.webp"
          }
        ]
      }
    ],
    "total": 50,
    "page": 1,
    "page_size": 20
  },
  "message": "ok"
}
```

- Timestamps are ISO 8601 in UTC. `last_crawled_at` is `null` for a pack that has been added but not crawled yet.
- `preview_emojis` is only present when `with_preview=true`. It takes the first emoji of each pack in insertion order and contains only static thumbnails (no `media_url` or `mime_type`), so a page full of previews does not start dozens of animations at once.
- `total` is the number of packs matching `source`.

**Errors**

| `code` | HTTP | When |
|--------|------|------|
| `4001` | 400 | Unknown `source` or `sort`; `page`, `page_size` or `preview_size` out of range; `with_preview` is not a boolean |

## POST /api/v1/packs

Add a Telegram custom emoji pack by its short name and queue it for crawling. The short name is the last segment of the pack link, for example `MicrosoftAnimatedEmojies` in `https://t.me/addemoji/MicrosoftAnimatedEmojies`.

**Request body**

```json
{ "short_name": "MicrosoftAnimatedEmojies" }
```

`short_name` must match `^[A-Za-z][A-Za-z0-9_]{0,63}$`. Pass the short name only, not the full link.

**What happens**

1. If a pack with this short name already exists, the server returns `409` with the existing `pack_id`.
2. The server requests `https://t.me/addemoji/{short_name}` to check that the pack exists on Telegram. If the page clearly says it does not, the request is rejected with `400`. Network errors and timeouts (10 s) do not block the request.
3. A pack row is created with `source = manual`.
4. A signal file `{short_name}.pending` is written to `paths.signals_dir`. The crawler, running in `watch` mode, picks it up and fetches the pack (see [architecture.md](architecture.md#server-crawler-communication-signal-files)).

**Response** `202 Accepted`

```json
{
  "code": 0,
  "data": {
    "pack_id": 12,
    "short_name": "MicrosoftAnimatedEmojies",
    "status": "pending_crawl",
    "message": "已加入爬取队列，预计 1-2 分钟内可搜索"
  },
  "message": "accepted"
}
```

The emoji become searchable once the crawler has processed the signal. If no crawler is running in `watch` mode, the pack stays in the list with `last_crawled_at: null` and no emoji.

**Response** `409 Conflict` (pack already exists)

```json
{
  "code": 4009,
  "data": {
    "pack_id": 12,
    "short_name": "MicrosoftAnimatedEmojies",
    "status": "already_exists"
  },
  "message": "pack 已存在"
}
```

**Errors**

| `code` | HTTP | When |
|--------|------|------|
| `4001` | 400 | Body is not valid JSON or `short_name` is missing; `short_name` does not match the pattern; Telegram reports that the pack does not exist |
| `4009` | 409 | The pack already exists (body includes `pack_id`) |
| `5000` | 500 | Database error, or the pack was created but the signal file could not be written. In the second case the pack row exists but will only be crawled by the next full crawl. |

```bash
curl -X POST "http://localhost:8080/api/v1/packs" \
  -H "Content-Type: application/json" \
  -d '{"short_name":"MicrosoftAnimatedEmojies"}'
```

## GET /api/v1/stats

Aggregate counters shown on the home page.

**Response** `200`

```json
{
  "code": 0,
  "data": {
    "total_emojis": 4892,
    "total_packs": 99,
    "animated_count": 4256,
    "animated_percentage": 87,
    "added_today": 12,
    "updated_at": "2026-05-19T13:00:00Z"
  },
  "message": "ok"
}
```

| Field | Description |
|-------|-------------|
| `total_emojis` | Number of indexed emoji. |
| `total_packs` | Number of packs, including packs that have not been crawled yet. |
| `animated_count` | Emoji whose `mime_type` is `video/webm` or `application/x-tgsticker`. |
| `animated_percentage` | `animated_count / total_emojis` as a rounded integer from 0 to 100; `0` for an empty database. |
| `added_today` | Emoji whose `created_at` is at or after midnight today, where midnight is computed in the server process's local time zone. |
| `updated_at` | When the numbers were computed (UTC). They are computed on every request; there is no cache. |

## GET /healthz

Liveness and database check. Not wrapped in the response envelope.

**Response** `200`

```json
{ "status": "ok", "db": "ok", "version": "0.1.0" }
```

**Response** `503` (database ping failed within 2 seconds)

```json
{ "status": "degraded", "db": "error: <driver error>", "version": "0.1.0" }
```

## Static files: /thumbs and /emoji

| Route | Serves | Allowed file names |
|-------|--------|--------------------|
| `GET /thumbs/{file}` | Thumbnails from `paths.thumbnail_dir` | `^[0-9]+\.(webp\|png\|gif)$` |
| `GET /emoji/{file}` | Original files from `paths.media_dir` | `^[0-9]+\.(webp\|webm\|tgs)$` |

Any other file name, or a missing file, returns `404` with the plain-text body `not found`. `/emoji/` sets an explicit `Content-Type` (`image/webp`, `video/webm` or `application/x-tgsticker`) so browsers play the file instead of downloading it. `.tgs` files are gzip-compressed Lottie animations; the bundled web UI renders them with `lottie-web`.

In the Docker deployment, Nginx in the `web` container forwards these paths to the Go server. In the bare-metal and BaoTa panel deployment guides, Nginx serves them directly from disk and the Go routes are not used.
