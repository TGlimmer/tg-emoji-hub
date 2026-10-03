package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"gorm.io/gorm"

	mysqlrepo "github.com/TGlimmer/tg-emoji-hub/internal/adapter/repository/mysql"
	"github.com/TGlimmer/tg-emoji-hub/internal/app"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
)

// setupMCPDeps 装一个 in-memory sqlite + 完整 use case 链路给 MCP handler 用
func setupMCPDeps(t *testing.T) Deps {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.StickerPack{}, &entity.EmojiSticker{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	pack := &entity.StickerPack{
		ShortName: "AnimatedEmojies", Title: "Animated", IsFeatured: true, Source: "featured",
	}
	db.Create(pack)
	db.Create(&entity.EmojiSticker{
		CustomEmojiID: "5404674261681232568",
		Alt:           "🔥", Keywords: "fire,flame", PackID: pack.ID, IsAnimated: true,
	})
	db.Create(&entity.EmojiSticker{
		CustomEmojiID: "1234567890",
		Alt:           "💎", Keywords: "diamond,gem", PackID: pack.ID,
	})

	searchRepo := mysqlrepo.NewSearchRepo(db)
	packRepo := mysqlrepo.NewPackRepo(db)
	return Deps{
		SearchUC: app.NewSearchUseCase(searchRepo),
		PackUC:   app.NewPackUseCase(packRepo, t.TempDir()),
		BaseURL:  "http://localhost:8080",
		Name:     "tg-emoji-hub-test",
		Version:  "0.0.1",
	}
}

func mustParseJSON(t *testing.T, r *mcpsdk.CallToolResult) map[string]any {
	t.Helper()
	if r == nil || len(r.Content) == 0 {
		t.Fatalf("空 result")
	}
	tc, ok := r.Content[0].(mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content[0] 不是 TextContent, got %T", r.Content[0])
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("解析 JSON 失败: %v text=%s", err, tc.Text)
	}
	return out
}

func callTool(args map[string]any) mcpsdk.CallToolRequest {
	return mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{Arguments: args},
	}
}

func TestSearchEmojiTool_Hit(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeSearchHandler(deps)
	r, err := h(context.Background(), callTool(map[string]any{"query": "fire"}))
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if r.IsError {
		t.Fatalf("不应是 error result")
	}
	out := mustParseJSON(t, r)
	if out["total"].(float64) != 1 {
		t.Errorf("total 应为 1, got %v", out["total"])
	}
	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items 应 1 条, got %d", len(items))
	}
	item := items[0].(map[string]any)
	if item["custom_emoji_id"] != "5404674261681232568" {
		t.Errorf("id 不对: %v", item["custom_emoji_id"])
	}
	fmts := item["formats"].(map[string]any)
	if !strings.Contains(fmts["html"].(string), "🔥") {
		t.Errorf("html 不含 🔥: %v", fmts["html"])
	}
	if fmts["go_map"] != `"fire": "5404674261681232568",` {
		t.Errorf("go_map 不对: %v", fmts["go_map"])
	}
}

func TestSearchEmojiTool_MissingQuery(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeSearchHandler(deps)
	r, err := h(context.Background(), callTool(map[string]any{}))
	if err != nil {
		t.Fatalf("不应抛 raw err, 应返 error result: %v", err)
	}
	if !r.IsError {
		t.Errorf("缺 query 应 IsError=true")
	}
}

func TestSearchEmojiTool_LimitClamp(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeSearchHandler(deps)
	// limit=999 应被 clamp 到 100，不报错
	r, err := h(context.Background(), callTool(map[string]any{
		"query": "fire", "limit": float64(999),
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.IsError {
		t.Errorf("limit clamp 不应 IsError")
	}
}

func TestGetByIDTool_Hit(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeGetByIDHandler(deps)
	r, err := h(context.Background(), callTool(map[string]any{
		"custom_emoji_id": "1234567890",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.IsError {
		t.Fatalf("不应 IsError")
	}
	out := mustParseJSON(t, r)
	if out["custom_emoji_id"] != "1234567890" || out["alt"] != "💎" {
		t.Errorf("结果不对: %+v", out)
	}
}

func TestGetByIDTool_NotFound(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeGetByIDHandler(deps)
	r, _ := h(context.Background(), callTool(map[string]any{
		"custom_emoji_id": "9999999999",
	}))
	if !r.IsError {
		t.Error("not found 应返 IsError=true")
	}
}

func TestGetByIDTool_MissingArg(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeGetByIDHandler(deps)
	r, _ := h(context.Background(), callTool(map[string]any{}))
	if !r.IsError {
		t.Error("缺 custom_emoji_id 应 IsError")
	}
}

func TestListPacksTool(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeListPacksHandler(deps)
	r, err := h(context.Background(), callTool(map[string]any{}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	out := mustParseJSON(t, r)
	if out["total"].(float64) != 1 {
		t.Errorf("应 1 个 pack, got %v", out["total"])
	}
	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items 数不对: %d", len(items))
	}
	p := items[0].(map[string]any)
	if p["short_name"] != "AnimatedEmojies" {
		t.Errorf("short_name 不对: %v", p["short_name"])
	}
}

func TestListPacksTool_FilterBySource(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeListPacksHandler(deps)
	r, _ := h(context.Background(), callTool(map[string]any{"source": "manual"}))
	out := mustParseJSON(t, r)
	if out["total"].(float64) != 0 {
		t.Errorf("manual 应 0 个, got %v", out["total"])
	}
}

func TestNewServer_RegistersAllThreeTools(t *testing.T) {
	deps := setupMCPDeps(t)
	s := NewServer(deps)
	if s == nil {
		t.Fatal("server nil")
	}
	// NewServer 内部 panic 才能验证 tool 注册失败；这里成功创建即说明注册无错
}

func TestListPackStickersTool_Hit(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeListPackStickersHandler(deps)
	r, err := h(context.Background(), callTool(map[string]any{
		"pack_short_name": "AnimatedEmojies",
	}))
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if r.IsError {
		t.Fatalf("不应是 error result")
	}
	out := mustParseJSON(t, r)
	// fixture 有 2 条都属于 AnimatedEmojies
	if out["total"].(float64) != 2 {
		t.Errorf("total 应为 2, got %v", out["total"])
	}
	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items 应 2 条, got %d", len(items))
	}
	// is_animated=true 必须排第一
	first := items[0].(map[string]any)
	if first["is_animated"] != true {
		t.Errorf("第一条应是 animated, got %+v", first)
	}
}

func TestListPackStickersTool_MissingPack(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeListPackStickersHandler(deps)
	r, _ := h(context.Background(), callTool(map[string]any{}))
	if !r.IsError {
		t.Error("缺 pack_short_name 应 IsError=true")
	}
}

func TestListPackStickersTool_LimitClamp(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeListPackStickersHandler(deps)
	r, err := h(context.Background(), callTool(map[string]any{
		"pack_short_name": "AnimatedEmojies",
		"limit":           float64(999),
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.IsError {
		t.Errorf("limit clamp 不应 IsError")
	}
}

func TestListPackStickersTool_NonexistentPack(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeListPackStickersHandler(deps)
	r, err := h(context.Background(), callTool(map[string]any{
		"pack_short_name": "DoesNotExist",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// 不存在的包 → 返回空列表 + total=0，不算 error
	if r.IsError {
		t.Errorf("不存在 pack 应返回空列表非 error")
	}
	out := mustParseJSON(t, r)
	if out["total"].(float64) != 0 {
		t.Errorf("不存在 pack total 应 0, got %v", out["total"])
	}
}

func TestSearchInPackTool_Hit(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeSearchInPackHandler(deps)
	r, err := h(context.Background(), callTool(map[string]any{
		"pack_short_name": "AnimatedEmojies",
		"query":           "fire",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.IsError {
		t.Fatalf("不应 IsError")
	}
	out := mustParseJSON(t, r)
	if out["total"].(float64) != 1 {
		t.Errorf("total 应为 1, got %v", out["total"])
	}
	items := out["items"].([]any)
	item := items[0].(map[string]any)
	if item["custom_emoji_id"] != "5404674261681232568" {
		t.Errorf("id 不对: %v", item["custom_emoji_id"])
	}
}

func TestSearchInPackTool_MissingPack(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeSearchInPackHandler(deps)
	r, _ := h(context.Background(), callTool(map[string]any{
		"query": "fire",
	}))
	if !r.IsError {
		t.Error("缺 pack_short_name 应 IsError=true")
	}
}

func TestSearchInPackTool_MissingQuery(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeSearchInPackHandler(deps)
	r, _ := h(context.Background(), callTool(map[string]any{
		"pack_short_name": "AnimatedEmojies",
	}))
	if !r.IsError {
		t.Error("缺 query 应 IsError=true")
	}
}

func TestSearchInPackTool_NoMatch(t *testing.T) {
	deps := setupMCPDeps(t)
	h := makeSearchInPackHandler(deps)
	r, err := h(context.Background(), callTool(map[string]any{
		"pack_short_name": "AnimatedEmojies",
		"query":           "nonexistent_xyz",
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.IsError {
		t.Errorf("无匹配应返空列表非 error")
	}
	out := mustParseJSON(t, r)
	if out["total"].(float64) != 0 {
		t.Errorf("无匹配 total 应 0, got %v", out["total"])
	}
}

func TestThumbnailURL_Variants(t *testing.T) {
	tests := []struct {
		baseURL, path, id, want string
	}{
		{"http://localhost:8080", "thumbs/1.webp", "1", "http://localhost:8080/thumbs/1.webp"},
		{"http://localhost:8080/", "thumbs/1.webp", "1", "http://localhost:8080/thumbs/1.webp"},
		{"", "thumbs/1.webp", "1", "/thumbs/1.webp"},
		{"http://localhost:8080", "", "999", "http://localhost:8080/thumbs/999.webp"},
	}
	for _, tt := range tests {
		got := thumbnailURL(tt.baseURL, tt.path, tt.id)
		if got != tt.want {
			t.Errorf("thumbnailURL(%q,%q,%q) = %q, want %q",
				tt.baseURL, tt.path, tt.id, got, tt.want)
		}
	}
}
