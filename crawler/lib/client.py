"""Telethon TelegramClient 初始化与首次登录。

session 文件路径由 cfg.session_path 控制，挂载 volume 持久化。
首次登录需手动跑 login_interactive() 输入验证码，之后自动复用 session。
"""

from __future__ import annotations

import asyncio
import logging
from typing import TYPE_CHECKING

from telethon import TelegramClient

if TYPE_CHECKING:
    from .config import Config

log = logging.getLogger(__name__)


async def get_client(cfg: Config) -> TelegramClient:
    """创建并连接 TelegramClient（不触发交互登录）。

    要求 .session 已存在；若不存在，调用方需先跑 login_interactive()。
    """
    if not cfg.tg_api_id or not cfg.tg_api_hash:
        raise RuntimeError("TG_API_ID / TG_API_HASH 未配置")

    client = TelegramClient(
        str(cfg.session_path),
        cfg.tg_api_id,
        cfg.tg_api_hash,
    )
    await client.connect()

    if not await client.is_user_authorized():
        await client.disconnect()
        raise RuntimeError(
            f"session 未授权: {cfg.session_path}\n"
            "请先跑: python -c \"import asyncio; from lib.client import login_interactive; "
            "from lib.config import load_config; asyncio.run(login_interactive(load_config()))\""
        )

    log.info("TelegramClient 已连接 session=%s", cfg.session_path.name)
    return client


async def login_interactive(cfg: Config) -> None:
    """首次登录：命令行输入手机号 + 验证码，生成 .session 文件。

    重复跑无害（已授权直接返回）。
    """
    if not cfg.tg_phone:
        raise RuntimeError("TG_PHONE 未配置，首次登录需要手机号")

    client = TelegramClient(
        str(cfg.session_path),
        cfg.tg_api_id,
        cfg.tg_api_hash,
    )
    await client.start(phone=cfg.tg_phone)
    log.info("登录成功，session 已保存至 %s", cfg.session_path)
    await client.disconnect()


async def disconnect_safely(client: TelegramClient | None) -> None:
    """关 client 不抛异常，用于 finally 清理。"""
    if client is None:
        return
    try:
        result = client.disconnect()
        if asyncio.iscoroutine(result):
            await result
    except Exception:  # noqa: BLE001
        log.exception("断开 TelegramClient 失败")
