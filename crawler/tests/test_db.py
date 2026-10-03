"""db.py 单测：upsert 幂等 / source 兜底 / 字段映射。"""


import pytest
from sqlalchemy import select

from lib import db


@pytest.fixture
def session_factory():
    engine = db.make_engine("sqlite:///:memory:")
    db.create_all(engine)
    return db.make_session_factory(engine)


def test_normalize_source():
    assert db.normalize_source("featured") == "featured"
    assert db.normalize_source("manual") == "manual"
    assert db.normalize_source("") == "featured"
    assert db.normalize_source("garbage") == "featured"
    assert db.normalize_source("FEATURED") == "featured"  # 大小写敏感，回退


def test_upsert_pack_creates_then_updates(session_factory):
    with session_factory() as session:
        with session.begin():
            pid1 = db.upsert_pack(
                session, short_name="MyPack", title="Title v1",
                tg_pack_id="100", is_featured=False, sticker_count=5,
                source="manual",
            )

        with session.begin():
            pid2 = db.upsert_pack(
                session, short_name="MyPack", title="Title v2",
                tg_pack_id="100", is_featured=True, sticker_count=10,
                source="featured",
            )
        assert pid1 == pid2, "同 short_name 应返回同一 pack_id"

        # 校验字段被更新
        pack = session.scalar(select(db.StickerPack).where(
            db.StickerPack.short_name == "MyPack"))
        assert pack.title == "Title v2"
        assert pack.is_featured is True
        assert pack.sticker_count == 10
        assert pack.source == "featured"


def test_upsert_pack_normalizes_invalid_source(session_factory):
    with session_factory() as session:
        with session.begin():
            db.upsert_pack(
                session, short_name="Bad", title="x", tg_pack_id="1",
                is_featured=False, sticker_count=0, source="invalid_value",
            )
        pack = session.scalar(select(db.StickerPack).where(
            db.StickerPack.short_name == "Bad"))
        assert pack.source == "featured", "非法 source 必须被兜底"


def test_upsert_emojis_basic(session_factory):
    with session_factory() as session:
        with session.begin():
            pid = db.upsert_pack(
                session, short_name="P1", title="t", tg_pack_id="1",
                is_featured=True, sticker_count=2, source="featured",
            )
            n = db.upsert_emojis(session, pack_id=pid, emojis=[
                {"custom_emoji_id": "1001", "alt": "🔥",
                 "keywords": "fire,flame", "is_free": False, "is_animated": True,
                 "thumbnail_path": "thumbs/1001.webp"},
                {"custom_emoji_id": "1002", "alt": "💎",
                 "keywords": "diamond", "is_free": True, "is_animated": False},
            ])
        assert n == 2

        rows = session.scalars(select(db.EmojiSticker).order_by(
            db.EmojiSticker.custom_emoji_id)).all()
        assert len(rows) == 2
        assert rows[0].custom_emoji_id == "1001"
        assert rows[0].alt == "🔥"
        assert rows[0].thumbnail_path == "thumbs/1001.webp"
        assert rows[0].is_animated is True
        assert rows[1].is_free is True
        # 默认 thumbnail_path 为空
        assert rows[1].thumbnail_path == ""


def test_upsert_emojis_idempotent(session_factory):
    """同 custom_emoji_id 二次 upsert 只更新不重复。"""
    with session_factory() as session:
        with session.begin():
            pid = db.upsert_pack(
                session, short_name="P", title="t", tg_pack_id="1",
                is_featured=False, sticker_count=0, source="featured",
            )
            db.upsert_emojis(session, pack_id=pid, emojis=[
                {"custom_emoji_id": "x1", "alt": "old", "keywords": "a",
                 "is_free": False, "is_animated": False},
            ])
        with session.begin():
            db.upsert_emojis(session, pack_id=pid, emojis=[
                {"custom_emoji_id": "x1", "alt": "new", "keywords": "b,c",
                 "is_free": True, "is_animated": True},
            ])

        rows = session.scalars(select(db.EmojiSticker)).all()
        assert len(rows) == 1
        assert rows[0].alt == "new"
        assert rows[0].keywords == "b,c"
        assert rows[0].is_free is True
        assert rows[0].is_animated is True


def test_upsert_emojis_empty_list(session_factory):
    with session_factory() as session:
        with session.begin():
            pid = db.upsert_pack(
                session, short_name="P", title="t", tg_pack_id="1",
                is_featured=False, sticker_count=0, source="featured",
            )
            assert db.upsert_emojis(session, pack_id=pid, emojis=[]) == 0


def test_list_manual_pack_short_names(session_factory):
    with session_factory() as session:
        with session.begin():
            db.upsert_pack(session, short_name="A", title="a", tg_pack_id="1",
                           is_featured=True, sticker_count=0, source="featured")
            db.upsert_pack(session, short_name="B", title="b", tg_pack_id="2",
                           is_featured=False, sticker_count=0, source="manual")
            db.upsert_pack(session, short_name="C", title="c", tg_pack_id="3",
                           is_featured=False, sticker_count=0, source="manual")

        names = db.list_manual_pack_short_names(session)
        assert sorted(names) == ["B", "C"]


def test_field_names_match_go_entity():
    """字段命名与 Go entity 100% 对齐。"""
    pack_cols = {c.name for c in db.StickerPack.__table__.columns}
    assert pack_cols == {
        "id", "short_name", "title", "tg_pack_id", "is_featured",
        "sticker_count", "source", "last_crawled_at", "created_at", "updated_at",
    }
    emoji_cols = {c.name for c in db.EmojiSticker.__table__.columns}
    assert emoji_cols == {
        "id", "custom_emoji_id", "alt", "keywords", "category", "pack_id",
        "thumbnail_path", "mime_type", "is_free", "is_animated",
        "created_at", "updated_at",
    }
