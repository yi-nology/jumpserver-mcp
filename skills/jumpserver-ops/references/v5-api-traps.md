# 共享参考：JumpServer v5 的 API 坑表

> 实测于 2026-10-25，堡垒机 `http://192.0.2.41`（v5）。**每一条都对着线上验过**，
> 不是从 OpenAPI schema 推的 —— schema 不可信，字段形状骗人。

## 1. 认证：只有 PrivateToken 能长期用

v5 砍掉了经典的 AccessKey/Bearer 长期认证。长期可用的只有：

```
Authorization: Token <PrivateToken>
```

`POST /api/v1/authentication/tokens/` 拿的 OAuth token **24 小时过期**，
对长跑的 MCP 进程没用。

## 2. 写接口的字段形状，按 endpoint 而非按字段名

同一个字段名在不同端点要不同形状，且**拒绝时不告诉你该用哪个**：

| 端点 | 形状 |
|---|---|
| `/perms/asset-permissions/` | `users` / `assets` / `nodes` / `accounts` → **普通数组** |
| `/acls/command-filter-acls/` | `users` / `assets` → **envelope** `{"type":"ids","ids":[...]}`；`accounts` / `command_groups` → 普通数组 |

envelope 里的 `ids` 是 **UUID 数组**。

## 3. 列表端点会把关联对象返回成 null

`GET /perms/asset-permissions/` 的每行里 `assets` / `users` / `user_groups` 全是 `null`，
只有 `assets_amount` / `users_amount` 这些**计数**是真的。
`GET /acls/command-filter-acls/` 同理（`users` / `assets` / `accounts` / `command_groups` 全 null）。

⇒ **要判断"某授权覆盖某资产吗"，必须取详情** `GET /perms/asset-permissions/{id}/`。
直接用列表会得出"没有任何授权覆盖这台资产"的错误结论。

## 4. 资产权限的 `accounts` 传的是**账号名**

`AssetPermission.accounts` 是**账号名**（`["opsuser"]`），不是账号 UUID。
传 UUID **不报错**，但授权等于什么都没授。

- `@ALL` 是「该授权下这些资产的**全部账号**」的字面量。
- **空列表 ≠ 全部账号**，是「一个账号都没授予」。

## 5. PATCH 关联字段是「替换」不是「追加」

给授权加用户/资产/账号时，PATCH 会**整个替换**该字段。
必须**读-合并-写**：先 GET 详情拿到现有集合，合并后再 PATCH。

**追加绝不能收窄范围**：已经是 `@ALL` 的授权，追加具名账号时必须保留 `@ALL`
（并集的结果就是 `["@ALL"]`）。把 `@ALL` 换成具名账号 = 静默撤权。

## 6. 节点（资产树）的可写字段是 `value`

`name` 是**派生的只读字段**。`POST /assets/nodes/ {"name":"x"}` 会建出一个
**没有名字**的节点，看起来跟根节点一模一样。

```json
{"value": "arm", "full_value": "/DEFAULT/studio/arm"}
```

## 7. 节点里有资产时删不掉

`DELETE /assets/nodes/{id}/` → `403 {"error":"Deletion failed, the node contains assets"}`。
先把资产移出（`PUT /assets/nodes/{id}/assets/remove/`）再删。

移出后资产回到它原来的节点（不是被删掉）。

## 8. 用户公钥不在 `User.public_key` 字段里

往那个字段 PATCH 会返回 200、`can_public_key_auth` 报 True，但**什么都没存**
（实测：`public_key` 仍为 `null`，`has_public_keys` 仍为 `False`），
密钥也**登不上 koko**。

真正生效的路径是 `POST /authentication/ssh-key/`，它写独立的 `SSHKey` 表。

⚠️ **但那个端点永远给「调用凭据所属的账号」加密钥**，body 里的 `user` 字段被**静默忽略**
（实测：用长期 token 建出来的是 admin 的密钥，用委派凭据建出来的是 ai 的密钥）。

⇒ 想给某个用户加公钥，只能**用那个用户自己的凭据**注册，或走堡垒机 UI。
本机靠**委派凭据**（身份 = `JMS_DELEGATION_USER_ID` = `ai`）给自己加。

## 9. `private_key` 字段同样是假的

给账号 PATCH `private_key` 返回 200 且 `secret_type` 也变了，但私钥没写进去。
**`secret`（密码）字段能正常写** ✓。
想配 ssh_key 账号只能在堡垒机上用 ORM 写。

## 10. 账号创建

`POST /accounts/accounts/`：

```json
{
  "name": "opsuser", "username": "opsuser",
  "secret": "the-password",
  "secret_type": {"value": "password"},
  "asset": "<资产 UUID 字符串>",
  "privileged": false, "is_active": true
}
```

- `asset` 传**资产 UUID 字符串**，不是对象。
- `secret_type` 是**带标签的选择**：`{"value":"password"}`。传裸字符串 `"password"` 是错的。
- 密码字段写完**永远读不回来**，所以只能靠回读记录（`GET /accounts/accounts/{id}/`）来验证。

## 11. 用户创建：`email` 必填

`POST /users/users/` 漏掉 email → `400 {"email":["This field is required."]}`，
**且不会创建任何用户**。错误信息不提示别的可能原因。

重名 → `400 {"username":["User with the same Username already exists. ..."]}`。

密码策略在这台堡垒机上**没有强制**（`"123"` 也能建），但别的部署可能有。

## 12. 改密码

`PUT /users/users/{id}/password/` body `{"password":"..."}`。

⚠️ `date_password_last_updated` **只有秒级精度** —— 写入与回读落在同一秒内，
和「根本没写」无法区分。判据要么跨过秒边界再写，要么换字段。

## 13. 标签不能 POST `/labels/labeled-resources/`（永远 500）

`LabeledResource.save()` 要求 `org_id`，create serializer 不填。正确路径是 UI 那条：

```
PUT /labels/labels/<label_uuid>/resource-types/<ct_id>/resources/
body: {"res_ids": [...]}
```

- **尾斜杠不能少**（少则 301）
- **是全量替换**：会先删掉不在 res_ids 里的 ⇒ 必须读-合并-写
- `ct_id` 从 `/rbac/content-types/` 查 `model=="asset"`（本机 6）

## 14. 账号发现不能 POST `/accounts/gathered-accounts/`（永远 500）

同样的 org 问题。真正触发的是 **GET**
`/accounts/gather-account-executions/adhoc/?asset_id=<uuid>`，返回 HTML 报告。

采纳发现结果也不能 PATCH 单条 status，要 `PUT /accounts/gathered-accounts/status/`
body `{"ids":[...],"status":"1"}`（服务端才调 `sync_accounts()`）。

## 15. 五个集合端点返回裸数组，不是 `{count, results}`

`/labels/labels/`、`/labels/labeled-resources/`、`/rbac/org-roles/`、`/rbac/content-types/`。
解到 map 会报 `cannot unmarshal array into Go value of type map[string]interface {}`。

labeled-resources 的过滤参数是 **`label__name` / `label__value`**，不是 `name`/`label`。

## 16. `/rbac/org-roles/{id}/permissions/tree/` 是平的

每行只有自己的 `title` + 父节点 `pId`，**没有 children**。按 children 递归会静默返回空列表。

## 17. 成功写入可能返回 204 空 body

`PUT /accounts/gathered-accounts/status/` 和所有 DELETE 都是。
解 JSON 会报 `unexpected end of JSON input`，把成功报成失败。

⚠️ **反过来 204 也什么都没证明** —— 所以工具一律**回读验证**。

## 18. 命令组与命令过滤器

- 命令组 `POST /acls/command-groups/`：`content` 是**每行一条命令**；
  `type` 缺省 `"command"`；`ignore_case` 缺省 `true`。
- `type="command"` 的匹配规则（堡垒机自己的说明）：
  **每行作为独立词出现在输入任意位置即命中** ——
  `"rm"` 命中 `rm -rf` 和 `sudo rm`，**不**命中 `rmdir`。
- `type="regex"` 每行是一个正则。
- 过滤器的 `users` / `assets` 为空 = **对所有人 / 所有资产生效**；
  非空 = 只对列出的那些生效。
- `action`：`reject` 拒绝 / `accept` 允许 / `warning` / `notify_and_warn` 记录并告警。
- 列表端点把 `users`/`assets`/`accounts`/`command_groups` 全返回 null ⇒ **必须取详情**。

## 19. 资产分类决定它需不需要终端协议

`category` 字段：

| category | 平台 | 需要 protocol / account 吗 |
|---|---|---|
| `host` | Linux / macOS / Windows / Gateway / 数据库 | **需要** |
| `web` | Website | 不需要（走 HTTP） |
| `cloud` | Kubernetes | 不需要（走 API） |

⚠️ 把 `web`/`cloud` 资产报成「没有协议、永远连不上」是**误报** ——
本机曾因此报出 6 条假警报，而那 6 个服务全都好好地在跑。

## 20. `assets_amount` 只数「直接点名的资产」，节点继承的资产不算

授权可以有两种范围：**直接点名的资产** 和 **资产树节点**。
而 `assets_amount` **只数前者**。

实测（本机，2026-10-25）：

```
授权「所有」   assets_amount = 16   nodes = [home, tencent, arm, x86]
                 但它实际覆盖 45 台资产
```

那 4 个节点各自持有 23 / 1 / 3 / 14 台，合计 41 个位置，
其中 3 台又与直接点名的 16 台重叠。**真实覆盖 45 台**（49 台里的 45 台）。

⇒ **用 `assets_amount` 做权限审计会得出完全错误的结论**，
而且错得很自然：一个报「16」的授权看起来像个窄授权。

⇒ 展开方式：节点的**祖先关系藏在 `key` 里** ——
`key` 是索引路径（`"1"` = /DEFAULT，`"1:0"` = /DEFAULT/studio，
`"1:0:1"` = /DEFAULT/studio/arm），**前缀即祖先**
（注意 `parent_id` / `parent` 字段在每个节点上都是 `null`，不能用）。

⇒ 展开后的数字要与堡垒机自己的有效权限视图
（`/perms/users/{uuid}/assets/`）**逐一对上**才算数。

## 21. 嵌套节点：父节点的授权覆盖子节点的资产

资产只会列出**自己所在的叶子节点**（在 /DEFAULT/studio/arm 下的资产，
`nodes` 里只有 `arm`）。所以「这个授权覆盖这台资产吗」不能直接比节点 id ——
**授权挂在 /DEFAULT/studio 时，它覆盖 /DEFAULT/studio/arm 下的所有资产**。

只比直接节点 id ⇒ 子节点里的资产会全部漏判 ⇒ 审计里显示「没人能访问」。

## 22. 委派凭据的身份

`X-JMS-AI-Delegation` 请求在堡垒机侧的身份是
**`JMS_DELEGATION_USER_ID` 指向的用户**（例如 `ai`，即你自己的那个账号），**不是** token 的所有者（admin）。

它**不额外授权**（走正常 RBAC），只是「一次性 + 绑定 method/path/query/body + 60 秒」。

用法：删除、改权限、重置密码等**不想长期可达**的操作。

## 23. 资产字段有**两种形状**，用错的那个恒为 null

同一个概念「这条记录属于哪台机器」，堡垒机给出两种完全不同的形状：

| 端点 | 形状 |
|---|---|
| `/assets/assets/`、`/accounts/accounts/`、`/accounts/gathered-accounts/` | **嵌套**：`{"asset": {"id": "...", "name": "...", "address": "..."}}` |
| `/terminal/sessions/`、`/terminal/commands/` | **扁平**：`{"asset": "name(address)", "asset_id": "..."}` |

实测（2026-10-25）：终端会话端点返回的是
`"asset": "node-031(198.51.100.31)"` 这个**字符串**，资产 id 在**平级的 `asset_id`** 上。

⇒ **对扁平行用「取嵌套字段」的方式读，结果恒为 null**。
本仓因此有四个工具长期报不出资产：会话列表、命令搜索、命令审计、活跃会话 ——
而且**按资产过滤会静默失效**（拿 null 去比 id，要么全空、要么全过）。

⇒ 读的时候要**兼容两种形状**：有嵌套对象就用它，是字符串就拆
`name(address)`（地址里也可能带括号，如 `https://host:6443`，所以名字取**第一个** `(` 之前，
地址取**最后一个** `)` 之后）。

⚠️ **命令端点没有 `asset_id`** —— 只有 session id。
所以「某条命令属于哪台机器」**只能顺着 session 查**，没有别的路。

## 24. 会话行上的 `has_command` 恒为 false，`has_replay` 也不代表录像有内容

`/terminal/sessions/` 每行都有 `has_command`、`command_amount`、
`has_replay`、`replay_size` 和 `error_reason`。**前两个会互相矛盾，
第三个会骗人**。

实测（2026-10-25，本机 617 个会话）：

| 口径 | 数字 |
|---|---|
| `has_command = true` | **0**（而 `command_amount>0` 的有 115 个） |
| `has_replay = true` | 614 —— 但其中 **518 个是 `error_reason=replay_unsupported`** |
| `error_reason` 为空（= 真有交互） | **98（15%）** |
| 那 98 个里有命令记录 | **91（92%）** |

⚠️ **`has_replay = true` 只说明录像文件存在**，那 518 个 `replay_unsupported`
会话只录下了 koko 握手的字节 —— 文件在，但没有可读的交互。
用 `has_replay` 统计「录像覆盖率」会得到 99.5% 这种**看着很美、其实假的**数字。

⇒ **判断一个会话有没有审计价值，看 `error_reason` 是否为空**。
`replay_unsupported` / `connect_failed` 这类值都是「这次连接没成事」。

⇒ **命令审计本身是健康的**：有价值的会话 92% 有命令记录。
所谓「覆盖率极低」是分母里灌了 518 个握手半成品造成的假象。

⚠️ `has_command` 恒为 false ⇒ 别用它判断「有没有命令记录」，
用 `command_amount` 或直接数 `/terminal/commands/` 的行数。

⚠️ **统计口径本身也要能被证伪**：本轮一度把「84 个 session 只有 1 个带命令
记录」写进 skill 当事实 —— 84 是**被 limit 截断**的会话数，1 是用坏字段数出来的。
两个错误叠加，结论看着像个可信的实测值。

## 25. 登录日志没有资产维度

`/audits/login-logs/` 的字段是
`username` / `ip` / `city` / `backend` / `status` / `datetime` / `user_agent`。

⚠️ **没有 `asset` 字段** —— 它记录的是「谁登录了堡垒机」，不是「谁登了哪台机器」。
⇒ **登录日志无法并进按资产的接触史**。想回答「谁碰过这台机器」，
唯一可靠的路径是会话（`/terminal/sessions/`），录像挂在会话上。

⚠️ 别拿 `/audits/operate-logs/` 的字段名去读登录日志：operate 用
`remote_addr` / `datetime`，login 用 `ip` / `datetime`，读错会得到**一片 null**。

## 26. `page_size` 会被端点忽略，limit 必须在客户端生效

`page_size=3` 打 `/terminal/sessions/`，回来的还是 **200** 条（端点自己的上限），
**不报错、不提示**。加上 `limit` 才会切到 DRF 真分页（返回 `{"count","results"}`）。

⇒ **在客户端截断**。只发 `page_size` 的工具，`limit` 参数是**假的** ——
调用方传 limit=3 会拿到 200 条。
（本仓因此有两个工具的 limit 形同虚设：登录日志、授权列表。）

⇒ 拉全量用 `listAll`（`limit=1000` + `offset` 翻页），别用单发 `page_size`。



## 27. 打了标签的资源，`resource` 是一串显示串，不是对象

`/labels/labeled-resources/` 的一行长这样：

```json
{"label":{"id":"…","display_name":"env:prod"},
 "res_type":{"value":6,"label":"Asset"},
 "res_id":"42e1a002-…",
 "resource":"node-071(192.0.2.71)"}
```

- **`resource` 是字符串 `"名字(地址)"`**，不是对象 ⇒ 名字和地址要自己拆
  （本仓用 `jmsapi.SplitAssetLabel`）。
- `res_type` 是 `{value,label}` 对：**要显示名就用 `label`（"Asset"）**，
  用 `value` 会拿到 `"6"`（那是个内容类型 id，模型读不懂）。
- ⚠️ **别去问第二个端点要名字和地址**：
  `GET /labels/labels/<label_uuid>/resource-types/<ct_id>/resources/?bound=1`
  只回 `{"id","name"}`，而那个 `name` **还是同一串显示串** —— 多花两次请求，
  换来一份重复的名字和一个空地址（`address` 字段压根不存在）。

## 28. 账号自动化：四族，两族要许可证，403 长得像缺权限

堡垒机有**四族**账号自动化，各有各的端点前缀，**不是互换的**：

| 族 | 端点前缀 | 社区版实测 | 能做什么 |
|---|---|---|---|
| **push** | `/accounts/push-account-*` | ✅ 200 | 堡垒机生成密码 → 推到机器上**还不存在**该名字的账号 → 存进自家 vault |
| **gather** | `/accounts/gather-account-*` | ✅ 200 | 登录机器 → 发现上面的账号 |
| change-secret | `/accounts/change-secret-*` | ❌ **403** | 轮换**已持有**账号的密码 |
| check-account | `/accounts/check-account-*` | ❌ **403** | 校验已持有的密钥还能不能用 |

⚠️ **那两个 403 不是「缺权限」**：上游是
`permission_classes = [RBACPermission, IsValidLicense]` ⇒ **功能未授权**
（`settings.XPACK_LICENSE_IS_VALID` 为假）⇒ **角色里给了权限也没用**。
本机 token 属主是 superuser（带全部系统角色），照样 403。
⇒ 排查时**别去加角色**，加了不起作用；要判「是缺权限还是缺许可证」，
**看上游那个视图有没有 `IsValidLicense`**。

⚠️ **push 不轮换已有账号的密码**（上游 `gen_nonlocal_accounts`）：
`create_usernames = set(要建的) - set(机器上已有的)` ⇒ **已存在的账号它不动**
⇒ 「换密码」是 change-secret 的活（许可证门槛）。
⇒ **能免密钥过 agent 的只有 push 与 gather 两族**。

## 29. 账号自动化：只有 push 有「每个账号的结果」

`/accounts/push-account-records/`（200，每账号一行 PushSecretRecord）。
⚠️ **`/accounts/gather-account-records/` 不存在**（实测 **404**，上游也没注册它）
⇒ **端点路径不能按名字推**，看着该有的那个没有。
gather 的结果是「**发现的账号**」⇒ `/accounts/gathered-accounts/`
（= `jms_gathered_accounts` 读的那份，`asset` 是**嵌套对象**）。

⚠️ **执行记录随计划一起删**：删掉计划，它跑过的 execution 也没了
（`AutomationExecution.automation` 是 `on_delete=CASCADE`）。

## 30. 账号自动化：执行是异步的，失败原因 API 不给

```
POST /accounts/push-account-executions/  {"automation": "<计划id>"}
  → 201 {"task": "<celery task id>"}
```

- ⚠️ **那个 task id 就是 execution id**（上游用 `current_task.request.id` 建 execution）
  ⇒ 拿它就能直接查那次执行。
- ⚠️ **execution 的 serializer 不暴露 `result`/`summary`** ⇒ 失败只说「failed」，
  **失败原因从 API 拿不到** ⇒ 答复必须说清这一点，否则调用方会以为「failed = 没做事」。
- ⚠️ **「连一条 record 都没有」本身就是信息**：堡垒机是**开始动某台机器时**才建 record 的
  ⇒ 一条都没有 ⇒ **失败在准备阶段，一台机器都没到**（多半是堡垒机自己的
  ansible/celery 组件没起）⇒ **答复要说这个**，否则调用方会跑去检查目标机。
- ⚠️ **状态会停在 `running`**：gather 触发后几十秒内还是 running（后台在跑），
  别把「还没结束」当成「卡住了」。

## 31. 建 push 计划：少 `password_rules` 会 400，且不说哪个字段错

```
POST /accounts/push-account-automations/
{"name":"…","accounts":["opsuser"],"secret_type":"password",
 "secret_strategy":"random",
 "password_rules":{"length":16,"lowercase":true,"uppercase":true,
                   "digit":true,"symbol":true,"exclude_symbols":""},
 "assets":["<资产id>"],"is_periodic":false}
  → 201
```

⚠️ **少 `password_rules` ⇒ 400 `["* Please enter a valid password length"]`** ——
一句话里有三个星号，**看不出是哪个字段错**（是 length？是规则集本身？）。
⚠️ `secret_type` / `secret_strategy` 要**值**（`password` / `random`），
**不是标签**（`Password` / `Randomly generate`）⇒ 读序列化结果时用
`jmsapi.Label`（取 value），写请求体时用**值**。
⚠️ `assets` 要**资产 id**，不是名字/地址。
⚠️ `accounts` 是**账号名数组**（不是 id）：上游注释写的是
"Please enter your account username"。
⚠️ **`length` 上限 36、下限 8**（上游 `PasswordRulesSerializer`）。
⚠️ **gather 族不需要 `secret_type`/`secret_strategy`/`password_rules`**
（它的 serializer 只有 `is_sync_account`/`check_risk`）⇒ 发过去也只是被忽略。

## 32. 账号自动化：两族的 `accounts` 字段意思**不一样**

同一个字段名，在两族里是两件事：

| 族 | `accounts` 是什么 | 必填？ |
|---|---|---|
| **push** | **要在机器上「创建」的账号名** | ✅ 必填（上游 `extra_kwargs` 标了 required） |
| **gather** | **不参与执行** —— gather 用「这些资产上已有的账号」去登录发现（`Account.objects.filter(asset__in=assets)`） | ❌ 不必填 |

⇒ **别给 gather 计划传 accounts**：那会让调用方以为「要发现哪些账号」，
而实际上它压根不读这个字段 ⇒ 传了等于没传，还误导人。
⚠️ gather 的「要发现什么」不是计划里的字段，而是**跑完之后的发现结果**
（`/accounts/gathered-accounts/`）。

⚠️ **push 与 gather 的 `secret_type` 也不对称**：
push 的 serializer 要 `secret_type`/`secret_strategy`/`password_rules`；
gather 的只要 `is_sync_account`/`check_risk` ⇒ 发过去也只是被忽略。

## 33. 网络拓扑：区域是资产上的字段，「网关」是平台名

`/assets/zones/` 与 `/assets/gateways/`（**v4 与 v5 路径逐条一致**，见上游
`apps/assets/urls/api_urls.py`）。

- ⚠️ **区域（zone）是资产上的一个外键**（`Asset.zone`，可空）⇒
  「某区域有哪些机器」用 `/assets/assets/?zone=<id>` 查；zone 自己的记录里
  只有**计数**没有列表。
- ⚠️ **「Gateway」是平台名，不是角色**：本机那 7 个里只有 2 个真归了区，
  其余是路由器 / WiFi AP / nginx 前端 ⇒ **「网关」≠「跳板」**。
  判它是不是跳板看**它有没有 zone**。
- ⚠️ **zone 的 `gateways` 是对象数组** `{"id","name"}`（不是名字数组）
  ⇒ 拿名字要自己取 `name`（本仓 `zoneGatewayNames`）。
  ⚠️ 而**网关自己的 `zone` 是嵌套对象**，且**大多数是 null**。
- ⚠️ **`zone.assets_amount` 排除网关资产**（上游 `self.assets.gateways(0).count()`
  = 该区里平台**不是** Gateway 的资产数）⇒ 那是「区里有多少台机器」，
  不是「区有多大」。
- ⚠️ **`/assets/zones/?gateway=<id>` 不会带密钥**（实测响应形状不变），
  但上游那个分支的 serializer 是 `GatewayWithAccountSecretSerializer`
  （**带账号 secret**）⇒ **别依赖这条路径**，也不要用它去取密钥。

## 34. 嵌套字段为 null 时，`convert.AnyToStr` 会给出 "<nil>"

`convert.AnyToStr(nil)` = `fmt.Sprintf("%v", nil)` = **字符串 `"<nil>"`**，
而它**不是空串** ⇒ `if zone == ""` 这种判空**抓不住它**。

本轮真实踩到：`Asset.zone` 是 null 时，`jmsapi.Nested(row,"zone","name")`
返回 nil ⇒ 被当成「区域名叫 `<nil>`」⇒ **每台没归区的机器都被编出了一条假路径**。

⇒ **读嵌套字段一律用 `jmsapi.Str`**（它把 `<nil>` 归一成 ""），
**不要用 `convert.AnyToStr`**。本仓 `jmsapi.Str` 的注释写的就是这件事：
「一个不存在的字段必须读成不存在」。

⚠️ 同一个坑在**字段名拼错**时也一样：读不到 ⇒ nil ⇒ `"<nil>"` ⇒
看起来像有值。

## 35. ACL 三族的 `users`/`assets` 是**选择器对象**，不是列表

`/acls/login-acls/`、`/acls/data-masking-rules/`、`/acls/clipboard-acls/`
的 `users` 与 `assets` 是 JSONManyToMany **选择器**：

```json
{"type":"all"}
{"type":"ids","ids":["<uuid>", …]}
{"type":"attrs","attrs":[{"name":"ip","match":"exact","value":"1.1.1.1"}]}
```

- ⚠️ 传 `["*"]` 或裸 id 列表 ⇒ **400**，且错误文本**只描述选择器形状、不说是哪个字段**
  ⇒ 400 里没字段名是这条 API 最差的提示。本仓工具把选择器做成**类型化入参**
  （`jmsms.ACLSelector`）+ 请求前形状校验（`validateSelector`，错误指名字段）。
- ⚠️ `{"type":"ids"}` 带空 `ids`、`{"type":"attrs"}` 带空或缺 name/value 的 attrs
  都会在请求前被拒 —— 服务端会把它们读成「匹配不到任何东西」，
  那永远不会是调用方的本意。

## 36. `action`/`masking_method`/`operations` 的出参是**标签对**，入参是字符串

出参（真机字节）：`"action":{"value":"reject","label":"Reject"}`、
`"masking_method":{"value":"fixed_char","label":"Fixed character replacement"}`、
`"operations":[{"value":"copy","label":"Copy (RDP, VNC)"}, …]`。
入参：`"action":"reject"`（裸字符串）、`"operations":["copy","paste"]`。

- ⚠️ 出参要**拍平回 value**（本仓 `jmsapi.Label(m, key)`）—— value 才是
  update 能原样喂回去的东西；label 是给人读的。
- ⚠️ 词表**每族不一样**（上游 serializer 无条件排除，与许可证无关）：
  login-acl / data-masking 认 `reject/accept/review/notice`，
  **clipboard 只认 `reject/accept`**，`operations` 只认 `copy/paste`。
  把 `review` 传给 clipboard 会被拒。
- ⚠️ `rules.time_period` 的元素是 `{"id": <周几>, "value": "HH:MM~HH:MM、…"}`，
  **id 0 = 周日 … 6 = 周六**；某天 `value` 为空 = 那天**不允许**，
  列表为空 = **没有时间限制**。把「空 = 不限」按天读会写出一条全拒的规则。

## 37. RBAC 权限目录：GET 双形态 + `?role=` 过滤是真的

`/rbac/permissions/`：

- ⚠️ **不带参数 = 裸数组**，**带 `?limit=` = `{count,next,previous,results}` 信封**
  —— 同一端点两种形状（分页包装全局关了，但只要参数里出现 limit 就有信封）。
  本仓 `ListAllCounted` 翻页带 `limit=1000`，看到的是信封。
- ✅ `?role=<id>` **真的按角色过滤**（org 与 system 角色都实测过），
  不像别的端点那样静默忽略未知参数。
- ⚠️ 行里的 `content_type` 是**嵌套对象** `{"id","app_label","model"}`，
  平铺读它是 null（同坑 34）；`id` 是**数字**。
- ⚠️ 角色列表有两个端点：`/rbac/org-roles/` 与 `/rbac/system-roles/`。
  权限目录用 `?scope=org|system` 选（org 265 行 / system 385 行，本机实测）。
- ⚠️ **矩阵是每角色一请求**（`?role=` 逐个问）⇒ 角色多时很贵；
  「某权限谁持有」的 join 在工具侧做。死权限（没有任何角色持有）
  **保留在矩阵里** —— 那是发现，不是噪声。

## 38. 「我是谁」问 `/users/profile/`，不是用户列表第一行

- ✅ `GET /api/v1/users/profile/` **v4 与 v5 都有**，答的就是**调用者本人**
  （v5 实测 200 + token 属主的行；v4 路由在，401=只差认证，带认证实测 200 同形）。
- ⚠️ `/users/users/profile/` 与 `/me/` **在 v5 是 404** ——
  「v5 没有身份端点」这个说法是**只探了这两个**得出来的，是错的。
- ⚠️ ⚠️ **用户列表的第一行不是身份**：`GET /users/users/?page_size=1` 的
  第一行是「列表排序恰好排第一的用户」——调用者不是 admin 时，
  `jms_status`/`jms_doctor` 就把**调用者报成 admin**（v4 探针账号实测踩过：
  身份显示 admin、实际是 zz_mcp_probe）。列表行是**列表事实**，不是身份事实。
- ⚠️ 身份探不到时要**说探不到**（`jms_status.error` 说原因、doctor 报 WARN），
  不要退回列表启发式猜一个名字 —— 报错名字比不报更糟。
