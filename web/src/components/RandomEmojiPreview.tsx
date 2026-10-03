import { RefreshCw } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { getRandomEmojis } from "@/api/client";
import type { EmojiSticker } from "@/api/types";
import EmojiCard from "./EmojiCard";
import { Skeleton } from "./ui/skeleton";

const RANDOM_LIMIT = 24;

export default function RandomEmojiPreview() {
  const [emojis, setEmojis] = useState<EmojiSticker[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [openIndex, setOpenIndex] = useState(-1);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    try {
      const res = await getRandomEmojis({ limit: RANDOM_LIMIT }, signal);
      setEmojis(res.items);
      setFailed(false);
    } catch {
      // 首次失败整块隐藏；refresh 失败保留旧数据
      setEmojis((prev) => {
        if (prev.length === 0) setFailed(true);
        return prev;
      });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const ctrl = new AbortController();
    void refresh(ctrl.signal);
    return () => ctrl.abort();
  }, [refresh]);

  if (failed) return null;

  return (
    <div>
      <div className="flex items-center justify-between">
        <h2 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
          🎲 random preview
        </h2>
        <button
          type="button"
          onClick={() => void refresh()}
          disabled={loading}
          className="inline-flex items-center gap-1 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring rounded px-1"
          aria-label="重新获取随机 emoji"
        >
          <RefreshCw
            className={`h-3 w-3 ${loading ? "animate-spin" : ""}`}
          />
          <span>refresh</span>
        </button>
      </div>

      <div
        className="mt-3 grid gap-2"
        style={{
          gridTemplateColumns: "repeat(auto-fill, minmax(82px, 1fr))",
        }}
      >
        {loading && emojis.length === 0
          ? Array.from({ length: RANDOM_LIMIT }).map((_, i) => (
              <div
                key={i}
                className="flex flex-col items-center gap-1.5 rounded-xl border border-border/60 bg-card/60 p-2"
                aria-hidden
              >
                <Skeleton className="aspect-square w-full rounded-lg" />
                <Skeleton className="h-2.5 w-12 rounded-sm" />
              </div>
            ))
          : emojis.map((emoji, i) => (
              <EmojiCard
                key={emoji.custom_emoji_id}
                emoji={emoji}
                open={openIndex === i}
                onOpenChange={(o) => setOpenIndex(o ? i : -1)}
              />
            ))}
      </div>
    </div>
  );
}
