package port

import (
	"context"
	"time"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
)

// SearchFilter 搜索条件
type SearchFilter struct {
	Query        string // 关键词或标准 emoji
	Limit        int    // 1-200
	Pack         string // 可选包 short_name 过滤
	Category     string // 可选分类过滤，合法值见 entity.NormalizeCategory
	AnimatedOnly bool   // 仅动态
}

// SearchRepository 搜索数据访问接口
type SearchRepository interface {
	SearchByKeyword(ctx context.Context, f SearchFilter) ([]entity.EmojiSticker, error)
	FindByCustomEmojiID(ctx context.Context, id string) (*entity.EmojiSticker, error)
	// Random 返回 limit 条随机 emoji（含 Pack 注入）
	Random(ctx context.Context, limit int) ([]entity.EmojiSticker, error)
}

// PackSort 排序方式
type PackSort string

const (
	// PackSortNewest 按 is_featured DESC, id DESC（默认，旧行为）
	PackSortNewest PackSort = "newest"
	// PackSortHot 按 sticker_count DESC, last_crawled_at DESC，featured 优先
	PackSortHot PackSort = "hot"
	// PackSortName 按 short_name ASC
	PackSortName PackSort = "name"
)

// NormalizePackSort 写入前兜底，非法值回退到 newest
func NormalizePackSort(s string) PackSort {
	switch PackSort(s) {
	case PackSortNewest, PackSortHot, PackSortName:
		return PackSort(s)
	default:
		return PackSortNewest
	}
}

// IsValidPackSort 显式校验
func IsValidPackSort(s string) bool {
	switch PackSort(s) {
	case PackSortNewest, PackSortHot, PackSortName:
		return true
	default:
		return false
	}
}

// PackFilter pack 列表筛选
type PackFilter struct {
	Source      string   // "" / "all" / "featured" / "manual"
	Sort        PackSort // newest / hot / name
	WithPreview bool     // 是否附带前 N 个 emoji 预览
	PreviewSize int      // preview 取多少个，默认 4，调用方不传则 repo 兜底
	Page        int      // 从 1 开始
	PageSize    int      // 默认 20，最大 100
}

// PackPreviewEmoji 一个 pack 的预览缩略图（只含必要字段，避免大载荷）
//
// 设计文档明确：preview 只返 thumbnail_url，不返 media_url/mime_type，
// 前端走静态图，避免一屏 32 张 webm/tgs 同时播放卡死
type PackPreviewEmoji struct {
	CustomEmojiID string
	Alt           string
	ThumbnailPath string // repo 层不拼 URL，handler 决定 baseURL
	IsAnimated    bool
}

// PackWithPreview pack + preview，handler 渲染时按 with_preview 决定是否带
type PackWithPreview struct {
	Pack    entity.StickerPack
	Preview []PackPreviewEmoji
}

// PackRepository 表情包数据访问接口
type PackRepository interface {
	// List 按 filter 分页查询，返回当页 items + 总数
	List(ctx context.Context, f PackFilter) ([]entity.StickerPack, int64, error)
	// ListWithPreview 在 List 基础上批量注入每个 pack 的 preview emoji（一次性 IN 查询防 N+1）
	ListWithPreview(ctx context.Context, f PackFilter) ([]PackWithPreview, int64, error)
	FindByShortName(ctx context.Context, shortName string) (*entity.StickerPack, error)
	FindByID(ctx context.Context, id uint64) (*entity.StickerPack, error)
	Create(ctx context.Context, pack *entity.StickerPack) error
}

// StatsOverview 首页 stats 数据
type StatsOverview struct {
	TotalEmojis   int64     // emoji_stickers 总数
	TotalPacks    int64     // sticker_packs 总数
	AnimatedCount int64     // mime IN ('video/webm','application/x-tgsticker')
	AddedToday    int64     // created_at >= 今天零点，按服务进程本地时区算，不是 UTC
	LastUpdatedAt time.Time // 取数时刻（数据生成时间），handler 序列化为 ISO 8601
}

// StatsRepository 统计数据接口
type StatsRepository interface {
	// Overview 一次性返回首页所需 4 个聚合指标
	Overview(ctx context.Context) (*StatsOverview, error)
}
