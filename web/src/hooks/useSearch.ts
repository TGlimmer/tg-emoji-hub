import { useEffect, useRef } from "react";
import { ApiError, searchEmoji } from "@/api/client";
import { useSearchStore } from "@/store/searchStore";
import { useDebouncedValue } from "./useDebouncedValue";

interface UseSearchOptions {
  /** 当前输入值，hook 内部 debounce */
  query: string;
  limit?: number;
  debounceMs?: number;
}

/** 输入触发的 debounce search —— 空查询自动清空结果 */
export function useSearch({ query, limit = 80, debounceMs = 300 }: UseSearchOptions) {
  const debounced = useDebouncedValue(query.trim(), debounceMs);
  const packFilter = useSearchStore((s) => s.packFilter);
  const categoryFilter = useSearchStore((s) => s.categoryFilter);
  const animatedOnly = useSearchStore((s) => s.animatedOnly);
  const setResults = useSearchStore((s) => s.setResults);
  const setStatus = useSearchStore((s) => s.setStatus);
  const reset = useSearchStore((s) => s.reset);

  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    // q / pack / category 任一有值才搜，都空回 idle
    if (!debounced && !packFilter && !categoryFilter) {
      abortRef.current?.abort();
      reset();
      return;
    }
    const ctrl = new AbortController();
    abortRef.current?.abort();
    abortRef.current = ctrl;

    setStatus("loading");
    searchEmoji(
      {
        ...(debounced ? { q: debounced } : {}),
        limit,
        ...(packFilter ? { pack: packFilter } : {}),
        ...(categoryFilter ? { category: categoryFilter } : {}),
        ...(animatedOnly ? { animated_only: true } : {}),
      },
      ctrl.signal,
    )
      .then((page) => {
        if (ctrl.signal.aborted) return;
        setResults(page.items, page.total);
      })
      .catch((err: unknown) => {
        if (ctrl.signal.aborted) return;
        // axios cancel 在新版会带 ERR_CANCELED code，跳过
        if (err instanceof ApiError && err.code === -1 && /cancel/i.test(err.message)) return;
        const msg = err instanceof Error ? err.message : "搜索失败";
        setStatus("error", msg);
      });

    return () => ctrl.abort();
  }, [debounced, limit, packFilter, categoryFilter, animatedOnly, setResults, setStatus, reset]);
}

/** 绕过 debounce 立即查询（Enter 键） */
export async function runImmediateSearch(query: string, limit = 80) {
  const trimmed = query.trim();
  const { packFilter, categoryFilter, animatedOnly, setResults, setStatus, reset } =
    useSearchStore.getState();

  if (!trimmed && !packFilter && !categoryFilter) {
    reset();
    return;
  }
  setStatus("loading");
  try {
    const page = await searchEmoji({
      ...(trimmed ? { q: trimmed } : {}),
      limit,
      ...(packFilter ? { pack: packFilter } : {}),
      ...(categoryFilter ? { category: categoryFilter } : {}),
      ...(animatedOnly ? { animated_only: true } : {}),
    });
    setResults(page.items, page.total);
  } catch (err) {
    const msg = err instanceof Error ? err.message : "搜索失败";
    setStatus("error", msg);
  }
}
