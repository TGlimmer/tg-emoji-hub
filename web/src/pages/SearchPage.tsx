import { Languages, Send, X } from "lucide-react";
import { useCallback, useState } from "react";
import AddPackDialog from "@/components/AddPackDialog";
import McpDialog from "@/components/McpDialog";
import EmojiGrid from "@/components/EmojiGrid";
import { Button } from "@/components/ui/button";
import HotPacks from "@/components/HotPacks";
import PackFilter from "@/components/PackFilter";
import RandomEmojiPreview from "@/components/RandomEmojiPreview";
import SearchBar from "@/components/SearchBar";
import StatsHero from "@/components/StatsHero";
import UsagePreview from "@/components/UsagePreview";
import { usePacks } from "@/hooks/usePacks";
import { runImmediateSearch, useSearch } from "@/hooks/useSearch";
import { useSearchStore } from "@/store/searchStore";
import { EMOJI_CATEGORIES } from "@/api/types";
import { cn } from "@/lib/utils";
import { useLang, useT } from "@/i18n/LanguageProvider";
import type { StringKey } from "@/i18n/strings";

const SEARCH_LIMIT = 80;

// 分类 key → i18n key 映射
const CATEGORY_I18N_KEY: Record<string, StringKey> = {
  emotion: "category_emotion",
  animal: "category_animal",
  person: "category_person",
  food: "category_food",
  crypto: "category_crypto",
  object: "category_object",
  symbol: "category_symbol",
  gesture: "category_gesture",
  meme: "category_meme",
  vehicle: "category_vehicle",
  nature: "category_nature",
};

export default function SearchPage() {
  const [input, setInput] = useState("");
  const packFilter = useSearchStore((s) => s.packFilter);
  const setPackFilter = useSearchStore((s) => s.setPackFilter);
  const categoryFilter = useSearchStore((s) => s.categoryFilter);
  const setCategoryFilter = useSearchStore((s) => s.setCategoryFilter);
  const animatedOnly = useSearchStore((s) => s.animatedOnly);
  const setAnimatedOnly = useSearchStore((s) => s.setAnimatedOnly);
  const { packs, loading: packsLoading, reload: reloadPacks } = usePacks();
  const { toggleLocale } = useLang();
  const t = useT();

  useSearch({ query: input, limit: SEARCH_LIMIT, debounceMs: 300 });

  // q / pack / category 任一有值 = 搜索态
  const isSearching = input.trim().length > 0 || packFilter !== "" || categoryFilter !== "";

  const handleInputChange = useCallback(
    (v: string) => {
      setInput(v);
      // 输新关键词时只清 pack；category 可以和 q 组合（分类内搜关键词）
      if (v.trim() && packFilter !== "") {
        setPackFilter("");
      }
    },
    [packFilter, setPackFilter],
  );

  const handleSubmit = useCallback(() => {
    void runImmediateSearch(input, SEARCH_LIMIT);
  }, [input]);

  const handleRetry = useCallback(() => {
    void runImmediateSearch(input, SEARCH_LIMIT);
  }, [input]);

  const handlePackSelect = useCallback(
    (shortName: string) => {
      setPackFilter(shortName);
      setInput("");
    },
    [setPackFilter],
  );

  const handleCategoryToggle = useCallback(
    (cat: string) => {
      // 点已选中的 = 取消
      setCategoryFilter(categoryFilter === cat ? "" : cat);
    },
    [categoryFilter, setCategoryFilter],
  );

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-40 border-b border-border/60 bg-background/85 backdrop-blur supports-[backdrop-filter]:bg-background/70">
        <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4 sm:px-6">
          <div className="flex items-center gap-2">
            <span className="font-mono text-base font-semibold tracking-tight">
              tg-emoji-hub
            </span>
            <span className="hidden font-mono text-[10px] text-muted-foreground sm:inline">
              v0.1
            </span>
          </div>
          <div className="flex items-center gap-2">
            {/* 语言切换 */}
            <button
              type="button"
              onClick={toggleLocale}
              className="inline-flex cursor-pointer items-center gap-1.5 rounded-md border border-input bg-transparent px-2.5 py-1.5 text-xs font-mono text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              aria-label="切换语言 / Switch language"
            >
              <Languages className="h-3.5 w-3.5" />
              <span>{t("lang_toggle")}</span>
            </button>
            <Button size="sm" variant="outline" className="gap-1.5" asChild>
              <a
                href="https://t.me/devqun"
                target="_blank"
                rel="noopener noreferrer"
              >
                <Send className="h-4 w-4" />
                <span>{t("header_community")}</span>
              </a>
            </Button>
            <McpDialog />
            <AddPackDialog onSuccess={reloadPacks} />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6 sm:py-10">
        {/* HERO */}
        <section className="mb-10 text-center sm:mb-12">
          <h1 className="font-mono text-4xl font-bold tracking-tight sm:text-5xl">
            tg-emoji-hub
          </h1>
          <div className="mx-auto mt-4 h-px w-8 border-t border-foreground/20" />
          <p className="mt-3 text-sm text-muted-foreground sm:text-base">
            {t("hero_subtitle")}
          </p>
        </section>

        <section className="mb-10">
          <StatsHero />
        </section>

        <section className="mb-5">
          <SearchBar value={input} onChange={handleInputChange} onSubmit={handleSubmit} />
        </section>

        {/* 闲置态：搜索用法提示紧跟搜索框 */}
        {!isSearching && (
          <section className="mb-10">
            <UsagePreview />
          </section>
        )}

        {/* 搜索态：筛选行 —— PackFilter + animated-only + 可移除的分类 chip */}
        {isSearching && (
          <section className="mb-6 flex flex-wrap items-center justify-center gap-2">
            <PackFilter
              packs={packs}
              value={packFilter}
              onChange={setPackFilter}
              loading={packsLoading}
            />
            {/* 分类筛选激活时展示可移除 chip */}
            {categoryFilter && (
              <span className="inline-flex items-center gap-1 rounded-full border border-primary bg-primary/15 px-2.5 py-1 text-xs font-mono text-primary">
                {t("active_category_label")}: {t(CATEGORY_I18N_KEY[categoryFilter] ?? "category_prefix")}
                <button
                  type="button"
                  onClick={() => setCategoryFilter("")}
                  className="ml-0.5 cursor-pointer rounded-full p-0.5 hover:bg-primary/20 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  aria-label={t("active_category_remove")}
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            )}
            <button
              type="button"
              onClick={() => setAnimatedOnly(!animatedOnly)}
              className={`inline-flex h-9 items-center gap-2 rounded-md border px-3 text-sm transition-colors ${
                animatedOnly
                  ? "border-primary bg-primary/15 text-primary"
                  : "border-input bg-card hover:bg-accent"
              }`}
              aria-pressed={animatedOnly}
            >
              <span className="font-mono text-xs">{t("filter_animated_only")}</span>
            </button>
          </section>
        )}

        {isSearching ? (
          <section>
            <EmojiGrid query={input} onRetry={handleRetry} />
          </section>
        ) : (
          <>
            {/* 按分类浏览 —— 标准 Marketplace 结构：搜索框下方、HOT PACKS 上方 */}
            <section className="mb-12">
              <CategorySection
                value={categoryFilter}
                onToggle={handleCategoryToggle}
              />
            </section>
            <section className="mb-12">
              <HotPacks onPackSelect={handlePackSelect} />
            </section>
            <section>
              <RandomEmojiPreview />
            </section>
          </>
        )}
      </main>

      <footer className="mt-10 border-t border-border/40 py-6">
        <div className="mx-auto max-w-6xl px-4 text-center font-mono text-xs text-muted-foreground sm:px-6">
          <span>tg-emoji-hub · MIT</span>
          <span className="mx-2">·</span>
          <kbd className="inline-block min-w-[1.5em] rounded border border-border bg-muted px-1 py-0.5 text-[10px]">
            /
          </kbd>{" "}
          {t("footer_search_hint")}
          <span className="mx-2">·</span>
          <kbd className="inline-block min-w-[1.5em] rounded border border-border bg-muted px-1 py-0.5 text-[10px]">
            ↵
          </kbd>{" "}
          {t("footer_copy_hint")}
        </div>
      </footer>
    </div>
  );
}

interface CategorySectionProps {
  value: string;
  onToggle: (cat: string) => void;
}

// 「按分类浏览」独立区块 —— heading 风格对齐 HotPacks 的 "Hot Packs" 标题
function CategorySection({ value, onToggle }: CategorySectionProps) {
  const t = useT();
  return (
    <div>
      <div className="flex items-center justify-between">
        <h2 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
          {t("browse_by_category")}
        </h2>
      </div>
      <div className="mt-3 flex flex-wrap gap-2">
        {EMOJI_CATEGORIES.map((cat) => {
          const active = value === cat;
          const labelKey = CATEGORY_I18N_KEY[cat];
          return (
            <button
              key={cat}
              type="button"
              onClick={() => onToggle(cat)}
              aria-pressed={active}
              className={cn(
                "inline-flex h-8 cursor-pointer items-center rounded-full border px-3 font-mono text-[12px] transition-colors duration-150",
                active
                  ? "border-primary bg-primary/15 text-primary"
                  : "border-border/60 bg-card/60 text-muted-foreground hover:border-primary/40 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              )}
            >
              {labelKey ? t(labelKey) : cat}
            </button>
          );
        })}
      </div>
    </div>
  );
}
