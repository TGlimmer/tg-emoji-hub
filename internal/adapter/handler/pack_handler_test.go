package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	mysqlrepo "github.com/TGlimmer/tg-emoji-hub/internal/adapter/repository/mysql"
	"github.com/TGlimmer/tg-emoji-hub/internal/app"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
)

func setupPackEngine(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.StickerPack{}, &entity.EmojiSticker{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	// seed 2 个包
	db.Create(&entity.StickerPack{ShortName: "Featured1", Title: "F1", IsFeatured: true, Source: "featured"})
	db.Create(&entity.StickerPack{ShortName: "Manual1", Title: "M1", IsFeatured: false, Source: "manual"})

	signalsDir := t.TempDir()
	searchRepo := mysqlrepo.NewSearchRepo(db)
	packRepo := mysqlrepo.NewPackRepo(db)
	searchUC := app.NewSearchUseCase(searchRepo)
	packUC := app.NewPackUseCase(packRepo, signalsDir)

	// mock Telegram 验证：始终返回"包存在"
	tgMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<a href="tg://addemoji?set=Test">Add Emoji</a>`)
	}))
	t.Cleanup(tgMock.Close)
	packUC.SetTelegramCheckURL(func(shortName string) string {
		return tgMock.URL + "/" + shortName
	})

	sh := NewSearchHandler(searchUC, "http://localhost:8080")
	ph := NewPackHandler(packUC, "http://localhost:8080")
	hh := NewHealthHandler(func() error { return nil }, "0.1.0")

	r := gin.New()
	Register(r, Deps{SearchHandler: sh, PackHandler: ph, HealthHandler: hh, ThumbnailDir: t.TempDir()})
	return r, db, signalsDir
}

func TestE2E_ListPacks(t *testing.T) {
	r, _, _ := setupPackEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items    []map[string]any `json:"items"`
			Total    int              `json:"total"`
			Page     int              `json:"page"`
			PageSize int              `json:"page_size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if resp.Code != 0 || resp.Data.Total != 2 || len(resp.Data.Items) != 2 {
		t.Errorf("响应不符: %+v", resp)
	}
	if resp.Data.Page != 1 || resp.Data.PageSize != 20 {
		t.Errorf("分页默认值不对: page=%d size=%d", resp.Data.Page, resp.Data.PageSize)
	}
}

func TestE2E_ListPacks_FilterBySource(t *testing.T) {
	r, _, _ := setupPackEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packs?source=manual", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Total != 1 {
		t.Errorf("仅 1 个 manual pack, got %d", resp.Data.Total)
	}
}

func TestE2E_ListPacks_RejectInvalidSource(t *testing.T) {
	r, _, _ := setupPackEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packs?source=garbage", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("非法 source 应 400, got %d", w.Code)
	}
}

func TestE2E_ListPacks_RejectInvalidPageSize(t *testing.T) {
	r, _, _ := setupPackEngine(t)
	for _, q := range []string{"?page=0", "?page_size=0", "?page_size=999", "?page=abc"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/packs"+q, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("非法分页 %q 应 400, got %d", q, w.Code)
		}
	}
}

func TestE2E_CreatePack_Success(t *testing.T) {
	r, _, signalsDir := setupPackEngine(t)

	body := []byte(`{"short_name":"TestPack123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("应 202, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			PackID    uint64 `json:"pack_id"`
			ShortName string `json:"short_name"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Status != "pending_crawl" {
		t.Errorf("status 错: %s", resp.Data.Status)
	}
	if resp.Data.PackID == 0 || resp.Data.ShortName != "TestPack123" {
		t.Errorf("data 错: %+v", resp.Data)
	}

	// 信号文件实际生成
	files, _ := filepath.Glob(filepath.Join(signalsDir, "*.pending"))
	if len(files) != 1 {
		t.Fatalf("应生成 1 个信号文件, got %d", len(files))
	}
	content, _ := os.ReadFile(files[0])
	if !bytes.Contains(content, []byte(`"retry_count":0`)) {
		t.Errorf("信号文件内容异常: %s", content)
	}
}

func TestE2E_CreatePack_AlreadyExists(t *testing.T) {
	r, _, _ := setupPackEngine(t)

	// Manual1 已 seed
	body := []byte(`{"short_name":"Manual1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("应 409, got %d", w.Code)
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			PackID    uint64 `json:"pack_id"`
			ShortName string `json:"short_name"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 4009 {
		t.Errorf("code 应为 4009, got %d", resp.Code)
	}
	if resp.Data.PackID == 0 {
		t.Error("contract 要求 409 仍返 pack_id")
	}
}

func TestE2E_CreatePack_RejectInvalidShortName(t *testing.T) {
	r, _, _ := setupPackEngine(t)
	for _, bad := range []string{
		`{"short_name":"../../etc/passwd"}`,
		`{"short_name":"1Bad"}`,
		`{"short_name":""}`,
		`{"short_name":"a/b"}`,
		`{"short_name":"a b"}`,
		`{}`, // 缺字段
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/packs", bytes.NewReader([]byte(bad)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("非法 body %q 应 400, got %d body=%s", bad, w.Code, w.Body.String())
		}
	}
}

func TestE2E_GetEmoji_NotFound(t *testing.T) {
	r, _, _ := setupPackEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/emoji/9999999999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("应 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestE2E_GetEmoji_Hit(t *testing.T) {
	r, db, _ := setupPackEngine(t)
	// seed 一个 emoji
	pack := &entity.StickerPack{ShortName: "P", Title: "P", Source: "featured"}
	db.Create(pack)
	db.Create(&entity.EmojiSticker{
		CustomEmojiID: "5404674261681232568",
		Alt:           "🔥",
		Keywords:      "fire,flame",
		PackID:        pack.ID,
		IsAnimated:    true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/emoji/5404674261681232568", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data["custom_emoji_id"] != "5404674261681232568" {
		t.Errorf("custom_emoji_id 不对: %v", resp.Data["custom_emoji_id"])
	}
	fmts, ok := resp.Data["formats"].(map[string]any)
	if !ok {
		t.Fatalf("formats 不是 map")
	}
	if fmts["html"] != `<tg-emoji emoji-id="5404674261681232568">🔥</tg-emoji>` {
		t.Errorf("html 格式不对: %v", fmts["html"])
	}
}
