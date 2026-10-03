import { Check, Copy, Plug } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { useT } from "@/i18n/LanguageProvider";

// 模块级同步锁 —— React state 异步 setter 在快速点击下能被绕过
let _copyLock = false;

interface Tool {
  name: string;
  description: string;
  params: { key: string; required: boolean; note?: string }[];
}

const TOOLS: Tool[] = [
  {
    name: "search_emoji",
    description: "按关键词或 emoji 字符搜索；query / pack / category 至少填一个",
    params: [
      { key: "query", required: false, note: "关键词或 emoji 字符" },
      { key: "pack", required: false, note: "限定包 short_name" },
      { key: "category", required: false, note: "限定分类，如 emotion / animal" },
      { key: "limit", required: false, note: "默认 20，上限 100" },
    ],
  },
  {
    name: "get_emoji_by_id",
    description: "按 custom_emoji_id 取单个",
    params: [{ key: "custom_emoji_id", required: true }],
  },
  {
    name: "list_packs",
    description: "列出所有已收录的包",
    params: [{ key: "source", required: false, note: "featured / manual" }],
  },
  {
    name: "list_pack_stickers",
    description: "浏览某个包内的全部 emoji",
    params: [
      { key: "pack_short_name", required: true, note: "包的 short_name" },
      { key: "limit", required: false, note: "默认 100，上限 100" },
    ],
  },
  {
    name: "search_in_pack",
    description: "在指定包内搜索 emoji",
    params: [
      { key: "pack_short_name", required: true, note: "包的 short_name" },
      { key: "query", required: true, note: "搜索关键词" },
      { key: "limit", required: false, note: "默认 20，上限 100" },
    ],
  },
];

type ClientId = "claude-code" | "cursor" | "codex";

const CLIENTS: { id: ClientId; label: string }[] = [
  { id: "claude-code", label: "Claude Code" },
  { id: "cursor", label: "Cursor" },
  { id: "codex", label: "Codex" },
];

interface ClientConfig {
  cliCommand?: string;
  configPath: string;
  configBody: string;
  configLang: "json" | "toml";
  note: string;
}

interface McpDialogProps {
  trigger?: React.ReactNode;
}

export default function McpDialog({ trigger }: McpDialogProps) {
  const [open, setOpen] = useState(false);
  const [endpoint, setEndpoint] = useState("/mcp");
  const [activeClient, setActiveClient] = useState<ClientId>("claude-code");
  const t = useT();

  useEffect(() => {
    if (typeof window !== "undefined") {
      setEndpoint(`${window.location.origin}/mcp`);
    }
  }, []);

  const configs = useMemo<Record<ClientId, ClientConfig>>(() => {
    const claudeBody = JSON.stringify(
      {
        mcpServers: {
          "tg-emoji-hub": { type: "http", url: endpoint },
        },
      },
      null,
      2,
    );
    const cursorBody = JSON.stringify(
      {
        mcpServers: {
          "tg-emoji-hub": { url: endpoint },
        },
      },
      null,
      2,
    );
    // TOML 用显式 \n 拼，避免模板字面量缩进
    const codexBody = `[mcp_servers.tg-emoji-hub]\nurl = "${endpoint}"`;
    return {
      "claude-code": {
        cliCommand: `claude mcp add --transport http tg-emoji-hub ${endpoint}`,
        configPath: "~/.claude.json (全局) 或项目根 .mcp.json",
        configBody: claudeBody,
        configLang: "json",
        note: "Streamable HTTP 是 Anthropic 当前官方推荐传输方式（SSE 已弃用）。CLI 一行命令是最快接入方式，手动编辑配置文件适合纳入版本管理。",
      },
      cursor: {
        configPath: "~/.cursor/mcp.json (全局) 或项目 .cursor/mcp.json",
        configBody: cursorBody,
        configLang: "json",
        note: "Cursor 自动识别 streamable-http 传输；总工具数上限约 40 个，注意配额。",
      },
      codex: {
        configPath:
          "~/.codex/config.toml (全局) 或项目 .codex/config.toml（仅 trusted projects）",
        configBody: codexBody,
        configLang: "toml",
        note: "Codex 用 TOML 而非 JSON。也可运行 `codex mcp` 命令交互式管理。",
      },
    };
  }, [endpoint]);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger ?? (
          <Button size="sm" variant="outline" className="gap-1.5">
            <Plug className="h-4 w-4" />
            <span>{t("mcp_btn")}</span>
          </Button>
        )}
      </DialogTrigger>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("mcp_dialog_title")}</DialogTitle>
          {/* TODO i18n: DialogDescription 内部正文量大，下轮统一翻 */}
          <DialogDescription>
            在 Claude Code / Cursor / Codex 等 MCP 客户端中接入，让 AI 直接搜
            Telegram premium emoji。
          </DialogDescription>
        </DialogHeader>

        <div className="flex max-h-[70vh] flex-col gap-6 overflow-y-auto pr-1">
          <Section title="接入端点">
            <div className="flex flex-col gap-1.5">
              <div className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
                Streamable HTTP
              </div>
              <CopyableBlock value={endpoint} mono />
            </div>
          </Section>

          <Section title="可用工具">
            <div className="flex flex-col gap-3">
              {TOOLS.map((t) => (
                <ToolCard key={t.name} tool={t} />
              ))}
            </div>
          </Section>

          <Section title="客户端配置">
            <Tabs
              value={activeClient}
              onValueChange={(v) => setActiveClient(v as ClientId)}
            >
              <TabsList>
                {CLIENTS.map((c) => (
                  <TabsTrigger key={c.id} value={c.id}>
                    {c.label}
                  </TabsTrigger>
                ))}
              </TabsList>

              {CLIENTS.map((c) => {
                const cfg = configs[c.id];
                return (
                  <TabsContent
                    key={c.id}
                    value={c.id}
                    className="flex flex-col gap-3"
                  >
                    {cfg.cliCommand && (
                      <div className="flex flex-col gap-1.5">
                        <Label>命令行（推荐）</Label>
                        <CopyableBlock value={cfg.cliCommand} mono />
                      </div>
                    )}
                    <div className="flex flex-col gap-1.5">
                      <Label>
                        配置文件
                        <span className="ml-2 font-mono normal-case tracking-normal text-foreground/80">
                          {cfg.configPath}
                        </span>
                      </Label>
                      <CopyableBlock value={cfg.configBody} mono multiline />
                    </div>
                    <p className="text-xs text-muted-foreground leading-relaxed">
                      {cfg.note}
                    </p>
                  </TabsContent>
                );
              })}
            </Tabs>
          </Section>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <h3 className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
          {title}
        </h3>
        <div className="h-px flex-1 bg-border/60" />
      </div>
      {children}
    </section>
  );
}

function Label({ children }: { children: React.ReactNode }) {
  return (
    <div className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
      {children}
    </div>
  );
}

function ToolCard({ tool }: { tool: Tool }) {
  return (
    <div className="rounded-md border border-border/60 bg-card/40 p-3">
      <div className="flex flex-wrap items-baseline gap-2">
        <code className="font-mono text-sm font-semibold">{tool.name}</code>
        <span className="text-xs text-muted-foreground">{tool.description}</span>
      </div>
      <ul className="mt-2 flex flex-col gap-1">
        {tool.params.map((p) => (
          <li
            key={p.key}
            className="flex items-baseline gap-2 text-xs text-muted-foreground"
          >
            <code className="font-mono text-foreground/80">{p.key}</code>
            <span
              className={cn(
                "rounded px-1.5 py-0.5 text-[10px] font-mono",
                p.required
                  ? "bg-primary/15 text-primary"
                  : "bg-muted text-muted-foreground",
              )}
            >
              {p.required ? "required" : "optional"}
            </span>
            {p.note && <span>{p.note}</span>}
          </li>
        ))}
      </ul>
    </div>
  );
}

function CopyableBlock({
  value,
  mono,
  multiline,
}: {
  value: string;
  mono?: boolean;
  multiline?: boolean;
}) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    if (_copyLock) return;
    _copyLock = true;
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      toast.error("复制失败，请手动选中复制");
    } finally {
      _copyLock = false;
    }
  }

  return (
    <div className="relative">
      <pre
        className={cn(
          "overflow-x-auto rounded-md border border-border/60 bg-muted/40 p-3 pr-10 text-xs leading-relaxed",
          mono && "font-mono",
          !multiline && "whitespace-nowrap",
        )}
      >
        {value}
      </pre>
      <button
        type="button"
        onClick={handleCopy}
        className="absolute right-2 top-2 inline-flex h-7 w-7 items-center justify-center rounded-md border border-border/60 bg-background/80 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
        aria-label={copied ? "已复制" : "复制"}
      >
        {copied ? (
          <Check className="h-3.5 w-3.5" />
        ) : (
          <Copy className="h-3.5 w-3.5" />
        )}
      </button>
    </div>
  );
}
