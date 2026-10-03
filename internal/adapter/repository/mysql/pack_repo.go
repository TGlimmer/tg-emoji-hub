package mysql

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
)

// PackRepo PackRepository 的 GORM 实现
type PackRepo struct {
	db *gorm.DB
}

func NewPackRepo(db *gorm.DB) *PackRepo {
	return &PackRepo{db: db}
}

// List 分页 + 按 source / sort 过滤
//
// f.Source 取值：""/"all" → 全部；"featured"/"manual" → 精确过滤
// f.Sort   取值：newest（默认） / hot / name
// page / page_size 在 usecase 层已做边界，repo 内不重复校验
func (r *PackRepo) List(ctx context.Context, f port.PackFilter) ([]entity.StickerPack, int64, error) {
	buildQuery := func() *gorm.DB {
		q := r.db.WithContext(ctx).Model(&entity.StickerPack{})
		if f.Source != "" && f.Source != "all" {
			q = q.Where("source = ?", f.Source)
		}
		return q
	}

	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		return nil, 0, apperr.ErrInternal.WithCause(err)
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	size := f.PageSize
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	q := buildQuery()
	q = applyPackSort(q, port.NormalizePackSort(string(f.Sort)))

	var rows []entity.StickerPack
	err := q.
		Offset((page - 1) * size).
		Limit(size).
		Find(&rows).Error
	if err != nil {
		return nil, 0, apperr.ErrInternal.WithCause(err)
	}
	return rows, total, nil
}

// ListWithPreview 在 List 基础上为每个 pack 注入 preview emoji（默认 4 个）
//
// 实现：先 List 拿到 pack 列表 → 收集 pack_id → 一条 IN 查询批量拿
// 每个 pack 前 N 个 emoji（按 id ASC，保证稳定性）→ 在内存中分组归位。
// 避免 N+1：N 个 pack 也只多 1 次 emoji 查询。
func (r *PackRepo) ListWithPreview(ctx context.Context, f port.PackFilter) ([]port.PackWithPreview, int64, error) {
	packs, total, err := r.List(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	if len(packs) == 0 {
		return []port.PackWithPreview{}, total, nil
	}

	previewSize := f.PreviewSize
	if previewSize <= 0 {
		previewSize = 4
	}
	if previewSize > 20 {
		previewSize = 20
	}

	previewMap, err := r.fetchPreviewMap(ctx, packs, previewSize)
	if err != nil {
		return nil, 0, err
	}

	out := make([]port.PackWithPreview, 0, len(packs))
	for i := range packs {
		out = append(out, port.PackWithPreview{
			Pack:    packs[i],
			Preview: previewMap[packs[i].ID],
		})
	}
	return out, total, nil
}

// fetchPreviewMap 拉每个 pack 的前 previewSize 个 emoji
//
// SQLite 无 LATERAL JOIN，MySQL 8 有但 GORM 拼起来太重；
// 用"每个 pack 限量"很难一条 SQL 搞定（要 ROW_NUMBER() 窗口函数），
// 折中：一次 IN 拉所有 emoji（按 pack_id, id ASC 排），内存裁剪到每包前 N 个。
// pack 数量 ≤ 100（page_size 上限），每包平均几十到几百 emoji，总条数可控。
// 真发现性能压力再换 UNION ALL 多子查询方案。
func (r *PackRepo) fetchPreviewMap(ctx context.Context, packs []entity.StickerPack, previewSize int) (map[uint64][]port.PackPreviewEmoji, error) {
	packIDs := make([]uint64, 0, len(packs))
	for i := range packs {
		packIDs = append(packIDs, packs[i].ID)
	}

	type row struct {
		PackID        uint64
		CustomEmojiID string
		Alt           string
		ThumbnailPath string
		IsAnimated    bool
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("emoji_stickers").
		Select("pack_id, custom_emoji_id, alt, thumbnail_path, is_animated").
		Where("pack_id IN ?", packIDs).
		Order("pack_id ASC").
		Order("id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}

	out := make(map[uint64][]port.PackPreviewEmoji, len(packs))
	for _, r := range rows {
		if len(out[r.PackID]) >= previewSize {
			continue
		}
		out[r.PackID] = append(out[r.PackID], port.PackPreviewEmoji{
			CustomEmojiID: r.CustomEmojiID,
			Alt:           r.Alt,
			ThumbnailPath: r.ThumbnailPath,
			IsAnimated:    r.IsAnimated,
		})
	}
	return out, nil
}

// applyPackSort 排序映射
//
// newest: is_featured DESC, id DESC（兼容旧默认行为）
// hot:    is_featured DESC（featured 优先）, sticker_count DESC, last_crawled_at DESC NULLS LAST
// name:   short_name ASC
func applyPackSort(q *gorm.DB, sort port.PackSort) *gorm.DB {
	switch sort {
	case port.PackSortHot:
		return q.
			Order("is_featured DESC").
			Order("sticker_count DESC").
			// last_crawled_at NULL 排在最后：用 IS NULL 表达式跨方言通用
			Order("last_crawled_at IS NULL ASC").
			Order("last_crawled_at DESC").
			Order("id DESC")
	case port.PackSortName:
		return q.Order("short_name ASC")
	default:
		return q.
			Order("is_featured DESC").
			Order("id DESC")
	}
}

func (r *PackRepo) FindByShortName(ctx context.Context, shortName string) (*entity.StickerPack, error) {
	var pack entity.StickerPack
	err := r.db.WithContext(ctx).Where("short_name = ?", shortName).First(&pack).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.ErrNotFound.WithMessage("pack 不存在")
		}
		return nil, apperr.ErrInternal.WithCause(err)
	}
	return &pack, nil
}

func (r *PackRepo) FindByID(ctx context.Context, id uint64) (*entity.StickerPack, error) {
	var pack entity.StickerPack
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&pack).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.ErrNotFound.WithMessage("pack 不存在")
		}
		return nil, apperr.ErrInternal.WithCause(err)
	}
	return &pack, nil
}

// Create 仅新建，调用方需先 FindByShortName 防重；这里若违反 UNIQUE 返 ErrConflict
func (r *PackRepo) Create(ctx context.Context, pack *entity.StickerPack) error {
	pack.Source = entity.NormalizeSource(pack.Source)
	if err := r.db.WithContext(ctx).Create(pack).Error; err != nil {
		return apperr.ErrConflict.WithCause(err)
	}
	return nil
}
