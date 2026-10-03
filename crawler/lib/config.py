"""集中读取 .env 配置。

启动期一次性读 + 校验，调用方拿到的是冻结的 dataclass 不会运行时变。
"""

from __future__ import annotations

import logging
import os
from dataclasses import dataclass
from pathlib import Path

from dotenv import load_dotenv


@dataclass(frozen=True)
class Config:
    tg_api_id: int
    tg_api_hash: str
    tg_phone: str
    tg_session_name: str
    session_dir: Path
    db_dsn: str
    signals_dir: Path
    thumbnail_dir: Path
    media_dir: Path
    log_level: str
    download_originals: bool

    @property
    def session_path(self) -> Path:
        return self.session_dir / self.tg_session_name


def load_config(env_file: str | os.PathLike | None = None, *, require_tg: bool = True) -> Config:
    """读取 .env 文件，返回 Config。

    require_tg=False 时允许 TG_API_* 为空（dry-run / 单测场景）。
    """
    if env_file is not None:
        load_dotenv(env_file, override=False)
    else:
        load_dotenv(override=False)

    api_id_raw = os.getenv("TG_API_ID", "").strip()
    api_hash = os.getenv("TG_API_HASH", "").strip()
    phone = os.getenv("TG_PHONE", "").strip()

    if require_tg:
        if not api_id_raw or not api_hash:
            raise RuntimeError(
                "TG_API_ID / TG_API_HASH 未配置；请填 .env 或加 --dry-run 跳过登录"
            )
        try:
            api_id = int(api_id_raw)
        except ValueError as exc:
            raise RuntimeError(f"TG_API_ID 必须是整数，当前: {api_id_raw!r}") from exc
    else:
        api_id = int(api_id_raw) if api_id_raw.isdigit() else 0

    cfg = Config(
        tg_api_id=api_id,
        tg_api_hash=api_hash,
        tg_phone=phone,
        tg_session_name=os.getenv("TG_SESSION_NAME", "tg-emoji-hub"),
        session_dir=Path(os.getenv("SESSION_DIR", "./.session")).resolve(),
        db_dsn=os.getenv("DB_DSN", ""),
        signals_dir=Path(os.getenv("CRAWLER_SIGNALS_DIR", "./var/signals")).resolve(),
        thumbnail_dir=Path(os.getenv("THUMBNAIL_DIR", "./var/thumbnails")).resolve(),
        media_dir=Path(os.getenv("MEDIA_DIR", "./web/public/emoji")).resolve(),
        log_level=os.getenv("LOG_LEVEL", "INFO").upper(),
        download_originals=os.getenv("DOWNLOAD_ORIGINALS", "1").strip() not in ("0", "false", "False", ""),
    )

    cfg.session_dir.mkdir(parents=True, exist_ok=True)
    cfg.signals_dir.mkdir(parents=True, exist_ok=True)
    cfg.thumbnail_dir.mkdir(parents=True, exist_ok=True)
    cfg.media_dir.mkdir(parents=True, exist_ok=True)
    return cfg


def configure_logging(level: str = "INFO") -> None:
    """全局日志：结构化 + 时间戳；ERROR 必有 stack trace。"""
    logging.basicConfig(
        level=getattr(logging, level.upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )
