"""单包爬取 CLI：python single_pack.py <short_name> [--dry-run]"""

from __future__ import annotations

import argparse
import asyncio
import logging
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from lib import client as client_mod  # noqa: E402
from lib import db as db_mod  # noqa: E402
from lib import mocks, pipeline  # noqa: E402
from lib.config import configure_logging, load_config  # noqa: E402

log = logging.getLogger("single_pack")


async def _run(short_name: str, *, dry_run: bool, no_originals: bool) -> int:
    cfg = load_config(require_tg=not dry_run)
    configure_logging(cfg.log_level)
    do_originals = (not no_originals) and cfg.download_originals

    if dry_run:
        engine = db_mod.make_engine("sqlite:///:memory:")
        db_mod.create_all(engine)
        session_factory = db_mod.make_session_factory(engine)
        fake = mocks.FakeClient()
        r = await pipeline.crawl_single_pack(
            fake, short_name, session_factory, cfg.thumbnail_dir,
            download_originals=do_originals, media_dir=cfg.media_dir,
        )
        log.info("dry-run 单包结果 %s", r)
        return 0

    engine = db_mod.make_engine(cfg.db_dsn)
    session_factory = db_mod.make_session_factory(engine)
    client = await client_mod.get_client(cfg)
    try:
        r = await pipeline.crawl_single_pack(
            client, short_name, session_factory, cfg.thumbnail_dir,
            download_originals=do_originals, media_dir=cfg.media_dir,
        )
        log.info("单包爬取结果 %s", r)
        return 0
    finally:
        await client_mod.disconnect_safely(client)


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(prog="single_pack")
    p.add_argument("short_name", help="TG 包短名，如 AnimatedEmojies")
    p.add_argument("--dry-run", action="store_true")
    p.add_argument("--no-originals", action="store_true",
                   help="跳过原文件下载")
    args = p.parse_args(argv)
    return asyncio.run(_run(args.short_name, dry_run=args.dry_run,
                             no_originals=args.no_originals))


if __name__ == "__main__":
    sys.exit(main())
