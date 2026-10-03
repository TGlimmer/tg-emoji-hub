package mysql

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
)

// SearchRepo 基于 GORM 的搜索仓库实现
type SearchRepo struct {
	db *gorm.DB
}

func NewSearchRepo(db *gorm.DB) *SearchRepo {
	return &SearchRepo{db: db}
}

// SearchByKeyword 实现搜索流程
//
// 分层相关性 score（生产 MySQL）：
//
//	100  alt 精确等于 query（如 query="🔥" 对应 alt="🔥"）
//	 80  keywords CSV 整词命中（LOCATE 包逗号匹配，等价 REGEXP '(^|,)q(,|$)'，无 ReDoS）
//	 50  keywords 子串命中（LIKE，覆盖中文/前缀场景）
//	0-30 FULLTEXT NATURAL LANGUAGE 兜底分（LEAST 钳到 30，防止通用词污染压过精确匹配）
//
// 取 GREATEST 作为最终 relevance，按 (relevance DESC, is_animated DESC, created_at DESC) 排序。
// 走 Raw SQL — GORM Select 子句嵌占位符 ? 在某些场景下参数绑定顺序会错位
func (r *SearchRepo) SearchByKeyword(ctx context.Context, f port.SearchFilter) ([]entity.EmojiSticker, error) {
	q := strings.TrimSpace(f.Query)
	pack := strings.TrimSpace(f.Pack)
	category := strings.TrimSpace(f.Category)
	if q == "" && pack == "" && category == "" {
		return nil, apperr.ErrInvalidParam.WithMessage("query / pack / category 至少传一个")
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var rows []entity.EmojiSticker
	dialect := r.db.Dialector.Name()

	// q == "" 走 GORM builder（仅 pack/category 过滤）
	if q == "" {
		tx := r.db.WithContext(ctx).
			Table("emoji_stickers AS s").
			Select("s.*").
			Joins("LEFT JOIN sticker_packs p ON s.pack_id = p.id")
		if pack != "" {
			tx = tx.Where("p.short_name = ?", pack)
		}
		if category != "" {
			tx = tx.Where("s.category = ?", category)
		}
		if f.AnimatedOnly {
			tx = tx.Where("s.is_animated = ?", true)
		}
		if err := tx.Order("s.is_animated DESC").Order("s.created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
			return nil, apperr.ErrInternal.WithCause(err)
		}
		if err := r.preloadPacks(ctx, rows); err != nil {
			return nil, err
		}
		return rows, nil
	}

	// q != "" 走 Raw SQL（避开 GORM 占位符歧义）
	csvNeedle := "," + q + ","
	likePattern := "%" + escapeLikeBang(q) + "%"

	whereExtra := ""
	extraArgs := []any{}
	if pack != "" {
		whereExtra += " AND p.short_name = ?"
		extraArgs = append(extraArgs, pack)
	}
	if category != "" {
		whereExtra += " AND s.category = ?"
		extraArgs = append(extraArgs, category)
	}
	if f.AnimatedOnly {
		whereExtra += " AND s.is_animated = 1"
	}

	var sqlStr string
	var args []any

	if dialect == "mysql" {
		sqlStr = `SELECT s.*,
			GREATEST(
				CASE WHEN s.alt = ? THEN 100 ELSE 0 END,
				CASE WHEN LOCATE(?, CONCAT(',', s.keywords, ',')) > 0 THEN 80 ELSE 0 END,
				CASE WHEN s.keywords LIKE ? ESCAPE '!' THEN 50 ELSE 0 END,
				LEAST(MATCH(s.keywords, s.alt) AGAINST(? IN NATURAL LANGUAGE MODE), 30)
			) AS relevance
			FROM emoji_stickers AS s
			LEFT JOIN sticker_packs p ON s.pack_id = p.id
			WHERE (s.alt = ?
				OR LOCATE(?, CONCAT(',', s.keywords, ',')) > 0
				OR s.keywords LIKE ? ESCAPE '!'
				OR MATCH(s.keywords, s.alt) AGAINST(? IN NATURAL LANGUAGE MODE))` + whereExtra + `
			ORDER BY relevance DESC, s.is_animated DESC, s.created_at DESC
			LIMIT ?`
		args = []any{q, csvNeedle, likePattern, q}        // SELECT 4
		args = append(args, q, csvNeedle, likePattern, q) // WHERE 4
		args = append(args, extraArgs...)
		args = append(args, limit)
	} else {
		// SQLite 降级：无 MATCH AGAINST，前 3 层 + MAX 求最大值
		sqlStr = `SELECT s.*,
			MAX(
				CASE WHEN s.alt = ? THEN 100 ELSE 0 END,
				CASE WHEN INSTR(',' || s.keywords || ',', ?) > 0 THEN 80 ELSE 0 END,
				CASE WHEN s.keywords LIKE ? ESCAPE '!' THEN 50 ELSE 0 END
			) AS relevance
			FROM emoji_stickers AS s
			LEFT JOIN sticker_packs p ON s.pack_id = p.id
			WHERE (s.alt = ?
				OR INSTR(',' || s.keywords || ',', ?) > 0
				OR s.keywords LIKE ? ESCAPE '!')` + whereExtra + `
			ORDER BY relevance DESC, s.is_animated DESC, s.created_at DESC
			LIMIT ?`
		args = []any{q, csvNeedle, likePattern}
		args = append(args, q, csvNeedle, likePattern)
		args = append(args, extraArgs...)
		args = append(args, limit)
	}

	if err := r.db.WithContext(ctx).Raw(sqlStr, args...).Scan(&rows).Error; err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	if err := r.preloadPacks(ctx, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// FindByCustomEmojiID 单条查询
func (r *SearchRepo) FindByCustomEmojiID(ctx context.Context, id string) (*entity.EmojiSticker, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, apperr.ErrInvalidParam.WithMessage("custom_emoji_id 不能为空")
	}
	var row entity.EmojiSticker
	err := r.db.WithContext(ctx).Where("custom_emoji_id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.ErrNotFound.WithMessage("emoji 不存在")
		}
		return nil, apperr.ErrInternal.WithCause(err)
	}
	if err := r.preloadPacks(ctx, []entity.EmojiSticker{row}); err != nil {
		return nil, err
	}
	// preloadPacks 是按值切片操作，需要重新读出来
	var withPack entity.EmojiSticker
	if err := r.db.WithContext(ctx).Where("custom_emoji_id = ?", id).First(&withPack).Error; err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	var pack entity.StickerPack
	if err := r.db.WithContext(ctx).Where("id = ?", withPack.PackID).First(&pack).Error; err == nil {
		withPack.Pack = &pack
	}
	return &withPack, nil
}

// Random 拉 limit 条随机 emoji + 注入 Pack
func (r *SearchRepo) Random(ctx context.Context, limit int) ([]entity.EmojiSticker, error) {
	if limit <= 0 {
		limit = 48
	}
	if limit > 100 {
		limit = 100
	}

	orderExpr := "RAND()"
	if r.db.Dialector.Name() != "mysql" {
		orderExpr = "random()"
	}

	var rows []entity.EmojiSticker
	err := r.db.WithContext(ctx).
		Table("emoji_stickers").
		Order(orderExpr).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	if err := r.preloadPacks(ctx, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// preloadPacks 批量注入 Pack，避免每条记录单独 SELECT
func (r *SearchRepo) preloadPacks(ctx context.Context, rows []entity.EmojiSticker) error {
	if len(rows) == 0 {
		return nil
	}
	packIDs := make([]uint64, 0, len(rows))
	seen := make(map[uint64]bool, len(rows))
	for _, e := range rows {
		if !seen[e.PackID] {
			seen[e.PackID] = true
			packIDs = append(packIDs, e.PackID)
		}
	}
	var packs []entity.StickerPack
	if err := r.db.WithContext(ctx).Where("id IN ?", packIDs).Find(&packs).Error; err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	packMap := make(map[uint64]*entity.StickerPack, len(packs))
	for i := range packs {
		packMap[packs[i].ID] = &packs[i]
	}
	for i := range rows {
		if p, ok := packMap[rows[i].PackID]; ok {
			rows[i].Pack = p
		}
	}
	return nil
}

// escapeLikeBang 转义 LIKE 通配符（配合 ESCAPE '!'）
// 用 '!' 而非 '\' 避免 MySQL 字符串 escape 二次处理
func escapeLikeBang(s string) string {
	r := strings.NewReplacer(`!`, `!!`, `%`, `!%`, `_`, `!_`)
	return r.Replace(s)
}
