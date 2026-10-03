"""下载 emoji 原文件（webp/webm/tgs）到本地。

与 thumbnail.py 同套路径穿越防御：custom_emoji_id 必须纯数字 + 扩展名白名单。
不像 thumb 用 thumb=-1 取缩略，这里 Telethon 默认下载完整 document 字节流。
"""

from __future__ import annotations

import logging
import re
from pathlib import Path
from typing import Any

log = logging.getLogger(__name__)

# 路径穿越防御：custom_emoji_id 必须纯数字
_VALID_ID_RE = re.compile(r"^\d+$")

# Telegram doc.mime_type → 原文件扩展名
_MIME_EXT_MAP = {
    "image/webp": "webp",
    "video/webm": "webm",
    "application/x-tgsticker": "tgs",
}

# 与 Go entity.MediaExtFromMime 对齐的合法扩展
_VALID_EXTS = frozenset(_MIME_EXT_MAP.values())


def ext_from_mime(mime: str) -> str:
    """mime 推扩展名；未知 mime 兜底为 webp（与 Go MediaExtFromMime 一致）。"""
    return _MIME_EXT_MAP.get(mime, "webp")


def _build_media_path(base_dir: Path, custom_emoji_id: str, ext: str) -> Path:
    """拼原文件本地路径，强校验防穿越。"""
    if not _VALID_ID_RE.fullmatch(custom_emoji_id):
        raise ValueError(f"非法 custom_emoji_id: {custom_emoji_id!r}")
    if ext not in _VALID_EXTS:
        raise ValueError(f"非法扩展名: {ext!r}")
    filename = f"{custom_emoji_id}.{ext}"
    full = (base_dir / filename).resolve()
    base_resolved = base_dir.resolve()
    if base_resolved not in full.parents and full.parent != base_resolved:
        raise ValueError(f"路径越界: {full}")
    return full


async def download_original(client: Any, doc: Any, base_dir: Path, *, force: bool = False) -> tuple[str, str]:
    """下载 doc 原文件到 base_dir/{custom_emoji_id}.{ext}。

    本地已存在非空文件则跳过（force=True 强制重下）。返回 (filename, mime_type)。
    """
    doc_id = getattr(doc, "id", None)
    if doc_id is None:
        raise ValueError("doc 缺少 id")
    mime = getattr(doc, "mime_type", "") or ""
    ext = ext_from_mime(mime)

    custom_emoji_id = str(doc_id)
    full = _build_media_path(base_dir, custom_emoji_id, ext)
    full.parent.mkdir(parents=True, exist_ok=True)

    if not force and full.exists() and full.stat().st_size > 0:
        log.debug("原文件已存在跳过 %s", full.name)
        return full.name, mime

    # 不传 thumb 参数 → Telethon 下载完整 document 字节流
    await client.download_media(doc, file=str(full))
    log.debug("下载原文件 %s → %s (%s)", custom_emoji_id, full, mime)
    return full.name, mime
