"""signals.py 单测：写信号 + 扫描 + 失败重试。"""

import json

import pytest

from lib import signals


def test_write_signal_creates_pending_file(tmp_path):
    p = signals.write_signal(tmp_path, "MyPack")
    assert p.exists()
    assert p.name == "MyPack.pending"
    data = json.loads(p.read_text(encoding="utf-8"))
    assert data["retry_count"] == 0
    assert "created_at" in data


def test_write_signal_rejects_invalid_short_name(tmp_path):
    for bad in ["", "1Pack", "Pack!", "../etc/passwd",
                "x" * 65, "Pack Name"]:
        with pytest.raises(ValueError):
            signals.write_signal(tmp_path, bad)


@pytest.mark.asyncio
async def test_scan_once_processes_and_deletes(tmp_path):
    signals.write_signal(tmp_path, "AnimatedEmojies")
    signals.write_signal(tmp_path, "AnotherPack")

    processed: list[str] = []

    async def proc(short_name: str) -> None:
        processed.append(short_name)

    n = await signals.scan_once(tmp_path, proc)
    assert n == 2
    assert sorted(processed) == ["AnimatedEmojies", "AnotherPack"]
    # 处理成功后文件应被删除
    assert list(tmp_path.glob("*.pending")) == []


@pytest.mark.asyncio
async def test_scan_once_retries_on_failure(tmp_path):
    signals.write_signal(tmp_path, "FailingPack")

    async def proc_fail(short_name: str) -> None:
        raise RuntimeError("boom")

    # 第一次失败 → retry_count 应变 1，文件仍在
    await signals.scan_once(tmp_path, proc_fail)
    files = list(tmp_path.glob("*.pending"))
    assert len(files) == 1
    data = json.loads(files[0].read_text(encoding="utf-8"))
    assert data["retry_count"] == 1


@pytest.mark.asyncio
async def test_scan_once_marks_failed_after_max_retries(tmp_path):
    p = tmp_path / "X.pending"
    p.write_text(json.dumps({"created_at": 1, "retry_count": signals.MAX_RETRY - 1}),
                 encoding="utf-8")

    async def proc_fail(short_name: str) -> None:
        raise RuntimeError("boom")

    await signals.scan_once(tmp_path, proc_fail)
    assert not p.exists()
    failed = tmp_path / "X.failed"
    assert failed.exists()


@pytest.mark.asyncio
async def test_scan_once_skips_illegal_filename(tmp_path):
    # 直接写一个非法 short_name 的文件（绕过 write_signal 校验）
    (tmp_path / "1bad.pending").write_text("{}", encoding="utf-8")

    async def proc(short_name: str) -> None:
        raise AssertionError("不该被调用")

    n = await signals.scan_once(tmp_path, proc)
    assert n == 1  # 计入扫描但不处理
    # 非法文件保留（人工排查）
    assert (tmp_path / "1bad.pending").exists()


@pytest.mark.asyncio
async def test_scan_once_handles_empty_dir(tmp_path):
    n = await signals.scan_once(tmp_path, lambda s: None)  # type: ignore[arg-type]
    assert n == 0
