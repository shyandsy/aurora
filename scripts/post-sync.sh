#!/usr/bin/env bash
# aurora post-sync —— **向后兼容 shim**,已被 scripts/sync-skills.sh 取代。
#
# 历史:skill 分发曾寄生在「整树 vendor + 从 third_party/aurora/.claude/skills 镜像」上,
#   消费项目 sync 完 aurora 后调 `bash third_party/aurora/scripts/post-sync.sh` 一次。
#   这把「拿 skill」和「整树 vendor」绑死,逼着只想要 skill 的 ① 项目去上 ② 的 vendoring。
#
# 现在:skill 分发收口到 scripts/sync-skills.sh —— 和代码消费**正交**,①(go get)/②(整树 vendor)
#   都能用,不碰 go.mod / third_party。本 shim 只为不破坏已经写了 `post-sync.sh` 调用的项目:
#   直接转发到 sync-skills.sh(它会自动发现本地 third_party/aurora 树,行为与旧版等价)。
#
# 新项目别再调 post-sync.sh —— 直接用 sync-skills.sh(见其头注)。
set -euo pipefail
exec bash "$(dirname "$0")/sync-skills.sh" "$@"
