// Package mcp 把 SearchUseCase / PackUseCase 暴露为 MCP tools。
//
// 5 个 tool: search_emoji / get_emoji_by_id / list_packs / list_pack_stickers / search_in_pack
// 出参格式与 HTTP 一致，复用 pkg/formats.Build 拼 4 种格式。
package mcp

import (
	"context"
	"encoding/json"
	stderrors "errors"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/TGlimmer/tg-emoji-hub/internal/app"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
	"github.com/TGlimmer/tg-emoji-hub/pkg/formats"
)

// Deps MCP server 装配依赖
type Deps struct {
	SearchUC *app.SearchUseCase
	PackUC   *app.PackUseCase
	BaseURL  string
	Name     string
	Version  string
}

// NewServer 创建并注册 3 个 tool 的 MCPServer
func NewServer(deps Deps) *server.MCPServer {
	s := server.NewMCPServer(
		nonEmpty(deps.Name, "tg-emoji-hub"),
		nonEmpty(deps.Version, "0.1.0"),
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)
	registerTools(s, deps)
	return s
}

func registerTools(s *server.MCPServer, deps Deps) {
	searchTool := mcp.NewTool("search_emoji",
		mcp.WithDescription(
			"Search Telegram Premium custom emoji by keyword (e.g. 'fire') or standard emoji (e.g. '🔥'). "+
				"Returns matching emojis with custom_emoji_id and 4 output formats (id/html/go_map/json). "+
				"At least one of query / pack / category is required.",
		),
		mcp.WithString("query",
			mcp.Description("Search keyword or standard emoji character"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results, default 20, max 100"),
		),
		mcp.WithString("pack",
			mcp.Description("Filter by pack short_name (optional)"),
		),
		mcp.WithString("category",
			mcp.Description("Filter by category: emotion/animal/person/food/crypto/object/symbol/gesture/meme/vehicle/nature"),
		),
	)
	s.AddTool(searchTool, makeSearchHandler(deps))

	getTool := mcp.NewTool("get_emoji_by_id",
		mcp.WithDescription("Fetch a single custom emoji by its custom_emoji_id."),
		mcp.WithString("custom_emoji_id",
			mcp.Required(),
			mcp.Description("Telegram Document.id, pure digits"),
		),
	)
	s.AddTool(getTool, makeGetByIDHandler(deps))

	listTool := mcp.NewTool("list_packs",
		mcp.WithDescription("List all indexed sticker packs (featured + manual)."),
		mcp.WithString("source",
			mcp.Description("Optional filter: all / featured / manual (default all)"),
		),
	)
	s.AddTool(listTool, makeListPacksHandler(deps))

	listPackStickersTool := mcp.NewTool("list_pack_stickers",
		mcp.WithDescription(
			"Browse all emojis inside a specific sticker pack. Use this when user asks "+
				"for 'show me everything in pack X' or wants to explore a pack. "+
				"Returns emojis sorted by animated first, then newest.",
		),
		mcp.WithString("pack_short_name",
			mcp.Required(),
			mcp.Description("Pack short_name (e.g. 'AnimatedEmojies', 'LoveIcons')"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results, default 100, max 100"),
		),
	)
	s.AddTool(listPackStickersTool, makeListPackStickersHandler(deps))

	searchInPackTool := mcp.NewTool("search_in_pack",
		mcp.WithDescription(
			"Search for emojis within a specific sticker pack only. Use this when user "+
				"constrains the search to one pack, e.g. 'find fire emoji in LoveIcons pack'. "+
				"Returns relevance-sorted matches scoped to the pack.",
		),
		mcp.WithString("pack_short_name",
			mcp.Required(),
			mcp.Description("Pack short_name to search within"),
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search keyword or standard emoji character"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results, default 20, max 100"),
		),
	)
	s.AddTool(searchInPackTool, makeSearchInPackHandler(deps))
}

// ---------------- handlers ----------------

func makeSearchHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		limit := req.GetInt("limit", 20)
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		pack := req.GetString("pack", "")
		category := req.GetString("category", "")

		items, err := deps.SearchUC.Search(ctx, port.SearchFilter{
			Query: query, Limit: limit, Pack: pack, Category: category,
		})
		if err != nil {
			return businessErrorResult(err), nil
		}
		return jsonResult(map[string]any{
			"items": renderItems(items, deps.BaseURL),
			"total": len(items),
		})
	}
}

func makeGetByIDHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("custom_emoji_id")
		if err != nil {
			return mcp.NewToolResultError("custom_emoji_id is required"), nil
		}
		e, err := deps.SearchUC.GetByID(ctx, id)
		if err != nil {
			return businessErrorResult(err), nil
		}
		return jsonResult(renderItem(e, deps.BaseURL))
	}
}

func makeListPacksHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		source := req.GetString("source", "all")
		// MCP 一次返回全量（最大 100），不分页（AI 调用场景）
		packs, total, err := deps.PackUC.List(ctx, source, 1, 100)
		if err != nil {
			return businessErrorResult(err), nil
		}
		return jsonResult(map[string]any{
			"items": renderPacks(packs),
			"total": total,
		})
	}
}

func makeListPackStickersHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pack, err := req.RequireString("pack_short_name")
		if err != nil {
			return mcp.NewToolResultError("pack_short_name is required"), nil
		}
		limit := req.GetInt("limit", 100)
		if limit <= 0 {
			limit = 100
		}
		if limit > 100 {
			limit = 100
		}

		// q="" + pack → SearchUseCase 走"列出包内 emoji"路径（is_animated DESC, created_at DESC）
		items, err := deps.SearchUC.Search(ctx, port.SearchFilter{
			Pack: pack, Limit: limit,
		})
		if err != nil {
			return businessErrorResult(err), nil
		}
		return jsonResult(map[string]any{
			"items": renderItems(items, deps.BaseURL),
			"total": len(items),
		})
	}
}

func makeSearchInPackHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pack, err := req.RequireString("pack_short_name")
		if err != nil {
			return mcp.NewToolResultError("pack_short_name is required"), nil
		}
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query is required"), nil
		}
		limit := req.GetInt("limit", 20)
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}

		items, err := deps.SearchUC.Search(ctx, port.SearchFilter{
			Query: query, Pack: pack, Limit: limit,
		})
		if err != nil {
			return businessErrorResult(err), nil
		}
		return jsonResult(map[string]any{
			"items": renderItems(items, deps.BaseURL),
			"total": len(items),
		})
	}
}

// ---------------- 渲染 helper ----------------

// renderItem 单条 emoji 渲染（与 HTTP search handler 字段保持一致）
func renderItem(e *entity.EmojiSticker, baseURL string) map[string]any {
	if e == nil {
		return map[string]any{}
	}
	pack := map[string]any{"short_name": "", "title": ""}
	if e.Pack != nil {
		pack["short_name"] = e.Pack.ShortName
		pack["title"] = e.Pack.Title
	}
	mime := entity.NormalizeMimeType(e.MimeType)
	return map[string]any{
		"custom_emoji_id": e.CustomEmojiID,
		"alt":             e.Alt,
		"keywords":        e.Keywords,
		"category":        e.Category,
		"pack":            pack,
		"thumbnail_url":   thumbnailURL(baseURL, e.ThumbnailPath, e.CustomEmojiID),
		"media_url":       mediaURL(baseURL, e.CustomEmojiID, mime),
		"mime_type":       mime,
		"is_free":         e.IsFree,
		"is_animated":     e.IsAnimated,
		"formats":         formats.Build(e),
	}
}

func renderItems(items []entity.EmojiSticker, baseURL string) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		out = append(out, renderItem(&items[i], baseURL))
	}
	return out
}

func renderPacks(packs []entity.StickerPack) []map[string]any {
	out := make([]map[string]any, 0, len(packs))
	for i := range packs {
		p := &packs[i]
		var lastCrawled any
		if p.LastCrawledAt != nil {
			lastCrawled = p.LastCrawledAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		out = append(out, map[string]any{
			"id":              p.ID,
			"short_name":      p.ShortName,
			"title":           p.Title,
			"is_featured":     p.IsFeatured,
			"sticker_count":   p.StickerCount,
			"source":          p.Source,
			"last_crawled_at": lastCrawled,
		})
	}
	return out
}

// thumbnailURL 与 search_handler.thumbnailURL 同逻辑
func thumbnailURL(baseURL, path, id string) string {
	rel := path
	if rel == "" {
		rel = "thumbs/" + id + ".webp"
	}
	if rel[0] != '/' {
		rel = "/" + rel
	}
	return joinBaseURL(baseURL, rel)
}

// mediaURL 原文件 URL：/emoji/{id}.{ext}，ext 由 mime 决定
func mediaURL(baseURL, id, mime string) string {
	rel := "/emoji/" + id + "." + entity.MediaExtFromMime(mime)
	return joinBaseURL(baseURL, rel)
}

// joinBaseURL baseURL 空则返回 rel；非空时去掉 baseURL 末尾的 /
func joinBaseURL(baseURL, rel string) string {
	if baseURL == "" {
		return rel
	}
	for len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	return baseURL + rel
}

// ---------------- 工具 ----------------

// jsonResult 把任意 Go 结构 marshal 成 JSON 后放进 text content
//
// 用 NewToolResultText 而非 NewToolResultStructured 是为了向后兼容旧客户端
// (Claude Desktop / Cursor 都直接显示 text 节点)
func jsonResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(b)), nil
}

// businessErrorResult 业务错误（如 not_found / invalid_param）→ tool error
//
// 系统错误（DB 挂了等）抛 raw error 让 mcp-go 框架包成 internal error
func businessErrorResult(err error) *mcp.CallToolResult {
	var ae *apperr.AppError
	if stderrors.As(err, &ae) {
		return mcp.NewToolResultErrorf("[%d] %s", ae.Code, ae.Message)
	}
	return mcp.NewToolResultErrorFromErr("internal error", err)
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
