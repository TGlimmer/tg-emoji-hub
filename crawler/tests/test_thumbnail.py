"""thumbnail.py 单测：白名单 + 路径穿越防御 + 下载调用。"""

from types import SimpleNamespace

import pytest

from lib import thumbnail
from lib.mocks import FakeClient


def test_build_thumb_path_valid(tmp_path):
    full = thumbnail._build_thumb_path(tmp_path, "12345", "webp")
    assert full.name == "12345.webp"
    assert full.parent.resolve() == tmp_path.resolve()


def test_build_thumb_path_rejects_path_traversal(tmp_path):
    with pytest.raises(ValueError):
        thumbnail._build_thumb_path(tmp_path, "../etc/passwd", "webp")
    with pytest.raises(ValueError):
        thumbnail._build_thumb_path(tmp_path, "abc", "webp")  # 非纯数字
    with pytest.raises(ValueError):
        thumbnail._build_thumb_path(tmp_path, "12345", "exe")  # 非法扩展
    with pytest.raises(ValueError):
        thumbnail._build_thumb_path(tmp_path, "12;rm-rf", "webp")
    with pytest.raises(ValueError):
        thumbnail._build_thumb_path(tmp_path, "", "webp")


@pytest.mark.asyncio
async def test_download_thumb_writes_file(tmp_path):
    client = FakeClient()
    doc = SimpleNamespace(id=98765)
    filename = await thumbnail.download_thumb(client, doc, tmp_path)
    assert filename == "98765.webp"
    written = tmp_path / "98765.webp"
    assert written.exists()
    # FakeClient 写 1 字节占位
    assert len(written.read_bytes()) >= 1
    # 调用记录
    assert len(client.downloaded) == 1
    assert client.downloaded[0][0] == 98765


@pytest.mark.asyncio
async def test_download_thumb_rejects_invalid_id(tmp_path):
    client = FakeClient()
    doc = SimpleNamespace(id="abc")  # 非数字
    with pytest.raises(ValueError):
        await thumbnail.download_thumb(client, doc, tmp_path)


@pytest.mark.asyncio
async def test_download_thumb_missing_id(tmp_path):
    client = FakeClient()
    doc = SimpleNamespace()
    with pytest.raises(ValueError):
        await thumbnail.download_thumb(client, doc, tmp_path)
