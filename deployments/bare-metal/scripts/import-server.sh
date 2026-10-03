#!/usr/bin/env bash
# 服务器侧导入脚本（Linux）
# 用法：
#   SERVER_MYSQL_USER=tgemoji \
#   SERVER_MYSQL_PASS=your-password \
#   SERVER_MYSQL_DB=tg_emoji_hub \
#     bash import-server.sh \
#       /path/to/dump.sql \
#       /path/to/media.tar.gz \
#       /path/to/web-dist.tar.gz \
#       /path/to/server
#
# 可选环境变量：
#   INSTALL_DIR    默认 /opt/tg-emoji-hub
#   SERVICE_USER   默认 tg-emoji（systemd 运行用户）

set -euo pipefail

# ---------- 参数校验 ----------
if [[ $# -lt 4 ]]; then
  echo "用法: bash import-server.sh <dump.sql> <media.tar.gz> <web-dist.tar.gz> <server-binary>"
  exit 1
fi

DUMP_FILE="$1"
MEDIA_TGZ="$2"
WEB_DIST_TGZ="$3"
SERVER_BIN="$4"

for f in "$DUMP_FILE" "$MEDIA_TGZ" "$WEB_DIST_TGZ" "$SERVER_BIN"; do
  if [[ ! -f "$f" ]]; then
    echo "错误：文件不存在: $f"
    exit 1
  fi
done

# ---------- 环境变量校验 ----------
if [[ -z "${SERVER_MYSQL_USER:-}" ]]; then
  echo "错误：需要设置 SERVER_MYSQL_USER 环境变量"
  exit 1
fi
if [[ -z "${SERVER_MYSQL_PASS:-}" ]]; then
  echo "错误：需要设置 SERVER_MYSQL_PASS 环境变量"
  exit 1
fi
if [[ -z "${SERVER_MYSQL_DB:-}" ]]; then
  echo "错误：需要设置 SERVER_MYSQL_DB 环境变量"
  exit 1
fi

MYSQL_USER="$SERVER_MYSQL_USER"
MYSQL_PASS="$SERVER_MYSQL_PASS"
MYSQL_DB="$SERVER_MYSQL_DB"
INSTALL_DIR="${INSTALL_DIR:-/opt/tg-emoji-hub}"
SERVICE_USER="${SERVICE_USER:-tg-emoji}"

echo "=== tg-emoji-hub 服务器导入 ==="
echo "安装目录: $INSTALL_DIR"
echo "运行用户: $SERVICE_USER"
echo ""

# ---------- Step 1: 创建目录结构 ----------
echo "[1/6] 创建目录结构..."
sudo mkdir -p \
  "$INSTALL_DIR/bin" \
  "$INSTALL_DIR/configs" \
  "$INSTALL_DIR/media/emoji" \
  "$INSTALL_DIR/media/thumbs" \
  "$INSTALL_DIR/web" \
  "$INSTALL_DIR/var/signals" \
  "$INSTALL_DIR/logs"

echo "    OK"

# ---------- Step 2: 导入 MySQL ----------
echo "[2/6] 导入 MySQL 数据（含 AI keywords，可能需要几十秒）..."

# 先建库（幂等，已存在也没事）
mysql -u "$MYSQL_USER" -p"${MYSQL_PASS}" -e "
  CREATE DATABASE IF NOT EXISTS \`${MYSQL_DB}\`
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;
" 2>/dev/null || {
  echo "    警告：建库命令执行失败（可能权限不足），尝试直接导入"
}

mysql -u "$MYSQL_USER" -p"${MYSQL_PASS}" "$MYSQL_DB" < "$DUMP_FILE"

# 简单验证一下行数
STICKER_COUNT=$(mysql -u "$MYSQL_USER" -p"${MYSQL_PASS}" "$MYSQL_DB" \
  -sNe "SELECT COUNT(*) FROM emoji_stickers" 2>/dev/null || echo "0")
echo "    完成：emoji_stickers 共 ${STICKER_COUNT} 行"

# ---------- Step 3: 解压媒体文件 ----------
echo "[3/6] 解压媒体文件（约 550MB）..."

# 用临时目录解压，再 mv 到正确位置
TMP_EXTRACT=$(mktemp -d)
tar xzf "$MEDIA_TGZ" -C "$TMP_EXTRACT"

# 处理 emoji
if [[ -d "$TMP_EXTRACT/web/public/emoji" ]]; then
  EMOJI_COUNT=$(ls "$TMP_EXTRACT/web/public/emoji/" | wc -l)
  sudo cp -a "$TMP_EXTRACT/web/public/emoji/." "$INSTALL_DIR/media/emoji/"
  echo "    emoji: ${EMOJI_COUNT} 文件"
fi

# 处理 thumbs
if [[ -d "$TMP_EXTRACT/web/public/thumbs" ]]; then
  THUMBS_COUNT=$(ls "$TMP_EXTRACT/web/public/thumbs/" | wc -l)
  sudo cp -a "$TMP_EXTRACT/web/public/thumbs/." "$INSTALL_DIR/media/thumbs/"
  echo "    thumbs: ${THUMBS_COUNT} 文件"
fi

rm -rf "$TMP_EXTRACT"
echo "    完成"

# ---------- Step 4: 解压前端 ----------
echo "[4/6] 解压前端 dist..."
sudo mkdir -p "$INSTALL_DIR/web/dist"
sudo tar xzf "$WEB_DIST_TGZ" -C "$INSTALL_DIR/web/"
# tar 解压出来是 dist/ 目录，移到正确位置
if [[ -d "$INSTALL_DIR/web/dist/dist" ]]; then
  # 嵌套了一层 dist/dist，修正
  sudo mv "$INSTALL_DIR/web/dist/dist" "$INSTALL_DIR/web/dist.tmp"
  sudo rm -rf "$INSTALL_DIR/web/dist"
  sudo mv "$INSTALL_DIR/web/dist.tmp" "$INSTALL_DIR/web/dist"
fi
DIST_FILES=$(find "$INSTALL_DIR/web/dist" -type f | wc -l)
echo "    完成：${DIST_FILES} 文件"

# ---------- Step 5: 安装 binary ----------
echo "[5/6] 安装 server binary..."
sudo cp "$SERVER_BIN" "$INSTALL_DIR/bin/server"
sudo chmod +x "$INSTALL_DIR/bin/server"
echo "    完成"

# ---------- Step 6: 权限 ----------
echo "[6/6] 设置文件权限..."

# 如果用户不存在则创建
if ! id "$SERVICE_USER" &>/dev/null; then
  echo "    创建系统用户: $SERVICE_USER"
  sudo useradd --system --no-create-home --shell /sbin/nologin "$SERVICE_USER"
fi

sudo chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"
# media 目录额外给 others 读权限，Nginx worker (www-data) 能直接读静态文件
sudo chmod -R o+r "$INSTALL_DIR/media"
sudo find "$INSTALL_DIR/media" -type d -exec chmod o+x {} \;

echo "    完成"

# ---------- 提示 ----------
echo ""
echo "=== 导入完成 ==="
echo ""
echo "下一步："
echo "  1. 上传并编辑配置文件："
echo "     sudo cp /path/to/config.yaml $INSTALL_DIR/configs/config.yaml"
echo "     sudo chown $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/configs/config.yaml"
echo "     sudo chmod 640 $INSTALL_DIR/configs/config.yaml"
echo ""
echo "  2. 安装 systemd unit（如果还没装）："
echo "     sudo cp tg-emoji-hub.service /etc/systemd/system/"
echo "     sudo systemctl daemon-reload"
echo "     sudo systemctl enable tg-emoji-hub"
echo "     sudo systemctl start tg-emoji-hub"
echo ""
echo "  3. 配置 Nginx 站点："
echo "     sudo cp tg-emoji-hub.conf /etc/nginx/conf.d/"
echo "     sudo sed -i 's/__DOMAIN__/your-domain.com/g' /etc/nginx/conf.d/tg-emoji-hub.conf"
echo "     sudo nginx -t && sudo systemctl reload nginx"
echo ""
echo "  4. 验证："
echo "     curl http://localhost:8080/healthz"
echo "     curl https://your-domain.com/healthz"
