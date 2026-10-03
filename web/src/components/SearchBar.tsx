import { Search, X } from "lucide-react";
import type { KeyboardEvent } from "react";
import { useEffect, useRef } from "react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { useT } from "@/i18n/LanguageProvider";

interface SearchBarProps {
  value: string;
  onChange: (v: string) => void;
  onSubmit: () => void;
  // placeholder 若外部传入则优先用外部值，否则走 i18n
  placeholder?: string;
  autoFocus?: boolean;
  className?: string;
}

export default function SearchBar({
  value,
  onChange,
  onSubmit,
  placeholder,
  autoFocus = true,
  className,
}: SearchBarProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const t = useT();

  // 按 "/" 全局聚焦搜索框（开发者快捷键）
  useEffect(() => {
    function onKey(e: globalThis.KeyboardEvent) {
      if (e.key !== "/" || e.metaKey || e.ctrlKey || e.altKey) return;
      const target = e.target as HTMLElement | null;
      if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable)) return;
      e.preventDefault();
      inputRef.current?.focus();
      inputRef.current?.select();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  function handleKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter") {
      e.preventDefault();
      onSubmit();
    } else if (e.key === "Escape" && value) {
      e.preventDefault();
      onChange("");
    }
  }

  return (
    <div className={cn("relative w-full max-w-2xl mx-auto", className)}>
      <Search
        className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
        aria-hidden
      />
      <Input
        ref={inputRef}
        autoFocus={autoFocus}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKey}
        placeholder={placeholder ?? t("search_placeholder")}
        spellCheck={false}
        autoCorrect="off"
        autoCapitalize="none"
        className="h-12 pl-10 pr-20 text-base font-mono tracking-tight bg-card border-border/80 shadow-lg shadow-black/20 focus-visible:ring-primary/60"
        aria-label={t("search_placeholder")}
      />
      {value && (
        <button
          type="button"
          onClick={() => onChange("")}
          className="absolute right-12 top-1/2 -translate-y-1/2 rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
          aria-label={t("search_clear")}
        >
          <X className="h-4 w-4" />
        </button>
      )}
      <kbd className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 hidden sm:inline-flex h-6 select-none items-center rounded border border-border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground">
        /
      </kbd>
    </div>
  );
}
