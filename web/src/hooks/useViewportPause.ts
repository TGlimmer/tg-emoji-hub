import { useEffect, useState } from "react";
import type { RefObject } from "react";

// 视口暂停 hook：一屏 80 个 lottie/video 同时跑会让 CPU 爆掉
// 只在元素进入视口时返回 true，触发动画播放；离开视口立刻 pause
export function useViewportPause(
  ref: RefObject<Element | null>,
  rootMargin = "100px",
): boolean {
  const [inView, setInView] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    // 浏览器太老没有 IntersectionObserver 直接当作在视口里，宁可播也别黑屏
    if (typeof IntersectionObserver === "undefined") {
      setInView(true);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          setInView(entry.isIntersecting);
        }
      },
      { rootMargin, threshold: 0 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [ref, rootMargin]);

  return inView;
}
