package bootstrap

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
)

// allModels AutoMigrate 的目标实体清单
// 新增 entity 必须追加到这里，否则 AutoMigrate 不会建表
var allModels = []any{
	&entity.StickerPack{},
	&entity.EmojiSticker{},
}

// criticalColumns AutoMigrate 后 sanity check，防 GORM NamingStrategy 意外
// (table_name → 必须存在的列清单)
var criticalColumns = map[string][]string{
	"sticker_packs":  {"id", "short_name", "title", "is_featured", "sticker_count", "source", "last_crawled_at"},
	"emoji_stickers": {"id", "custom_emoji_id", "alt", "keywords", "pack_id", "thumbnail_path", "mime_type", "is_free", "is_animated"},
}

// MustInitDB 启动期初始化 DB；dev 模式跑 AutoMigrate + sanity check
func MustInitDB(cfg *Config, logger *zap.Logger) *gorm.DB {
	gormLog := gormlogger.New(
		zapWriter{logger: logger},
		gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(mysql.Open(cfg.Database.DSN()), &gorm.Config{
		Logger:                                   gormLog,
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		logger.Fatal("DB 连接失败", zap.Error(err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal("取 sql.DB 失败", zap.Error(err))
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		logger.Fatal("DB ping 失败", zap.Error(err))
	}
	logger.Info("DB 连接成功",
		zap.String("host", cfg.Database.Host),
		zap.Int("port", cfg.Database.Port),
		zap.String("name", cfg.Database.Name),
	)

	if cfg.Server.Mode == "dev" {
		autoMigrateWithSanity(db, logger)
	}

	return db
}

// autoMigrateWithSanity 逐 entity 调用 + 检查关键列存在 + 补 GORM 不会建的索引
func autoMigrateWithSanity(db *gorm.DB, logger *zap.Logger) {
	for _, m := range allModels {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(m); err != nil {
			logger.Error("解析 entity 失败", zap.Error(err))
			continue
		}
		table := stmt.Schema.Table
		if err := db.AutoMigrate(m); err != nil {
			logger.Error("AutoMigrate 失败",
				zap.String("table", table),
				zap.Error(err),
			)
			continue
		}
		logger.Info("AutoMigrate 完成", zap.String("table", table))

		if cols, ok := criticalColumns[table]; ok {
			for _, col := range cols {
				if !db.Migrator().HasColumn(m, col) {
					logger.Warn("关键列缺失",
						zap.String("table", table),
						zap.String("column", col),
					)
				}
			}
		}
	}

	ensureFullTextIndex(db, logger,
		"emoji_stickers", "ft_search",
		"ALTER TABLE emoji_stickers ADD FULLTEXT KEY ft_search (keywords, alt) WITH PARSER ngram",
	)
}

// ensureFullTextIndex 幂等创建 FULLTEXT 索引（GORM 标签表达不出）
// 只在 MySQL 驱动下生效；sqlite 测试库跳过（不支持 FULLTEXT 语法）
func ensureFullTextIndex(db *gorm.DB, logger *zap.Logger, table, indexName, createSQL string) {
	if db.Dialector.Name() != "mysql" {
		return
	}
	var count int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
		table, indexName,
	).Scan(&count).Error; err != nil {
		logger.Warn("查询 FULLTEXT 索引失败", zap.String("index", indexName), zap.Error(err))
		return
	}
	if count > 0 {
		return
	}
	if err := db.Exec(createSQL).Error; err != nil {
		logger.Error("创建 FULLTEXT 索引失败", zap.String("index", indexName), zap.Error(err))
		return
	}
	logger.Info("创建 FULLTEXT 索引完成", zap.String("table", table), zap.String("index", indexName))
}

// MustClose 关 DB
func MustClose(db *gorm.DB, logger *zap.Logger) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Warn("取 sql.DB 失败，跳过关闭", zap.Error(err))
		return
	}
	_ = sqlDB.Close()
}

// zapWriter 把 GORM logger 输出桥接到 zap
type zapWriter struct {
	logger *zap.Logger
}

func (w zapWriter) Printf(format string, args ...any) {
	w.logger.Info(fmt.Sprintf(format, args...))
}
