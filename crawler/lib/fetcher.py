"""Telethon MTProto RPC 封装：拉 featured / 拉单包。

只暴露干净的 async 接口，把 InputStickerSetID/ShortName 等 TL 类型包到内部。
FLOOD_WAIT 由 Telethon 内置自动 sleep；调用方仅在单包之间额外加 1s 礼貌延迟。
"""

from __future__ import annotations

import asyncio
import logging
from typing import Any

from telethon import functions
from telethon.tl.types import InputStickerSetID, InputStickerSetShortName

log = logging.getLogger(__name__)

# 单包之间礼貌延迟，叠加在 Telethon FLOOD_WAIT 之上
_INTER_PACK_DELAY_SEC = 1.0


async def fetch_featured_inputs(client: Any) -> list[InputStickerSetID]:
    """拉 Featured 表情包列表，转成 InputStickerSetID 列表方便后续 GetStickerSet。"""
    log.info("拉取 Featured 表情包列表")
    result = await client(functions.messages.GetFeaturedEmojiStickersRequest(hash=0))
    sets = getattr(result, "sets", []) or []
    inputs: list[InputStickerSetID] = []
    for cover in sets:
        s = getattr(cover, "set", cover)
        set_id = getattr(s, "id", None)
        access_hash = getattr(s, "access_hash", None)
        if set_id is None or access_hash is None:
            continue
        inputs.append(InputStickerSetID(id=int(set_id), access_hash=int(access_hash)))
    log.info("Featured 包数量: %d", len(inputs))
    return inputs


async def fetch_pack_by_short_name(client: Any, short_name: str) -> Any:
    """按 short_name 拉单个包。返回 messages.StickerSet。"""
    log.info("拉取单包 short_name=%s", short_name)
    return await client(
        functions.messages.GetStickerSetRequest(
            stickerset=InputStickerSetShortName(short_name=short_name),
            hash=0,
        )
    )


async def fetch_pack_by_input(client: Any, input_set: InputStickerSetID) -> Any:
    """按 InputStickerSetID 拉单个包（用于 featured 来源）。"""
    return await client(
        functions.messages.GetStickerSetRequest(stickerset=input_set, hash=0)
    )


async def polite_sleep() -> None:
    """单包之间礼貌延迟，避免触发 FLOOD_WAIT。"""
    await asyncio.sleep(_INTER_PACK_DELAY_SEC)
