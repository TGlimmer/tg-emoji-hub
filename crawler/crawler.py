"""crawler 主入口。

CLI:
  python crawler.py full              # 全量爬 Featured + DB.manual 包
  python crawler.py full --dry-run    # 用 FakeClient 跑完整 pipeline 不联网（用 sqlite in-memory）
  python crawler.py watch             # 后台监听 signals/ 目录
"""

from __future__ import annotations

import argparse
import asyncio
import logging
import sys
from pathlib import Path

# 让 `python crawler.py` 也能 import lib
sys.path.insert(0, str(Path(__file__).resolve().parent))

from lib import client as client_mod  # noqa: E402
from lib import db as db_mod  # noqa: E402
from lib import mocks, pipeline, signals  # noqa: E402
from lib.config import Config, configure_logging, load_config  # noqa: E402

log = logging.getLogger("crawler")


async def cmd_full(cfg: Config, *, dry_run: bool,
                   no_thumbs: bool = False, no_originals: bool = False) -> int:
    do_originals = (not no_originals) and cfg.download_originals
    if dry_run:
        log.warning("dry-run 模式：FakeClient + sqlite in-memory，不联网不入生产库")
        engine = db_mod.make_engine("sqlite:///:memory:")
        db_mod.create_all(engine)
        session_factory = db_mod.make_session_factory(engine)
        fake = mocks.FakeClient(num_featured=2)
        stats = await pipeline.run_full_crawl(
            fake, session_factory, cfg.thumbnail_dir,
            download_thumbs=not no_thumbs,
            download_originals=do_originals,
            media_dir=cfg.media_dir,
        )
        log.info("dry-run 统计 %s", stats)
        log.info("FakeClient 调用记录 %s", fake.calls)
        log.info("FakeClient 下载记录 %d 个文件 (含缩略图与原文件)", len(fake.downloaded))
        return 0

    engine = db_mod.make_engine(cfg.db_dsn)
    session_factory = db_mod.make_session_factory(engine)
    client = None
    try:
        client = await client_mod.get_client(cfg)
        stats = await pipeline.run_full_crawl(
            client, session_factory, cfg.thumbnail_dir,
            download_thumbs=not no_thumbs,
            download_originals=do_originals,
            media_dir=cfg.media_dir,
        )
        log.info("全量爬取统计 %s", stats)
        return 0
    finally:
        await client_mod.disconnect_safely(client)


async def cmd_watch(cfg: Config, *, dry_run: bool, no_originals: bool = False) -> int:
    do_originals = (not no_originals) and cfg.download_originals
    if dry_run:
        log.warning("dry-run watch：用 FakeClient，写入 sqlite in-memory")
        engine = db_mod.make_engine("sqlite:///:memory:")
        db_mod.create_all(engine)
        session_factory = db_mod.make_session_factory(engine)
        fake = mocks.FakeClient()

        async def processor(short_name: str) -> None:
            await pipeline.crawl_single_pack(
                fake, short_name, session_factory, cfg.thumbnail_dir,
                download_originals=do_originals, media_dir=cfg.media_dir,
            )

        # dry-run 模式只扫一次 + 退出，避免 CI 死循环
        n = await signals.scan_once(cfg.signals_dir, processor)
        log.info("dry-run 扫描完成，处理 %d 个信号", n)
        return 0

    engine = db_mod.make_engine(cfg.db_dsn)
    session_factory = db_mod.make_session_factory(engine)
    client = await client_mod.get_client(cfg)

    async def processor(short_name: str) -> None:
        await pipeline.crawl_single_pack(
            client, short_name, session_factory, cfg.thumbnail_dir,
            download_originals=do_originals, media_dir=cfg.media_dir,
        )

    try:
        await signals.watch_loop(cfg.signals_dir, processor)
        return 0
    finally:
        await client_mod.disconnect_safely(client)


def _build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="crawler", description="tg-emoji-hub Telegram 爬虫")
    p.add_argument("--dry-run", action="store_true",
                   help="用 FakeClient + sqlite in-memory，不联网不入库")
    sub = p.add_subparsers(dest="cmd", required=True)
    # 子 parser 重复声明 --dry-run，让 `crawler.py full --dry-run` 也能识别
    full_p = sub.add_parser("full", help="全量爬取 Featured + manual")
    full_p.add_argument("--dry-run", action="store_true",
                        help="用 FakeClient + sqlite in-memory，不联网不入库")
    full_p.add_argument("--no-thumbs", action="store_true",
                        help="跳过缩略图下载，仅入元数据（快速建库；图后续 watch 阶段补）")
    full_p.add_argument("--no-originals", action="store_true",
                        help="跳过原文件下载（webp/webm/tgs），动态 emoji 前端只能显示静态缩略图")
    watch_p = sub.add_parser("watch", help="监听 signals/ 目录")
    watch_p.add_argument("--dry-run", action="store_true",
                         help="用 FakeClient + sqlite in-memory，不联网不入库")
    watch_p.add_argument("--no-originals", action="store_true",
                         help="跳过原文件下载")
    return p


def main(argv: list[str] | None = None) -> int:
    args = _build_parser().parse_args(argv)
    cfg = load_config(require_tg=not args.dry_run)
    configure_logging(cfg.log_level)

    if args.cmd == "full":
        return asyncio.run(cmd_full(
            cfg, dry_run=args.dry_run,
            no_thumbs=getattr(args, "no_thumbs", False),
            no_originals=getattr(args, "no_originals", False),
        ))
    if args.cmd == "watch":
        return asyncio.run(cmd_watch(
            cfg, dry_run=args.dry_run,
            no_originals=getattr(args, "no_originals", False),
        ))
    return 2


if __name__ == "__main__":
    sys.exit(main())
