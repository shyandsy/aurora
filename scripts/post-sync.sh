#!/usr/bin/env bash
# aurora post-sync —— 消费项目 sync 完 aurora 后调用一次,把「随仓分发的 skill」
# 从 vendored 位置镜像进消费项目根的 .claude/skills/,让 agent 能发现。
#
# 为什么需要:sync 已经把 aurora 整棵树(含 .claude/skills/)拷进了
# third_party/aurora/.claude/skills/,但 agent 只扫**项目根**的 .claude/skills/,
# 不扫 third_party/。故要把它镜像到项目根。
#
# 为什么放在 aurora(而不是各项目 Makefile 里写拷贝逻辑):
#   sync 工具本身是各项目各一份(都从 cicd 骨架 fork)。若把「拷哪些、怎么拷」写进
#   各自 Makefile,就又是一处 N 份的分叉——aurora 以后加/改 skill,4 个项目 Makefile 都得跟。
#   把逻辑收口到 aurora 这个脚本,各项目 sync-aurora 只加**一行**调用:
#       bash third_party/aurora/scripts/post-sync.sh
#   之后新增/改 skill 全在 aurora 单一源,消费项目 sync 即自动获得,永不再改各自 Makefile。
#
# 幂等 & 托管副本:每次覆盖式镜像。项目根 .claude/skills/<name> 是托管副本,别手改,
# 下次 sync 覆盖(要改去 aurora 改)。
set -euo pipefail

SRC="third_party/aurora/.claude/skills"
DST=".claude/skills"

if [ ! -d "$SRC" ]; then
  echo "post-sync: 未找到 $SRC(此版本 aurora 无分发 skill),跳过。"
  exit 0
fi

mkdir -p "$DST"
count=0
for d in "$SRC"/*/; do
  [ -d "$d" ] || continue
  name="$(basename "$d")"
  rm -rf "${DST:?}/$name"
  cp -R "$d" "$DST/$name"
  echo "post-sync: 已镜像 skill '$name' → $DST/$name(托管副本,别手改)"
  count=$((count + 1))
done
echo "post-sync: 完成,共镜像 $count 个 skill。"
