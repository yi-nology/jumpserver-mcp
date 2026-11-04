---
name: jms-onboarding
description: |
  Bring a machine under JumpServer bastion control: register the asset, attach a login
  account, grant a permission, create the user, register an RSA SSH key, and file the
  asset in the tree. Use when onboarding / registering / 登记 / 上线 a host, when a machine
  fails to connect with "no authorized account or terminal protocol", or when fixing a
  bastion asset's protocols, accounts, permissions or access keys. Covers the four
  silent failures that only show up at login time.
descriptions:
  zh-Hans: "把机器纳入 JumpServer 堡垒机：建资产、加账号、授权限、建用户、注册 RSA 公钥、归入资产树。涉及资产登记/上线/登记新机器/连不上提示没有授权账号时使用。"
displayNames:
  zh-Hans: "堡垒机资产上线"
---

# 堡垒机资产上线

**目标状态**：这台机器能被堡垒机连上，而且**指定的人**能登进去。

四步都做完才算上线。**漏任何一步都不会立刻报错**，而是等到有人登录那一刻才炸，
且症状长得像「凭据错了」—— 排查方向整个跑偏。

---

## 1. 标准流程（按顺序）

```bash
# ⓪ 先查这台机器在不在 —— 别重复登记
jms_assets_list {"search":"192.0.2.71"}
#   查到了 → 它已经登记过：改协议/加账号走 jms_asset_update / jms_account_create，
#            别再建一个（同名同址的第二个资产是清理噩梦）
#   没查到 → 往下走

# ① 建资产 —— protocols 必填！
jms_asset_create {"name":"node-071","address":"192.0.2.71","platform":"Linux",
                  "protocols":["ssh","sftp"],"confirm":true}

# ② 加账号 —— 机器上的登录凭据
jms_account_create {"asset":"192.0.2.71","account":"opsuser",
                    "secret":"<密码>","confirm":true}

# ③ 授权限 —— 谁能登哪台、用哪个账号
jms_permission_create {"name":"团队-研发机","users":["opsuser"],
                       "assets":["192.0.2.71"],"all_accounts":true,"confirm":true}

# ④ 建用户 —— 堡垒机登录账号（email 必填！）
jms_user_create {"user":"opsuser","name":"Ops User","email":"opsuser@example.com",
                 "password":"<初始密码>","confirm":true}

# ⑤ 注册公钥 —— 让它能免密登 koko（必须 RSA）
jms_user_sshkey_add {"user":"opsuser","public_key":"ssh-rsa AAAA...","confirm":true}

# ⑥ 归位 —— 放进资产树
jms_asset_move {"assets":["192.0.2.71"],"node":"/DEFAULT/studio/arm","action":"add","confirm":true}
```

**批量登记**（一次登记一批机器）：

```bash
jms_assets_bulk_create {"assets":[
  {"name":"node-072","address":"192.0.2.72","platform":"Linux","protocols":["ssh"]},
  {"name":"node-073","address":"192.0.2.73","platform":"Linux","protocols":["ssh"]}],
  "confirm":true}
```

每台**独立创建、独立报错** —— 一条写错不影响其他（错误里会列出可用平台名）。

**做完用 `jms_asset_audit` 验收**（一个调用给出全部 + 结论）：

```bash
jms_asset_audit {"asset":"192.0.2.71"}
```

它会直接给出 `status`（ok/warn/fail）、一句 `verdict`，以及逐项的
`checks[]`——每项带 `detail` 和能修它的工具名，把上面四类静默失败点名。

---

## 2. 四个静默失败

| 现象 | 根因 | 修法 |
|---|---|---|
| 登录报「没有可用的授权账号或终端协议」 | 资产没 protocols / 没账号 / 授权的 accounts 是空列表 | ①②③ |
| 登录报认证失败（像密码错） | 端口配错（22 被占时配 2222） | `jms_asset_update` 的 `protocols:["ssh:2222"]` |
| koko 登录 `Permission denied (publickey)` | 密钥是 ed25519（koko 只认 RSA），或公钥没进 `SSHKey` 表 | ⑤，且必须是 `ssh-rsa` |
| 用户建不出来 | v5 硬性要求 email | ④ |

### 授权的账号粒度

```json
{"all_accounts": true}          → 该资产的全部账号（字面量 @ALL）
{"accounts": ["opsuser"]}       → 只有 opsuser（最小权限）
（都不传）                       → 一个账号都没授予 —— 登录必失败
```

⚠️ `accounts` 传**账号名**（`"opsuser"`），传 UUID 不报错但等于什么都没授。
⚠️ `jms_permission_grant` 是**只增不减**的：给 `@ALL` 的授权追加具名账号时 `@ALL` 保留。
**收窄授权是另一个动作**，本套工具不提供。

### 资产树的两个坑

- 节点的**可写字段是 `value`**（`name` 是只读派生字段）。`jms_nodes_create` 已经处理好了。
- 节点里**还有资产时删不掉**（403）。先把资产移出再删。

---

## 3. 已有资产的重整

```bash
# 补协议（资产建的时候漏了）
jms_asset_update {"asset":"192.0.2.71","protocols":["ssh:2222","sftp:2222"],"confirm":true}
# 改密码 / 停用账号
jms_account_update {"asset":"192.0.2.71","account":"opsuser","password":"<新密码>","confirm":true}
jms_account_update {"asset":"192.0.2.71","account":"opsuser","active":false,"confirm":true}

# 停用账号的干净修法
# 「认证失败」常常只是一个密钥不匹配的账号在拖累 —— 停用它，
# 堡垒机就不再拿它去试、也不再把资产标成故障。

# 给新机器追加授权（追加，不替换）
jms_permission_grant {"permission":"团队-研发机","assets":["192.0.2.71"],"confirm":true}

# 盘点机器上的账号漂移
jms_gathered_accounts {"asset":"192.0.2.71"}
#   present:false = 你管理的账号在机器上消失了
jms_gather_run {"asset":"192.0.2.71","confirm":true}
jms_gather_apply {"asset":"192.0.2.71","confirm":true}
```

---

## 4. 走不通的时候

| 症状 | 先查 |
|---|---|
| 建资产报 "cannot be created directly" | v5 按 category 分端点，工具已自动选；报错说明平台名不认识 |
| 建用户 400 `{"email":[...]}` | v5 硬性要求 email |
| 建用户 400 `{"username":["... already exists"]}` | 重名 |
| 授权里点名的账号不存在 | `jms_asset_audit` 会点名；`jms_permission_get` 看 accounts 字段 |
| 资产连不上 | `jmsx_reachable` → 不通则 `jms_connectivity_check` 看协议与连通性 |
| koko 登不上 | `jumpserver-exec --doctor` 的 `key_type`（必须 ssh-rsa） |

**v5 API 的完整坑表**见
[`jumpserver-ops/references/v5-api-traps.md`](../jumpserver-ops/references/v5-api-traps.md)。

---

## 5. 安全约束

- 写操作全部要 `confirm=true`。
- **删资产不可逆**，会连它的账号一起删。
- **删授权不可逆**，所有被授权的人同时失去访问。
- 密码是**只写**的：设进去就再也读不回来，只能靠回读记录（`date_password_last_updated`）
  判断是否生效。
- 高危操作用 `jms_delegation_call`（一次性绑定凭据，60 秒过期）。
  它的 `method` 是参数：读（GET/HEAD）不用确认，写（POST/PATCH/PUT/DELETE）
  必须带 `confirm=true`，否则工具会回一句「would change the bastion」并停手。
