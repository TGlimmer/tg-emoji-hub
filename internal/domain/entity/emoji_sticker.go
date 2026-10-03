package entity

import "time"

// EmojiSticker 单个 custom emoji
//
// MimeType 用 varchar 不用 ENUM：sqlite AutoMigrate 不识别 ENUM 语法（双轨教训）；
// 生产 schema 在 migrations/002_add_mime_type.up.sql 仍可用 ENUM 真约束。
// 合法值: image/webp / video/webm / application/x-tgsticker。
type EmojiSticker struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	CustomEmojiID string    `gorm:"column:custom_emoji_id;type:varchar(32);uniqueIndex;not null"`
	Alt           string    `gorm:"column:alt;type:varchar(16);index;not null"`
	Keywords      string    `gorm:"column:keywords;type:text;not null"`
	Category      string    `gorm:"column:category;type:varchar(20);index;not null;default:''"`
	PackID        uint64    `gorm:"column:pack_id;not null;index"`
	ThumbnailPath string    `gorm:"column:thumbnail_path;type:varchar(255);not null;default:''"`
	MimeType      string    `gorm:"column:mime_type;type:varchar(64);not null;default:'image/webp'"`
	IsFree        bool      `gorm:"column:is_free;not null;default:0"`
	IsAnimated    bool      `gorm:"column:is_animated;not null;default:0"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`

	// Pack 关联包，搜索 SQL 用 JOIN 后通过 Preload-like 手动注入
	Pack *StickerPack `gorm:"foreignKey:PackID;references:ID"`
}

func (EmojiSticker) TableName() string {
	return "emoji_stickers"
}

// MimeType 合法值
const (
	MimeWebp = "image/webp"
	MimeWebm = "video/webm"
	MimeTGS  = "application/x-tgsticker"
)

// Category 11 值白名单，空串 ” 是未打标哨兵
const (
	CategoryEmotion = "emotion"
	CategoryAnimal  = "animal"
	CategoryPerson  = "person"
	CategoryFood    = "food"
	CategoryCrypto  = "crypto"
	CategoryObject  = "object"
	CategorySymbol  = "symbol"
	CategoryGesture = "gesture"
	CategoryMeme    = "meme"
	CategoryVehicle = "vehicle"
	CategoryNature  = "nature"
)

var validCategories = map[string]struct{}{
	CategoryEmotion: {},
	CategoryAnimal:  {},
	CategoryPerson:  {},
	CategoryFood:    {},
	CategoryCrypto:  {},
	CategoryObject:  {},
	CategorySymbol:  {},
	CategoryGesture: {},
	CategoryMeme:    {},
	CategoryVehicle: {},
	CategoryNature:  {},
}

// NormalizeCategory 写入前兜底；非 11 值或空则回退 ”（未打标哨兵）
func NormalizeCategory(s string) string {
	if _, ok := validCategories[s]; ok {
		return s
	}
	return ""
}

// IsValidCategory 显式校验
func IsValidCategory(s string) bool {
	_, ok := validCategories[s]
	return ok
}

// NormalizeMimeType 写入前兜底；空或非法值回退到 image/webp（最常见的静态情况）
func NormalizeMimeType(s string) string {
	switch s {
	case MimeWebp, MimeWebm, MimeTGS:
		return s
	default:
		return MimeWebp
	}
}

// IsValidMimeType 显式校验
func IsValidMimeType(s string) bool {
	return s == MimeWebp || s == MimeWebm || s == MimeTGS
}

// MediaExtFromMime 根据 mime 推出原文件扩展名（用于拼 /emoji/{id}.{ext} URL）
func MediaExtFromMime(mime string) string {
	switch mime {
	case MimeWebm:
		return "webm"
	case MimeTGS:
		return "tgs"
	default:
		return "webp"
	}
}
