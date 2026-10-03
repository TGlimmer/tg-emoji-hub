import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { useT } from "@/i18n/LanguageProvider";

const SAMPLE_CODE = `<tg-emoji emoji-id="5404674261681232568">🔥</tg-emoji>`;

// 模块级同步锁 —— Zustand/setState 异步，快速点击能绕过 isCopying
let _copyLock = false;

export default function UsagePreview() {
  const [copied, setCopied] = useState(false);
  const t = useT();

  async function handleCopy() {
    if (_copyLock) return;
    _copyLock = true;
    try {
      await navigator.clipboard.writeText(SAMPLE_CODE);
      setCopied(true);
      window.setTimeout(() => {
        setCopied(false);
        _copyLock = false;
      }, 1500);
    } catch {
      _copyLock = false;
    }
  }

  return (
    <div className="mx-auto max-w-2xl rounded-md border border-border/60 bg-card/50 p-3">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-xs font-mono text-muted-foreground">
          {t("usage_hint")}
        </span>
        <button
          type="button"
          onClick={handleCopy}
          className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-mono text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={copied ? t("usage_copied") : t("usage_copy")}
        >
          {copied ? (
            <Check className="h-3 w-3" />
          ) : (
            <Copy className="h-3 w-3" />
          )}
          <span>{copied ? t("usage_copied") : t("usage_copy")}</span>
        </button>
      </div>
      <pre className="overflow-x-auto whitespace-pre font-mono text-xs text-foreground/90">
        {SAMPLE_CODE}
      </pre>
    </div>
  );
}
