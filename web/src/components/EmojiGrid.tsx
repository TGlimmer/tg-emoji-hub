import { Search as SearchIcon, ServerCrash } from "lucide-react";
import type { RefObject } from "react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSearchStore } from "@/store/searchStore";
import { useT } from "@/i18n/LanguageProvider";
import { copyEmojiFormat } from "./CopyMenu";
import EmojiCard from "./EmojiCard";
import { Button } from "./ui/button";
import { Skeleton } from "./ui/skeleton";

interface EmojiGridProps {
  query: string;
  onRetry?: () => void;
}

export default function EmojiGrid({ query, onRetry }: EmojiGridProps) {
  const status = useSearchStore((s) => s.status);
  const results = useSearchStore((s) => s.results);
  const total = useSearchStore((s) => s.total);
  const errorMessage = useSearchStore((s) => s.errorMessage);
  const focusedIndex = useSearchStore((s) => s.focusedIndex);
  const setFocusedIndex = useSearchStore((s) => s.setFocusedIndex);
  const categoryFilter = useSearchStore((s) => s.categoryFilter);
  const packFilter = useSearchStore((s) => s.packFilter);
  const t = useT();

  const containerRef = useRef<HTMLDivElement>(null);
  const cardRefs = useRef<Map<number, HTMLButtonElement>>(new Map());
  const [openIndex, setOpenIndex] = useState<number>(-1);
  const [colsPerRow, setColsPerRow] = useState(8);

  useLayoutEffect(() => {
    if (results.length === 0) return;
    const grid = containerRef.current?.querySelector<HTMLDivElement>(".emoji-grid");
    if (!grid) return;
    const first = grid.firstElementChild as HTMLElement | null;
    if (!first) return;
    const firstTop = first.offsetTop;
    let count = 0;
    for (const child of Array.from(grid.children)) {
      if ((child as HTMLElement).offsetTop !== firstTop) break;
      count += 1;
    }
    setColsPerRow(Math.max(1, count));
  }, [results]);

  const handleKey = useCallback(
    async (e: globalThis.KeyboardEvent) => {
      if (results.length === 0) return;
      const target = e.target as HTMLElement | null;
      const isInputFocus =
        target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA");
      if (isInputFocus && e.key !== "ArrowDown") return;

      if (e.key === "ArrowRight") {
        e.preventDefault();
        const next = focusedIndex < 0 ? 0 : Math.min(results.length - 1, focusedIndex + 1);
        setFocusedIndex(next);
      } else if (e.key === "ArrowLeft") {
        e.preventDefault();
        const next = focusedIndex < 0 ? 0 : Math.max(0, focusedIndex - 1);
        setFocusedIndex(next);
      } else if (e.key === "ArrowDown") {
        e.preventDefault();
        const next =
          focusedIndex < 0
            ? 0
            : Math.min(results.length - 1, focusedIndex + colsPerRow);
        setFocusedIndex(next);
      } else if (e.key === "ArrowUp") {
        if (focusedIndex < colsPerRow && focusedIndex >= 0) return;
        e.preventDefault();
        const next = focusedIndex < 0 ? 0 : Math.max(0, focusedIndex - colsPerRow);
        setFocusedIndex(next);
      } else if (e.key === "Enter" && focusedIndex >= 0) {
        e.preventDefault();
        const emoji = results[focusedIndex];
        if (emoji) await copyEmojiFormat(emoji, "html");
      } else if (e.key === "Escape") {
        setOpenIndex(-1);
        setFocusedIndex(-1);
      }
    },
    [results, focusedIndex, colsPerRow, setFocusedIndex],
  );

  useEffect(() => {
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [handleKey]);

  useEffect(() => {
    if (focusedIndex < 0) return;
    const el = cardRefs.current.get(focusedIndex);
    if (el) {
      el.focus({ preventScroll: false });
      el.scrollIntoView({ block: "nearest", inline: "nearest" });
    }
  }, [focusedIndex]);

  const showHint = useMemo(
    () => results.length > 0 && status === "success",
    [results, status],
  );

  // 根据当前筛选条件生成合适的 empty 标题
  function buildEmptyTitle(): string {
    if (query) return t("grid_empty_query", query);
    if (categoryFilter) return t("grid_empty_category", categoryFilter);
    if (packFilter) return t("grid_empty_pack", packFilter);
    return t("grid_empty_fallback");
  }

  if (status === "idle") {
    return (
      <EmptyHint
        title={t("grid_idle_title")}
        sub={t("grid_idle_sub")}
      />
    );
  }

  if (status === "loading") {
    return (
      <div>
        <div className="mb-3 flex items-center gap-2">
          <Skeleton className="h-3.5 w-24 rounded-sm" />
        </div>
        <div
          className="grid gap-2"
          style={{
            gridTemplateColumns: "repeat(auto-fill, minmax(82px, 1fr))",
          }}
        >
          {Array.from({ length: 12 }).map((_, i) => (
            <div
              key={i}
              className="flex flex-col items-center gap-1.5 rounded-xl border border-border/60 bg-card/60 p-2"
              aria-hidden
            >
              <Skeleton className="aspect-square w-full rounded-lg" />
              <Skeleton className="h-2.5 w-12 rounded-sm" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (status === "error") {
    return (
      <div className="flex flex-col items-center justify-center gap-4 py-20">
        <ServerCrash className="h-10 w-10 text-destructive/80" />
        <div className="text-center">
          <p className="text-sm font-medium">{t("grid_error_title")}</p>
          <p className="mt-1 text-xs text-muted-foreground font-mono max-w-md">
            {errorMessage || t("grid_error_sub")}
          </p>
        </div>
        {onRetry && (
          <Button variant="outline" size="sm" onClick={onRetry}>
            {t("grid_retry")}
          </Button>
        )}
      </div>
    );
  }

  if (status === "empty") {
    return (
      <EmptyHint
        title={buildEmptyTitle()}
        sub={t("grid_empty_sub")}
      />
    );
  }

  return (
    <div ref={containerRef} className="animate-in-soft">
      {showHint && (
        <div className="mb-3 flex items-center justify-between text-xs text-muted-foreground font-mono">
          <span>
            {t("grid_found")}{" "}
            <span className="text-primary font-semibold">{total}</span>{" "}
            {t("grid_found_unit")}
          </span>
          <span className="hidden sm:inline">
            {t("grid_keyboard_hint")}
          </span>
        </div>
      )}
      <div
        className="emoji-grid grid gap-2"
        style={{
          gridTemplateColumns: "repeat(auto-fill, minmax(82px, 1fr))",
        }}
      >
        {results.map((emoji, i) => (
          <EmojiCard
            key={emoji.custom_emoji_id}
            emoji={emoji}
            focused={focusedIndex === i}
            open={openIndex === i}
            onOpenChange={(o) => {
              setOpenIndex(o ? i : -1);
              if (o) setFocusedIndex(i);
            }}
            onFocusRequest={() => setFocusedIndex(i)}
            ref={(el) => registerCardRef(cardRefs, i, el)}
          />
        ))}
      </div>
    </div>
  );

  function registerCardRef(
    map: RefObject<Map<number, HTMLButtonElement>>,
    index: number,
    el: HTMLButtonElement | null,
  ) {
    if (el) map.current.set(index, el);
    else map.current.delete(index);
  }
}

function EmptyHint({ title, sub }: { title: string; sub: string }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-24 text-center">
      <SearchIcon className="h-10 w-10 text-muted-foreground/40" />
      <p className="text-sm font-medium">{title}</p>
      <p className="max-w-md text-xs text-muted-foreground font-mono">{sub}</p>
    </div>
  );
}
