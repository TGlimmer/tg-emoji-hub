package app

import (
	"context"
	"errors"
	"testing"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
)

// mockSearchRepo 测试用 mock
type mockSearchRepo struct {
	searchFn  func(ctx context.Context, f port.SearchFilter) ([]entity.EmojiSticker, error)
	findFn    func(ctx context.Context, id string) (*entity.EmojiSticker, error)
	lastQuery port.SearchFilter
}

func (m *mockSearchRepo) SearchByKeyword(ctx context.Context, f port.SearchFilter) ([]entity.EmojiSticker, error) {
	m.lastQuery = f
	if m.searchFn != nil {
		return m.searchFn(ctx, f)
	}
	return nil, nil
}

func (m *mockSearchRepo) FindByCustomEmojiID(ctx context.Context, id string) (*entity.EmojiSticker, error) {
	if m.findFn != nil {
		return m.findFn(ctx, id)
	}
	return nil, nil
}

func (m *mockSearchRepo) Random(ctx context.Context, limit int) ([]entity.EmojiSticker, error) {
	return nil, nil
}

func TestSearch_EmptyQueryRejected(t *testing.T) {
	uc := NewSearchUseCase(&mockSearchRepo{})
	_, err := uc.Search(context.Background(), port.SearchFilter{Query: "  "})
	if !errors.Is(err, apperr.ErrInvalidParam) {
		t.Fatalf("空 query 应返回 ErrInvalidParam, got %v", err)
	}
}

func TestSearch_LimitOver200Rejected(t *testing.T) {
	uc := NewSearchUseCase(&mockSearchRepo{})
	_, err := uc.Search(context.Background(), port.SearchFilter{Query: "fire", Limit: 999})
	if !errors.Is(err, apperr.ErrInvalidParam) {
		t.Fatalf("limit 超 200 应返回 ErrInvalidParam, got %v", err)
	}
}

func TestSearch_DefaultLimit50(t *testing.T) {
	mock := &mockSearchRepo{
		searchFn: func(ctx context.Context, f port.SearchFilter) ([]entity.EmojiSticker, error) {
			return []entity.EmojiSticker{}, nil
		},
	}
	uc := NewSearchUseCase(mock)
	_, err := uc.Search(context.Background(), port.SearchFilter{Query: "fire", Limit: 0})
	if err != nil {
		t.Fatalf("正常输入不应出错: %v", err)
	}
	if mock.lastQuery.Limit != 50 {
		t.Errorf("默认 limit 应为 50, got %d", mock.lastQuery.Limit)
	}
}

func TestSearch_TrimsQuery(t *testing.T) {
	mock := &mockSearchRepo{
		searchFn: func(ctx context.Context, f port.SearchFilter) ([]entity.EmojiSticker, error) {
			return nil, nil
		},
	}
	uc := NewSearchUseCase(mock)
	_, _ = uc.Search(context.Background(), port.SearchFilter{Query: "  fire  ", Limit: 10})
	if mock.lastQuery.Query != "fire" {
		t.Errorf("query 应被 trim, got %q", mock.lastQuery.Query)
	}
}

func TestSearch_PassesFilters(t *testing.T) {
	mock := &mockSearchRepo{
		searchFn: func(ctx context.Context, f port.SearchFilter) ([]entity.EmojiSticker, error) {
			return nil, nil
		},
	}
	uc := NewSearchUseCase(mock)
	_, _ = uc.Search(context.Background(), port.SearchFilter{
		Query: "fire", Limit: 10, Pack: "AnimatedEmojies", AnimatedOnly: true,
	})
	if mock.lastQuery.Pack != "AnimatedEmojies" {
		t.Errorf("Pack 没透传, got %q", mock.lastQuery.Pack)
	}
	if !mock.lastQuery.AnimatedOnly {
		t.Errorf("AnimatedOnly 没透传")
	}
}

func TestGetByID_EmptyRejected(t *testing.T) {
	uc := NewSearchUseCase(&mockSearchRepo{})
	_, err := uc.GetByID(context.Background(), "  ")
	if !errors.Is(err, apperr.ErrInvalidParam) {
		t.Fatalf("空 ID 应返回 ErrInvalidParam, got %v", err)
	}
}

func TestGetByID_PropagatesNotFound(t *testing.T) {
	mock := &mockSearchRepo{
		findFn: func(ctx context.Context, id string) (*entity.EmojiSticker, error) {
			return nil, apperr.ErrNotFound
		},
	}
	uc := NewSearchUseCase(mock)
	_, err := uc.GetByID(context.Background(), "123")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("NotFound 应被传递, got %v", err)
	}
}
