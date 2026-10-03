"""dry-run / 测试用的假 Telethon 客户端 + StickerSet 数据。

设计原则：用 SimpleNamespace 模拟 Telethon 返回对象，extractor.py 的鸭子类型识别
靠 type().__name__ 检查 "DocumentAttributeCustomEmoji"，所以这里必须用动态创建的同名类。
"""

from __future__ import annotations

from pathlib import Path
from types import SimpleNamespace
from typing import Any

# 用 type() 动态创建，让 type(obj).__name__ == "DocumentAttributeCustomEmoji"
DocumentAttributeCustomEmoji = type(
    "DocumentAttributeCustomEmoji", (), {}
)


def make_fake_attr(alt: str, free: bool = False) -> Any:
    attr = DocumentAttributeCustomEmoji()
    attr.alt = alt
    attr.free = free
    return attr


def make_fake_doc(doc_id: int, alt: str, *, mime: str = "image/webp",
                  free: bool = False) -> Any:
    """构造一个鸭子型 Document：含 id / attributes / mime_type。"""
    return SimpleNamespace(
        id=doc_id,
        mime_type=mime,
        attributes=[make_fake_attr(alt, free=free)],
    )


def make_fake_sticker_set(*,
                          set_id: int = 1001,
                          short_name: str = "FakePack",
                          title: str = "Fake Pack",
                          docs: list[Any] | None = None,
                          packs: list[Any] | None = None,
                          keywords: list[Any] | None = None) -> Any:
    """构造一个鸭子型 messages.StickerSet：含 set/documents/packs/keywords。"""
    if docs is None:
        docs = [
            make_fake_doc(11111, "🔥", mime="video/webm"),
            make_fake_doc(22222, "💎"),
            make_fake_doc(33333, "✨", mime="application/x-tgsticker"),
        ]
    if packs is None:
        # StickerPack: emoticon → documents
        packs = [
            SimpleNamespace(emoticon="🔥", documents=[11111]),
            SimpleNamespace(emoticon="💎", documents=[22222]),
            SimpleNamespace(emoticon="✨", documents=[33333]),
        ]
    if keywords is None:
        keywords = [
            SimpleNamespace(document_id=11111, keyword=["fire", "flame"]),
            SimpleNamespace(document_id=22222, keyword=["diamond", "gem"]),
            SimpleNamespace(document_id=33333, keyword=["sparkle", "star"]),
        ]
    return SimpleNamespace(
        set=SimpleNamespace(
            id=set_id, short_name=short_name, title=title, count=len(docs),
            access_hash=999,
        ),
        documents=docs,
        packs=packs,
        keywords=keywords,
    )


def make_fake_featured_response(num_packs: int = 2) -> Any:
    """构造 GetFeaturedEmojiStickersRequest 的返回。"""
    sets = []
    for i in range(num_packs):
        sets.append(SimpleNamespace(set=SimpleNamespace(
            id=1000 + i,
            access_hash=2000 + i,
            short_name=f"Featured{i}",
            title=f"Featured Pack {i}",
            count=3,
        )))
    return SimpleNamespace(sets=sets)


class FakeClient:
    """模拟 Telethon TelegramClient：__call__ 路由不同请求到 fixture 数据。

    用法：传给 pipeline.run_full_crawl(client=FakeClient(), ...)，整条 pipeline 跑通不联网。
    """

    def __init__(self, *, num_featured: int = 2) -> None:
        self.num_featured = num_featured
        self.calls: list[str] = []
        self.downloaded: list[tuple[int, str]] = []

    async def __call__(self, request: Any) -> Any:
        req_name = type(request).__name__
        self.calls.append(req_name)
        if req_name == "GetFeaturedEmojiStickersRequest":
            return make_fake_featured_response(self.num_featured)
        if req_name == "GetStickerSetRequest":
            stickerset = getattr(request, "stickerset", None)
            short_name = getattr(stickerset, "short_name", None)
            if short_name:
                return make_fake_sticker_set(short_name=short_name,
                                             title=f"Pack {short_name}")
            set_id = getattr(stickerset, "id", 9999)
            return make_fake_sticker_set(set_id=set_id,
                                         short_name=f"FeaturedPack{set_id}",
                                         title=f"Featured {set_id}")
        raise RuntimeError(f"FakeClient 不支持请求类型: {req_name}")

    async def download_media(self, doc: Any, *, file: str, thumb: int = -1) -> str:
        """模拟下载：写一个 1 字节占位文件，记录调用。"""
        self.downloaded.append((getattr(doc, "id", 0), file))
        Path(file).parent.mkdir(parents=True, exist_ok=True)
        Path(file).write_bytes(b"\x00")
        return file

    async def connect(self) -> None:  # 兼容真实 client API
        return None

    async def disconnect(self) -> None:
        return None

    async def is_user_authorized(self) -> bool:
        return True
