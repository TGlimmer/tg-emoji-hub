import { useEffect, useState } from "react";
import { listPacks } from "@/api/client";
import type { StickerPack } from "@/api/types";

interface UsePacksState {
  packs: StickerPack[];
  loading: boolean;
  error: string;
  reload: () => void;
}

export function usePacks(): UsePacksState {
  const [packs, setPacks] = useState<StickerPack[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const ctrl = new AbortController();
    setLoading(true);
    setError("");
    listPacks({ page: 1, page_size: 100, source: "all" }, ctrl.signal)
      .then((page) => {
        if (ctrl.signal.aborted) return;
        setPacks(page.items);
      })
      .catch((err: unknown) => {
        if (ctrl.signal.aborted) return;
        setError(err instanceof Error ? err.message : "加载表情包失败");
      })
      .finally(() => {
        if (!ctrl.signal.aborted) setLoading(false);
      });
    return () => ctrl.abort();
  }, [reloadKey]);

  return {
    packs,
    loading,
    error,
    reload: () => setReloadKey((k) => k + 1),
  };
}
