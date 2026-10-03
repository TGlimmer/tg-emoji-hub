-- 加 mime_type 字段：标记 emoji 原文件类型（webp/webm/tgs）。
-- 前端需要它来决定用 <img> / <video> / Lottie player 渲染。
-- 默认 image/webp 兼容老数据；新爬入的数据由 crawler 写真实值。

ALTER TABLE emoji_stickers
    ADD COLUMN mime_type VARCHAR(64) NOT NULL DEFAULT 'image/webp' AFTER thumbnail_path;

-- 老数据回填：is_animated=1 但 mime_type 还是默认 'image/webp' 的，
-- 无法分辨 webm 还是 tgs，统一标 webm（多数动态 emoji 是 webm）；
-- 真实类型由 crawler 重新爬一遍覆盖。
UPDATE emoji_stickers SET mime_type = 'video/webm' WHERE is_animated = 1 AND mime_type = 'image/webp';
