#!/usr/bin/env bash
# 本地一键导出脚本（Git Bash on Windows 可用）
# 用法：在项目根目录执行
#   bash deployments/bare-metal/scripts/export-local.sh
#
# 可选环境变量：
#   LOCAL_MYSQL_USER  默认 root
#   LOCAL_MYSQL_PASS  默认 root
#   LOCAL_MYSQL_DB    默认 tg_emoji_hub
#   LOCAL_MYSQL_HOST  默认 127.0.0.1
#   LOCAL_MYSQL_PORT  默认 3306

set -euo pipefail

# ---------- 配置 ----------
MYSQL_USER="${LOCAL_MYSQL_USER:-root}"
MYSQL_PASS="${LOCAL_MYSQL_PASS:-root}"
MYSQL_DB="${LOCAL_MYSQL_DB:-tg_emoji_hub}"
MYSQL_HOST="${LOCAL_MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${LOCAL_MYSQL_PORT:-3306}"

OUT_DIR="deployments/bare-metal/out"
# --------------------------

echo "=== tg-emoji-hub 本地导出 ==="
echo "输出目录: $OUT_DIR"
echo ""

# 确认在项目根目录
if [[ ! -f "go.mod" ]]; then
  echo "错误：请在项目根目录执行本脚本（当前目录没有 go.mod）"
  exit 1
fi

mkdir -p "$OUT_DIR"

# ---------- Step 1: mysqldump ----------
echo "[1/5] 导出 MySQL 数据库..."
if ! command -v mysqldump &>/dev/null; then
  echo "错误：找不到 mysqldump，请确认 MySQL 客户端已安装且在 PATH 中"
  exit 1
fi

mysqldump \
  -h "$MYSQL_HOST" \
  -P "$MYSQL_PORT" \
  -u "$MYSQL_USER" \
  -p"${MYSQL_PASS}" \
  --default-character-set=utf8mb4 \
  --single-transaction \
  --no-tablespaces \
  --routines \
  --triggers \
  "$MYSQL_DB" \
  > "$OUT_DIR/dump.sql"

DUMP_INSERTS=$(grep -c '^INSERT' "$OUT_DIR/dump.sql" 2>/dev/null || echo "0")
echo "    完成：dump.sql（INSERT 语句 ${DUMP_INSERTS} 条）"

# ---------- Step 2: 打包媒体文件 ----------
echo "[2/5] 打包媒体文件（emoji + thumbs，约 550MB，稍等）..."

# 检查媒体目录
if [[ ! -d "web/public/emoji" ]]; then
  echo "警告：web/public/emoji 目录不存在，跳过 emoji 打包"
  EMOJI_COUNT=0
else
  EMOJI_COUNT=$(ls web/public/emoji/ 2>/dev/null | wc -l)
fi

if [[ ! -d "web/public/thumbs" ]]; then
  echo "警告：web/public/thumbs 目录不存在，跳过 thumbs 打包"
  THUMBS_COUNT=0
else
  THUMBS_COUNT=$(ls web/public/thumbs/ 2>/dev/null | wc -l)
fi

if [[ "$EMOJI_COUNT" -eq 0 && "$THUMBS_COUNT" -eq 0 ]]; then
  echo "错误：emoji 和 thumbs 目录均为空，没有媒体文件可打包"
  exit 1
fi

tar czf "$OUT_DIR/media.tar.gz" \
  ${EMOJI_COUNT:+web/public/emoji/} \
  ${THUMBS_COUNT:+web/public/thumbs/}

echo "    完成：media.tar.gz（emoji: ${EMOJI_COUNT} 文件，thumbs: ${THUMBS_COUNT} 文件）"

# ---------- Step 3: 交叉编译 Linux binary ----------
echo "[3/5] 交叉编译 Linux amd64 binary..."
if ! command -v go &>/dev/null; then
  echo "错误：找不到 go 命令，请确认 Go 已安装"
  exit 1
fi

# modernc.org/sqlite 是纯 Go，CGO_ENABLED=0 没问题
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-s -w" \
  -o "$OUT_DIR/server" \
  ./cmd/server/main.go

# Windows 下 file 命令可能没有，跳过 ELF 检查
if command -v file &>/dev/null; then
  FILE_TYPE=$(file "$OUT_DIR/server")
  echo "    完成：server（${FILE_TYPE##*:}）"
else
  echo "    完成：server（ELF binary，Windows 下无法用 file 命令验证）"
fi

# ---------- Step 4: 前端构建 ----------
echo "[4/5] 前端构建..."
if [[ ! -d "web" ]]; then
  echo "错误：web 目录不存在"
  exit 1
fi

if ! command -v npm &>/dev/null; then
  echo "错误：找不到 npm 命令"
  exit 1
fi

(
  cd web
  # npm ci 用 lockfile 精确复现，不会悄悄升级依赖
  npm ci --prefer-offline 2>/dev/null || npm ci
  npm run build
)

tar czf "$OUT_DIR/web-dist.tar.gz" \
  -C web \
  dist/

DIST_FILES=$(tar tzf "$OUT_DIR/web-dist.tar.gz" | wc -l)
echo "    完成：web-dist.tar.gz（${DIST_FILES} 文件）"

# ---------- Step 5: 汇总 ----------
echo "[5/5] 产物清单："
echo ""
echo "  文件                        大小"
echo "  --------------------------  --------"
for f in dump.sql media.tar.gz server web-dist.tar.gz; do
  if [[ -f "$OUT_DIR/$f" ]]; then
    # du -h 在 Git Bash 可用
    SIZE=$(du -h "$OUT_DIR/$f" | cut -f1)
    printf "  %-28s  %s\n" "$f" "$SIZE"
  fi
done
echo ""
echo "下一步：scp 上传到服务器，然后跑 import-server.sh"
echo ""
echo "  SERVER=\"user@your-server-ip\""
echo "  scp $OUT_DIR/* \$SERVER:/tmp/tg-emoji-hub-deploy/"
echo "  scp deployments/bare-metal/scripts/import-server.sh \$SERVER:/tmp/tg-emoji-hub-deploy/"
