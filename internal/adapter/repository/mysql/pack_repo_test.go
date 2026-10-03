package mysql

import (
	"context"
	"testing"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
)

func TestPackRepo_List_HotSort(t *testing.T) {
	db := setupDB(t)

	// 3 个包，sticker_count 升序入库，hot 应反过来
	p1 := &entity.StickerPack{ShortName: "Low", Title: "Low", Source: entity.PackSourceFeatured, StickerCount: 10}
	p2 := &entity.StickerPack{ShortName: "High", Title: "High", Source: entity.PackSourceFeatured, StickerCount: 100}
	p3 := &entity.StickerPack{ShortName: "Mid", Title: "Mid", Source: entity.PackSourceFeatured, StickerCount: 50}
	for _, p := range []*entity.StickerPack{p1, p2, p3} {
		if err := db.Create(p).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := NewPackRepo(db)
	got, total, err := repo.List(context.Background(), port.PackFilter{
		Sort:     port.PackSortHot,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if total != 3 {
		t.Errorf("total=%d want 3", total)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].ShortName != "High" || got[1].ShortName != "Mid" || got[2].ShortName != "Low" {
		t.Errorf("hot 排序错: %s/%s/%s", got[0].ShortName, got[1].ShortName, got[2].ShortName)
	}
}

func TestPackRepo_ListWithPreview(t *testing.T) {
	db := setupDB(t)

	p1 := &entity.StickerPack{ShortName: "PackA", Title: "A", Source: entity.PackSourceFeatured, StickerCount: 5}
	p2 := &entity.StickerPack{ShortName: "PackB", Title: "B", Source: entity.PackSourceManual, StickerCount: 3}
	if err := db.Create(p1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(p2).Error; err != nil {
		t.Fatal(err)
	}

	// 给 p1 加 6 个 emoji，p2 加 2 个 — preview 默认取 4 个
	for i := 0; i < 6; i++ {
		e := &entity.EmojiSticker{
			CustomEmojiID: "p1_" + itoa(i),
			Alt:           "🔥",
			Keywords:      "a",
			PackID:        p1.ID,
			ThumbnailPath: "thumbs/p1_" + itoa(i) + ".webp",
		}
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		e := &entity.EmojiSticker{
			CustomEmojiID: "p2_" + itoa(i),
			Alt:           "💎",
			Keywords:      "b",
			PackID:        p2.ID,
		}
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := NewPackRepo(db)
	got, total, err := repo.ListWithPreview(context.Background(), port.PackFilter{
		WithPreview: true,
		PreviewSize: 4,
		Page:        1,
		PageSize:    10,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if total != 2 {
		t.Errorf("total=%d want 2", total)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}

	previewByPack := map[string][]port.PackPreviewEmoji{}
	for _, item := range got {
		previewByPack[item.Pack.ShortName] = item.Preview
	}
	if n := len(previewByPack["PackA"]); n != 4 {
		t.Errorf("PackA preview 应取 4 个, got %d", n)
	}
	if n := len(previewByPack["PackB"]); n != 2 {
		t.Errorf("PackB preview 应取 2 个, got %d", n)
	}
}

func TestPackRepo_ListWithPreview_DefaultPreviewSize(t *testing.T) {
	db := setupDB(t)
	p := &entity.StickerPack{ShortName: "P", Title: "P", Source: entity.PackSourceFeatured}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err := db.Create(&entity.EmojiSticker{
			CustomEmojiID: "x_" + itoa(i),
			Alt:           "🔥",
			Keywords:      "a",
			PackID:        p.ID,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := NewPackRepo(db)
	got, _, err := repo.ListWithPreview(context.Background(), port.PackFilter{
		WithPreview: true,
		Page:        1,
		PageSize:    10,
		// PreviewSize 留 0，repo 应兜底 4
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if n := len(got[0].Preview); n != 4 {
		t.Errorf("默认 preview_size 应为 4, got %d", n)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
