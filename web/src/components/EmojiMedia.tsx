import { useEffect, useRef, useState } from "react";
import type { AnimationItem } from "lottie-web";
import lottie from "lottie-web";
import pako from "pako";
import { useViewportPause } from "@/hooks/useViewportPause";

interface EmojiMediaProps {
  /** 原文件 URL，如 /emoji/{id}.tgs */
  url: string;
  /** image/webp | video/webm | application/x-tgsticker，未知值降级静态图 */
  mime: string;
  /** 渲染尺寸（正方形），默认 96 */
  size?: number;
  /** 任一渲染失败时回落的静态缩略图 */
  thumbnailUrl?: string;
  /** 同时给 <img>/<video>/lottie aria-label 用 */
  alt?: string;
  /** 包括 fallback 静态图也加载失败时通知外层（用于显示终极占位符） */
  onAllFailed?: () => void;
}

type Kind = "image" | "video" | "tgs" | "fallback";

function resolveKind(mime: string): Kind {
  if (mime === "video/webm") return "video";
  if (mime === "application/x-tgsticker") return "tgs";
  if (mime.startsWith("image/")) return "image";
  // 后端还没返 mime / 未知值 → 当静态图
  return "image";
}

export default function EmojiMedia({
  url,
  mime,
  size = 96,
  thumbnailUrl,
  alt = "",
  onAllFailed,
}: EmojiMediaProps) {
  const [failed, setFailed] = useState(false);
  const kind: Kind = failed ? "fallback" : resolveKind(mime);

  if (kind === "fallback" || (kind === "image" && !url)) {
    return (
      <img
        src={thumbnailUrl ?? url}
        alt={alt}
        width={size}
        height={size}
        loading="lazy"
        decoding="async"
        style={{ width: size, height: size, objectFit: "contain" }}
        onError={onAllFailed}
      />
    );
  }

  if (kind === "image") {
    return (
      <img
        src={url}
        alt={alt}
        width={size}
        height={size}
        loading="lazy"
        decoding="async"
        style={{ width: size, height: size, objectFit: "contain" }}
        onError={() => setFailed(true)}
      />
    );
  }

  if (kind === "video") {
    return (
      <VideoMedia
        url={url}
        size={size}
        alt={alt}
        thumbnailUrl={thumbnailUrl}
        onError={() => setFailed(true)}
      />
    );
  }

  return (
    <TgsMedia
      url={url}
      size={size}
      alt={alt}
      thumbnailUrl={thumbnailUrl}
      onError={() => setFailed(true)}
    />
  );
}

// ============ video/webm ============
// 首屏没进视口的不挂 src，省去 preload metadata 的请求
function VideoMedia({
  url,
  size,
  alt,
  thumbnailUrl,
  onError,
}: {
  url: string;
  size: number;
  alt: string;
  thumbnailUrl?: string;
  onError: () => void;
}) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const inView = useViewportPause(wrapperRef);
  // 进过一次视口就给 src，之后不再卸载，避免来回滚动重发请求
  const [activated, setActivated] = useState(false);

  useEffect(() => {
    if (inView && !activated) setActivated(true);
  }, [inView, activated]);

  useEffect(() => {
    const v = videoRef.current;
    if (!v || !activated) return;
    if (inView) {
      const p = v.play();
      if (p && typeof p.catch === "function") p.catch(() => {});
    } else {
      v.pause();
    }
  }, [inView, activated]);

  return (
    <div
      ref={wrapperRef}
      style={{ width: size, height: size, position: "relative" }}
    >
      {!activated && (
        <img
          src={thumbnailUrl ?? ""}
          alt={alt}
          width={size}
          height={size}
          loading="lazy"
          decoding="async"
          style={{ width: size, height: size, objectFit: "contain" }}
        />
      )}
      {activated && (
        <video
          ref={videoRef}
          src={url}
          width={size}
          height={size}
          autoPlay
          loop
          muted
          playsInline
          preload="metadata"
          aria-label={alt}
          style={{ width: size, height: size, objectFit: "contain" }}
          onError={onError}
        />
      )}
    </div>
  );
}

// ============ application/x-tgsticker (gzip lottie JSON) ============

function TgsMedia({
  url,
  size,
  alt,
  thumbnailUrl,
  onError,
}: {
  url: string;
  size: number;
  alt: string;
  thumbnailUrl?: string;
  onError: () => void;
}) {
  // observerRef: 外层包装，给 IntersectionObserver 监视位置
  // animContainerRef: 独立 div，lottie 专属注入 SVG，React 不动它的 children
  // 之前用同一个 ref 会让 React 卸占位 img 时清掉 lottie 的 SVG
  const observerRef = useRef<HTMLDivElement>(null);
  const animContainerRef = useRef<HTMLDivElement>(null);
  const animRef = useRef<AnimationItem | null>(null);
  const inView = useViewportPause(observerRef);
  const [loaded, setLoaded] = useState(false);

  const onErrorRef = useRef(onError);
  onErrorRef.current = onError;

  useEffect(() => {
    if (!inView || loaded) return;
    let cancelled = false;

    (async () => {
      try {
        const buf = await fetch(url).then((r) => {
          if (!r.ok) throw new Error(`HTTP ${r.status}`);
          return r.arrayBuffer();
        });
        const jsonStr = pako.ungzip(new Uint8Array(buf), { to: "string" });
        const animationData = JSON.parse(jsonStr) as unknown;
        if (cancelled || !animContainerRef.current) return;

        animRef.current = lottie.loadAnimation({
          container: animContainerRef.current,
          animationData,
          loop: true,
          autoplay: true,
          renderer: "svg",
        });
        setLoaded(true);
      } catch (err) {
        console.error("[tgs] load failed", url, err);
        if (!cancelled) onErrorRef.current();
      }
    })();

    // 注意：cleanup 只设 cancelled，不要 destroy lottie。
    // 否则 setLoaded 触发的下一轮 effect cleanup 会把刚建好的 SVG 清掉。
    // lottie 销毁交给下面的 unmount-only effect。
    return () => {
      cancelled = true;
    };
  }, [url, inView, loaded]);

  // 组件真正卸载时才 destroy lottie，避免 strict mode + deps 变化引起的误清
  useEffect(() => {
    return () => {
      animRef.current?.destroy();
      animRef.current = null;
    };
  }, []);

  useEffect(() => {
    const anim = animRef.current;
    if (!anim || !loaded) return;
    if (inView) anim.play();
    else anim.pause();
  }, [inView, loaded]);

  return (
    <div
      ref={observerRef}
      role="img"
      aria-label={alt}
      style={{ position: "relative", width: size, height: size }}
    >
      {!loaded && thumbnailUrl && (
        <img
          src={thumbnailUrl}
          alt={alt}
          width={size}
          height={size}
          loading="lazy"
          decoding="async"
          style={{ position: "absolute", inset: 0, width: size, height: size, objectFit: "contain" }}
        />
      )}
      <div
        ref={animContainerRef}
        style={{ position: "absolute", inset: 0, width: size, height: size }}
      />
    </div>
  );
}
