/**
 * 字段定义以 docs/api.md 为准
 * 所有字段保持 snake_case，禁止 axios 自动驼峰转换
 */

export interface ApiResponse<T> {
  code: number;
  data: T;
  message: string;
}

export interface PageResult<T> {
  items: T[];
  total: number;
  page?: number;
  page_size?: number;
}

export interface EmojiPackBrief {
  short_name: string;
  title: string;
}

export interface EmojiFormatJSON {
  id: string;
  alt: string;
  pack: string;
}

export interface EmojiFormats {
  id: string;
  html: string;
  go_map: string;
  json: EmojiFormatJSON;
}

// 后端正在补 mime_type + media_url，老接口只有 thumbnail_url
// 这两个字段标记为可选，渲染层做降级（无 mime 时按静态 webp 走）
export type EmojiMimeType =
  | "image/webp"
  | "video/webm"
  | "application/x-tgsticker"
  | (string & {});

export interface EmojiSticker {
  custom_emoji_id: string;
  alt: string;
  keywords: string;
  pack: EmojiPackBrief;
  thumbnail_url: string;
  media_url?: string;
  mime_type?: EmojiMimeType;
  is_free: boolean;
  is_animated: boolean;
  formats: EmojiFormats;
  // rollout 期后端老数据可能没有，标可选
  category?: string;
}

export type PackSource = "featured" | "manual" | "all";

export interface StickerPack {
  id: number;
  short_name: string;
  title: string;
  is_featured: boolean;
  sticker_count: number;
  source: "featured" | "manual";
  last_crawled_at: string | null;
  created_at: string;
}

// 11 个固定分类，与后端枚举完全一致
export const EMOJI_CATEGORIES = [
  "emotion",
  "animal",
  "person",
  "food",
  "crypto",
  "object",
  "symbol",
  "gesture",
  "meme",
  "vehicle",
  "nature",
] as const;

export type EmojiCategory = (typeof EMOJI_CATEGORIES)[number];

export interface SearchParams {
  q?: string;
  limit?: number;
  pack?: string;
  animated_only?: boolean;
  category?: string;
}

export interface ListPacksParams {
  source?: PackSource;
  sort?: "newest" | "hot" | "name";
  with_preview?: boolean;
  page?: number;
  page_size?: number;
}

export interface StatsOverview {
  total_emojis: number;
  total_packs: number;
  animated_count: number;
  animated_percentage: number;
  added_today: number;
  updated_at: string;
}

// 列表预览用的瘦身版 emoji —— 后端只回 thumbnail_url + alt，强制走静态分支防卡顿
export interface EmojiPreviewBrief {
  custom_emoji_id: string;
  alt: string;
  thumbnail_url: string;
  media_url?: string;
  mime_type?: EmojiMimeType;
  is_animated: boolean;
}

export interface StickerPackWithPreview extends StickerPack {
  preview_emojis?: EmojiPreviewBrief[];
}

export interface RandomEmojiParams {
  limit?: number;
  animated_only?: boolean;
}

export interface AddPackRequest {
  short_name: string;
}

export interface AddPackResponse {
  pack_id: number;
  short_name: string;
  status: string;
  message: string;
}

export interface HealthStatus {
  status: string;
  db: string;
  version: string;
  uptime_seconds?: number;
}
