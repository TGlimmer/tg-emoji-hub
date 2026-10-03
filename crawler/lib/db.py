"""SQLAlchemy ORM 模型 + upsert 接口。

字段命名与 Go 侧 entity/*.go 100% 对齐（snake_case，列名显式指定）。
upsert 用 MySQL ON DUPLICATE KEY UPDATE 实现幂等（单测用 sqlite 则降级为 INSERT OR REPLACE）。
"""

from __future__ import annotations

import logging
from collections.abc import Iterable
from datetime import datetime
from typing import Any

from sqlalchemy import (
    BigInteger,
    Boolean,
    Column,
    DateTime,
    ForeignKey,
    Integer,
    String,
    Text,
    create_engine,
    select,
)
from sqlalchemy.dialects.mysql import insert as mysql_insert
from sqlalchemy.dialects.sqlite import insert as sqlite_insert
from sqlalchemy.engine import Engine
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

log = logging.getLogger(__name__)

# 与 Go entity.NormalizeSource 等价的合法值
PACK_SOURCE_FEATURED = "featured"
PACK_SOURCE_MANUAL = "manual"
_VALID_SOURCES = (PACK_SOURCE_FEATURED, PACK_SOURCE_MANUAL)


def normalize_source(s: str) -> str:
    """非法值兜底回 featured（与 Go entity.NormalizeSource 行为一致）。"""
    return s if s in _VALID_SOURCES else PACK_SOURCE_FEATURED


# 与 Go entity.NormalizeMimeType 一致的合法 mime 集合
MIME_WEBP = "image/webp"
MIME_WEBM = "video/webm"
MIME_TGS = "application/x-tgsticker"
_VALID_MIMES = (MIME_WEBP, MIME_WEBM, MIME_TGS)


def _normalize_mime(s: str) -> str:
    """非法/空值兜底为 image/webp（最常见的静态情况）。"""
    return s if s in _VALID_MIMES else MIME_WEBP


class Base(DeclarativeBase):
    pass


class StickerPack(Base):
    __tablename__ = "sticker_packs"

    id = Column(BigInteger().with_variant(Integer, "sqlite"), primary_key=True, autoincrement=True)
    short_name = Column(String(64), nullable=False, unique=True)
    title = Column(String(255), nullable=False)
    tg_pack_id = Column(String(32), nullable=False, default="")
    is_featured = Column(Boolean, nullable=False, default=False)
    sticker_count = Column(Integer, nullable=False, default=0)
    source = Column(String(16), nullable=False, default=PACK_SOURCE_FEATURED)
    last_crawled_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)


class EmojiSticker(Base):
    __tablename__ = "emoji_stickers"

    id = Column(BigInteger().with_variant(Integer, "sqlite"), primary_key=True, autoincrement=True)
    custom_emoji_id = Column(String(32), nullable=False, unique=True)
    alt = Column(String(16), nullable=False, index=True)
    keywords = Column(Text, nullable=False, default="")
    category = Column(String(20), nullable=False, default="")
    pack_id = Column(
        BigInteger().with_variant(Integer, "sqlite"),
        ForeignKey("sticker_packs.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
    )
    thumbnail_path = Column(String(255), nullable=False, default="")
    mime_type = Column(String(64), nullable=False, default="image/webp")
    is_free = Column(Boolean, nullable=False, default=False)
    is_animated = Column(Boolean, nullable=False, default=False)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)


def make_engine(dsn: str, *, echo: bool = False) -> Engine:
    """根据 DSN 创建 Engine。

    生产 DSN 形如 mysql+pymysql://...，单测可传 sqlite:///:memory:
    """
    if not dsn:
        raise RuntimeError("DB_DSN 未配置")
    engine = create_engine(dsn, echo=echo, pool_pre_ping=True, future=True)
    return engine


def create_all(engine: Engine) -> None:
    """开发/测试场景建表；生产用 migrations/*.up.sql 走 golang-migrate。"""
    Base.metadata.create_all(engine)


def make_session_factory(engine: Engine) -> sessionmaker[Session]:
    return sessionmaker(engine, expire_on_commit=False, future=True)


# ---------- upsert helpers ----------

def _upsert_dialect(engine_or_session: Engine | Session):
    """根据当前连接的方言返回对应 insert helper。"""
    bind = engine_or_session.bind if isinstance(engine_or_session, Session) else engine_or_session
    name = bind.dialect.name
    if name == "mysql":
        return mysql_insert
    if name == "sqlite":
        return sqlite_insert
    raise RuntimeError(f"unsupported dialect: {name}")


def upsert_pack(session: Session, *, short_name: str, title: str, tg_pack_id: str,
                is_featured: bool, sticker_count: int, source: str) -> int:
    """upsert sticker_packs，返回 pack.id。

    幂等：再次跑同 short_name 只更新非主键字段，不重复创建。
    """
    source = normalize_source(source)  # 兜底防脏值
    insert = _upsert_dialect(session)

    stmt = insert(StickerPack.__table__).values(
        short_name=short_name,
        title=title,
        tg_pack_id=tg_pack_id,
        is_featured=is_featured,
        sticker_count=sticker_count,
        source=source,
        last_crawled_at=datetime.utcnow(),
    )
    dialect_name = session.bind.dialect.name
    if dialect_name == "mysql":
        stmt = stmt.on_duplicate_key_update(
            title=stmt.inserted.title,
            tg_pack_id=stmt.inserted.tg_pack_id,
            is_featured=stmt.inserted.is_featured,
            sticker_count=stmt.inserted.sticker_count,
            source=stmt.inserted.source,
            last_crawled_at=stmt.inserted.last_crawled_at,
        )
    else:  # sqlite (单测)
        stmt = stmt.on_conflict_do_update(
            index_elements=["short_name"],
            set_={
                "title": stmt.excluded.title,
                "tg_pack_id": stmt.excluded.tg_pack_id,
                "is_featured": stmt.excluded.is_featured,
                "sticker_count": stmt.excluded.sticker_count,
                "source": stmt.excluded.source,
                "last_crawled_at": stmt.excluded.last_crawled_at,
            },
        )
    session.execute(stmt)

    pack = session.scalar(select(StickerPack).where(StickerPack.short_name == short_name))
    if pack is None:
        raise RuntimeError(f"upsert_pack 后查不到 short_name={short_name}")
    return pack.id


def upsert_emojis(session: Session, *, pack_id: int, emojis: Iterable[dict[str, Any]]) -> int:
    """批量 upsert emoji_stickers，返回写入条数。

    每条 dict 必须含 custom_emoji_id/alt/keywords/is_free/is_animated；thumbnail_path 可选。
    """
    rows = [
        {
            "custom_emoji_id": e["custom_emoji_id"],
            "alt": e["alt"],
            "keywords": e.get("keywords", ""),
            "pack_id": pack_id,
            "thumbnail_path": e.get("thumbnail_path", ""),
            "mime_type": _normalize_mime(e.get("mime_type", "")),
            "is_free": bool(e.get("is_free", False)),
            "is_animated": bool(e.get("is_animated", False)),
        }
        for e in emojis
    ]
    if not rows:
        return 0

    insert = _upsert_dialect(session)
    stmt = insert(EmojiSticker.__table__).values(rows)
    dialect_name = session.bind.dialect.name
    if dialect_name == "mysql":
        stmt = stmt.on_duplicate_key_update(
            alt=stmt.inserted.alt,
            keywords=stmt.inserted.keywords,
            pack_id=stmt.inserted.pack_id,
            thumbnail_path=stmt.inserted.thumbnail_path,
            mime_type=stmt.inserted.mime_type,
            is_free=stmt.inserted.is_free,
            is_animated=stmt.inserted.is_animated,
        )
    else:
        stmt = stmt.on_conflict_do_update(
            index_elements=["custom_emoji_id"],
            set_={
                "alt": stmt.excluded.alt,
                "keywords": stmt.excluded.keywords,
                "pack_id": stmt.excluded.pack_id,
                "thumbnail_path": stmt.excluded.thumbnail_path,
                "mime_type": stmt.excluded.mime_type,
                "is_free": stmt.excluded.is_free,
                "is_animated": stmt.excluded.is_animated,
            },
        )
    session.execute(stmt)
    return len(rows)


def list_manual_pack_short_names(session: Session) -> list[str]:
    """读出所有 source='manual' 的 pack short_name，用于全量爬取时合并。"""
    rows = session.scalars(
        select(StickerPack.short_name).where(StickerPack.source == PACK_SOURCE_MANUAL)
    ).all()
    return list(rows)
