ALTER TABLE emoji_stickers ADD COLUMN category VARCHAR(20) NOT NULL DEFAULT '' AFTER keywords;
ALTER TABLE emoji_stickers ADD KEY idx_category (category);
