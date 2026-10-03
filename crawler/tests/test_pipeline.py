"""pipeline.py 端到端：FakeClient + sqlite in-memory 跑完整 pipeline。"""

import pytest
from sqlalchemy import select

from lib import db, mocks, pipeline


@pytest.fixture
def session_factory():
    engine = db.make_engine("sqlite:///:memory:")
    db.create_all(engine)
    return db.make_session_factory(engine)


@pytest.mark.asyncio
async def test_process_pack_writes_pack_and_emojis(session_factory, tmp_path):
    client = mocks.FakeClient()
    ss = mocks.make_fake_sticker_set(short_name="TestPack", title="Test")

    r = await pipeline.process_pack(
        client, ss, session_factory, tmp_path,
        db.PACK_SOURCE_FEATURED, download_thumbs=True,
    )
    assert r["emoji_count"] == 3
    assert r["skipped"] == 0
    assert r["pack_id"] > 0

    with session_factory() as session:
        pack = session.scalar(select(db.StickerPack).where(
            db.StickerPack.short_name == "TestPack"))
        assert pack is not None
        assert pack.is_featured is True
        assert pack.source == "featured"
        emojis = session.scalars(select(db.EmojiSticker).where(
            db.EmojiSticker.pack_id == pack.id)).all()
        assert len(emojis) == 3
        ids = sorted(e.custom_emoji_id for e in emojis)
        assert ids == ["11111", "22222", "33333"]
        # 缩略图路径已注入
        for e in emojis:
            assert e.thumbnail_path == f"thumbs/{e.custom_emoji_id}.webp"
        # 缩略图文件实际写到 tmp_path
        for cid in ids:
            assert (tmp_path / f"{cid}.webp").exists()


@pytest.mark.asyncio
async def test_process_pack_no_thumb_download(session_factory, tmp_path):
    client = mocks.FakeClient()
    ss = mocks.make_fake_sticker_set(short_name="NoThumb")

    r = await pipeline.process_pack(
        client, ss, session_factory, tmp_path,
        db.PACK_SOURCE_MANUAL, download_thumbs=False,
    )
    assert r["emoji_count"] == 3
    assert client.downloaded == [], "download_thumbs=False 应跳过下载"


@pytest.mark.asyncio
async def test_run_full_crawl_with_fake_client(session_factory, tmp_path):
    """完整 dry-run：FakeClient 提供 featured 列表 + 单包数据，pipeline 全跑通。"""
    client = mocks.FakeClient(num_featured=2)

    # 预先 seed 一个 manual 包，确保 pipeline 会走 manual 分支
    with session_factory() as session:
        with session.begin():
            db.upsert_pack(
                session, short_name="MyManualPack", title="t", tg_pack_id="0",
                is_featured=False, sticker_count=0, source="manual",
            )

    stats = await pipeline.run_full_crawl(
        client, session_factory, tmp_path, download_thumbs=True,
    )
    assert stats["packs_total"] == 3  # 2 featured + 1 manual
    assert stats["packs_success"] == 3
    assert stats["packs_failed"] == 0
    assert stats["emoji_total"] == 9  # 3 包 × 3 emoji

    # 校验 DB 中确实有 3 个包
    with session_factory() as session:
        packs = session.scalars(select(db.StickerPack)).all()
        assert len(packs) == 3
        emojis = session.scalars(select(db.EmojiSticker)).all()
        # FakeClient 三个包的 doc_id 都是 11111/22222/33333
        # custom_emoji_id 是 UNIQUE，所以只会有 3 条不重复 emoji
        assert len(emojis) == 3


@pytest.mark.asyncio
async def test_crawl_single_pack_writes_manual(session_factory, tmp_path):
    client = mocks.FakeClient()
    r = await pipeline.crawl_single_pack(
        client, "MyPack", session_factory, tmp_path,
    )
    assert r["emoji_count"] == 3

    with session_factory() as session:
        pack = session.scalar(select(db.StickerPack).where(
            db.StickerPack.short_name == "MyPack"))
        assert pack.source == "manual", "crawl_single_pack 应入 manual"
