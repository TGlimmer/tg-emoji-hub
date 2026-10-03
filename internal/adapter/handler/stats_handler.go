package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/TGlimmer/tg-emoji-hub/internal/app"
	"github.com/TGlimmer/tg-emoji-hub/pkg/response"
)

// StatsHandler 首页统计接口
type StatsHandler struct {
	uc *app.StatsUseCase
}

func NewStatsHandler(uc *app.StatsUseCase) *StatsHandler {
	return &StatsHandler{uc: uc}
}

// Overview GET /api/v1/stats，字段说明见 docs/api.md「GET /api/v1/stats」
//
// animated_percentage 是 int 0-100（前端 StatsHero 直接 toLocaleString() 显示 + 静态 % 后缀）
func (h *StatsHandler) Overview(c *gin.Context) {
	stats, err := h.uc.Overview(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	percentage := 0
	if stats.TotalEmojis > 0 {
		// 整数百分比：四舍五入到 0-100
		// stats.AnimatedCount * 100 / stats.TotalEmojis 用整数除会截断，加 50 后除等价四舍五入
		percentage = int((stats.AnimatedCount*200 + stats.TotalEmojis) / (stats.TotalEmojis * 2))
		if percentage > 100 {
			percentage = 100
		}
	}

	response.OK(c, gin.H{
		"total_emojis":        stats.TotalEmojis,
		"total_packs":         stats.TotalPacks,
		"animated_count":      stats.AnimatedCount,
		"animated_percentage": percentage,
		"added_today":         stats.AddedToday,
		"updated_at":          stats.LastUpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}
