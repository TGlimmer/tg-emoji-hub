package formats

import (
	"testing"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
)

func TestBuild_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		emoji       *entity.EmojiSticker
		wantID      string
		wantHTML    string
		wantGoMap   string
		wantPack    string
		wantJSONAlt string
	}{
		{
			name: "标准 emoji 含英文关键词",
			emoji: &entity.EmojiSticker{
				CustomEmojiID: "5404674261681232568",
				Alt:           "🔥",
				Keywords:      "fire,flame",
				Pack:          &entity.StickerPack{ShortName: "AnimatedEmojies"},
			},
			wantID:      "5404674261681232568",
			wantHTML:    `<tg-emoji emoji-id="5404674261681232568">🔥</tg-emoji>`,
			wantGoMap:   `"fire": "5404674261681232568",`,
			wantPack:    "AnimatedEmojies",
			wantJSONAlt: "🔥",
		},
		{
			name: "关键词为空时用 alt 兜底",
			emoji: &entity.EmojiSticker{
				CustomEmojiID: "1234567890",
				Alt:           "💎",
				Keywords:      "",
				Pack:          nil,
			},
			wantID:      "1234567890",
			wantHTML:    `<tg-emoji emoji-id="1234567890">💎</tg-emoji>`,
			wantGoMap:   `"💎": "1234567890",`,
			wantPack:    "",
			wantJSONAlt: "💎",
		},
		{
			name: "关键词含特殊字符空格大写时清理为下划线小写",
			emoji: &entity.EmojiSticker{
				CustomEmojiID: "999",
				Alt:           "✨",
				Keywords:      "Sparkle Star,magic-stars",
				Pack:          &entity.StickerPack{ShortName: "TestPack"},
			},
			wantID:      "999",
			wantHTML:    `<tg-emoji emoji-id="999">✨</tg-emoji>`,
			wantGoMap:   `"sparkle_star": "999",`,
			wantPack:    "TestPack",
			wantJSONAlt: "✨",
		},
		{
			name: "首关键词全非 ASCII 时跳到第二个英文",
			emoji: &entity.EmojiSticker{
				CustomEmojiID: "42",
				Alt:           "🎨",
				Keywords:      "画画,painting,art",
				Pack:          nil,
			},
			wantID:      "42",
			wantHTML:    `<tg-emoji emoji-id="42">🎨</tg-emoji>`,
			wantGoMap:   `"painting": "42",`,
			wantPack:    "",
			wantJSONAlt: "🎨",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Build(tt.emoji)
			if got.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tt.wantID)
			}
			if got.HTML != tt.wantHTML {
				t.Errorf("HTML = %q, want %q", got.HTML, tt.wantHTML)
			}
			if got.GoMap != tt.wantGoMap {
				t.Errorf("GoMap = %q, want %q", got.GoMap, tt.wantGoMap)
			}
			if got.JSON["alt"] != tt.wantJSONAlt {
				t.Errorf("JSON.alt = %v, want %v", got.JSON["alt"], tt.wantJSONAlt)
			}
			if got.JSON["pack"] != tt.wantPack {
				t.Errorf("JSON.pack = %v, want %v", got.JSON["pack"], tt.wantPack)
			}
			if got.JSON["id"] != tt.wantID {
				t.Errorf("JSON.id = %v, want %v", got.JSON["id"], tt.wantID)
			}
		})
	}
}

func TestBuild_Nil(t *testing.T) {
	got := Build(nil)
	if got.ID != "" || got.HTML != "" {
		t.Errorf("Build(nil) 应返回零值，got %+v", got)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		keywords, alt, want string
	}{
		{"fire,flame", "🔥", "fire"},
		{"", "💎", "💎"},
		{"", "", "emoji"},
		{"中文,english", "🎨", "english"},
		{"Hello World", "x", "hello_world"},
	}
	for _, tt := range tests {
		got := slugify(tt.keywords, tt.alt)
		if got != tt.want {
			t.Errorf("slugify(%q, %q) = %q, want %q", tt.keywords, tt.alt, got, tt.want)
		}
	}
}
