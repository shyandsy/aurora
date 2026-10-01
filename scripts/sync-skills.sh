#!/usr/bin/env bash
# aurora sync-skills —— 把 aurora「随仓分发的 skill」同步进**当前消费项目根**的 .claude/skills/。
#
# ★ 与「怎么消费 aurora 的代码」**完全正交**:不管你是 ① go get 直接依赖,还是 ② 整树 vendor,
#   都能用这一个机制拿 skill;**不碰 go.mod、不碰 third_party/aurora**。
#   (历史上 skill 分发寄生在「整树 vendor + post-sync 镜像」上,逼着只想要个 skill 的 ① 项目
#    去上 ② 的整套 vendoring —— 这就是误导源。此脚本把两者拆开。)
#
# 取源(自动择一,优先无网络):
#   A. 本地已 vendor 整树:存在 ./third_party/aurora/.claude/skills  → 直接从那镜像(② 项目,零网络)。
#   B. 否则:从 aurora 上游**只** sparse-checkout `.claude/skills` @ REF  → 镜像(① 项目的正路)。
#
# 用法(在消费项目根执行):
#   bash <path>/sync-skills.sh                      # 有本地树则用之;否则从上游 main 拉
#   REF=<tag/commit/branch> bash <path>/sync-skills.sh      # B 情形指定版本(默认 main)
#   SKILLS="design-nav foo"  bash <path>/sync-skills.sh     # 只同步这几个(默认:全部)
#   AURORA_REPO=<git-url>    bash <path>/sync-skills.sh      # 覆盖上游地址
#
# 怎么拿到本脚本(别掉进"为拿脚本去 vendor aurora"的坑):
#   ② 项目:third_party/aurora/scripts/ 里已有,直接 `bash third_party/aurora/scripts/sync-skills.sh`。
#   ① 项目:本地没有 aurora 树,**把本文件(零依赖单文件)拷一份进你自己仓的 scripts/**
#           (托管副本,别手改,升级重拷)——**这不是 vendor aurora**,只是带个独立小工具。
#   消费项目在自己的 Makefile 加**一行** `sync-aurora-skills` 目标调用它即可(和 `sync-aurora` /
#   `sync-aurora-web` 这些**代码**同步目标平级、互不依赖)。
#
# 托管副本:项目根 .claude/skills/<name> 是托管副本,**别手改**,下次 sync 覆盖(要改去 aurora 改)。
set -euo pipefail

AURORA_REPO="${AURORA_REPO:-git@github.com:shyandsy/aurora.git}"
REF="${REF:-main}"
DST=".claude/skills"
SKILLS="${SKILLS:-}"   # 空 = 全部

tmp=""
cleanup() { [ -n "$tmp" ] && rm -rf "$tmp"; return 0; }
trap cleanup EXIT

if [ -d "third_party/aurora/.claude/skills" ]; then
  SRC="third_party/aurora/.claude/skills"
  echo "sync-skills: 取源 = 本地已 vendor 的 $SRC(零网络)。"
else
  tmp="$(mktemp -d)"
  echo "sync-skills: 本地无 third_party/aurora 树 → 从上游只拉 .claude/skills @ $REF ..."
  git clone -q --no-checkout --filter=tree:0 "$AURORA_REPO" "$tmp/src"
  git -C "$tmp/src" sparse-checkout init --cone >/dev/null
  git -C "$tmp/src" sparse-checkout set .claude/skills >/dev/null
  git -C "$tmp/src" checkout -q "$REF"
  SRC="$tmp/src/.claude/skills"
fi

if [ ! -d "$SRC" ]; then
  echo "sync-skills: $SRC 不存在(此版本 aurora 无分发 skill),跳过。"
  exit 0
fi

mkdir -p "$DST"
count=0
for d in "$SRC"/*/; do
  [ -d "$d" ] || continue
  name="$(basename "$d")"
  if [ -n "$SKILLS" ] && ! printf '%s\n' $SKILLS | grep -qx "$name"; then
    continue
  fi
  rm -rf "${DST:?}/$name"
  cp -R "$d" "$DST/$name"
  echo "sync-skills: 已同步 skill '$name' → $DST/$name(托管副本,别手改)"
  count=$((count + 1))
done
echo "sync-skills: 完成,共同步 $count 个 skill。"
