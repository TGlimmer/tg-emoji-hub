"""把 client + fetcher + extractor + thumbnail + db 串成完整 pipeline。

对外仅暴露 process_pack / run_full_crawl，调用者无需关心 TL 细节。
"""

from __future__ import annotations

import logging
from pathlib import Path
from typing import Any, Protocol

from sqlalchemy.orm import sessionmaker

from . import db as db_mod
from . import extractor, fetcher, media, thumbnail
from .db import PACK_SOURCE_FEATURED, PACK_SOURCE_MANUAL

log = logging.getLogger(__name__)


class _ClientProtocol(Protocol):
    """对 client 的最小要求：能 await(req)、download_media。"""

    async def __call__(self, req: Any) -> Any: ...

    async def download_media(self, doc: Any, *, file: str, thumb: int) -> Any: ...


async def process_pack(
    client: Any,
    sticker_set: Any,
    session_factory: sessionmaker,
    thumbnail_dir: Path,
    source: str,
    *,
    download_thumbs: bool = True,
    download_originals: bool = True,
    media_dir: Path | None = None,
) -> dict[str, int]:
    """处理一个已拉回来的 messages.StickerSet：解构 + 下载缩略图/原文件 + 入库。

    download_originals=True 时把 webp/webm/tgs 原文件落到 media_dir（前端 /emoji/{id}.{ext} 直读）。
    缩略图/原文件下载失败不阻塞主流程，仅记录 log。
    返回统计 {pack_id, emoji_count, skipped}。
    """
    pack_meta = extractor.extract_pack_meta(sticker_set)
    short_name = pack_meta["short_name"]
    if not short_name:
        raise RuntimeError("StickerSet 缺少 short_name，跳过")

    kw_index = extractor.build_keyword_index(
        getattr(sticker_set, "packs", None),
        getattr(sticker_set, "keywords", None),
    )

    documents = getattr(sticker_set, "documents", None) or []
    log.info("开始处理包 short_name=%s docs=%d", short_name, len(documents))

    emoji_dicts: list[dict[str, Any]] = []
    skipped = 0
    for doc in documents:
        item = extractor.extract_emoji(doc, kw_index)
        if item is None:
            skipped += 1
            continue
        # 缩略图下载（失败不阻塞主流程）
        if download_thumbs:
            try:
                filename = await thumbnail.download_thumb(client, doc, thumbnail_dir)
                item["thumbnail_path"] = f"thumbs/{filename}"
            except Exception:  # noqa: BLE001
                log.exception("缩略图下载失败 doc_id=%s，使用空路径继续",
                              item.get("custom_emoji_id"))
                item["thumbnail_path"] = ""
        else:
            item["thumbnail_path"] = ""
        # 原文件下载：前端动态 emoji 必须拿到 webm/tgs 才能播放
        if download_originals and media_dir is not None:
            try:
                await media.download_original(client, doc, media_dir)
            except Exception:  # noqa: BLE001
                log.exception("原文件下载失败 doc_id=%s，仅入元数据继续",
                              item.get("custom_emoji_id"))
        emoji_dicts.append(item)

    # 单包一个事务：pack 元信息 + emoji 列表原子提交
    with session_factory() as session:
        with session.begin():
            pack_id = db_mod.upsert_pack(
                session,
                short_name=short_name,
                title=pack_meta["title"],
                tg_pack_id=pack_meta["tg_pack_id"],
                is_featured=(source == PACK_SOURCE_FEATURED),
                sticker_count=pack_meta["sticker_count"] or len(emoji_dicts),
                source=source,
            )
            inserted = db_mod.upsert_emojis(session, pack_id=pack_id, emojis=emoji_dicts)
    log.info("包入库完成 short_name=%s pack_id=%d emoji=%d skipped=%d",
             short_name, pack_id, inserted, skipped)
    return {"pack_id": pack_id, "emoji_count": inserted, "skipped": skipped}


async def run_full_crawl(
    client: Any,
    session_factory: sessionmaker,
    thumbnail_dir: Path,
    *,
    download_thumbs: bool = True,
    download_originals: bool = True,
    media_dir: Path | None = None,
) -> dict[str, int]:
    """全量爬取：featured 列表 + DB 中所有 manual 包。

    每个包独立失败不影响其他包；统计写入数量返回。
    """
    stats = {"packs_total": 0, "packs_success": 0, "packs_failed": 0,
             "emoji_total": 0}

    # 1) Featured
    try:
        featured_inputs = await fetcher.fetch_featured_inputs(client)
    except Exception:  # noqa: BLE001
        log.exception("拉 Featured 列表失败，跳过 featured 部分")
        featured_inputs = []

    for input_set in featured_inputs:
        stats["packs_total"] += 1
        try:
            ss = await fetcher.fetch_pack_by_input(client, input_set)
            r = await process_pack(
                client, ss, session_factory, thumbnail_dir,
                PACK_SOURCE_FEATURED,
                download_thumbs=download_thumbs,
                download_originals=download_originals,
                media_dir=media_dir,
            )
            stats["packs_success"] += 1
            stats["emoji_total"] += r["emoji_count"]
        except Exception:  # noqa: BLE001
            stats["packs_failed"] += 1
            log.exception("featured 包失败 input_set_id=%s", getattr(input_set, "id", None))
        await fetcher.polite_sleep()

    # 2) Manual（从 DB 读 short_name）
    with session_factory() as session:
        manual_names = db_mod.list_manual_pack_short_names(session)

    for short_name in manual_names:
        stats["packs_total"] += 1
        try:
            ss = await fetcher.fetch_pack_by_short_name(client, short_name)
            r = await process_pack(
                client, ss, session_factory, thumbnail_dir,
                PACK_SOURCE_MANUAL,
                download_thumbs=download_thumbs,
                download_originals=download_originals,
                media_dir=media_dir,
            )
            stats["packs_success"] += 1
            stats["emoji_total"] += r["emoji_count"]
        except Exception:  # noqa: BLE001
            stats["packs_failed"] += 1
            log.exception("manual 包失败 short_name=%s", short_name)
        await fetcher.polite_sleep()

    log.info("全量爬取结束 %s", stats)
    return stats


async def crawl_single_pack(
    client: Any,
    short_name: str,
    session_factory: sessionmaker,
    thumbnail_dir: Path,
    *,
    download_thumbs: bool = True,
    download_originals: bool = True,
    media_dir: Path | None = None,
) -> dict[str, int]:
    """爬取单个包（供 signals watcher / CLI single_pack.py 调用）。"""
    ss = await fetcher.fetch_pack_by_short_name(client, short_name)
    return await process_pack(
        client, ss, session_factory, thumbnail_dir,
        PACK_SOURCE_MANUAL,
        download_thumbs=download_thumbs,
        download_originals=download_originals,
        media_dir=media_dir,
    )
