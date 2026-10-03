import { Check, ChevronDown, Filter } from "lucide-react";
import { useMemo, useState } from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Input } from "@/components/ui/input";
import type { StickerPack } from "@/api/types";
import { cn } from "@/lib/utils";

interface PackFilterProps {
  packs: StickerPack[];
  value: string;
  onChange: (shortName: string) => void;
  loading?: boolean;
}

export default function PackFilter({ packs, value, onChange, loading }: PackFilterProps) {
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState("");

  const filtered = useMemo(() => {
    const k = keyword.trim().toLowerCase();
    if (!k) return packs;
    return packs.filter(
      (p) =>
        p.short_name.toLowerCase().includes(k) ||
        p.title.toLowerCase().includes(k),
    );
  }, [packs, keyword]);

  const current = value ? packs.find((p) => p.short_name === value) : null;
  const label = current ? current.title || current.short_name : "全部表情包";

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className={cn(
            "inline-flex items-center gap-2 h-9 px-3 rounded-md border border-input bg-card text-sm",
            "hover:bg-accent transition-colors",
            "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
          )}
          aria-haspopup="listbox"
        >
          <Filter className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="max-w-[180px] truncate">{label}</span>
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-72 p-2">
        <div className="px-1 pb-2">
          <Input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder={loading ? "加载中…" : "筛选表情包"}
            className="h-8 text-sm"
          />
        </div>
        <div className="max-h-64 overflow-y-auto py-1">
          <button
            type="button"
            onClick={() => {
              onChange("");
              setOpen(false);
            }}
            className={cn(
              "flex w-full items-center justify-between rounded-md px-2.5 py-1.5 text-sm",
              "hover:bg-accent transition-colors",
              value === "" && "text-primary",
            )}
          >
            <span>全部表情包</span>
            {value === "" && <Check className="h-3.5 w-3.5" />}
          </button>
          {filtered.length === 0 && !loading && (
            <div className="px-2.5 py-2 text-xs text-muted-foreground">
              无匹配
            </div>
          )}
          {filtered.map((p) => (
            <button
              key={p.id}
              type="button"
              onClick={() => {
                onChange(p.short_name);
                setOpen(false);
              }}
              className={cn(
                "flex w-full items-center justify-between rounded-md px-2.5 py-1.5 text-sm",
                "hover:bg-accent transition-colors",
                value === p.short_name && "text-primary",
              )}
            >
              <span className="flex-1 truncate text-left">
                <span className="font-medium">{p.title || p.short_name}</span>
                <span className="ml-1 text-xs text-muted-foreground font-mono">
                  · {p.sticker_count}
                </span>
              </span>
              {value === p.short_name && <Check className="h-3.5 w-3.5 shrink-0" />}
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}
