package handler

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// thumbFilenameRe 白名单：纯数字 ID + 已知扩展，防路径穿越
var thumbFilenameRe = regexp.MustCompile(`^[0-9]+\.(webp|png|gif)$`)

// mediaFilenameRe 原文件白名单：纯数字 ID + webp/webm/tgs
var mediaFilenameRe = regexp.MustCompile(`^[0-9]+\.(webp|webm|tgs)$`)

// mediaContentTypes 显式 Content-Type 映射；tgs / webm 的 mime 不能依赖 Go 自动嗅探
var mediaContentTypes = map[string]string{
	"webp": "image/webp",
	"webm": "video/webm",
	"tgs":  "application/x-tgsticker",
}

// Deps 路由所需依赖集中传入
//
// 加 handler 要改三处：这里加字段、bootstrap/http.go 的 handler.Deps 赋值、Register 里挂路由。
// 漏了赋值编译照过，字段是 nil，要到请求进来才炸。
type Deps struct {
	SearchHandler *SearchHandler
	PackHandler   *PackHandler
	StatsHandler  *StatsHandler
	HealthHandler *HealthHandler
	ThumbnailDir  string
	MediaDir      string
}

// Register 注册全部 HTTP 路由
//
// 路由顺序敏感：/emoji/random 必须早于 /emoji/:custom_emoji_id，
// 否则 random 会被 :custom_emoji_id 当作 ID="random" 兜走
func Register(r *gin.Engine, deps Deps) {
	r.GET("/healthz", deps.HealthHandler.Healthz)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/search", deps.SearchHandler.Search)
		v1.GET("/emoji/random", deps.SearchHandler.Random)
		v1.GET("/emoji/:custom_emoji_id", deps.SearchHandler.GetByID)

		v1.GET("/packs", deps.PackHandler.List)
		v1.POST("/packs", deps.PackHandler.Create)

		if deps.StatsHandler != nil {
			v1.GET("/stats", deps.StatsHandler.Overview)
		}
	}

	r.GET("/thumbs/:filename", thumbnailHandler(deps.ThumbnailDir))
	r.GET("/emoji/:filename", mediaHandler(deps.MediaDir))
}

// thumbnailHandler 静态缩略图托管，正则白名单 + 仅 ThumbnailDir 子目录
func thumbnailHandler(dir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("filename")
		if !thumbFilenameRe.MatchString(name) {
			c.String(http.StatusNotFound, "not found")
			return
		}
		full, ok := safeJoin(dir, name)
		if !ok {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.File(full)
	}
}

// mediaHandler 静态原文件托管，专给前端动态播放用（.tgs / .webm）
//
// Content-Type 显式设置：Go http.DetectContentType 不识别 tgs/webm 头部魔数会兜底 octet-stream，
// 浏览器拿 octet-stream 就当下载文件不会播放
func mediaHandler(dir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("filename")
		if !mediaFilenameRe.MatchString(name) {
			c.String(http.StatusNotFound, "not found")
			return
		}
		full, ok := safeJoin(dir, name)
		if !ok {
			c.String(http.StatusNotFound, "not found")
			return
		}
		// 取扩展名（已通过白名单，不会越界）
		ext := name[strings.LastIndexByte(name, '.')+1:]
		if ct, found := mediaContentTypes[ext]; found {
			c.Header("Content-Type", ct)
		}
		c.File(full)
	}
}

// safeJoin 校验 filename 不含路径分隔 + 拼出的绝对路径仍在 dir 内
func safeJoin(dir, name string) (string, bool) {
	clean := filepath.Clean(name)
	if strings.Contains(clean, "..") || strings.ContainsRune(clean, filepath.Separator) {
		return "", false
	}
	full := filepath.Join(dir, clean)
	absDir, err1 := filepath.Abs(dir)
	absFull, err2 := filepath.Abs(full)
	if err1 != nil || err2 != nil || !strings.HasPrefix(absFull, absDir) {
		return "", false
	}
	return full, true
}
