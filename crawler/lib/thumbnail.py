"""缩略图下载到本地（白名单防路径穿越）。"""

from __future__ import annotations

import logging
import re
from pathlib import Path
from typing import Any

log = logging.getLogger(__name__)

# 白名单：custom_emoji_id 必须是纯数字 → filename 形如 12345.webp
# 与 Go 侧 /thumbs/:filename 路由 `^[0-9]+\.(webp|png|gif)$` 对齐
_VALID_ID_RE = re.compile(r"^\d+$")


def _build_thumb_path(base_dir: Path, custom_emoji_id: str, ext: str = "webp") -> Path:
    """拼缩略图本地路径，强校验 custom_emoji_id 防路径穿越。"""
    if not _VALID_ID_RE.fullmatch(custom_emoji_id):
        raise ValueError(f"非法 custom_emoji_id: {custom_emoji_id!r}")
    if ext not in ("webp", "png", "gif"):
        raise ValueError(f"非法扩展名: {ext!r}")
    filename = f"{custom_emoji_id}.{ext}"
    full = (base_dir / filename).resolve()
    # 二次确认 full 仍在 base_dir 下
    base_resolved = base_dir.resolve()
    if base_resolved not in full.parents and full.parent != base_resolved:
        raise ValueError(f"路径越界: {full}")
    return full


async def download_thumb(client: Any, doc: Any, base_dir: Path, *, force: bool = False) -> str:
    """下载 doc 缩略图到 base_dir/{custom_emoji_id}.webp，返回相对文件名。

    本地已存在非空文件则跳过（force=True 强制重下）。
    """
    doc_id = getattr(doc, "id", None)
    if doc_id is None:
        raise ValueError("doc 缺少 id")
    custom_emoji_id = str(doc_id)
    full = _build_thumb_path(base_dir, custom_emoji_id, "webp")
    full.parent.mkdir(parents=True, exist_ok=True)

    if not force and full.exists() and full.stat().st_size > 0:
        log.debug("缩略图已存在跳过 %s", full.name)
        return full.name

    # thumb=-1 → 取最大尺寸缩略图（一般 100x100 webp）
    await client.download_media(doc, file=str(full), thumb=-1)
    log.debug("下载缩略图 %s → %s", custom_emoji_id, full)
    return full.name
