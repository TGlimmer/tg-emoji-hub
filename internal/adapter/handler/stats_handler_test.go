package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	mysqlrepo "github.com/TGlimmer/tg-emoji-hub/internal/adapter/repository/mysql"
	"github.com/TGlimmer/tg-emoji-hub/internal/app"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
)

// setupFullEngine 装 search + pack + stats 三件套，给新接口跑 e2e
func setupFullEngine(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.StickerPack{}, &entity.EmojiSticker{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	searchRepo := mysqlrepo.NewSearchRepo(db)
	packRepo := mysqlrepo.NewPackRepo(db)
	statsRepo := mysqlrepo.NewStatsRepo(db)

	searchUC := app.NewSearchUseCase(searchRepo)
	packUC := app.NewPackUseCase(packRepo, t.TempDir())
	statsUC := app.NewStatsUseCase(statsRepo)

	sh := NewSearchHandler(searchUC, "http://localhost:8080")
	ph := NewPackHandler(packUC, "http://localhost:8080")
	sth := NewStatsHandler(statsUC)
	hh := NewHealthHandler(func() error { return nil }, "0.1.0")

	r := gin.New()
	Register(r, Deps{
		SearchHandler: sh,
		PackHandler:   ph,
		StatsHandler:  sth,
		HealthHandler: hh,
		ThumbnailDir:  t.TempDir(),
		MediaDir:      t.TempDir(),
	})
	return r, db
}

func TestE2E_Stats_Empty(t *testing.T) {
	r, _ := setupFullEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("code=%d want 0", resp.Code)
	}
	// 空库时百分比应为 0，不应除 0 崩溃
	if v, ok := resp.Data["animated_percentage"]; !ok || toInt(v) != 0 {
		t.Errorf("animated_percentage 应为 0, got %v", v)
	}
	if _, ok := resp.Data["updated_at"]; !ok {
		t.Error("缺 updated_at")
	}
}

func TestE2E_Stats_WithData(t *testing.T) {
	r, db := setupFullEngine(t)

	p := &entity.StickerPack{ShortName: "P", Title: "P", Source: entity.PackSourceFeatured}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	// 4 个 emoji：3 动态 + 1 静态 = 75%
	emojis := []*entity.EmojiSticker{
		{CustomEmojiID: "1", Alt: "🔥", Keywords: "fire", PackID: p.ID, MimeType: entity.MimeWebm, IsAnimated: true},
		{CustomEmojiID: "2", Alt: "💎", Keywords: "gem", PackID: p.ID, MimeType: entity.MimeWebm, IsAnimated: true},
		{CustomEmojiID: "3", Alt: "✨", Keywords: "star", PackID: p.ID, MimeType: entity.MimeTGS, IsAnimated: true},
		{CustomEmojiID: "4", Alt: "🍎", Keywords: "apple", PackID: p.ID, MimeType: entity.MimeWebp, IsAnimated: false},
	}
	for _, e := range emojis {
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if toInt(resp.Data["total_emojis"]) != 4 {
		t.Errorf("total_emojis=%v want 4", resp.Data["total_emojis"])
	}
	if toInt(resp.Data["total_packs"]) != 1 {
		t.Errorf("total_packs=%v want 1", resp.Data["total_packs"])
	}
	if toInt(resp.Data["animated_count"]) != 3 {
		t.Errorf("animated_count=%v want 3", resp.Data["animated_count"])
	}
	if toInt(resp.Data["animated_percentage"]) != 75 {
		t.Errorf("animated_percentage=%v want 75", resp.Data["animated_percentage"])
	}
}

func TestE2E_Random_DefaultLimit(t *testing.T) {
	r, db := setupFullEngine(t)
	p := &entity.StickerPack{ShortName: "P", Title: "P", Source: entity.PackSourceFeatured}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err := db.Create(&entity.EmojiSticker{
			CustomEmojiID: "rnd_" + strItoa(i),
			Alt:           "🔥",
			Keywords:      "fire",
			PackID:        p.ID,
			MimeType:      entity.MimeWebp,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/emoji/random", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Total != 10 {
		t.Errorf("total=%d want 10", resp.Data.Total)
	}
	// 复用 search renderItem：每条必带 formats / media_url / thumbnail_url
	if len(resp.Data.Items) > 0 {
		it := resp.Data.Items[0]
		if _, ok := it["formats"]; !ok {
			t.Error("random item 缺 formats（应复用 search 渲染）")
		}
		if _, ok := it["media_url"]; !ok {
			t.Error("random item 缺 media_url")
		}
	}
}

func TestE2E_Random_RejectLimitTooLarge(t *testing.T) {
	r, _ := setupFullEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/emoji/random?limit=999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("limit 超 100 应 400, got %d", w.Code)
	}
}

func TestE2E_Random_RejectInvalidLimit(t *testing.T) {
	r, _ := setupFullEngine(t)
	for _, q := range []string{"?limit=0", "?limit=-5", "?limit=abc"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/emoji/random"+q, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("非法 limit %q 应 400, got %d", q, w.Code)
		}
	}
}

func TestE2E_PacksHotSort_WithPreview(t *testing.T) {
	r, db := setupFullEngine(t)

	low := &entity.StickerPack{ShortName: "Low", Title: "Low", Source: entity.PackSourceFeatured, StickerCount: 1}
	high := &entity.StickerPack{ShortName: "High", Title: "High", Source: entity.PackSourceFeatured, StickerCount: 100}
	mid := &entity.StickerPack{ShortName: "Mid", Title: "Mid", Source: entity.PackSourceFeatured, StickerCount: 50}
	for _, p := range []*entity.StickerPack{low, high, mid} {
		if err := db.Create(p).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 6; i++ {
		if err := db.Create(&entity.EmojiSticker{
			CustomEmojiID: "h_" + strItoa(i),
			Alt:           "🔥",
			Keywords:      "fire",
			PackID:        high.ID,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packs?sort=hot&with_preview=true&page_size=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Total != 3 {
		t.Errorf("total=%d want 3", resp.Data.Total)
	}
	if len(resp.Data.Items) < 1 || resp.Data.Items[0]["short_name"] != "High" {
		t.Errorf("hot 排序首位应为 High, got %+v", resp.Data.Items)
	}
	prev, ok := resp.Data.Items[0]["preview_emojis"].([]any)
	if !ok {
		t.Fatalf("preview_emojis 应是数组, got %T", resp.Data.Items[0]["preview_emojis"])
	}
	if len(prev) != 4 {
		t.Errorf("High pack preview 应取 4 个, got %d", len(prev))
	}
	// preview item 必须只含 custom_emoji_id/alt/thumbnail_url，不含 media_url/mime_type
	if first, ok := prev[0].(map[string]any); ok {
		if _, has := first["media_url"]; has {
			t.Error("preview 不应含 media_url（设计文档约束）")
		}
		if _, has := first["mime_type"]; has {
			t.Error("preview 不应含 mime_type")
		}
		if first["thumbnail_url"] == nil || first["thumbnail_url"] == "" {
			t.Error("preview 应有 thumbnail_url")
		}
	}
}

func TestE2E_PacksWithoutPreview(t *testing.T) {
	r, db := setupFullEngine(t)
	p := &entity.StickerPack{ShortName: "P", Title: "P", Source: entity.PackSourceFeatured, StickerCount: 5}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Items) < 1 {
		t.Fatal("应有 1 个 pack")
	}
	if _, has := resp.Data.Items[0]["preview_emojis"]; has {
		t.Error("with_preview 未传时不应返 preview_emojis")
	}
}

func TestE2E_PacksRejectInvalidSort(t *testing.T) {
	r, _ := setupFullEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packs?sort=garbage", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("非法 sort 应 400, got %d", w.Code)
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

func strItoa(i int) string {
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
