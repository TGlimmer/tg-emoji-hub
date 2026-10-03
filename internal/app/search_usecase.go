package app

import (
	"context"
	"strings"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
)

// SearchUseCase 搜索用例
type SearchUseCase struct {
	repo port.SearchRepository
}

func NewSearchUseCase(repo port.SearchRepository) *SearchUseCase {
	return &SearchUseCase{repo: repo}
}

// Search 校验输入 → 走 repo
func (uc *SearchUseCase) Search(ctx context.Context, f port.SearchFilter) ([]entity.EmojiSticker, error) {
	f.Query = strings.TrimSpace(f.Query)
	f.Pack = strings.TrimSpace(f.Pack)
	f.Category = strings.TrimSpace(f.Category)
	if f.Query == "" && f.Pack == "" && f.Category == "" {
		return nil, apperr.ErrInvalidParam.WithMessage("q / pack / category 至少传一个")
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		return nil, apperr.ErrInvalidParam.WithMessage("limit 不能超过 200")
	}
	return uc.repo.SearchByKeyword(ctx, f)
}

// Random 校验 limit 后走 repo
func (uc *SearchUseCase) Random(ctx context.Context, limit int) ([]entity.EmojiSticker, error) {
	if limit <= 0 {
		limit = 48
	}
	if limit > 100 {
		return nil, apperr.ErrInvalidParam.WithMessage("limit 不能超过 100")
	}
	return uc.repo.Random(ctx, limit)
}

// GetByID 取单条详情
func (uc *SearchUseCase) GetByID(ctx context.Context, id string) (*entity.EmojiSticker, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, apperr.ErrInvalidParam.WithMessage("custom_emoji_id 不能为空")
	}
	return uc.repo.FindByCustomEmojiID(ctx, id)
}
