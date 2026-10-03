package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/TGlimmer/tg-emoji-hub/internal/app"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
	"github.com/TGlimmer/tg-emoji-hub/pkg/response"
)

// PackHandler 表情包 HTTP 接口
type PackHandler struct {
	uc      *app.PackUseCase
	baseURL string
}

func NewPackHandler(uc *app.PackUseCase, baseURL string) *PackHandler {
	return &PackHandler{uc: uc, baseURL: strings.TrimRight(baseURL, "/")}
}

// List GET /api/v1/packs?source=&sort=&with_preview=&page=&page_size=
//
// 新参数（首页 HotPacks 需要）：
//
//	sort         newest（默认） / hot / name
//	with_preview bool，true 时每个 pack 多带 preview_emojis（最多 4 条 thumbnail）
func (h *PackHandler) List(c *gin.Context) {
	source := c.Query("source")
	sort := c.Query("sort")

	page, err := parsePositiveInt(c.Query("page"), 1, 0)
	if err != nil {
		response.Err(c, apperr.CodeInvalidParam, "page 必须 ≥ 1")
		return
	}
	pageSize, err := parsePositiveInt(c.Query("page_size"), 20, 100)
	if err != nil {
		response.Err(c, apperr.CodeInvalidParam, err.Error())
		return
	}

	withPreview := false
	if raw := c.Query("with_preview"); raw != "" {
		v, perr := strconv.ParseBool(raw)
		if perr != nil {
			response.Err(c, apperr.CodeInvalidParam, "with_preview 必须是布尔值")
			return
		}
		withPreview = v
	}

	previewSize := 4
	if raw := c.Query("preview_size"); raw != "" {
		n, perr := strconv.Atoi(raw)
		if perr != nil || n < 1 || n > 20 {
			response.Err(c, apperr.CodeInvalidParam, "preview_size 必须 1-20")
			return
		}
		previewSize = n
	}

	items, total, err := h.uc.ListExtended(c.Request.Context(), app.ListParams{
		Source:      source,
		Sort:        sort,
		WithPreview: withPreview,
		PreviewSize: previewSize,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.OK(c, gin.H{
		"items":     h.renderPackList(items, withPreview),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// parsePositiveInt 解析 query 整数：空字符串走 defaultVal；解析失败/<1 报错
// max=0 表示无上限
func parsePositiveInt(raw string, defaultVal, max int) (int, error) {
	if raw == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, errPositiveInt
	}
	if max > 0 && n > max {
		return 0, errExceedsMax(max)
	}
	return n, nil
}

type positiveIntError struct{ msg string }

func (e *positiveIntError) Error() string { return e.msg }

var errPositiveInt = &positiveIntError{msg: "必须 ≥ 1"}

func errExceedsMax(max int) error {
	return &positiveIntError{msg: "不能超过 " + strconv.Itoa(max)}
}

// createPackReq POST /api/v1/packs 请求体
type createPackReq struct {
	ShortName string `json:"short_name" binding:"required"`
}

// Create POST /api/v1/packs，响应形状见 docs/api.md「POST /api/v1/packs」
//
// 已存在时回 409 但 data 里照样带现有 pack_id，跟其它错误 data=null 不一样
func (h *PackHandler) Create(c *gin.Context) {
	var req createPackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Err(c, apperr.CodeInvalidParam, "请求体格式错误：缺 short_name")
		return
	}

	result, err := h.uc.AddManual(c.Request.Context(), req.ShortName)
	if err != nil {
		response.Fail(c, err)
		return
	}

	if result.AlreadyExisted {
		c.JSON(http.StatusConflict, response.Body{
			Code: apperr.CodeConflict,
			Data: gin.H{
				"pack_id":    result.Pack.ID,
				"short_name": result.Pack.ShortName,
				"status":     "already_exists",
			},
			Message: "pack 已存在",
		})
		return
	}

	c.JSON(http.StatusAccepted, response.Body{
		Code: apperr.CodeOK,
		Data: gin.H{
			"pack_id":    result.Pack.ID,
			"short_name": result.Pack.ShortName,
			"status":     "pending_crawl",
			"message":    "已加入爬取队列，预计 1-2 分钟内可搜索",
		},
		Message: "accepted",
	})
}

func (h *PackHandler) renderPackList(items []port.PackWithPreview, withPreview bool) []gin.H {
	out := make([]gin.H, 0, len(items))
	for i := range items {
		out = append(out, h.renderPack(&items[i], withPreview))
	}
	return out
}

func (h *PackHandler) renderPack(pp *port.PackWithPreview, withPreview bool) gin.H {
	if pp == nil {
		return gin.H{}
	}
	p := &pp.Pack
	var lastCrawled any
	if p.LastCrawledAt != nil {
		lastCrawled = p.LastCrawledAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	row := gin.H{
		"id":              p.ID,
		"short_name":      p.ShortName,
		"title":           p.Title,
		"is_featured":     p.IsFeatured,
		"sticker_count":   p.StickerCount,
		"source":          p.Source,
		"last_crawled_at": lastCrawled,
		"created_at":      p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if withPreview {
		row["preview_emojis"] = h.renderPreview(pp.Preview)
	}
	return row
}

// renderPreview 仅渲染 thumbnail，不带 media_url/mime_type
//
// 设计文档明确：前端 32 张 webm/tgs 同播会卡死，preview 强制走静态 webp
func (h *PackHandler) renderPreview(items []port.PackPreviewEmoji) []gin.H {
	out := make([]gin.H, 0, len(items))
	for i := range items {
		out = append(out, gin.H{
			"custom_emoji_id": items[i].CustomEmojiID,
			"alt":             items[i].Alt,
			"thumbnail_url":   h.thumbnailURL(items[i].ThumbnailPath, items[i].CustomEmojiID),
		})
	}
	return out
}

// thumbnailURL 与 SearchHandler 同样规则：disk 没记则按 /thumbs/{id}.webp 兜底
func (h *PackHandler) thumbnailURL(path, id string) string {
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
