"""extractor.py 表驱动测试。"""

from types import SimpleNamespace

from lib import extractor, mocks


def test_extract_emoji_basic():
    doc = mocks.make_fake_doc(11111, "🔥", mime="video/webm")
    kw_index = {11111: ["fire", "flame"]}
    out = extractor.extract_emoji(doc, kw_index)
    assert out == {
        "custom_emoji_id": "11111",
        "alt": "🔥",
        "keywords": "fire,flame",
        "mime_type": "video/webm",
        "is_free": False,
        "is_animated": True,
    }


def test_extract_emoji_mime_passthrough():
    """三种 mime 都要原样透传给入库 dict。"""
    for mime in ("image/webp", "video/webm", "application/x-tgsticker"):
        doc = mocks.make_fake_doc(1, "x", mime=mime)
        out = extractor.extract_emoji(doc, {})
        assert out is not None
        assert out["mime_type"] == mime


def test_extract_emoji_non_emoji_doc():
    """没有 DocumentAttributeCustomEmoji 的 doc 返回 None。"""
    doc = SimpleNamespace(id=99, mime_type="image/jpeg",
                          attributes=[SimpleNamespace()])
    assert extractor.extract_emoji(doc, {}) is None


def test_extract_emoji_no_id():
    """缺 id 视为非法，返 None。"""
    attr = mocks.make_fake_attr("🔥")
    doc = SimpleNamespace(mime_type="image/webp", attributes=[attr])
    assert extractor.extract_emoji(doc, {}) is None


def test_extract_emoji_is_free():
    attr = mocks.make_fake_attr("🆓", free=True)
    doc = SimpleNamespace(id=42, mime_type="image/webp", attributes=[attr])
    out = extractor.extract_emoji(doc, {})
    assert out["is_free"] is True


def test_extract_emoji_keywords_empty():
    doc = mocks.make_fake_doc(7, "🌟")
    out = extractor.extract_emoji(doc, {})
    assert out["keywords"] == ""


def test_extract_emoji_animated_detection():
    """三种 mime: webm/tgsticker 是动态，webp 静态。"""
    for mime, expected in [
        ("video/webm", True),
        ("application/x-tgsticker", True),
        ("image/webp", False),
        ("image/png", False),
    ]:
        doc = mocks.make_fake_doc(1, "x", mime=mime)
        out = extractor.extract_emoji(doc, {})
        assert out is not None
        assert out["is_animated"] is expected, f"mime={mime}"


def test_build_keyword_index_merges_packs_and_keywords():
    packs = [
        SimpleNamespace(emoticon="🔥", documents=[1, 2]),
        SimpleNamespace(emoticon="💎", documents=[3]),
    ]
    keywords = [
        SimpleNamespace(document_id=1, keyword=["fire", "flame"]),
        SimpleNamespace(document_id=3, keyword=["diamond"]),
    ]
    idx = extractor.build_keyword_index(packs, keywords)
    assert sorted(idx[1]) == ["fire", "flame", "🔥"]
    assert sorted(idx[2]) == ["🔥"]
    assert sorted(idx[3]) == ["diamond", "💎"]


def test_build_keyword_index_handles_none():
    """packs/keywords 为 None 不应崩溃。"""
    assert extractor.build_keyword_index(None, None) == {}


def test_build_keyword_index_dedups_within_keyword_list():
    """同一 doc 同关键词重复时去重。"""
    keywords = [
        SimpleNamespace(document_id=1, keyword=["fire", "fire", "flame"]),
    ]
    idx = extractor.build_keyword_index([], keywords)
    assert sorted(idx[1]) == ["fire", "flame"]


def test_extract_pack_meta():
    ss = mocks.make_fake_sticker_set(set_id=42, short_name="TestPack", title="Test")
    meta = extractor.extract_pack_meta(ss)
    assert meta == {
        "short_name": "TestPack",
        "title": "Test",
        "tg_pack_id": "42",
        "sticker_count": 3,
    }
