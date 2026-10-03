import { useEffect, useState } from "react";
import { getStats } from "@/api/client";
import type { StatsOverview } from "@/api/types";
import { useCountUp } from "@/hooks/useCountUp";
import { useT } from "@/i18n/LanguageProvider";
import { Skeleton } from "./ui/skeleton";

export default function StatsHero() {
  const [stats, setStats] = useState<StatsOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const t = useT();

  useEffect(() => {
    const ctrl = new AbortController();
    let cancelled = false;
    getStats(ctrl.signal)
      .then((s) => {
        if (!cancelled) setStats(s);
      })
      .catch(() => {
        // 优雅降级：接口挂了就静默隐藏整块
        if (!cancelled) setFailed(true);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
      ctrl.abort();
    };
  }, []);

  if (failed) return null;

  const ratioPct = stats?.animated_percentage ?? 0;

  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-x-6 gap-y-4 sm:divide-x sm:divide-border/40">
      <StatCell
        value={stats?.total_emojis ?? 0}
        label={t("stat_emoji")}
        loading={loading}
      />
      <StatCell
        value={stats?.total_packs ?? 0}
        label={t("stat_packs")}
        loading={loading}
        className="sm:pl-6"
      />
      <StatCell
        value={ratioPct}
        label={t("stat_animated")}
        suffix="%"
        loading={loading}
        className="sm:pl-6"
      />
      <StatCell
        value={stats?.added_today ?? 0}
        label={t("stat_today")}
        loading={loading}
        className="sm:pl-6"
      />
    </div>
  );
}

interface StatCellProps {
  value: number;
  label: string;
  suffix?: string;
  loading?: boolean;
  className?: string;
}

function StatCell({
  value,
  label,
  suffix = "",
  loading,
  className = "",
}: StatCellProps) {
  // loading 期 target=0 不滚；数据到位后才滚到真实值
  const display = useCountUp(loading ? 0 : value, 1000);
  return (
    <div
      className={className}
      aria-label={loading ? `${label} loading` : `${value}${suffix} ${label}`}
    >
      {loading ? (
        <div>
          <Skeleton className="h-8 w-16 rounded-md" />
          <Skeleton className="mt-2 h-3.5 w-12 rounded-sm" />
        </div>
      ) : (
        <>
          <div
            className="text-2xl sm:text-3xl font-mono font-semibold tabular-nums"
            aria-live="polite"
          >
            {`${display.toLocaleString()}${suffix}`}
          </div>
          <div className="mt-1 text-xs font-mono text-muted-foreground uppercase tracking-wider">
            {label}
          </div>
        </>
      )}
    </div>
  );
}
