import { Zap } from "lucide-react";
import { forwardRef, useState } from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import type { EmojiSticker } from "@/api/types";
import CopyMenu from "./CopyMenu";
import EmojiMedia from "./EmojiMedia";
import { cn } from "@/lib/utils";

interface EmojiCardProps {
  emoji: EmojiSticker;
  focused?: boolean;
  /** 外部受控开/关 popover —— 配合键盘导航 */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  onFocusRequest?: () => void;
}

const EmojiCard = forwardRef<HTMLButtonElement, EmojiCardProps>(
  function EmojiCard({ emoji, focused, open, onOpenChange, onFocusRequest }, ref) {
    const [thumbBroken, setThumbBroken] = useState(false);

    return (
      <Popover open={open} onOpenChange={onOpenChange}>
        <PopoverTrigger asChild>
          <button
            ref={ref}
            type="button"
            onFocus={onFocusRequest}
            className={cn(
              "group relative flex flex-col items-center gap-1.5 rounded-xl border border-border/60 bg-card/60 p-2",
              "transition-colors duration-150",
              "hover:border-primary/40 hover:bg-accent/30",
              "focus-visible:outline-hidden focus-visible:border-primary focus-visible:bg-card",
              focused && "border-primary ring-2 ring-primary/40 ring-offset-1 ring-offset-background",
            )}
            aria-label={`复制 ${emoji.alt}（${emoji.pack.short_name}）`}
            title={emoji.pack.short_name}
          >
            <div className="relative flex h-16 w-16 items-center justify-center rounded-lg bg-muted/30">
              {thumbBroken ? (
                <span className="text-3xl leading-none" aria-hidden>
                  {emoji.alt}
                </span>
              ) : (
                <div className="h-full w-full">
                  <EmojiMedia
                    url={emoji.media_url ?? emoji.thumbnail_url}
                    mime={emoji.mime_type ?? "image/webp"}
                    thumbnailUrl={emoji.thumbnail_url}
                    alt={emoji.alt}
                    size={64}
                    onAllFailed={() => setThumbBroken(true)}
                  />
                </div>
              )}
              {emoji.is_animated && (
                <span
                  className="absolute -top-1 -right-1 flex h-4 w-4 items-center justify-center rounded-full bg-primary/95 text-primary-foreground shadow"
                  title="animated"
                >
                  <Zap className="h-2.5 w-2.5" />
                </span>
              )}
            </div>
            <span className="w-full truncate text-center text-[10px] font-mono text-muted-foreground">
              {emoji.pack.short_name}
            </span>
            {emoji.category && (
              <span className="rounded-sm bg-muted/60 px-1 py-px font-mono text-[9px] text-muted-foreground/70">
                {emoji.category}
              </span>
            )}
          </button>
        </PopoverTrigger>
        <PopoverContent align="center" className="w-80 p-1.5">
          <CopyMenu
            emoji={emoji}
            onCopied={() => onOpenChange?.(false)}
          />
        </PopoverContent>
      </Popover>
    );
  },
);

export default EmojiCard;
