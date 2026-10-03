# MCP Server Reference

tg-emoji-hub includes a [Model Context Protocol](https://modelcontextprotocol.io/) server, so an AI coding assistant can look up `custom_emoji_id` values while it writes Telegram Bot code, without you copying IDs by hand.

It is the same Go binary as the web API and reads the same MySQL database. There is no separate service to run.

## Contents

- [Transports](#transports)
- [Client setup](#client-setup)
- [Tools](#tools)
- [Result format](#result-format)
- [Errors](#errors)
- [Troubleshooting](#troubleshooting)

## Transports

| Transport | How to start | Endpoint | Typical use |
|-----------|--------------|----------|-------------|
| Streamable HTTP | Built into the normal server process | `http://<host>:<port>/mcp` | Claude Code, Cursor, Codex and any client that can connect to a URL |
| stdio | `server --mcp-stdio` | stdin / stdout of the process | Clients that launch a local command, such as Claude Desktop |

The legacy SSE transport (`/mcp/sse`) is not provided.

### Streamable HTTP

Enabled by default. Configure it in `configs/config.yaml`:

```yaml
mcp:
  enabled: true   # set to false to disable the HTTP endpoint
  path: /mcp      # mount path
```

The server answers on `path` and on any sub-path of it, for all HTTP methods. With the defaults, the endpoint is `http://localhost:8080/mcp`. In the Docker deployment the Go server is not published directly; the `web` container (Nginx) proxies `/mcp` on `HTTP_PORT` (default `8080`), so the address is the same.

The endpoint has no authentication. Do not expose it to the internet without a reverse proxy that restricts access. If you put Nginx in front of it, disable response buffering and raise the read timeout for this location; the bare-metal guide contains a working example (`deployments/bare-metal/nginx/tg-emoji-hub.conf`).

### stdio

```bash
./server --mcp-stdio
```

In this mode the process does not start the HTTP server. It reads JSON-RPC from stdin, writes responses to stdout, and sends all logs to stderr.

It still needs the configuration file and a reachable MySQL database:

- The configuration file is taken from the `CONFIG_PATH` environment variable, or else from `configs/config.yaml` or `config.yaml` relative to the working directory. MCP clients often start commands from an unrelated directory, so set `CONFIG_PATH` explicitly.
- If the database cannot be reached at startup, the process exits and the client will report that the server failed to start.

## Client setup

Replace `http://localhost:8080` with your server address if it runs elsewhere.

### Claude Code

```bash
claude mcp add --transport http tg-emoji-hub http://localhost:8080/mcp
```

To share the configuration with a project, add it to `.mcp.json` in the project root instead:

```json
{
  "mcpServers": {
    "tg-emoji-hub": {
      "type": "http",
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

### Cursor

`~/.cursor/mcp.json` (global) or `.cursor/mcp.json` (per project):

```json
{
  "mcpServers": {
    "tg-emoji-hub": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

### Codex CLI

`~/.codex/config.toml`:

```toml
[mcp_servers.tg-emoji-hub]
url = "http://localhost:8080/mcp"
```

### Claude Desktop (stdio)

Claude Desktop launches local commands, so use stdio mode. Edit `claude_desktop_config.json`:

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

**Option A: a local server binary**

Build the binary (`go build -o server ./cmd/server`) and point it at a configuration file whose `database` section reaches your MySQL instance:

```json
{
  "mcpServers": {
    "tg-emoji-hub": {
      "command": "/absolute/path/to/server",
      "args": ["--mcp-stdio"],
      "env": {
        "CONFIG_PATH": "/absolute/path/to/tg-emoji-hub/configs/config.yaml"
      }
    }
  }
}
```

On Windows, escape backslashes in paths (`"C:\\tools\\tg-emoji-hub\\server.exe"`).

**Option B: the Docker deployment**

If the stack from the README is running, start the stdio server inside the existing `server` container. The container's entry script `server-entrypoint.sh` writes the configuration from `deployments/.env` and then starts the binary with the arguments you pass. `-T` is required so Docker does not allocate a terminal:

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

### Other clients

Any MCP client that supports the Streamable HTTP transport can use `http://localhost:8080/mcp`. Clients that only support stdio can use the `--mcp-stdio` command shown above.

The web UI also has an **MCP** button in the header that shows the endpoint for the server you are viewing and copy-ready snippets for Claude Code, Cursor and Codex.

## Tools

The server registers five tools. Server name: `tg-emoji-hub`, version `0.1.0`.

| Tool | Purpose |
|------|---------|
| [`search_emoji`](#search_emoji) | Search by keyword or emoji character, optionally within a pack or category |
| [`get_emoji_by_id`](#get_emoji_by_id) | Look up one emoji by `custom_emoji_id` |
| [`list_packs`](#list_packs) | List indexed packs |
| [`list_pack_stickers`](#list_pack_stickers) | Browse every emoji in one pack |
| [`search_in_pack`](#search_in_pack) | Search inside one pack |

Numeric `limit` arguments never cause an error: values of 0 or below fall back to the default, and values above the maximum are reduced to the maximum.

### search_emoji

Search custom emoji by keyword (for example `fire`) or by standard emoji (for example `🔥`).

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `query` | string | no | Keyword or standard emoji character. |
| `pack` | string | no | Only search the pack with this `short_name`. |
| `category` | string | no | One of `emotion`, `animal`, `person`, `food`, `crypto`, `object`, `symbol`, `gesture`, `meme`, `vehicle`, `nature`. An unknown value matches nothing. |
| `limit` | number | no | Default `20`, maximum `100`. |

At least one of `query`, `pack` or `category` must be provided. Ranking is the same as the HTTP search: exact `alt` match, then whole-keyword, substring and full-text matches; ties go to animated emoji, then newest. Without `query`, results are ordered animated first, then newest.

Returns `{ "items": [emoji...], "total": N }`, where `total` is the number of returned items.

The HTTP API's `animated_only` filter is not available through MCP.

### get_emoji_by_id

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `custom_emoji_id` | string | yes | Telegram `Document.id`, digits only. |

Returns a single emoji object, or an error result with code `4004` if the ID is not indexed.

### list_packs

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `source` | string | no | `all` (default), `featured` or `manual`. |

Returns at most 100 packs, featured packs first and then the most recently added:

```json
{
  "items": [
    {
      "id": 1,
      "short_name": "AnimatedEmojies",
      "title": "Animated Emojies",
      "is_featured": true,
      "sticker_count": 120,
      "source": "featured",
      "last_crawled_at": "2026-05-16T03:00:00Z"
    }
  ],
  "total": 150
}
```

`total` is the number of all packs matching `source`, so it can be larger than the number of items returned. There is no paging argument; use the HTTP endpoint `GET /api/v1/packs` to page through larger collections.

### list_pack_stickers

Browse every emoji in one pack.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `pack_short_name` | string | yes | Pack `short_name`, for example `AnimatedEmojies`. |
| `limit` | number | no | Default `100`, maximum `100`. |

Results are ordered animated first, then newest. An unknown pack returns an empty list, not an error.

### search_in_pack

Search within one pack only.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `pack_short_name` | string | yes | Pack to search in. |
| `query` | string | yes | Keyword or standard emoji character. |
| `limit` | number | no | Default `20`, maximum `100`. |

Results are ranked by relevance, as in `search_emoji`.

## Result format

Every successful call returns one `text` content block containing a JSON document. Emoji objects have the same fields as the HTTP API (see [api.md](api.md#the-emoji-object)):

```json
{
  "items": [
    {
      "custom_emoji_id": "5404674261681232568",
      "alt": "🔥",
      "keywords": "fire,flame,火",
      "category": "emotion",
      "pack": { "short_name": "AnimatedEmojies", "title": "Animated Emojies" },
      "thumbnail_url": "http://localhost:8080/thumbs/5404674261681232568.webp",
      "media_url": "http://localhost:8080/emoji/5404674261681232568.webm",
      "mime_type": "video/webm",
      "is_free": false,
      "is_animated": true,
      "formats": {
        "id": "5404674261681232568",
        "html": "<tg-emoji emoji-id=\"5404674261681232568\">🔥</tg-emoji>",
        "go_map": "\"fire\": \"5404674261681232568\",",
        "json": { "id": "5404674261681232568", "alt": "🔥", "pack": "AnimatedEmojies" }
      }
    }
  ],
  "total": 1
}
```

Notes:

- The JSON is compact (no indentation), and key order may differ from the example above. The `<` and `>` in `formats.html` are encoded as `\u003c` and `\u003e` in the raw text; they decode to the normal characters when the JSON is parsed.
- `thumbnail_url` and `media_url` are prefixed with `server.base_url`. Set `base_url` to an address the client can reach if you want the assistant to open preview images.

## Errors

Business errors come back as a tool result with `isError: true` and a text of the form `[code] message`, for example:

```text
[4004] emoji 不存在
[4001] q / pack / category 至少传一个
```

The codes are the same as in the HTTP API ([api.md](api.md#response-envelope-and-error-codes)). The message text is currently in Chinese. A missing required argument returns a plain message such as `custom_emoji_id is required`.

## Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| `404` from `/mcp/sse` | That path is from an older version. Use `/mcp`. |
| Client connects but every search is empty | The database has no emoji yet. Run a full crawl first (see the README). |
| stdio server exits immediately | The configuration file was not found, or MySQL is not reachable. Run the same command in a terminal and read the log on stderr. |
| Connection drops after about a minute behind Nginx | Nginx is buffering or timing out the MCP location. Use `proxy_buffering off` and a long `proxy_read_timeout` for `/mcp`. |
