import { Code2, Copy, FileJson, FileType, Hash } from "lucide-react";
import { toast } from "sonner";
import type { EmojiSticker } from "@/api/types";
import { copyText } from "@/lib/clipboard";
import { cn } from "@/lib/utils";

interface CopyMenuProps {
  emoji: EmojiSticker;
  /** 默认高亮项；键盘 Enter 时直接复制此项 */
  defaultFormat?: CopyFormat;
  onCopied?: (format: CopyFormat) => void;
  className?: string;
  compact?: boolean;
}

export type CopyFormat = "id" | "html" | "go_map" | "json";

interface Row {
  key: CopyFormat;
  label: string;
  preview: string;
  payload: string;
  Icon: typeof Hash;
}

function buildRows(e: EmojiSticker): Row[] {
  const jsonStr = JSON.stringify(e.formats.json);
  return [
    {
      key: "id",
      label: "ID",
      preview: e.formats.id,
      payload: e.formats.id,
      Icon: Hash,
    },
    {
      key: "html",
      label: "HTML",
      preview: e.formats.html,
      payload: e.formats.html,
      Icon: Code2,
    },
    {
      key: "go_map",
      label: "Go map",
      preview: e.formats.go_map,
      payload: e.formats.go_map,
      Icon: FileType,
    },
    {
      key: "json",
      label: "JSON",
      preview: jsonStr,
      payload: jsonStr,
      Icon: FileJson,
    },
  ];
}

const formatNames: Record<CopyFormat, string> = {
  id: "ID",
  html: "HTML",
  go_map: "Go map",
  json: "JSON",
};

export async function copyEmojiFormat(
  emoji: EmojiSticker,
  format: CopyFormat,
): Promise<boolean> {
  const rows = buildRows(emoji);
  const target = rows.find((r) => r.key === format);
  if (!target) return false;
  const ok = await copyText(target.payload);
  if (ok) {
    toast.success(`已复制 ${formatNames[format]}`);
  } else {
    toast.error("浏览器不支持复制，请手动选中文本");
  }
  return ok;
}

export default function CopyMenu({
  emoji,
  defaultFormat = "html",
  onCopied,
  className,
  compact = false,
}: CopyMenuProps) {
  const rows = buildRows(emoji);

  async function handleCopy(row: Row) {
    const ok = await copyText(row.payload);
    if (ok) {
      toast.success(`已复制 ${formatNames[row.key]}`);
      onCopied?.(row.key);
    } else {
      toast.error("浏览器不支持复制，请手动选中文本");
    }
  }

  return (
    <div className={cn("flex flex-col gap-0.5", className)}>
      {!compact && (
        <div className="flex items-baseline justify-between px-2 pt-1 pb-2">
          <div className="flex items-center gap-2 min-w-0">
            <span className="text-base shrink-0">{emoji.alt}</span>
            <span className="truncate text-xs text-muted-foreground font-mono">
              {emoji.pack.short_name}
            </span>
          </div>
          {emoji.is_animated && (
            <span className="shrink-0 text-[10px] font-mono text-primary">
              animated
            </span>
          )}
        </div>
      )}
      {rows.map((row) => {
        const isDefault = row.key === defaultFormat;
        return (
          <button
            key={row.key}
            type="button"
            onClick={() => handleCopy(row)}
            className={cn(
              "copy-row group flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left",
              "hover:bg-accent active:bg-accent/80",
              isDefault && "bg-accent/40",
            )}
          >
            <row.Icon
              className="h-4 w-4 shrink-0 text-muted-foreground group-hover:text-primary transition-colors"
              aria-hidden
            />
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-1.5">
                <span className="text-xs font-medium">{row.label}</span>
                {isDefault && (
                  <span className="text-[9px] font-mono uppercase tracking-wide text-primary/80">
                    enter
                  </span>
                )}
              </div>
              <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                {row.preview}
              </div>
            </div>
            <Copy className="h-3.5 w-3.5 shrink-0 text-muted-foreground/60 group-hover:text-foreground transition-colors" />
          </button>
        );
      })}
    </div>
  );
}
