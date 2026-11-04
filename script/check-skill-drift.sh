#!/usr/bin/env bash
# 检查技能文档与已注册的工具有没有漂移。
#
# 为什么有这个脚本：技能文档是 agent 挑工具时读的那一份 ⇒ 文档里写着一个
# 已经被改名或删掉的工具名，agent 就会去找一个不存在的东西，而且是在最不该
# 出错的时候。
#
# 两个方向，重要性不一样：
#
#   1. 文档 → 代码（**是错**）：文档提到的每一个工具都必须存在。
#      一个改了名或被删掉的工具，被文档指着的后果是 agent 去追一个不存在
#      的东西。
#
#   2. 代码 → 文档（**是报告**，不是错）：技能里的速查表是一份**选路**指南，
#      不是全集 —— 没必要把一个只读小助手也塞进速查表。这个方向只报覆盖率。
#
# 扫的是**整个工作集**（本仓 + 同级的另外两个仓），不是只有本仓：
# 管理面的技能里就写着执行面的工具（jmsx_reachable、jmsx_exec……），所以
# 「这个工具存在吗」的判据不能只看本仓的代码。
#
# 用法：
#     ./script/check-skill-drift.sh
#     JMS_SIBLING_ROOTS=/path/to/kit:/path/to/exec ./script/check-skill-drift.sh
#
# 没有 JMS_SIBLING_ROOTS 时用同级目录约定（三个仓并排放着）：
#     ../jumpserver-mcp-kit  ../jumpserver-mcp  ../jumpserver-mcp-exec
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PARENT="$(cd "$ROOT/.." && pwd)"

# 多仓工作集：环境变量优先（`:` 分隔），否则用同级目录约定。
# ⚠️ 三个仓的名字是**约定**（和守卫用的是同一套）—— 一份拷贝会漂。
# ⚠️ **另一份在底座仓**：`jumpserver-mcp-kit/script/workset.py` 的
# `REPO_NAMES`（两个变异脚本读它）。改了这份名单 ⇒ 那边也要改。
roots() {
  local d
  if [ -n "${JMS_SIBLING_ROOTS:-}" ]; then
    echo "$JMS_SIBLING_ROOTS" | tr ':' '\n'
    return
  fi
  for d in jumpserver-mcp-kit jumpserver-mcp jumpserver-mcp-exec; do
    [ -d "$PARENT/$d" ] && echo "$PARENT/$d"
  done
}

ROOTS=("$ROOT")
while IFS= read -r r; do
  [ -n "$r" ] || continue
  [ "$r" = "$ROOT" ] && continue
  ROOTS+=("$r")
done < <(roots)

doc=$(mktemp)
code=$(mktemp)
trap 'rm -f "$doc" "$code"' EXIT

for r in "${ROOTS[@]}"; do
  [ -d "$r/skills" ] || continue
  grep -ohE '\bjmsx?_[a-z][a-z_]*\b' "$r"/skills/*/SKILL.md 2>/dev/null
done | sort -u > "$doc"

# 工具住在各仓的 biz/*/handler/ 下，目录在 biz/*/policy/ 下 ⇒ 两处都要递归扫。
# 测试文件里带的是样例工具名（"jms_thing"），它们不是工具。
for r in "${ROOTS[@]}"; do
  [ -d "$r/biz" ] || continue
  grep -rhoE 'Name: *"jmsx?_[a-z][a-z_]*"' --include='*.go' --exclude='*_test.go' \
    "$r/biz" "$r/cmd" 2>/dev/null
done | sed 's/.*"\(.*\)"/\1/' | sort -u > "$code"

doc_count=$(wc -l < "$doc" | tr -d ' ')
code_count=$(wc -l < "$code" | tr -d ' ')

missing=$(comm -23 "$doc" "$code")
if [ -n "$missing" ]; then
  echo "FAIL: 技能里提到的工具在整个工作集里都不存在："
  echo "$missing" | sed 's/^/    /'
  exit 1
fi

# 速查表里的工具名必须**是全名**。
#
# 「jms_asset_create / update / delete」人读着顺，但 agent 是照抄的 —— 而
# "update" 不是一个工具名 ⇒ 调用失败在一个从来不存在过的名字上。
# ⇒ 斜杠后面的那个词也得是完整的工具名。
shorthand=$(for r in "${ROOTS[@]}"; do
  [ -d "$r/skills" ] || continue
  grep -ohE 'jmsx?_[a-z_]+ / [a-z][a-z_]*' "$r"/skills/*/SKILL.md 2>/dev/null
done | grep -vE 'jmsx?_[a-z_]+ / jmsx?_[a-z_]+' | sort -u || true)
if [ -n "$shorthand" ]; then
  echo "FAIL: 技能里用了简写的工具名 —— 斜杠后面那半不是工具："
  echo "$shorthand" | sed 's/^/    /'
  exit 1
fi

undocumented=$(comm -13 "$doc" "$code")
undoc_count=$(echo "$undocumented" | grep -c . || true)
if [ "$code_count" -gt 0 ]; then
  covered=$((doc_count * 100 / code_count))
else
  covered=0
fi

echo "OK: 技能里提到的工具都存在（文档 $doc_count 个 / 代码 $code_count 个）"
echo "    技能覆盖了 $covered% 的已注册工具"
if [ "$undoc_count" -gt 0 ]; then
  echo "    没进技能速查表（是小助手就没事）："
  echo "$undocumented" | sed 's/^/        /'
fi
exit 0
