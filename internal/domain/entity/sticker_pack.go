package entity

import "time"

// StickerPack 表情包元信息
//
// Source 字段在 entity 层用 varchar(16)，避免 sqlite/MySQL AutoMigrate 兼容差异
// （sqlite 不识别 ENUM 语法）。生产 schema 在 migrations/001_init.up.sql 仍是 ENUM。
// 写入前必须经 NormalizeSource 兜底防脏值，等同强约束。
type StickerPack struct {
	ID            uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	ShortName     string     `gorm:"column:short_name;type:varchar(64);uniqueIndex;not null"`
	Title         string     `gorm:"column:title;type:varchar(255);not null"`
	TGPackID      string     `gorm:"column:tg_pack_id;type:varchar(32);index"`
	IsFeatured    bool       `gorm:"column:is_featured;not null;default:0"`
	StickerCount  int        `gorm:"column:sticker_count;not null;default:0"`
	Source        string     `gorm:"column:source;type:varchar(16);not null;default:'featured'"`
	LastCrawledAt *time.Time `gorm:"column:last_crawled_at"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 显式表名，避开 GORM NamingStrategy 边角问题
func (StickerPack) TableName() string {
	return "sticker_packs"
}

// Source 合法值
const (
	PackSourceFeatured = "featured"
	PackSourceManual   = "manual"

	// 保持向后兼容（旧名引用）
	SourceFeatured = PackSourceFeatured
	SourceManual   = PackSourceManual
)

// NormalizeSource 写入前兜底，非法值统一回退到 featured
// 业务层调用：pack.Source = entity.NormalizeSource(rawInput)
func NormalizeSource(s string) string {
	switch s {
	case PackSourceFeatured, PackSourceManual:
		return s
	default:
		return PackSourceFeatured
	}
}

// IsValidSource 显式校验（service 层需要拒绝非法值时用）
func IsValidSource(s string) bool {
	return s == PackSourceFeatured || s == PackSourceManual
}
