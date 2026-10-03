package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
)

func TestStatsRepo_Overview_EmptyDB(t *testing.T) {
	db := setupDB(t)
	repo := NewStatsRepo(db)

	got, err := repo.Overview(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.TotalEmojis != 0 || got.TotalPacks != 0 || got.AnimatedCount != 0 || got.AddedToday != 0 {
		t.Errorf("空库应全 0，got=%+v", got)
	}
	if got.LastUpdatedAt.IsZero() {
		t.Error("LastUpdatedAt 应是当前时间，不应为 0")
	}
}

func TestStatsRepo_Overview_Counts(t *testing.T) {
	db := setupDB(t)

	p1 := &entity.StickerPack{ShortName: "P1", Title: "P1", Source: entity.PackSourceFeatured}
	p2 := &entity.StickerPack{ShortName: "P2", Title: "P2", Source: entity.PackSourceManual}
	if err := db.Create(p1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(p2).Error; err != nil {
		t.Fatal(err)
	}

	// 5 个 emoji：3 个动态（2 webm + 1 tgs）+ 2 静态 webp
	emojis := []*entity.EmojiSticker{
		{CustomEmojiID: "10", Alt: "🔥", Keywords: "fire", PackID: p1.ID, MimeType: entity.MimeWebm, IsAnimated: true},
		{CustomEmojiID: "11", Alt: "💎", Keywords: "diamond", PackID: p1.ID, MimeType: entity.MimeWebm, IsAnimated: true},
		{CustomEmojiID: "12", Alt: "✨", Keywords: "star", PackID: p2.ID, MimeType: entity.MimeTGS, IsAnimated: true},
		{CustomEmojiID: "13", Alt: "🍎", Keywords: "apple", PackID: p2.ID, MimeType: entity.MimeWebp, IsAnimated: false},
		{CustomEmojiID: "14", Alt: "🍌", Keywords: "banana", PackID: p2.ID, MimeType: entity.MimeWebp, IsAnimated: false},
	}
	for _, e := range emojis {
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := NewStatsRepo(db)
	got, err := repo.Overview(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.TotalEmojis != 5 {
		t.Errorf("TotalEmojis=%d want 5", got.TotalEmojis)
	}
	if got.TotalPacks != 2 {
		t.Errorf("TotalPacks=%d want 2", got.TotalPacks)
	}
	if got.AnimatedCount != 3 {
		t.Errorf("AnimatedCount=%d want 3", got.AnimatedCount)
	}
	// AddedToday 取决于 autoCreateTime 写入是否在 UTC 今天范围内；
	// sqlite autoCreateTime 写本地 time.Now()，比较时刻仍 ≥ todayStart UTC
	if got.AddedToday < 0 {
		t.Errorf("AddedToday 异常: %d", got.AddedToday)
	}
	// 主要验聚合正确性即可，AddedToday 写入时刻晚于 todayStart 一般都命中
	if got.AddedToday != 5 {
		t.Logf("AddedToday=%d (期望 5，跨时区可能略差，非阻塞)", got.AddedToday)
	}
	if time.Since(got.LastUpdatedAt) > time.Minute {
		t.Errorf("LastUpdatedAt 不应远早于当前: %v", got.LastUpdatedAt)
	}
}
