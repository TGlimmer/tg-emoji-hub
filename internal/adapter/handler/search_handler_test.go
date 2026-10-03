package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/TGlimmer/tg-emoji-hub/internal/adapter/repository/mysql"
	"github.com/TGlimmer/tg-emoji-hub/internal/app"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
)

// setupEngine 装一个 in-memory sqlite + 完整 handler 链路，方便 e2e 验证 API 响应结构
func setupEngine(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.StickerPack{}, &entity.EmojiSticker{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	// seed
	pack := &entity.StickerPack{
		ShortName: "AnimatedEmojies",
		Title:     "Animated Emojies",
		Source:    entity.NormalizeSource("featured"),
	}
	if err := db.Create(pack).Error; err != nil {
		t.Fatal(err)
	}
	emojis := []*entity.EmojiSticker{
		{CustomEmojiID: "5404674261681232568", Alt: "🔥", Keywords: "fire,flame", PackID: pack.ID, IsAnimated: true},
		{CustomEmojiID: "1234567890", Alt: "💎", Keywords: "diamond,gem", PackID: pack.ID},
	}
	for _, e := range emojis {
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}

	searchRepo := mysql.NewSearchRepo(db)
	packRepo := mysql.NewPackRepo(db)
	searchUC := app.NewSearchUseCase(searchRepo)
	packUC := app.NewPackUseCase(packRepo, t.TempDir())
	sh := NewSearchHandler(searchUC, "http://localhost:8080")
	ph := NewPackHandler(packUC, "http://localhost:8080")
	hh := NewHealthHandler(func() error { return nil }, "0.1.0")

	r := gin.New()
	Register(r, Deps{
		SearchHandler: sh,
		PackHandler:   ph,
		HealthHandler: hh,
		ThumbnailDir:  "./thumbs",
		MediaDir:      "./emoji",
	})
	return r, db
}

func TestE2E_SearchHit(t *testing.T) {
	r, _ := setupEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=fire&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v, body=%s", err, w.Body.String())
	}
	if resp.Code != 0 {
		t.Errorf("code = %d, want 0", resp.Code)
	}
	if resp.Data.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Data.Total)
	}
	if len(resp.Data.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(resp.Data.Items))
	}
	item := resp.Data.Items[0]
	if item["custom_emoji_id"] != "5404674261681232568" {
		t.Errorf("custom_emoji_id 不对: %v", item["custom_emoji_id"])
	}
	formats, ok := item["formats"].(map[string]any)
	if !ok {
		t.Fatalf("formats 不是 map: %T", item["formats"])
	}
	if formats["id"] != "5404674261681232568" {
		t.Errorf("formats.id 不对: %v", formats["id"])
	}
	if formats["html"] != `<tg-emoji emoji-id="5404674261681232568">🔥</tg-emoji>` {
		t.Errorf("formats.html 不对: %v", formats["html"])
	}
	if formats["go_map"] != `"fire": "5404674261681232568",` {
		t.Errorf("formats.go_map 不对: %v", formats["go_map"])
	}
}

func TestE2E_SearchEmptyQuery(t *testing.T) {
	r, _ := setupEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("空 q 应 400, got %d", w.Code)
	}
}

func TestE2E_SearchLimitTooLarge(t *testing.T) {
	r, _ := setupEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=fire&limit=999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("limit 超 200 应 400, got %d", w.Code)
	}
}

func TestE2E_Healthz(t *testing.T) {
	r, _ := setupEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" || resp["db"] != "ok" {
		t.Errorf("healthz 响应不对: %+v", resp)
	}
}

func TestE2E_ThumbnailPathTraversal(t *testing.T) {
	r, _ := setupEngine(t)
	// 路径穿越尝试 → 必须 404
	tests := []string{
		"/thumbs/..%2F..%2Fetc%2Fpasswd",
		"/thumbs/abc.exe",
		"/thumbs/123.txt",
	}
	for _, p := range tests {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("路径 %q 应 404，got %d", p, w.Code)
		}
	}
}
