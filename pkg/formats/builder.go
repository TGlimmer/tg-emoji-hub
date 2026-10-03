package formats

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
)

// FormatSet 4 种输出格式
type FormatSet struct {
	ID    string         `json:"id"`
	HTML  string         `json:"html"`
	GoMap string         `json:"go_map"`
	JSON  map[string]any `json:"json"`
}

// 仅允许字母数字下划线
var slugCleaner = regexp.MustCompile(`[^a-z0-9_]+`)

// Build 根据 EmojiSticker 拼装 4 种格式
// HTML 字段用 html.EscapeString 兜底 emoji_id 字符（虽然实际是纯数字，预防性处理）
func Build(e *entity.EmojiSticker) FormatSet {
	if e == nil {
		return FormatSet{}
	}

	escapedID := html.EscapeString(e.CustomEmojiID)
	escapedAlt := html.EscapeString(e.Alt)

	htmlTag := fmt.Sprintf(`<tg-emoji emoji-id="%s">%s</tg-emoji>`, escapedID, escapedAlt)

	key := slugify(e.Keywords, e.Alt)
	goMap := fmt.Sprintf(`%q: %q,`, key, e.CustomEmojiID)

	packShortName := ""
	if e.Pack != nil {
		packShortName = e.Pack.ShortName
	}

	return FormatSet{
		ID:    e.CustomEmojiID,
		HTML:  htmlTag,
		GoMap: goMap,
		JSON: map[string]any{
			"id":   e.CustomEmojiID,
			"alt":  e.Alt,
			"pack": packShortName,
		},
	}
}

// slugify 取第一个英文关键词作 map key；keywords 为空或全是非 ASCII 时用 alt 兜底
func slugify(keywords, alt string) string {
	if keywords != "" {
		for _, raw := range strings.Split(keywords, ",") {
			cand := strings.TrimSpace(strings.ToLower(raw))
			cand = slugCleaner.ReplaceAllString(cand, "_")
			cand = strings.Trim(cand, "_")
			if cand != "" {
				return cand
			}
		}
	}
	if alt != "" {
		return alt
	}
	return "emoji"
}
