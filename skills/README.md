# 技能家族（管理面）

给 AI agent 用的操作手册。**每个 skill 是一个目录，含一个 `SKILL.md`**，
靠 frontmatter 的 `description` 决定什么时候被加载。

```
skills/
├── README.md                  ← 本文件：家族总览 + 安装
├── jumpserver-ops/            ← 入口：分工、工具速查、铁律、参考
│   ├── SKILL.md
│   └── references/
│       ├── v5-api-traps.md    ← v5 的 API 坑表（20 条，全部实测）
│       └── version-compat.md  ← v4 与 v5 的差异（源码级核对，未实测）
├── jms-onboarding/SKILL.md    ← 资产上线（写操作）
└── jms-access-review/SKILL.md ← 权限审计（只读）
```

## 三个 skill 的分工

| skill | 什么时候加载 | 写操作? |
|---|---|---|
| `jumpserver-ops` | **入口**。任何堡垒机任务先看它：两个服务怎么分工、工具速查、铁律、配置与迭代 | 视情况 |
| `jms-onboarding` | 把机器纳入堡垒机：建资产/账号/授权/用户/公钥/归位 | 是 |
| `jms-access-review` | 谁能登哪台机器、授权语义、权限过大核查、离职审计 | 否（只读） |

⚠️ **执行面的 skill 在另一个仓**：`jms-exec-ops`（在机器上跑命令、批量并发、长任务、传文件、
大输出）在 [`yi-nology/jumpserver-mcp-exec`](https://github.com/yi-nology/jumpserver-mcp-exec) 仓里 ——
它跟着执行面服务走，不跟着管理面走。

**分工原则**：入口 skill 不重复专项 skill 的细节，只给「该加载哪个」和速查表。
这样每个 skill 都保持短，agent 只加载它真正需要的那一份。

## 安装

技能要放在 agent 的 skills 目录才会被加载（路径随客户端而定，例如
`~/.minimax/agents/mavis/skills/`、`~/.claude/skills/`、`~/.codex/skills/`）：

装法（把仓里的 skill 复制过去）：

```bash
cd /path/to/jumpserver-mcp
for s in jumpserver-ops jms-onboarding jms-access-review; do
  mkdir -p ~/.minimax/agents/mavis/skills/$s
  cp skills/$s/SKILL.md ~/.minimax/agents/mavis/skills/$s/
  # references 目录（只有 jumpserver-ops 有）一起拷
  [ -d skills/$s/references ] && cp -r skills/$s/references ~/.minimax/agents/mavis/skills/$s/
done
```

执行面的 skill 同理（在执行面仓里装）：

```bash
cd /path/to/jumpserver-mcp-exec
s=jms-exec-ops
mkdir -p ~/.minimax/agents/mavis/skills/$s
cp skills/$s/SKILL.md ~/.minimax/agents/mavis/skills/$s/
```

**skill 放在仓里而不是只放在 agent 目录**，是因为：
- 跟着代码走版本化，改了工具表不用手动同步两份
- 新会话、新 agent 装上就有
- 评审时能看出「文档说的」和「代码做的」是否一致

> ⚠️ 旧的 skill 只存在于 agent 目录、仓里没有，所以它既没版本化、
> 也随时可能跟代码脱节（koko 协议变过一次，文档就整整失效过一轮）。
> 现在这份是修好之后的版本。

## frontmatter 约定

```yaml
---
name: <目录同名>
description: |
  <英文描述：什么时候用、触发词、什么时候不要用>
descriptions:
  zh-Hans: "<中文描述>"
displayNames:
  zh-Hans: "<中文显示名>"
---
```

`description` 是**唯一**决定加载时机的字段，所以必须写清
**触发词**（堡垒机 / 跳板机 / 资产 / 执行命令 …）和**边界**（什么时候不要用）。

## 维护

改完 MCP 工具后，**这些 skill 的工具表要一起改**。

这不靠自觉 —— `script/check-skill-drift.sh` 在构建时自动跑：

```bash
./script/check-skill-drift.sh
```

它检查三个方向，其中两个算错误：

- **文档 → 代码（错误）**：skill 里提到的工具必须真实存在。
  skill 写着一个已经改名或删掉的工具，会让 agent 去调一个不存在的东西 ——
  而且往往在最不该失败的时候失败。
- **工具名必须写全（错误）**：`jms_asset_create / update / delete` 这种简写，
  人读得懂，agent 抄过去就成了调一个从不存在的东西（`update` 不是工具名）。
  斜杠后面也必须是完整工具名。
- **代码 → 文档（只报告）**：skill 的速查表是**选择指南**而不是穷举，
  一个只读小工具没进表是正常的。这个方向只报覆盖率，让新加的工具至少被看见。
