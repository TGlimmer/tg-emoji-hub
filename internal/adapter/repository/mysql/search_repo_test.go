package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
)

// setupDB 用 in-memory sqlite 替代 MySQL，dialect 不是 mysql → repo 走 LIKE 降级路径
func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.StickerPack{}, &entity.EmojiSticker{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func seed(t *testing.T, db *gorm.DB) (*entity.StickerPack, *entity.StickerPack) {
	t.Helper()
	p1 := &entity.StickerPack{ShortName: "AnimatedEmojies", Title: "Animated Emojies", IsFeatured: true, Source: entity.SourceFeatured}
	p2 := &entity.StickerPack{ShortName: "MicrosoftEmoji", Title: "MS Emoji", IsFeatured: false, Source: entity.SourceManual}
	if err := db.Create(p1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(p2).Error; err != nil {
		t.Fatal(err)
	}

	emojis := []*entity.EmojiSticker{
		{CustomEmojiID: "1001", Alt: "🔥", Keywords: "fire,flame", PackID: p1.ID, IsAnimated: true},
		{CustomEmojiID: "1002", Alt: "💎", Keywords: "diamond,gem", PackID: p1.ID, IsAnimated: false},
		{CustomEmojiID: "1003", Alt: "🔥", Keywords: "fire,hot,burn", PackID: p2.ID, IsAnimated: false},
		{CustomEmojiID: "1004", Alt: "✨", Keywords: "sparkle,star,magic", PackID: p2.ID, IsAnimated: true},
	}
	for _, e := range emojis {
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}
	return p1, p2
}

func TestSearchRepo_AltExactMatch(t *testing.T) {
	db := setupDB(t)
	seed(t, db)
	repo := NewSearchRepo(db)

	results, err := repo.SearchByKeyword(context.Background(), port.SearchFilter{Query: "🔥", Limit: 10})
	if err != nil {
		t.Fatalf("search err: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("应命中 2 条 🔥, got %d", len(results))
	}
	for _, r := range results {
		if r.Alt != "🔥" {
			t.Errorf("alt 不对, got %q", r.Alt)
		}
		if r.Pack == nil {
			t.Errorf("Pack 未注入")
		}
	}
}

func TestSearchRepo_KeywordLike(t *testing.T) {
	db := setupDB(t)
	seed(t, db)
	repo := NewSearchRepo(db)

	results, err := repo.SearchByKeyword(context.Background(), port.SearchFilter{Query: "fire", Limit: 10})
	if err != nil {
		t.Fatalf("search err: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("keywords like 'fire' 应命中 2 条, got %d", len(results))
	}
}

func TestSearchRepo_PackFilter(t *testing.T) {
	db := setupDB(t)
	_, p2 := seed(t, db)
	repo := NewSearchRepo(db)

	results, err := repo.SearchByKeyword(context.Background(), port.SearchFilter{
		Query: "fire", Limit: 10, Pack: p2.ShortName,
	})
	if err != nil {
		t.Fatalf("search err: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("按包过滤后应只剩 1 条, got %d", len(results))
	}
	if results[0].CustomEmojiID != "1003" {
		t.Errorf("应命中 1003, got %q", results[0].CustomEmojiID)
	}
}

func TestSearchRepo_AnimatedOnly(t *testing.T) {
	db := setupDB(t)
	seed(t, db)
	repo := NewSearchRepo(db)

	results, err := repo.SearchByKeyword(context.Background(), port.SearchFilter{
		Query: "fire", Limit: 10, AnimatedOnly: true,
	})
	if err != nil {
		t.Fatalf("search err: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("仅 1 条动态 fire, got %d", len(results))
	}
	if !results[0].IsAnimated {
		t.Errorf("结果应是 animated")
	}
}

func TestSearchRepo_EmptyQuery(t *testing.T) {
	db := setupDB(t)
	repo := NewSearchRepo(db)
	_, err := repo.SearchByKeyword(context.Background(), port.SearchFilter{Query: "   "})
	if !errors.Is(err, apperr.ErrInvalidParam) {
		t.Fatalf("空 query 应返回 ErrInvalidParam, got %v", err)
	}
}

func TestSearchRepo_FindByCustomEmojiID(t *testing.T) {
	db := setupDB(t)
	seed(t, db)
	repo := NewSearchRepo(db)

	e, err := repo.FindByCustomEmojiID(context.Background(), "1002")
	if err != nil {
		t.Fatalf("find err: %v", err)
	}
	if e.Alt != "💎" {
		t.Errorf("alt 不对, got %q", e.Alt)
	}
	if e.Pack == nil || e.Pack.ShortName != "AnimatedEmojies" {
		t.Errorf("Pack 未注入或不对")
	}
}

func TestSearchRepo_FindByID_NotFound(t *testing.T) {
	db := setupDB(t)
	repo := NewSearchRepo(db)
	_, err := repo.FindByCustomEmojiID(context.Background(), "9999")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("应返回 ErrNotFound, got %v", err)
	}
}

// TestSearchRepo_RelevanceOrdering 验证 score 优先级：alt 精确(100) > CSV 整词(80) > 子串(50)
func TestSearchRepo_RelevanceOrdering(t *testing.T) {
	db := setupDB(t)
	p := &entity.StickerPack{ShortName: "TestPack", Title: "Test", Source: entity.SourceManual}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}

	// 同一 query "fire" 触发三种不同 score 路径
	// substring:  keywords="firework,celebration" 子串命中 → score 50
	// csv_word:   keywords="fire,red,hot" 整词命中 → score 80
	// alt_exact:  alt="fire" 精确等于 → score 100（同时 csv 也命中，但 GREATEST 取最高）
	emojis := []*entity.EmojiSticker{
		{CustomEmojiID: "subs", Alt: "🎆", Keywords: "firework,celebration", PackID: p.ID},
		{CustomEmojiID: "csv", Alt: "🔥", Keywords: "fire,red,hot", PackID: p.ID},
		{CustomEmojiID: "alt", Alt: "fire", Keywords: "fire", PackID: p.ID},
	}
	for _, e := range emojis {
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := NewSearchRepo(db)
	results, err := repo.SearchByKeyword(context.Background(), port.SearchFilter{Query: "fire", Limit: 10})
	if err != nil {
		t.Fatalf("search err: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("应命中 3 条, got %d", len(results))
	}
	// 期望顺序：alt(100) > csv(80) > subs(50)
	wantOrder := []string{"alt", "csv", "subs"}
	for i, want := range wantOrder {
		if results[i].CustomEmojiID != want {
			t.Errorf("位置 %d 应是 %q，got %q (full order: %v)",
				i, want, results[i].CustomEmojiID, customIDs(results))
		}
	}
}

// TestSearchRepo_ChineseSubstring 验证中文 query 走子串路径（CSV 整词也能匹配）
func TestSearchRepo_ChineseSubstring(t *testing.T) {
	db := setupDB(t)
	p := &entity.StickerPack{ShortName: "CN", Title: "中文", Source: entity.SourceManual}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}

	emojis := []*entity.EmojiSticker{
		// 整词命中 (CSV 80 分)
		{CustomEmojiID: "cn_word", Alt: "🕯️", Keywords: "蜡烛,火焰", PackID: p.ID},
		// 子串命中 (50 分，"生日蜡烛" 含 "蜡烛" 但不是独立 CSV 词)
		{CustomEmojiID: "cn_sub", Alt: "🎂", Keywords: "生日蜡烛,蛋糕", PackID: p.ID},
		// 不命中：无关 emoji，确保不会被错误带出
		{CustomEmojiID: "cn_none", Alt: "🇮🇹", Keywords: "italy,flag", PackID: p.ID},
	}
	for _, e := range emojis {
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := NewSearchRepo(db)
	results, err := repo.SearchByKeyword(context.Background(), port.SearchFilter{Query: "蜡烛", Limit: 10})
	if err != nil {
		t.Fatalf("search err: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("蜡烛应命中 2 条, got %d (%v)", len(results), customIDs(results))
	}
	if results[0].CustomEmojiID != "cn_word" {
		t.Errorf("第 1 条应是 cn_word (CSV 整词 80 分), got %q", results[0].CustomEmojiID)
	}
	if results[1].CustomEmojiID != "cn_sub" {
		t.Errorf("第 2 条应是 cn_sub (子串 50 分), got %q", results[1].CustomEmojiID)
	}
}

// TestSearchRepo_LikeEscape 验证 query 含 LIKE 通配符不会越界匹配
func TestSearchRepo_LikeEscape(t *testing.T) {
	db := setupDB(t)
	p := &entity.StickerPack{ShortName: "Esc", Title: "Esc", Source: entity.SourceManual}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	emojis := []*entity.EmojiSticker{
		{CustomEmojiID: "lit", Alt: "X", Keywords: "fi_re,literal", PackID: p.ID},
		{CustomEmojiID: "fake", Alt: "Y", Keywords: "fire,not_match", PackID: p.ID},
	}
	for _, e := range emojis {
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewSearchRepo(db)
	// "fi_re" 字面查找，`_` 不应通配命中 "fire"
	results, err := repo.SearchByKeyword(context.Background(), port.SearchFilter{Query: "fi_re", Limit: 10})
	if err != nil {
		t.Fatalf("search err: %v", err)
	}
	if len(results) != 1 || results[0].CustomEmojiID != "lit" {
		t.Errorf("escape 失败，预期仅命中 lit，got %v", customIDs(results))
	}
}

func customIDs(rows []entity.EmojiSticker) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.CustomEmojiID)
	}
	return out
}

func TestSearchRepo_LimitClamp(t *testing.T) {
	db := setupDB(t)
	seed(t, db)
	repo := NewSearchRepo(db)

	// Limit=0 应被 clamp 到 50
	results, err := repo.SearchByKeyword(context.Background(), port.SearchFilter{Query: "fire", Limit: 0})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(results) > 50 {
		t.Errorf("limit clamp 失败")
	}
}
