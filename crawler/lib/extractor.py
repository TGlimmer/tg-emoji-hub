"""把 Telethon 的 Document / StickerSet 结构解构成可入库的 dict。

不依赖 Telethon 运行（只依赖 isinstance + 鸭子类型），方便单测用 mock 替代。
"""

from __future__ import annotations

from collections import defaultdict
from collections.abc import Iterable
from typing import Any

# Telethon 的 DocumentAttributeCustomEmoji 类位置：tl.types.DocumentAttributeCustomEmoji
# 单测用 mock 对象时只需具备 .alt + .free 属性，故这里用名字而非类匹配
_CUSTOM_EMOJI_ATTR = "DocumentAttributeCustomEmoji"


def _is_custom_emoji_attr(attr: Any) -> bool:
    """判定 attr 是否是 DocumentAttributeCustomEmoji。

    既支持真实 Telethon 类（按 type().__name__ 检查），也支持单测 mock。
    """
    return type(attr).__name__ == _CUSTOM_EMOJI_ATTR


def _find_custom_emoji_attr(doc: Any) -> Any | None:
    """从 doc.attributes 中找 DocumentAttributeCustomEmoji；找不到返回 None。"""
    attrs = getattr(doc, "attributes", None) or []
    for attr in attrs:
        if _is_custom_emoji_attr(attr):
            return attr
    return None


def _is_animated(doc: Any) -> bool:
    """根据 mime_type 判定是否动态 emoji。

    - application/x-tgsticker: Telegram 自家的 .tgs (Lottie 矢量动画)
    - video/webm: 视频 emoji（VP9 WebM）
    """
    mime = getattr(doc, "mime_type", "") or ""
    return mime in ("application/x-tgsticker", "video/webm")


def extract_emoji(doc: Any, kw_index: dict[int, list[str]]) -> dict[str, Any] | None:
    """把单个 Telethon Document 解构为可入库的 dict。

    返回 None 表示该 doc 不是 custom emoji（跳过）。
    """
    attr = _find_custom_emoji_attr(doc)
    if attr is None:
        return None

    doc_id = getattr(doc, "id", None)
    if doc_id is None:
        return None

    keywords = kw_index.get(int(doc_id), [])
    mime = getattr(doc, "mime_type", "") or ""
    return {
        "custom_emoji_id": str(doc_id),
        "alt": getattr(attr, "alt", "") or "",
        "keywords": ",".join(keywords),
        "mime_type": mime,
        "is_free": bool(getattr(attr, "free", False)),
        "is_animated": _is_animated(doc),
    }


def build_keyword_index(
    packs: Iterable[Any] | None,
    keywords: Iterable[Any] | None,
) -> dict[int, list[str]]:
    """合并 StickerSet.packs[].documents 与 StickerSet.keywords[].document_id 得到关键词索引。

    - StickerPack: emoticon → [document_ids]  (Telethon 命名 packs，实为 emoji 分组)
    - StickerKeyword: document_id → [keyword_strs]  (英文/多语言关键词)
    """
    idx: dict[int, set[str]] = defaultdict(set)

    # packs (按 emoji 分组的 document_id 列表)
    for sp in packs or []:
        emoticon = getattr(sp, "emoticon", "") or ""
        doc_ids = getattr(sp, "documents", None) or []
        if not emoticon:
            continue
        for doc_id in doc_ids:
            idx[int(doc_id)].add(emoticon)

    # keywords (StickerKeyword)
    for kw in keywords or []:
        doc_id = getattr(kw, "document_id", None)
        if doc_id is None:
            continue
        kw_list = getattr(kw, "keyword", None) or []
        for k in kw_list:
            k = (k or "").strip()
            if k:
                idx[int(doc_id)].add(k)

    return {k: sorted(v) for k, v in idx.items()}


def extract_pack_meta(sticker_set: Any) -> dict[str, Any]:
    """从 messages.StickerSet.set 提取包元信息。"""
    s = getattr(sticker_set, "set", None) or sticker_set
    return {
        "short_name": getattr(s, "short_name", "") or "",
        "title": getattr(s, "title", "") or "",
        "tg_pack_id": str(getattr(s, "id", "") or ""),
        "sticker_count": int(getattr(s, "count", 0) or 0),
    }
