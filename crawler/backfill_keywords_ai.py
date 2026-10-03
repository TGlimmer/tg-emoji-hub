"""给 emoji 缩略图打中英文标签，写回 emoji_stickers.keywords。

支持多 provider：
- gemini  Google Gemini 2.0 Flash（默认，风控宽松、免费 1500 req/天）
- zhipu   智谱 GLM-4.6V-FlashX（备选，风控严易跳过）

用法:
  python backfill_keywords_ai.py                       # 用 .env 的 AI_PROVIDER（默认 gemini）
  python backfill_keywords_ai.py --provider zhipu      # 临时切到智谱
  python backfill_keywords_ai.py --all                 # 全量重补
  python backfill_keywords_ai.py --limit 50            # 仅前 N 张（验证质量用）
  python backfill_keywords_ai.py --concurrency 3       # 并发上限（默认 3）

依赖:
  pip install google-genai zai-sdk pillow

环境变量（沿用 crawler/.env）:
  AI_PROVIDER       gemini | zhipu （默认 gemini）
  GEMINI_API_KEY    Gemini API Key，https://aistudio.google.com/apikey
  ZAI_API_KEY       智谱 API Key，https://bigmodel.cn/usercenter/apikeys
  DB_DSN            crawler 同款数据库
  THUMBNAIL_DIR     缩略图本地目录
"""

from __future__ import annotations

import argparse
import asyncio
import base64
import json
import logging
import os
import sys
from pathlib import Path
from typing import Protocol

from dotenv import load_dotenv
from sqlalchemy import not_, select, update

sys.path.insert(0, str(Path(__file__).resolve().parent))

from lib import db as db_mod  # noqa: E402
from lib.config import configure_logging, load_config  # noqa: E402

log = logging.getLogger("backfill")

# 智谱/Gemini 内容审查命中后写入的 placeholder keyword
# 写库后下次跑 SQL 过滤会跳过（含连续 ASCII 3+ 字母）
AI_FILTERED_MARKER = "ai_filtered"

PROMPT = """这是一个 Telegram 自定义表情包里的单个 emoji 图标。

参考信息（仅供消歧，不一定等于画面，画面为准）：
- 作者标注的对应标准 emoji：{alt}
- 所属表情包：{pack_title}

任务：看图识别主体/情绪/动作/物体/场景/风格，生成搜索标签 + 分类。

只输出 JSON（无 markdown / 无代码块 / 无多余文字）：
{{"zh":["..."],"en":["..."],"category":"..."}}

规则：
1. 以画面为准。{alt} 只当参考帮助消歧；若 {alt} 与画面明显不符，以画面为准。
2. zh / en 各 3~8 个，数量尽量一致，去重，不写句子。中文每个 ≤4 字，英文每个 ≤2 词。
3. 覆盖维度：主体（猫/狗/火/爱心）、情绪（开心/愤怒/惊讶/委屈/害羞）、风格（可爱/动画/像素/霓虹/手绘/复古/抽象）。
   动作（笑/哭/挥手/点赞）仅在画面能确认时写，看不出就不写，禁止编造。
4. 特殊识别（优先具体名称，作为 zh/en 标签）：
   - 加密货币 Logo → 具体币种：TON / TRX(波场) / BTC(比特币) / ETH(以太坊) / SOL / USDT / BNB / DOGE / XRP
   - 国旗 → 国家名
   - 食物 / 动物 / 交通工具 / 武器 / 建筑 → 具体名称
   - 数字 / 字母 / 符号 / 手势 → 作为标签
5. category 必须从这 11 个里单选一个：
   emotion / animal / person / food / crypto / object / symbol / gesture / meme / vehicle / nature
   判定：表情情绪为主→emotion；动物→animal；人物→person；食物→food；加密货币 Logo→crypto；
   手势→gesture；数字/字母/符号→symbol；网络梗图→meme；交通工具→vehicle；自然/天气/植物→nature；其它物品→object。
6. 禁止：品牌名 / 作者名 / 拼音 / 主观评价 / 无根据猜测。

示例：
{{"zh":["金币","上升","收益","卡通"],"en":["coin","growth","profit","cartoon"],"category":"crypto"}}
"""


class ContentFilterError(Exception):
    """内容审查命中（图片本身被风控判敏感），不应重试"""


# ---------- 通用辅助 ----------

def encode_image_base64(path: Path) -> str:
    return base64.b64encode(path.read_bytes()).decode("utf-8")


def webp_to_png_bytes(path: Path) -> bytes:
    """webp/gif → PNG，兜底用。Pillow 读 webp/gif 都行。"""
    from io import BytesIO

    from PIL import Image

    with Image.open(path) as im:
        # 动画 gif/webp 取首帧（emoji 缩略图本身就是静态图）
        if getattr(im, "is_animated", False):
            im.seek(0)
        buf = BytesIO()
        im.convert("RGBA").save(buf, format="PNG")
        return buf.getvalue()


def _strip_markdown_fence(s: str) -> str:
    """模型偶尔会用 ```json ... ``` 包裹，剥掉。"""
    s = s.strip()
    if s.startswith("```"):
        s = s.strip("`")
        if s.lower().startswith("json"):
            s = s[4:]
        s = s.strip()
    return s


# 11 值白名单（与 entity.NormalizeCategory 保持一致）
_VALID_CATEGORIES = frozenset({
    "emotion", "animal", "person", "food", "crypto",
    "object", "symbol", "gesture", "meme", "vehicle", "nature",
})


def parse_keywords_json(content: str) -> dict | None:
    """解析模型返回的 JSON，失败返 None。"""
    content = _strip_markdown_fence(content)
    try:
        data = json.loads(content)
    except json.JSONDecodeError:
        return None
    zh = [str(x).strip() for x in data.get("zh", []) if str(x).strip()]
    en = [str(x).strip() for x in data.get("en", []) if str(x).strip()]
    raw_cat = str(data.get("category", "")).strip().lower()
    category = raw_cat if raw_cat in _VALID_CATEGORIES else ""
    return {"zh": zh, "en": en, "category": category}


_MIME_MAP = {
    "webp": "image/webp",
    "png": "image/png",
    "gif": "image/gif",
    "jpg": "image/jpeg",
    "jpeg": "image/jpeg",
}


def _detect_mime(path: Path) -> str | None:
    return _MIME_MAP.get(path.suffix.lstrip(".").lower())


# ---------- Provider 抽象 ----------

class AIProvider(Protocol):
    """图像 → 关键词+分类的 provider。

    call(image_path, alt, pack_title) 返回 {"zh": [...], "en": [...], "category": "..."} 或 None（解析失败）。
    遇到内容审查命中时抛 ContentFilterError，调用方写 placeholder 跳过。
    """

    name: str

    def call(self, image_path: Path, alt: str, pack_title: str) -> dict | None: ...


class GeminiProvider:
    """Google Gemini Flash。风控宽松（safety_settings 全 BLOCK_NONE），免费 1500 req/天。

    默认 gemini-3.5-flash —— 实测对 emoji 图标打标质量 > 2.5-flash > 3-pro-preview
    （pro 慢 50% 但 tag 质量没胜出）。
    用 GEMINI_MODEL 环境变量覆盖。
    """

    name = "gemini"
    DEFAULT_MODEL = "gemini-3.5-flash"

    def __init__(self, api_key: str, model: str | None = None):
        from google import genai  # type: ignore[import-not-found]
        from google.genai import types  # type: ignore[import-not-found]

        self._client = genai.Client(api_key=api_key)
        self._types = types
        self._model = model or os.getenv("GEMINI_MODEL") or self.DEFAULT_MODEL
        log.info("Gemini provider 使用模型: %s", self._model)
        # emoji 图标全部无害，关掉所有 safety filter
        self._safety = [
            types.SafetySetting(category=c, threshold="BLOCK_NONE")
            for c in (
                "HARM_CATEGORY_HATE_SPEECH",
                "HARM_CATEGORY_DANGEROUS_CONTENT",
                "HARM_CATEGORY_SEXUALLY_EXPLICIT",
                "HARM_CATEGORY_HARASSMENT",
            )
        ]

    def _call_once(self, image_bytes: bytes, mime: str, prompt: str) -> str | None:
        types = self._types
        try:
            resp = self._client.models.generate_content(
                model=self._model,
                contents=[
                    types.Part.from_bytes(data=image_bytes, mime_type=mime),
                    prompt,
                ],
                config=types.GenerateContentConfig(
                    temperature=0.3,
                    safety_settings=self._safety,
                ),
            )
        except Exception as exc:  # noqa: BLE001
            msg = str(exc).lower()
            # Gemini 服务端拒绝（极少触发，因为 safety 已关）
            if "blocked" in msg or "safety" in msg or "prohibited" in msg:
                raise ContentFilterError(str(exc)) from exc
            log.warning("Gemini 调用异常: %s", exc)
            return None

        # 即便 BLOCK_NONE，仍可能因 prompt_feedback.block_reason 被服务端阻断
        feedback = getattr(resp, "prompt_feedback", None)
        if feedback is not None and getattr(feedback, "block_reason", None):
            raise ContentFilterError(f"prompt blocked: {feedback.block_reason}")

        candidates = getattr(resp, "candidates", None) or []
        if candidates:
            finish_reason = getattr(candidates[0], "finish_reason", None)
            # SAFETY / RECITATION / PROHIBITED_CONTENT 等命中风控
            if finish_reason is not None:
                reason_str = str(finish_reason).upper()
                if "SAFETY" in reason_str or "PROHIBITED" in reason_str or "RECITATION" in reason_str:
                    raise ContentFilterError(f"finish_reason: {finish_reason}")

        # response.text 是个 helper，可能 None
        text = getattr(resp, "text", None)
        return text or ""

    def call(self, image_path: Path, alt: str, pack_title: str) -> dict | None:
        mime = _detect_mime(image_path)
        if mime is None:
            log.warning("未知图像格式 %s，跳过", image_path.name)
            return None

        prompt = PROMPT.format(alt=alt or "无", pack_title=pack_title or "无")
        image_bytes = image_path.read_bytes()
        content = self._call_once(image_bytes, mime, prompt)
        if content:
            parsed = parse_keywords_json(content)
            if parsed is not None:
                return parsed
            log.info("JSON 解析失败 %s，尝试转 PNG 重试", image_path.name)
        else:
            log.info("原格式调用失败 %s，尝试转 PNG 重试", image_path.name)

        # 兜底：转 PNG 重试一次
        try:
            png_bytes = webp_to_png_bytes(image_path)
        except Exception:  # noqa: BLE001
            log.exception("转 PNG 失败 %s", image_path.name)
            return None
        content = self._call_once(png_bytes, "image/png", prompt)
        if not content:
            return None
        parsed = parse_keywords_json(content)
        if parsed is None:
            log.warning("PNG 重试仍解析失败 %s content=%r", image_path.name, content[:200])
        return parsed


class ZhipuProvider:
    """智谱 GLM-4.6V-FlashX。风控严，1301 命中较多。"""

    name = "zhipu"
    MODEL_ID = "GLM-4.6V-FlashX"

    def __init__(self, api_key: str):
        from zai import ZhipuAiClient  # type: ignore[import-not-found]

        self._client = ZhipuAiClient(api_key=api_key)

    def _call_with_data_url(self, data_url: str, prompt: str) -> str | None:
        try:
            resp = self._client.chat.completions.create(
                model=self.MODEL_ID,
                messages=[{
                    "role": "user",
                    "content": [
                        {"type": "text", "text": prompt},
                        {"type": "image_url", "image_url": {"url": data_url}},
                    ],
                }],
                temperature=0.3,
            )
        except Exception as exc:  # noqa: BLE001
            msg = str(exc)
            if "1301" in msg or "contentFilter" in msg:
                raise ContentFilterError(msg) from exc
            log.warning("GLM 调用异常: %s", exc)
            return None
        return resp.choices[0].message.content or ""

    def call(self, image_path: Path, alt: str, pack_title: str) -> dict | None:
        mime = _detect_mime(image_path)
        if mime is None:
            log.warning("未知图像格式 %s，跳过", image_path.name)
            return None

        prompt = PROMPT.format(alt=alt or "无", pack_title=pack_title or "无")
        b64 = encode_image_base64(image_path)
        content = self._call_with_data_url(f"data:{mime};base64,{b64}", prompt)
        if content:
            parsed = parse_keywords_json(content)
            if parsed is not None:
                return parsed
            log.info("JSON 解析失败 %s，尝试转 PNG 重试", image_path.name)
        else:
            log.info("原格式调用失败 %s，尝试转 PNG 重试", image_path.name)

        try:
            png_bytes = webp_to_png_bytes(image_path)
        except Exception:  # noqa: BLE001
            log.exception("转 PNG 失败 %s", image_path.name)
            return None
        png_b64 = base64.b64encode(png_bytes).decode("utf-8")
        content = self._call_with_data_url(f"data:image/png;base64,{png_b64}", prompt)
        if not content:
            return None
        parsed = parse_keywords_json(content)
        if parsed is None:
            log.warning("PNG 重试仍解析失败 %s content=%r", image_path.name, content[:200])
        return parsed


def build_provider(name: str) -> AIProvider:
    """根据名字实例化 provider。缺 key 时直接退出。"""
    name = name.lower().strip()
    if name == "gemini":
        api_key = os.getenv("GEMINI_API_KEY")
        if not api_key:
            sys.stderr.write(
                "缺 GEMINI_API_KEY 环境变量。去 https://aistudio.google.com/apikey "
                "拿 key 填入 crawler/.env（或用 --provider zhipu 切到智谱）\n"
            )
            sys.exit(1)
        try:
            return GeminiProvider(api_key)
        except ImportError:
            sys.stderr.write("缺 google-genai。先装：pip install google-genai\n")
            sys.exit(1)

    if name == "zhipu":
        api_key = os.getenv("ZAI_API_KEY") or os.getenv("ZHIPU_API_KEY")
        if not api_key:
            sys.stderr.write(
                "缺 ZAI_API_KEY 环境变量。去 https://bigmodel.cn/usercenter/apikeys 拿 key 填入 crawler/.env\n"
            )
            sys.exit(1)
        try:
            return ZhipuProvider(api_key)
        except ImportError:
            sys.stderr.write("缺 zai-sdk。先装：pip install zai-sdk\n")
            sys.exit(1)

    sys.stderr.write(f"未知 provider: {name}（支持 gemini / zhipu）\n")
    sys.exit(1)


# ---------- DB 操作 + 调度 ----------

def _find_image(thumbnail_dir: Path, custom_emoji_id: str) -> Path | None:
    for ext in ("webp", "png", "gif"):
        p = thumbnail_dir / f"{custom_emoji_id}.{ext}"
        if p.exists():
            return p
    return None


def _merge_keywords(alt: str, existing: str, ai_result: dict[str, list[str]]) -> str:
    parts: set[str] = set()
    if alt:
        parts.add(alt)
    if existing:
        for kw in existing.split(","):
            kw = kw.strip()
            if kw and kw != "?":
                parts.add(kw)
    parts.update(ai_result["zh"])
    parts.update(ai_result["en"])
    return ",".join(sorted(parts))


def _write_filtered_placeholder(session_factory, custom_emoji_id, alt, existing) -> None:
    """风控命中：写 placeholder keywords（含 ai_filtered marker），下次跳过"""
    parts: set[str] = set()
    if alt:
        parts.add(alt)
    if existing:
        for kw in existing.split(","):
            kw = kw.strip()
            if kw and kw != "?":
                parts.add(kw)
    parts.add(AI_FILTERED_MARKER)
    new_keywords = ",".join(sorted(parts))
    with session_factory() as s:
        s.execute(
            update(db_mod.EmojiSticker)
            .where(db_mod.EmojiSticker.custom_emoji_id == custom_emoji_id)
            .values(keywords=new_keywords)
        )
        s.commit()


async def process_one(provider: AIProvider, session_factory, thumbnail_dir, row, sem) -> bool:
    """处理一条 emoji。返回是否成功更新。"""
    async with sem:
        custom_emoji_id, alt, existing_keywords, pack_title = row
        img = _find_image(thumbnail_dir, custom_emoji_id)
        if img is None:
            log.warning("缩略图缺失 %s（先跑 crawler full 下载图）", custom_emoji_id)
            return False

        loop = asyncio.get_event_loop()
        try:
            ai_result = await loop.run_in_executor(
                None, provider.call, img, alt or "", pack_title or ""
            )
        except ContentFilterError:
            log.warning("⊘ %s 被 %s 内容审查拒绝，写 placeholder 跳过", custom_emoji_id, provider.name)
            _write_filtered_placeholder(session_factory, custom_emoji_id, alt, existing_keywords)
            return False
        if ai_result is None:
            return False

        new_keywords = _merge_keywords(alt or "", existing_keywords or "", ai_result)
        with session_factory() as s:
            s.execute(
                update(db_mod.EmojiSticker)
                .where(db_mod.EmojiSticker.custom_emoji_id == custom_emoji_id)
                .values(keywords=new_keywords, category=ai_result["category"])
            )
            s.commit()
        log.info("✓ %s alt=%s tags=%d category=%s", custom_emoji_id, alt,
                 len(new_keywords.split(",")), ai_result["category"] or "(未识别)")
        return True


def _select_targets(session_factory, *, all_mode: bool, limit: int) -> list[tuple]:
    """选出要补全的行：默认跳过已 AI 补完的；--all 时全选。

    判定"已补全"：keywords 含连续 ≥3 个英文字母（如 fire/cute/cat）。
    MySQL 5.7 REGEXP 是字节匹配，[一-龥] 这种 unicode range 会把所有 emoji
    都误判成"含中文"，所以反过来看英文 token —— emoji 字符必不含字母。

    返回四元组 (custom_emoji_id, alt, keywords, pack_title)。
    """
    with session_factory() as s:
        q = (
            select(
                db_mod.EmojiSticker.custom_emoji_id,
                db_mod.EmojiSticker.alt,
                db_mod.EmojiSticker.keywords,
                db_mod.StickerPack.title,
            )
            .join(db_mod.StickerPack, db_mod.EmojiSticker.pack_id == db_mod.StickerPack.id)
        )
        if not all_mode:
            q = q.where(
                not_(db_mod.EmojiSticker.keywords.op("REGEXP")(r"[a-zA-Z]{3,}"))
            )
        if limit > 0:
            q = q.limit(limit)
        return list(s.execute(q))


async def main_async(args, cfg) -> int:
    provider_name = args.provider or os.getenv("AI_PROVIDER", "gemini")
    provider = build_provider(provider_name)
    log.info("使用 AI provider: %s", provider.name)

    engine = db_mod.make_engine(cfg.db_dsn)
    session_factory = db_mod.make_session_factory(engine)

    rows = _select_targets(session_factory, all_mode=args.all, limit=args.limit)
    if not rows:
        log.info("没有需要处理的 emoji（增量模式下，可能都已补全；用 --all 强制重补）")
        return 0

    log.info("准备处理 %d 张 emoji，并发=%d", len(rows), args.concurrency)
    sem = asyncio.Semaphore(args.concurrency)

    tasks = [
        process_one(provider, session_factory, cfg.thumbnail_dir, r, sem)
        for r in rows
    ]
    results = await asyncio.gather(*tasks, return_exceptions=False)
    succeeded = sum(1 for r in results if r)
    log.info("完成 %d/%d 成功", succeeded, len(rows))
    return 0


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(prog="backfill_keywords_ai")
    p.add_argument("--provider", choices=["gemini", "zhipu"], default=None,
                   help="覆盖 .env AI_PROVIDER（默认 gemini）")
    p.add_argument("--all", action="store_true",
                   help="全量重补，否则跳过已补全的 emoji")
    p.add_argument("--limit", type=int, default=0,
                   help="只处理前 N 张（验证质量用）")
    p.add_argument("--concurrency", type=int, default=3,
                   help="并发上限，默认 3（智谱限流较紧时调小到 1-2）")
    args = p.parse_args(argv)

    load_dotenv()
    cfg = load_config(require_tg=False)
    configure_logging(cfg.log_level)

    # HTTPS 代理（Gemini 在国内/部分地区直连 API 会被拒，需走代理）
    # httpx（google-genai 内部用）自动读 HTTPS_PROXY/HTTP_PROXY 环境变量
    proxy = os.getenv("HTTPS_PROXY") or os.getenv("https_proxy")
    if proxy:
        log.info("HTTPS 代理已启用: %s", proxy)
        # 同步同名小写变量，确保部分库能读到
        os.environ.setdefault("HTTPS_PROXY", proxy)
        os.environ.setdefault("https_proxy", proxy)
    elif os.getenv("AI_PROVIDER", "gemini").lower() == "gemini":
        log.info("未配置 HTTPS_PROXY，Gemini 直连（如 'User location is not supported' 报错请在 .env 加 HTTPS_PROXY=http://proxy:port）")

    return asyncio.run(main_async(args, cfg))


if __name__ == "__main__":
    sys.exit(main())
