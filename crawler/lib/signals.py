"""signals/ 目录观察器：检测 *.pending 文件 → 触发单包爬取 → 删信号。

文件命名: signals_dir/{short_name}.pending  (内容 JSON {"created_at": ts, "retry_count": N})
失败 5 次后改名 .failed 停止重试。
"""

from __future__ import annotations

import asyncio
import json
import logging
import re
import time
from collections.abc import Awaitable, Callable
from pathlib import Path

log = logging.getLogger(__name__)

# 跟 Go 侧 internal/app/pack_usecase.go 的 shortNameRe 必须一致
_SHORT_NAME_RE = re.compile(r"^[A-Za-z][A-Za-z0-9_]{0,63}$")

MAX_RETRY = 5

# 单包处理器签名：async def(short_name: str) -> None
PackProcessor = Callable[[str], Awaitable[None]]


def _short_name_from_filename(path: Path) -> str | None:
    """从 xxx.pending 提取 short_name，校验合法性。"""
    if path.suffix != ".pending":
        return None
    name = path.stem
    if not _SHORT_NAME_RE.fullmatch(name):
        return None
    return name


def _read_retry_count(path: Path) -> int:
    """读 .pending JSON 的 retry_count；解析失败返 0。"""
    try:
        with path.open("r", encoding="utf-8") as fp:
            data = json.load(fp)
        return int(data.get("retry_count", 0))
    except (OSError, json.JSONDecodeError, ValueError):
        return 0


def write_signal(signals_dir: Path, short_name: str) -> Path:
    """写 .pending 信号文件。

    crawler 运行时不调用它；Go 侧 POST /api/v1/packs 写的文件格式必须跟这里一致。
    """
    if not _SHORT_NAME_RE.fullmatch(short_name):
        raise ValueError(f"非法 short_name: {short_name!r}")
    signals_dir.mkdir(parents=True, exist_ok=True)
    p = signals_dir / f"{short_name}.pending"
    p.write_text(
        json.dumps({"created_at": int(time.time()), "retry_count": 0}),
        encoding="utf-8",
    )
    return p


async def _process_one(path: Path, processor: PackProcessor) -> None:
    """处理一个 .pending 文件；成功删除，失败 retry_count+1 或改名 .failed。"""
    short_name = _short_name_from_filename(path)
    if short_name is None:
        log.warning("跳过非法信号文件: %s", path.name)
        return

    retry = _read_retry_count(path)
    try:
        await processor(short_name)
    except Exception:  # noqa: BLE001
        log.exception("处理信号失败 short_name=%s retry=%d", short_name, retry)
        retry += 1
        if retry >= MAX_RETRY:
            failed = path.with_suffix(".failed")
            try:
                path.rename(failed)
                log.error("超出重试上限 %d，已标记为 %s", MAX_RETRY, failed.name)
            except OSError:
                log.exception("改名为 .failed 失败")
            return
        # 更新 retry_count 继续等下次扫描
        try:
            path.write_text(
                json.dumps({"created_at": int(time.time()), "retry_count": retry}),
                encoding="utf-8",
            )
        except OSError:
            log.exception("更新 retry_count 失败")
        return

    # 成功 → 删信号
    try:
        path.unlink()
        log.info("信号处理完成 short_name=%s", short_name)
    except OSError:
        log.exception("删除信号文件失败 %s", path)


async def scan_once(signals_dir: Path, processor: PackProcessor) -> int:
    """扫描一次 signals_dir 下的存量 .pending，返回处理数量。"""
    if not signals_dir.exists():
        return 0
    pending_files = sorted(signals_dir.glob("*.pending"))
    for f in pending_files:
        await _process_one(f, processor)
    return len(pending_files)


async def watch_loop(
    signals_dir: Path,
    processor: PackProcessor,
    *,
    poll_interval_sec: float = 2.0,
    stop_event: asyncio.Event | None = None,
) -> None:
    """轮询监听模式。

    没用 watchdog inotify 是因为：
    1. 跨平台一致（Windows 测试 + Linux 生产都用同一份代码）
    2. 信号文件出现到处理延迟 ≤ poll_interval_sec 足够（业务可接受 2s 延迟）
    3. 简化故障排查，watchdog 在容器内 inotify 表现飘忽
    """
    signals_dir.mkdir(parents=True, exist_ok=True)
    log.info("启动信号监听 dir=%s interval=%.1fs", signals_dir, poll_interval_sec)
    while True:
        if stop_event is not None and stop_event.is_set():
            log.info("收到停止信号，watcher 退出")
            return
        try:
            await scan_once(signals_dir, processor)
        except Exception:  # noqa: BLE001
            log.exception("扫描信号目录异常，2s 后重试")
        try:
            if stop_event is not None:
                await asyncio.wait_for(stop_event.wait(), timeout=poll_interval_sec)
            else:
                await asyncio.sleep(poll_interval_sec)
        except TimeoutError:
            continue
