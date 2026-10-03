#!/bin/bash
# 只跑 *.up.sql：直接把整个 migrations 目录挂进 initdb.d 会把 *.down.sql 也按字母序执行，
# 002.down 去 DROP 一个还不存在的列，初始化当场失败
set -eu

for f in $(ls /migrations/*.up.sql | sort); do
  echo "[init] apply $f"
  mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE}" < "$f"
done
