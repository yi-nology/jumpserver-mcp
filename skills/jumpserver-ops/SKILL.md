---
name: jumpserver-ops
description: |
  Operate machines through the JumpServer bastion with the jms-exec and
  jumpserver MCP servers. This is the ENTRY skill: it explains the split between the two
  servers, which specialist skill to load, and the rules that hold for all of them.
  Use when a request involves 堡垒机 / 跳板机 / 资产 / 连某台机器执行命令 / 批量下发命令 /
  查某台服务器的日志或磁盘 / 加一台机器进堡垒机 / 谁有权限登哪台机器, or when any
  jms_* / jmsx_* tool is available. Not for direct SSH to a host that does not go
  through the bastion.
descriptions:
  zh-Hans: "通过 JumpServer 堡垒机运维的总入口：两个 MCP 服务怎么分工、该加载哪个专项 skill、通用铁律。涉及堡垒机、跳板机、连机器执行命令、批量下发、资产上线、权限核查时使用。"
displayNames:
  zh-Hans: "堡垒机运维"
---

# JumpServer 堡垒机运维（入口）

两套 MCP 服务器，**职责不同**：

| 服务器 | 用途 | 工具数 |
|---|---|---|
| `jumpserver` | **管理面**：资产、账号、权限、用户、资产树、命令过滤、审计 | 84 |
| `jms-exec` | **执行面**：在受管机器上跑命令、传文件 | 15 |

所有连接都经堡垒机（koko 2222），会话在 JumpServer 里有录像。

**铁律：AI 访问机器一律经堡垒机，不接受直连 SSH** —— 直连会丢审计录像。

---

## 1. 该加载哪个专项 skill

| 你要做的事 | 加载 |
|---|---|
| 把一台新机器纳入堡垒机（建资产/账号/授权/用户/公钥/归位） | **`jms-onboarding`** |
| 谁能登哪台机器、授权里都有谁、某人能访问什么、最小权限 | **`jms-access-review`** |
| 在机器上跑命令、批量扫、传文件、传大输出、跑长任务 | **`jms-exec-ops`** |

只做只读查询、或者任务很单纯，直接用本 skill 的速查表即可。

---

## 2. 工具选择速查

```
要在这台机器上执行命令
├── 一条命令                  → jmsx_exec
├── 多条命令 / 有依赖关系     → jmsx_run      （一次会话，实测省 ~40%）
├── 采集多个事实（要带标签）   → jmsx_collect  （一次会话，返回命名好的 map）
├── 多台机器采同一组事实      → jmsx_collect_multi（矩阵：行=机器，列=事实）
├── 多台机器同一条命令        → jmsx_exec_multi（默认并发 4 台，带回每台输出）
├── 超过 10 分钟的长任务      → jmsx_bg_start + jmsx_bg_check
├── 输出超过 40 行            → jmsx_exec 加 full=true
└── 传文件
    ├── 本地已有的文件        → jmsx_put_file（local_path）
    ├── 内容已经在手上         → jmsx_put_file（content / content_base64）
    └── 只取远端文件的一段      → jmsx_get_file（offset / length）

要**看**堡垒机里有什么（先看，再动）
├── 接线与规模（多少资产/怎么分布/几台连不上）→ jms_status
├── 有哪些资产                    → jms_assets_list（按平台/连通性/关键字筛）
├── 这台机器上有哪些账号           → jms_accounts_list {"asset":"192.0.2.71"}
├── 这台机器的完整记录            → jms_asset_get {"ref":"192.0.2.71"}（平台/协议/账号/连通性/标签/树位置）
├── 有哪些人                      → jms_users_list
├── 有哪些授权                    → jms_permissions_list
├── 谁在线 / 有哪些会话            → jms_sessions_live / jms_sessions_list
├── 资产树长什么样                 → jms_nodes_list（路径在 full_value，name 只是节点名）
├── 有哪些平台/角色/标签            → jms_platforms_list / jms_roles_list / jms_labels_list
├── 有哪些命令组/过滤器             → jms_cmdgroup_list / jms_cmdfilter_list
├── 某人敲过什么命令               → jms_commands_search
└── 谁登录过                      → jms_login_logs

要维护堡垒机本身
├── 加/删资产                 → jms_asset_create / jms_asset_update / jms_asset_delete
├── 批量登记资产              → jms_assets_bulk_create
├── 加账号                    → jms_account_create / jms_account_update / jms_account_delete
├── 加用户 / 改密码           → jms_user_create / jms_user_password_set
├── 授权                      → jms_permission_create / jms_permission_grant / jms_permission_delete
│                              账号粒度: all_accounts=true 或 accounts=["账号名"]
├── 看授权里都有谁            → jms_permission_get
├── 这个授权实际覆盖哪些资产  → jms_permission_coverage（展开节点继承）
├── 总体谁权限最大            → jms_access_matrix
├── 两个授权差在哪            → jms_permission_compare
├── 安全态势体检              → jms_security_review
├── 看某人能访问什么          → jms_user_access
├── 看用户组                  → jms_user_groups_list
├── 资产树                    → jms_nodes_list / jms_nodes_create / jms_asset_move
├── 堡垒机注册了哪些终端       → jms_terminals_list
├── 命令过滤                  → jms_cmdgroup_create → jms_cmdfilter_create
├── 跑命令前预检会不会被拦     → jms_command_guard
├── 给资产分类                → jms_label_attach / jms_label_resources
├── 盘点账号漂移              → jms_gathered_accounts（present:false 就是漂移）
├── 查角色能干什么            → jms_role_permissions / jms_system_role_permissions
├── RBAC 权限全景/矩阵        → jms_rbac_permissions / jms_rbac_role_matrix（谁能干什么+为什么；死权限也报）
├── 登录策略（谁能登堡垒机）   → jms_login_acl_create / _update / _delete（读面 jms_login_asset_check）
├── 密钥显示脱敏              → jms_data_masking_rules / jms_data_masking_rule_create / _update / _delete
├── 剪贴板/文件大小策略        → jms_clipboard_acls / jms_clipboard_acl_create / _update / _delete
├── 这台机器现在什么状态       → jms_asset_audit（一个调用给出全部 + 结论）
├── 网络是怎么分的             → jms_net_zones_list / jms_net_gateways_list
├── 到这台机器经哪一跳         → jms_net_path（区域 + 跳板 + 通不通）
└── 安全审计
    ├── 谁改了配置            → jms_operate_logs
    ├── 谁改了谁的密码        → jms_password_change_logs
    ├── 谁能登某资产          → jms_login_asset_check
    ├── 谁碰过这台机器        → jms_asset_touch_history（谁/何时/从哪/用什么账号）
    ├── 在这台机器上干了什么  → jms_asset_evidence（命令级时间线,凭证自动遮蔽）
    ├── 什么文件被搬进搬出   → jms_file_transfers（谁传的/哪个文件/传去哪台/成没成）
    └── 高危操作不留长期凭证  → jms_delegation_call
        ├── 凭据配好了吗      → jms_delegation_status
        └── 它真的绑住了吗    → jms_delegation_probe（只读，逐条看堡垒机怎么拒）
```

**改密码而不让密钥过一遍 agent**（堡垒机自己生成、自己推送、自己存）：

```
├── 有哪些自动化计划        → jms_account_automations_list（family=push/gather）
├── 建一个计划              → jms_account_automation_create（family=push, accounts, assets, confirm）
├── 触发一次                → jms_account_automation_run（**异步**：答的是「已受理」，结果另查）
├── 执行跑完没有            → jms_account_automation_executions（status；⚠️ 失败原因 API 不给）
└── 每个账号的结果          → jms_account_automation_records（**仅 push**；gather 的结果看 jms_gathered_accounts）
```

⚠️ `family=push` 才是「密钥不出堡垒机」的那一种（堡垒机生成密码 → 推到目标机 → 存进自家 vault）。
⚠️ **push 是「在机器上铺新账号」，不是「轮换已有账号的密码」** ——
已存在的账号它不动（轮换密码要企业版的 change-secret，本部署不可用）。
⚠️ **两族的 `accounts` 意思不一样**：push 填「**要创建**的账号名」（必填）；
gather **不用填**（它用资产上已有的账号登录去发现，填了也没用）。
⚠️ **自动改密（change-secret）与账号检查（check-account）在这台部署上不可用** ——
它们要企业版许可证，堡垒机会回 403，**看起来像缺权限、其实不是**。
⚠️ 触发是**异步**的：答复说「已受理」，不等于做完了 —— 要用 executions/records 查结果。
⚠️ **只有 push 有「每个账号的结果」**（records）；gather 的结果是
**发现的账号**（`jms_gathered_accounts`）。

**处置正在发生的事**（写操作，但都是「收尾」而不是「配置」）：

```bash
jms_session_terminate {"session_id":"...","confirm":true}   # 踢掉占着终端的人
jms_job_stop          {"task_id":"...","confirm":true}      # 停掉跑飞的批量作业
```

**不确定哪里断了 → 先自检**：

```bash
jumpserver-mgmt --doctor    # 管理面：接线 + 堡垒机配置体检
jumpserver-exec --doctor      # 执行面：接线 + 真实 koko 登录验证
```

`doctor` 也是子命令形态（两种等价）；`--help` 打印用法。
退出码：`0` 正常 / `1` 有问题 / `2` 用法错误。

⚠️ **没有终端也能自检**（agent 会话里通常没有）：
会话内用 `jms_status`（接线对不对、资产多少台、几台连不上）与 `jms_doctor`
（更细：资产缺协议/缺账号、授权没授账号）。CLI 的 `--doctor` 是同一件事的
终端形态。

---

## 3. 通用铁律

1. **一切写操作都要 `confirm=true`。** 不可逆的（删资产、删授权、改密码）在
   description 里写明了后果，不要为了少一轮交互就绕过。
2. **改源码 ≠ 生效。** 完整链路：
   `./script/build-mcp.sh` → `./script/publish-mcp.py` → `mcp update` → **重启会话**。
3. **命令以资产账号身份执行，不提权。** 要 root 自己写 `sudo -n`。
4. **不要原地覆盖二进制。** 沙箱按路径执行策略，覆盖已存在路径的二进制会被 SIGKILL
   （exit 137、无 stderr）。`publish-mcp.py` 已经进新的时间戳目录，别自己写 `cp`。
5. **高危操作用委派凭据。** `jms_delegation_call` 一次性、绑定、60 秒过期，
   适合删除、改权限这类不想长期可达的操作。委派**不额外授权**，走正常 RBAC。
6. **`err` 不一定是故障。** 路由器/网关只有 80/443 时，连通性 `err` 就是对的 ——
   判断依据是**资产平台类型**，不是 `err` 这个字。
7. **列表工具的行都在 `items` 里。** 统一信封是
   `{count, total, truncated, items[], note?}`：`count` 是这次返回了几条，
   `total` 是过滤后命中几条，`truncated` 说有没有被截断，**`items` 才是那些行**。
   每一行的字段在工具的 `outputSchema` 里写着（`tools/list` 能看到），不用背。

---

## 4. 资产引用必须精确

资产引用（`asset` 参数）支持**地址 / 名称 / UUID**，但必须**精确匹配**：
`node` 匹配不到 `node-071`。传错会给出候选列表。

**koko 的搜索框按资产名匹配**（名字是列表里那一列），所以执行面优先传**名称**。

---

## 5. 参考资料

| 文档 | 内容 |
|---|---|
| [`references/v5-api-traps.md`](references/v5-api-traps.md) | **v5 的 API 坑表**（20 条，全部实测）。写操作前先看 |
| [`references/version-compat.md`](references/version-compat.md) | **v4 与 v5 的差异**（⚠️ 源码级核对，**未实测**）。面对 v4 堡垒机、或不确定某个行为是不是版本差异时看 |
| [koko TUI 协议](https://github.com/yi-nology/jumpserver-exec/blob/main/skills/jms-exec-ops/references/koko-tui.md) | koko TUI 协议。改握手前先看（**在执行面仓里**） |
| `docs/CONFIGURATION.md` | 环境变量、接线、资产上线流程、故障对照表（仓库内） |

---

## 6. 改了 MCP 之后怎么验证

```bash
cd /path/to/jumpserver-mgmt
./script/build-mcp.sh      # 编译 + 单测 + 启动自检（测试失败会打印日志）
./script/publish-mcp.py    # 挑最新可用构建，打印 mcp update 命令
# 按输出 mcp update，然后重启会话
```

**必须串行跑回归** —— 两套都会在 `node-071` 上建临时对象，并发会互相删除。

⚠️ **本会话的工具目录是启动时缓存的**，新加的工具在重启前调不到
（`Unknown tool_name`）。想在重启前就真调，用 JSON-RPC 直接驱动已发布的二进制：
写 `initialize` → `tools/list` → `tools/call`。

**别把「编译过」说成「已完成」。** 汇报时给出：编译结果、测试结果、工具数变化、
发布路径、自检结果，以及「需要重启会话才生效」这句话。
