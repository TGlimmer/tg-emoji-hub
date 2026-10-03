import { useEffect, useState } from "react";
import { getHotPacks } from "@/api/client";
import type { EmojiPreviewBrief, StickerPackWithPreview } from "@/api/types";
import { useT } from "@/i18n/LanguageProvider";
import EmojiMedia from "./EmojiMedia";
import { Skeleton } from "./ui/skeleton";

const HOT_LIMIT = 12;

interface HotPacksProps {
  onPackSelect: (shortName: string) => void;
}

export default function HotPacks({ onPackSelect }: HotPacksProps) {
  const [packs, setPacks] = useState<StickerPackWithPreview[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const t = useT();

  useEffect(() => {
    const ctrl = new AbortController();
    let cancelled = false;
    getHotPacks(HOT_LIMIT, ctrl.signal)
      .then((res) => {
        if (!cancelled) setPacks(res.items);
      })
      .catch(() => {
        // 优雅降级：API 失败整块隐藏
        if (!cancelled) setFailed(true);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
      ctrl.abort();
    };
  }, []);

  // 没数据也没在加载（接口挂了或返回空）静默隐藏
  if (failed) return null;
  if (!loading && packs.length === 0) return null;

  return (
    <div>
      <div className="flex items-center justify-between">
        <h2 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
          {t("hot_packs")}
        </h2>
      </div>
      <div className="mt-3 divide-y divide-border/40 border-y border-border/40">
        {loading
          ? Array.from({ length: 8 }).map((_, i) => (
              <div
                key={i}
                className="grid grid-cols-[2.5rem_1fr_3rem] items-center gap-3 px-2 py-3"
                aria-hidden
              >
                <Skeleton className="h-5 w-5 rounded" />
                <div className="flex flex-col gap-1.5">
                  <Skeleton className="h-4 w-28 rounded-sm" />
                  <Skeleton className="h-3 w-20 rounded-sm" />
                </div>
                <Skeleton className="ml-auto h-4 w-8 rounded-sm" />
              </div>
            ))
          : packs.map((pack, i) => (
              <PackRow
                key={pack.id}
                index={i}
                pack={pack}
                onSelect={() => onPackSelect(pack.short_name)}
              />
            ))}
      </div>
    </div>
  );
}

interface PackRowProps {
  index: number;
  pack: StickerPackWithPreview;
  onSelect: () => void;
}

function PackRow({ index, pack, onSelect }: PackRowProps) {
  const previews = (pack.preview_emojis ?? []).slice(0, 4);
  return (
    <button
      type="button"
      onClick={onSelect}
      className="grid w-full grid-cols-[2.5rem_1fr_3rem] items-center gap-3 px-2 py-3 text-left transition-colors hover:bg-accent/30 focus-visible:outline-hidden focus-visible:bg-accent/30 sm:grid-cols-[2.5rem_1fr_3rem_auto]"
    >
      <span className="font-mono text-xs text-muted-foreground tabular-nums">
        {String(index + 1).padStart(2, "0")}
      </span>
      <span className="min-w-0">
        <span className="block truncate text-sm font-medium text-foreground">
          {pack.title || pack.short_name}
        </span>
        <span className="block truncate font-mono text-xs text-muted-foreground">
          {pack.short_name}
        </span>
      </span>
      <span className="text-right font-mono text-sm tabular-nums">
        {pack.sticker_count}
      </span>
      <span className="hidden items-center gap-1.5 sm:flex">
        {previews.map((e) => (
          <PreviewThumb key={e.custom_emoji_id} emoji={e} />
        ))}
        {/* 不足 4 个用占位防止行高抖动 */}
        {Array.from({ length: Math.max(0, 4 - previews.length) }).map(
          (_, i) => (
            <span
              key={`ph-${i}`}
              className="h-8 w-8 rounded-md bg-muted/20"
              aria-hidden
            />
          ),
        )}
      </span>
    </button>
  );
}

function PreviewThumb({ emoji }: { emoji: EmojiPreviewBrief }) {
  // 32x8 个动态 lottie 会卡死，所以强制只用 thumbnail_url + image/webp 走静态分支
  return (
    <span className="flex h-8 w-8 items-center justify-center rounded-md bg-muted/30">
      <EmojiMedia
        url={emoji.thumbnail_url}
        mime="image/webp"
        thumbnailUrl={emoji.thumbnail_url}
        alt={emoji.alt}
        size={28}
      />
    </span>
  );
}
