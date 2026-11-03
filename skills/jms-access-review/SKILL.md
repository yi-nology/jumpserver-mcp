---
name: jms-access-review
description: |
  Answer "who can log into what" on the JumpServer bastion, read-only. Use for access
  reviews, least-privilege audits, permission reviews, offboarding checks, and questions
  like 谁有权限登哪台机器 / 某人能访问什么 / 这个授权里都有谁 / 有没有权限过大的人.
  Also covers contact history — 谁在什么时候碰过这台机器、从哪、用什么账号 — for
  when the question is what happened on a box rather than who may reach it.
  Covers the difference between a permission that grants an account and one that grants
  nothing, permissions held by user groups rather than named users, and the four
  configurations that fail silently at login time.
descriptions:
  zh-Hans: "堡垒机访问权限审计（只读）：谁能登哪台机器、授权里都有谁、某人能访问什么、权限是否过大、离职核查，以及谁在什么时候碰过哪台机器。涉及权限核查/最小权限/授权审计/谁有权限登哪台机器/谁动过这台机器时使用。"
displayNames:
  zh-Hans: "堡垒机权限审计"
---

# 堡垒机访问权限审计

**全部只读。** 这个 skill 回答一个问题：**谁，能用哪个账号，登上哪台机器。**

---

## 1. 按问题选工具

| 问题 | 工具 |
|---|---|
| 这台机器现在什么状态、谁能登它 | **`jms_asset_audit {"asset":"192.0.2.71"}`** |
| 某人能访问哪些资产 | `jms_user_access {"user":"opsuser"}` |
| **这个授权实际覆盖哪些资产** | **`jms_permission_coverage {"permission":"..."}`** |
| 总体：谁权限最大 | `jms_access_matrix` |
| 两个授权差在哪（换授权前预览） | `jms_permission_compare {"left":"...","right":"..."}` |
| 安全态势体检 | `jms_security_review {"days":30}` |
| 某个授权里都有谁 | `jms_permission_get {"permission":"ai-全部资产"}` |
| 有哪些用户组、每个组持有哪些授权 | `jms_user_groups_list` |
| 某人能登某台机器吗（实时 ACL） | `jms_login_asset_check {"asset":"...","user":"..."}` |
| 角色/系统角色能干什么 | `jms_role_permissions` / `jms_system_role_permissions` |
| 谁改了权限/资产（追责） | `jms_operate_logs {"resource_type":"Asset Authorization"}` |
| 谁改了谁的密码 | `jms_password_change_logs` |
| **谁碰过这台机器**（谁/何时/从哪/用什么账号） | **`jms_asset_touch_history {"asset":"192.0.2.71"}`** |
| **在这台机器上干了什么**（命令级时间线） | **`jms_asset_evidence {"asset":"192.0.2.71"}`** |

**`jms_asset_audit` 是首选**：一个调用给出协议、账号、树位置、标签、覆盖它的授权
（带实际授了哪些账号），外加逐项 `checks[]`（每项带 `detail` 和能修它的工具名）
和一句 `verdict`。它把散在 5 个端点里的信息合成一次调用，
并替你判断出「这台机器根本没人能登」。

---

## 1b. ⚠️ 授权的计数器会骗人

`assets_amount` **只数直接点名的资产**，**节点继承的资产不算**。

实测：本机授权「所有」报 `assets_amount = 16`，**实际覆盖 45 台**
（它还覆盖 home / tencent / arm / x86 四个节点）。

⇒ **做权限审计绝不能直接用这个计数**。用 `jms_permission_coverage`，
它把节点子树展开，并明确报出「报告数 vs 实际数」。

⇒ 展开靠节点的 `key` 前缀（`"1:0"` = studio，`"1:0:1"` = studio/arm）——
`parent_id` 字段在每个节点上都是 `null`，不能用。

⇒ 展开结果要与堡垒机自己的有效权限视图
（`/perms/users/{uuid}/assets/`，也就是 `jms_user_access`）**对得上**才算数。

**嵌套节点**同理：授权挂在 `/DEFAULT/studio`，就覆盖 `/DEFAULT/studio/arm` 下的资产 ——
而资产只列自己的叶子节点，比直接节点 id 会漏判。

---

## 2. 三个最容易看漏的东西

### ① 授权「一个账号都没授予」

```json
"accounts": []      ← 不是「全部账号」，是「没有账号」
"accounts": ["@ALL"] ← 全部账号
```

用这个账号去登会失败，提示「没有可用的授权账号或终端协议」。
**这个错误在列表里看不出来** —— 列表只给 `accounts_amount` 之类的计数，
必须 `jms_permission_get` 取详情，或直接 `jms_asset_audit` 看结论。

### ② 授权点名了机器上根本没有的账号

`accounts: ["opsuser"]` 但那台资产上只有 `root` ⇒ 这条授权**永远用不了**，
而且**不报错**。`jms_doctor` 的 `permissions_with_unknown_accounts` 会点名。

### ③ 权限挂在用户组上

授权的持有者可能是**组**而不是人。用户列表看不到组，授权列表只给计数
（`users_amount`）—— 组授权看起来就像「没人有权限」。

`jms_user_groups_list` 会把组、成员、以及**每个组持有的授权**一起列出来。

---

## 3. 有效权限视图

`jms_user_access` 走堡垒机自己的权限图（`/perms/users/{uuid}/assets/`），
所以它包含**通过节点或用户组继承**到的资产，不只是被直接点名的。

每个资产还带 `granted_by` —— **是哪个授权给的**。
审计里「他能访问 45 台」如果不说清「通过哪个授权」，
读者就无从下手：要改的是那个授权本身。

⚠️ 这个端点**只认 UUID**，传用户名会 404 `User object does not exist` ——
即使那个用户真实存在。工具内部已经做了用户名 → UUID 的解析。

`jms_access_matrix` 是顶层视图：**所有真人用户**（组件账号 `[KoKo]-...` 会被跳过）
各自能到达多少台、由哪些授权给的，按可达数从大到小排。
用它找「谁权限过大」，也用于离职核查。

---

## 4. 体检：一次扫出配置里的雷

```bash
jumpserver-mcp --doctor     # 或会话内 jms_doctor
```

会报出：

| 检查项 | 含义 |
|---|---|
| `assets_without_protocols` | 主机类资产没有终端协议 → 永远连不上 |
| `assets_without_accounts` | 主机类资产没有账号 → 登录必失败 |
| `permissions_without_accounts` | 授权一个账号都没授予 |
| `permissions_without_members` | 授权没有人持有 |
| `permissions_with_unknown_accounts` | 授权点名的账号在目标资产上不存在 |

⚠️ **`web`（Website）和 `cloud`（Kubernetes）类资产不算在内** ——
它们本来就没有终端协议和 SSH 账号，把它们报成故障只是噪音
（本机曾因此报出 6 条假警报，而那 6 个服务全都好好地在跑）。

---

## 5. 最小权限怎么做

```bash
# 建授权时只授需要的账号
jms_permission_create {"name":"运维-仅opsuser","users":["opsuser"],
                       "assets":["192.0.2.71"],"accounts":["opsuser"],"confirm":true}

# 给已有授权追加（只增不减）
jms_permission_grant {"permission":"运维-仅opsuser","accounts":["root"],"confirm":true}
```

⚠️ **追加绝不能收窄范围**：已经是 `@ALL` 的授权，追加具名账号时 `@ALL` 会被保留
（并集的结果就是 `["@ALL"]`）。把 `@ALL` 换成具名账号 = 静默撤权，
而撤权**不报错**，直到有人登录失败才被发现，且症状指向凭据而不是权限。

**收窄授权是另一个动作**（revoke），本套工具故意不提供 ——
「加」和「减」必须是两个入口。

---

## 6. 追责与取证

```bash
# 谁改了配置（资产/授权/账号/设置）
jms_operate_logs {"resource_type":"Asset Authorization","limit":50}

# 谁改了谁的密码（change_by 不是本人 = 别人重置的，安全事件）
jms_password_change_logs {"user":"opsuser"}

# 谁碰过这台机器：谁、何时、从哪、用什么账号
jms_asset_touch_history {"asset":"192.0.2.71","limit":5}

# 别人以前在这台机器上跑过什么
jms_asset_evidence {"asset":"192.0.2.71","limit":5}
jms_command_audit {"asset":"192.0.2.71","limit":20}
jms_session_transcript {"session_id":"<上面的 session>"}

# 原始终端录像
jms_session_replay {"session_id":"<session id>"}
```

### 「在这台机器上干了什么」用取证工具

```bash
jms_asset_evidence {"asset":"192.0.2.71","limit":5}
```

一条命令给出一台机器的**命令级时间线**：谁、什么时候、从哪、用什么账号、
敲了什么、返回了什么。命令日志按 **session** 索引、资产按 **asset** 索引，
中间那道 join 就是这个工具做的。

- 只报**有审计价值**的会话（`error_reason` 为空），握手半成品自动排除，
  并告诉你排除了多少。
- 会话没有命令记录时，可加 `include_replay: true` 顺手把录像文本取回来
  （实测拉一个录像约 0.2 秒）。
- ⚠️ **凭证样式的值会被自动遮蔽**（私钥、password/token/bearer、PEM 块），
  命令本身照常显示。输出里带 `redacted: true` 就是遮过。
  这个遮蔽是**必要的**：审计输出会进 AI 上下文和终端回滚区，
  本机就有一台机器的 WireGuard 私钥曾经被原样打印出来过。

### 批量变更的追责

```bash
jms_job_audit {"only_failed":true}
```

批量作业（`jms_job_run`）的**审计线**：谁跑的、什么时候、**实际下发的命令**、
耗时、**哪台目标失败且失败原因**。

⚠️ `jms_job_list` 只有 per-host 的成功/失败汇总，`jms_job_status` **完全没有命令**；
「上周那批机器到底跑了什么」只有这个工具答得出来。

⚠️ 失败原因里常是**命令本身写错了**——本机就有一条 2026-10-14 的记录，
失败原因是 `template error ... unexpected '.'`：命令里的 Go template
`{{.Names}}` 被堡垒机当模板渲染了。

⚠️ 审计记录比操作记录**活得久**：作业的操作侧记录被清理后，审计侧还在，
此时该条没有 per-host 明细（`without_host_detail`），但命令与执行者仍在。

### ⚠️ 命令日志的盲区：登入了却什么都没记

有会话**有真实交互、但一条命令都没记**时，命令日志就说不出话了。
这类会话**单独列在 `sessions_without_commands` 里**，且**不受 `limit` 影响**
（limit 取最新，正好会把它们埋掉）。

⚠️ **别把它当成「丢记录了」**：看 `replay_bytes` 与 `duration` 的组合——
**30 分钟的会话只有 955 字节录像**，那是挂着的空 shell（终端没回显），
不是审计缺失。本机 7 个这样的会话，4 个是真人。

⚠️ **机器已从堡垒机删掉也能查** —— 会话轨迹里还留着它的历史，
工具会回退到会话里的资产标签，并标明 `asset_deleted_from_bastion: true`。

### 「谁碰过这台机器」用接触史工具

```bash
jms_asset_touch_history {"asset":"192.0.2.71"}
```

按资产给出：会话数、首次/末次接触、涉及的人、用的账号、来源 IP、
录像覆盖、以及**被多于一人碰过的资产**（`assets_touched_by_several_users`）——
后者是「这台机器上的人是不是太多了」的直接信号。

⚠️ **登录日志进不了这个视图** —— 堡垒机的登录记录**没有资产字段**，
它只说「谁登录了堡垒机」。想归因到机器，唯一可靠路径是会话。

⚠️ **先分清「有录像」和「有审计价值」**（2026-10-25 实测本机 617 个会话）：

| 口径 | 数字 |
|---|---|
| 全量会话 | 617 |
| `error_reason` 为空（= 真有交互） | **98（15%）** |
| 其中有命令记录 | **91（92%）** |
| `has_replay = true` | 614 —— 但其中 **518 个是 `replay_unsupported`** |

⚠️ **`has_replay = true` 不等于录像有内容**。那 518 个 `replay_unsupported`
会话只录下了 koko 握手的字节：录像文件在，但没有可读的交互。
所以「录像覆盖 99.5%」是假象 —— 用它做统计会得出「一切都被审计了」的错误结论。

⇒ **命令审计本身是健康的**（有价值的会话 92% 有命令记录）；
覆盖率低纯粹是因为分母里灌进了 518 个握手半成品。

⇒ 判断一个会话有没有审计价值，看 **`error_reason` 是否为空**，
不要看 `has_replay`，也不要用会话行上的 `has_command`（**它恒为 false**）。
用 `command_amount`，或直接数 `/terminal/commands/` 的行数。

⚠️ 上面这些数字是**按全量会话**算的。要谈「某个人/某台机器」，
用 `jms_asset_evidence` 的口径（有证据的会话），别自己拿全量除。


---

### 换授权前先看差在哪

```bash
jms_permission_compare {"left":"所有","right":"ai-全部资产"}
```

会给出「只有左边有 / 只有右边有 / 两边都有」的资产清单，并在会**减少**访问权时明确告警：

> swapping 所有 for ai-全部资产 would REMOVE access to 4 asset(s): Nexus, bridge-studio, fnos-studio, main-router

两侧都按**实际覆盖**展开（不是 `assets_amount` 那个计数）。

## 6b. 安全态势

```bash
jms_security_review {"days":30}
```

一次给出：谁能重配堡垒机（超级用户）、从未登录的账号、
失败登录按 IP/账号的分布、堡垒机已封禁的地址，以及逐项 `checks[]`
和总体 `status`（ok / warn / fail）。

⚠️ 失败登录**不等于**爆破：同时有成功登录的地址是正常客户端偶尔打错密钥。
判定用的是「失败多且几乎没有成功」。

## 7. 离线/离职核查清单

1. `jms_user_access {"user":"<人>"}` —— 他能碰哪些资产
2. `jms_user_groups_list` —— 他在哪些组里，组持有哪些授权
3. `jms_asset_audit` —— 关键资产的实际可用状态
4. `jms_permission_get` —— 逐个授权看 `accounts` 是不是空
5. `jms_operate_logs` —— 他最近改过什么
6. 需要时 `jms_user_set_active {"user":"<人>","active":false,"confirm":true}`
   （停用会阻断新会话，不删除数据）
