import { Loader2, Plus, AlertCircle } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { ApiError, addPack } from "@/api/client";
import { useT } from "@/i18n/LanguageProvider";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

const SHORT_NAME_RE = /^[A-Za-z][A-Za-z0-9_]{0,63}$/;

/**
 * 模块级同步锁 —— React state 异步 setter 在快速点击下能被绕过，
 * 这里同步置 true，finally 释放
 */
let _submitLock = false;

interface AddPackDialogProps {
  onSuccess?: () => void;
}

export default function AddPackDialog({ onSuccess }: AddPackDialogProps) {
  const [open, setOpen] = useState(false);
  const [shortName, setShortName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [touched, setTouched] = useState(false);
  const t = useT();

  const trimmed = shortName.trim();
  const isValid = SHORT_NAME_RE.test(trimmed);
  const showError = touched && trimmed.length > 0 && !isValid;
  const canSubmit = isValid && !submitting;

  function resetForm() {
    setShortName("");
    setTouched(false);
    setSubmitting(false);
    _submitLock = false;
  }

  function handleOpenChange(next: boolean) {
    if (submitting) return; // 提交中不让关
    setOpen(next);
    if (!next) resetForm();
  }

  async function handleSubmit() {
    if (_submitLock) return;
    if (!isValid) {
      setTouched(true);
      return;
    }
    _submitLock = true;
    setSubmitting(true);
    try {
      const data = await addPack({ short_name: trimmed });
      toast.success(data.message || "已加入爬取队列，刷新后可见");
      onSuccess?.();
      setOpen(false);
      resetForm();
    } catch (err) {
      if (err instanceof ApiError && err.code === 4009) {
        toast.error("该表情包已存在");
      } else if (err instanceof ApiError && err.code === 4001) {
        toast.error(err.message || "short_name 格式无效");
      } else {
        toast.error(err instanceof Error ? err.message : "添加失败");
      }
    } finally {
      setSubmitting(false);
      _submitLock = false;
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm" variant="default" className="gap-1.5">
          <Plus className="h-4 w-4" />
          <span>{t("add_pack_btn")}</span>
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("add_pack_dialog_title")}</DialogTitle>
          {/* TODO i18n: DialogDescription 内部正文下轮统一翻 */}
          <DialogDescription>
            输入 Telegram 包的 short_name（包链接最后一段），加入爬取队列。
            通常 1-2 分钟内可搜索。
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            void handleSubmit();
          }}
          className="flex flex-col gap-3"
        >
          <div className="flex flex-col gap-1.5">
            <label
              htmlFor="short_name"
              className="text-xs font-medium text-muted-foreground"
            >
              short_name
            </label>
            <Input
              id="short_name"
              autoFocus
              value={shortName}
              onChange={(e) => {
                setShortName(e.target.value);
                if (!touched) setTouched(true);
              }}
              onBlur={() => setTouched(true)}
              placeholder="例如 MicrosoftAnimatedEmojies"
              spellCheck={false}
              autoCorrect="off"
              autoCapitalize="none"
              disabled={submitting}
              className={cn(
                "font-mono",
                showError && "border-destructive focus-visible:ring-destructive",
              )}
              aria-invalid={showError}
              aria-describedby="short_name_hint"
            />
            <p
              id="short_name_hint"
              className={cn(
                "text-xs flex items-start gap-1",
                showError ? "text-destructive" : "text-muted-foreground",
              )}
            >
              {showError && <AlertCircle className="h-3.5 w-3.5 mt-0.5 shrink-0" />}
              <span>
                {showError
                  ? "首字符必须为字母，仅允许字母数字下划线，长度 1-64"
                  : "首字符字母，可含字母数字下划线，长度 1-64"}
              </span>
            </p>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => handleOpenChange(false)}
              disabled={submitting}
            >
              取消
            </Button>
            <Button type="submit" disabled={!canSubmit}>
              {submitting && <Loader2 className="h-4 w-4 animate-spin" />}
              <span>{submitting ? "提交中" : "加入队列"}</span>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
