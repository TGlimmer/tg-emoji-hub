import { useEffect, useRef, useState } from "react";

// raF 自实现数字滚动，不引依赖
// target 变化时从当前显示值平滑滚动到新 target，尊重 prefers-reduced-motion
export function useCountUp(target: number, duration = 1000): number {
  const [display, setDisplay] = useState(0);
  const displayRef = useRef(0);

  useEffect(() => {
    displayRef.current = display;
  }, [display]);

  useEffect(() => {
    if (target === displayRef.current) return;

    // 系统级减少动画偏好直接落定，不耗 CPU
    const prefersReduced =
      typeof window !== "undefined" &&
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (prefersReduced) {
      setDisplay(target);
      return;
    }

    const start = displayRef.current;
    const delta = target - start;
    const startedAt = performance.now();
    let raf = 0;

    const step = (now: number) => {
      const elapsed = now - startedAt;
      const t = Math.min(1, elapsed / duration);
      const eased = 1 - Math.pow(1 - t, 4); // ease-out-quart
      setDisplay(Math.round(start + delta * eased));
      if (t < 1) raf = requestAnimationFrame(step);
    };

    raf = requestAnimationFrame(step);
    return () => cancelAnimationFrame(raf);
  }, [target, duration]);

  return display;
}
