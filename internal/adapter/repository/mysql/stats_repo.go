package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
)

// StatsRepo StatsRepository 的 GORM 实现
//
// 设计文档明确：不引入 Redis（与 energy-rental 不同）。
// 万级数据 4 个 COUNT(*) < 50ms 直接查 DB；后续真有性能压力再上 sync.Map + TTL 内存缓存。
type StatsRepo struct {
	db *gorm.DB
}

func NewStatsRepo(db *gorm.DB) *StatsRepo {
	return &StatsRepo{db: db}
}

// Overview 串行跑 4 个 COUNT
//
// 万级数据每个 COUNT < 50ms，串行也在 200ms 内。曾试过 goroutine 并发但
// in-memory sqlite 走连接隔离测试跑不通；MySQL 生产侧瓶颈也不在这里，KISS。
func (r *StatsRepo) Overview(ctx context.Context) (*port.StatsOverview, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	out := &port.StatsOverview{LastUpdatedAt: now}

	if err := r.db.WithContext(ctx).Model(&entity.EmojiSticker{}).Count(&out.TotalEmojis).Error; err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	if err := r.db.WithContext(ctx).Model(&entity.StickerPack{}).Count(&out.TotalPacks).Error; err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	if err := r.db.WithContext(ctx).Model(&entity.EmojiSticker{}).
		Where("mime_type IN ?", []string{entity.MimeWebm, entity.MimeTGS}).
		Count(&out.AnimatedCount).Error; err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	if err := r.db.WithContext(ctx).Model(&entity.EmojiSticker{}).
		Where("created_at >= ?", todayStart).
		Count(&out.AddedToday).Error; err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	return out, nil
}
