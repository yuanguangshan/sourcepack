#!/bin/bash
# 把 book/src/ 的所有文件快照到 book/.backup/（含时间戳）。
#
# 为什么需要它：2026-10-13 曾发生一次事故 —— 子写者已完成的附录与序
# （附录 A 有 145 条术语）被一段"补齐缺失文件"的脚手架逻辑覆盖，
# 且因为后续 `git add -A` 把覆盖后的 stub 状态提交进了仓库，
# 正文从未进入 git，`git fsck --lost-found` 也找不回（全仓库只有 2 个
# dangling blob，都是 Go 源码）。
#
# 教训有两条，本脚本与 README 都应记住：
#   1. **绝不对他人正在写的目录做批量写入**，即使有 `[ -f ]` 守卫
#      —— 守卫只保证"不覆盖已存在"，挡不住"把已有的判成缺失后再重建"。
#   2. 交付物落盘后要**立刻快照**，别等所有子任务做完（那时一次事故全没）。

set -euo pipefail
cd "$(dirname "$0")/../.."          # → 仓库根

SRC=book/src
BK=book/.backup
mkdir -p "$BK"

count=0
for f in "$SRC"/*.md; do
  [ -e "$f" ] || continue
  cp "$f" "$BK/$(basename "$f")"
  count=$((count + 1))
done
echo "$(date '+%F %T')  已快照 $count 个文件 → $BK/"
wc -l "$BK"/*.md 2>/dev/null | tail -1
