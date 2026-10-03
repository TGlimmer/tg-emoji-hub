package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/TGlimmer/tg-emoji-hub/internal/app"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
	"github.com/TGlimmer/tg-emoji-hub/pkg/formats"
	"github.com/TGlimmer/tg-emoji-hub/pkg/response"
)

// SearchHandler 搜索接口
type SearchHandler struct {
	uc      *app.SearchUseCase
	baseURL string
}

func NewSearchHandler(uc *app.SearchUseCase, baseURL string) *SearchHandler {
	return &SearchHandler{uc: uc, baseURL: strings.TrimRight(baseURL, "/")}
}

// Search GET /api/v1/search
func (h *SearchHandler) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	pack := strings.TrimSpace(c.Query("pack"))
	category := strings.TrimSpace(c.Query("category"))
	if q == "" && pack == "" && category == "" {
		response.Err(c, apperr.CodeInvalidParam, "q / pack / category 至少传一个")
		return
	}

	limit := 50
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			response.Err(c, apperr.CodeInvalidParam, "limit 必须是正整数")
			return
		}
		if n > 200 {
			response.Err(c, apperr.CodeInvalidParam, "limit 不能超过 200")
			return
		}
		limit = n
	}

	animatedOnly := false
	if raw := c.Query("animated_only"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			response.Err(c, apperr.CodeInvalidParam, "animated_only 必须是布尔值")
			return
		}
		animatedOnly = v
	}

	items, err := h.uc.Search(c.Request.Context(), port.SearchFilter{
		Query:        q,
		Limit:        limit,
		Pack:         pack,
		Category:     category,
		AnimatedOnly: animatedOnly,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.OK(c, gin.H{
		"items": h.renderItems(items),
		"total": len(items),
	})
}

// Random GET /api/v1/emoji/random?limit=48
//
// 复用 EmojiItem 渲染（含 mime_type/media_url/thumbnail_url/pack/formats）
func (h *SearchHandler) Random(c *gin.Context) {
	limit := 48
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			response.Err(c, apperr.CodeInvalidParam, "limit 必须是正整数")
			return
		}
		if n > 100 {
			response.Err(c, apperr.CodeInvalidParam, "limit 不能超过 100")
			return
		}
		limit = n
	}

	items, err := h.uc.Random(c.Request.Context(), limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"items": h.renderItems(items),
		"total": len(items),
	})
}

// GetByID GET /api/v1/emoji/:custom_emoji_id
func (h *SearchHandler) GetByID(c *gin.Context) {
	id := c.Param("custom_emoji_id")
	e, err := h.uc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, h.renderItem(e))
}

// renderItems 把 entity 列表渲染为 API 响应 item 列表
func (h *SearchHandler) renderItems(items []entity.EmojiSticker) []gin.H {
	out := make([]gin.H, 0, len(items))
	for i := range items {
		out = append(out, h.renderItem(&items[i]))
	}
	return out
}

// renderItem 单条渲染：组合 entity + formats + thumbnail_url + media_url
func (h *SearchHandler) renderItem(e *entity.EmojiSticker) gin.H {
	if e == nil {
		return gin.H{}
	}
	packShortName := ""
	packTitle := ""
	if e.Pack != nil {
		packShortName = e.Pack.ShortName
		packTitle = e.Pack.Title
	}
	mime := entity.NormalizeMimeType(e.MimeType)
	return gin.H{
		"custom_emoji_id": e.CustomEmojiID,
		"alt":             e.Alt,
		"keywords":        e.Keywords,
		"category":        e.Category,
		"pack": gin.H{
			"short_name": packShortName,
			"title":      packTitle,
		},
		"thumbnail_url": h.thumbnailURL(e.ThumbnailPath, e.CustomEmojiID),
		"media_url":     h.mediaURL(e.CustomEmojiID, mime),
		"mime_type":     mime,
		"is_free":       e.IsFree,
		"is_animated":   e.IsAnimated,
		"formats":       formats.Build(e),
	}
}

// thumbnailURL 拼缩略图 URL，磁盘没存就回落到 /thumbs/{id}.webp 约定
func (h *SearchHandler) thumbnailURL(path, id string) string {
	rel := path
	if rel == "" {
		rel = "thumbs/" + id + ".webp"
	}
	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}
	if h.baseURL == "" {
		return rel
	}
	return h.baseURL + rel
}

// mediaURL 拼原文件 URL，扩展名由 mime 决定（webp/webm/tgs）
func (h *SearchHandler) mediaURL(id, mime string) string {
	rel := "/emoji/" + id + "." + entity.MediaExtFromMime(mime)
	if h.baseURL == "" {
		return rel
	}
	return h.baseURL + rel
}

// HealthHandler /healthz 用，独立放这里保持模块聚拢
type HealthHandler struct {
	pinger func() error
	ver    string
}

func NewHealthHandler(pinger func() error, version string) *HealthHandler {
	return &HealthHandler{pinger: pinger, ver: version}
}

func (h *HealthHandler) Healthz(c *gin.Context) {
	dbStatus := "ok"
	if h.pinger != nil {
		if err := h.pinger(); err != nil {
			dbStatus = "error: " + err.Error()
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "degraded",
				"db":      dbStatus,
				"version": h.ver,
			})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"db":      dbStatus,
		"version": h.ver,
	})
}
