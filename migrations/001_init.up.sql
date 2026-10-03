-- tg-emoji-hub 初始化迁移
-- 创建 sticker_packs 和 emoji_stickers 两张表

CREATE TABLE IF NOT EXISTS sticker_packs (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    short_name      VARCHAR(64)  NOT NULL,
    title           VARCHAR(255) NOT NULL,
    tg_pack_id      VARCHAR(32)  NOT NULL DEFAULT '',
    is_featured     TINYINT(1)   NOT NULL DEFAULT 0,
    sticker_count   INT          NOT NULL DEFAULT 0,
    source          ENUM('featured','manual') NOT NULL DEFAULT 'featured',
    last_crawled_at TIMESTAMP NULL DEFAULT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_short_name (short_name),
    KEY idx_tg_pack_id (tg_pack_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS emoji_stickers (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    custom_emoji_id VARCHAR(32)  NOT NULL,
    alt             VARCHAR(16)  NOT NULL,
    keywords        TEXT         NOT NULL,
    pack_id         BIGINT UNSIGNED NOT NULL,
    thumbnail_path  VARCHAR(255) NOT NULL DEFAULT '',
    is_free         TINYINT(1)   NOT NULL DEFAULT 0,
    is_animated     TINYINT(1)   NOT NULL DEFAULT 0,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_custom_emoji_id (custom_emoji_id),
    KEY idx_alt (alt),
    KEY idx_pack_alt (pack_id, alt),
    FULLTEXT KEY ft_search (keywords, alt),
    CONSTRAINT fk_emoji_pack FOREIGN KEY (pack_id) REFERENCES sticker_packs(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
