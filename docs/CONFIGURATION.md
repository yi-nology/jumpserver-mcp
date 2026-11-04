# JumpServer MCP 管理面配置指南

管理面 MCP 服务（`jumpserver-mgmt`）的配置说明：
怎么接线、每个环境变量是什么、怎么自检、出问题怎么查。

> 本文面向**要装、要调、要修**的人。只想知道「用哪个工具」看
> [§6 工具选择速查](#6-工具选择速查)；只想跑起来，看 [§2 五分钟装好](#2-五分钟装好)。
>
> **执行面**（另一个服务）的配置在
> [jumpserver-exec 仓的 CONFIGURATION.md](https://github.com/yi-nology/jumpserver-exec/blob/main/docs/CONFIGURATION.md)。

---

## 1. 架构：管理面

```
       ┌──────────────────┐         ┌──────────────────┐
       │  jumpserver-mgmt  │  REST   │  JumpServer 部署  │
       │  (本服务, 97 工具)├────────►│                  │
       └──────────────────┘         └────────┬─────────┘
                                             │ koko :2222
       ┌──────────────────┐         ┌────────┴─────────┐
       │ jumpserver-mgmt-  │  SSH    │  受管机器          │
       │ exec (执行面,    ├────────►│  (录像可审计)      │
       │ 15 工具)          │         │                   │
       └──────────────────┘         └───────────────────┘
```

**铁律：AI 访问机器一律经堡垒机 koko(2222)，不接受直连 SSH。** 直连会丢审计录像。

| 服务 | 管什么 | 不管什么 |
|---|---|---|
| `jumpserver-mgmt`（本服务） | 资产、账号、权限、标签、命令过滤、审计、授权 | 不在机器上执行命令 |
| `jumpserver-exec`（执行面） | 在机器上跑命令、传文件 | 不改堡垒机配置 |

本服务读 `JUMPSERVER_URL` + `JUMPSERVER_TOKEN`，并且**共用同一份 REST 协议实现**（在
[jumpserver-kit](https://github.com/yi-nology/jumpserver-kit) 仓：
传输、重试、限流、v5 的两种列表形状、**端点表**）。
协议只有一份，所以在管理面修好的东西（比如按字符边界截断、避免切坏多字节字符）
执行面自动就有。

端点表也在同一处：`jmsapi.ResPermissions` 这样的常量**就是**堡垒机的 URL。
工具层只说「我要权限列表」，不说「GET /perms/asset-permissions/」——
所以改端点只改一个文件，写错路径编译器当场报错，机器上也跑不出 404。

---

## 2. 五分钟装好

### 2.1 前置条件（堡垒机侧）

| 需要的东西 | 怎么拿 | 缺了会怎样 |
|---|---|---|
| JumpServer 部署地址 | 你自己的部署，例如 `https://jumpserver.example.com`（`JUMPSERVER_URL`，**无默认值**） | 管理面直接启动失败并报「JUMPSERVER_URL is required」 |
| PrivateToken | 堡垒机 UI → 个人设置 → API 密钥 → 新建（v5 是「设置 → API 授权」） | 管理面全部 401 |
| 一个 JumpServer 用户 | 建议单独建 `ai` 账号，别复用真人账号 | koko 登录失败 |
| 该用户的 **RSA** SSH 公钥 | 见下方 2.2 | koko 拒绝登录（执行面受影响） |
| 给 `ai` 的资产授权 | `accounts` 必须是 `@ALL` 或显式账号 | 登录报「没有可用的授权账号或终端协议」 |

### 2.2 注册 SSH 公钥（最容易踩的一步）

koko（2222）**只接受 `ssh-rsa`**。ed25519 公钥堡垒机会「安静地接受」，
但 koko 一律拒绝 —— 症状是 `Permission denied (password,publickey)`，
看起来像凭据错，其实是密钥类型错。

```bash
# 1) 生成 RSA 私钥（如果还没有）
ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa

# 2) 拿到公钥内容
cat ~/.ssh/id_rsa.pub
```

注册方式有两条：

- **走 API**：用本服务的管理面工具 `jms_user_sshkey_add`（`confirm=true`）。
  它会写 `SSHKey` 表 —— 也就是真正生效的那张表 —— 并回读验证。
- **走堡垒机 UI**：个人设置 → SSH 公钥 → 粘贴。

⚠️ **不要往 `User.public_key` 字段写**。那个字段 PATCH 回来是 200、
`can_public_key_auth` 报 True，但**什么都没存**，密钥也登不上。

### 2.3 接线

服务是 **stdio** 传输，用 `mcp update` 指向运行时的二进制：

```bash
mcp update name=jumpserver command=<jumpserver-mgmt 的绝对路径>
```

`env` 是只写字段（写进去读不回来），一次性把变量都给全：

```bash
mcp update name=jumpserver \
  command=/usr/local/bin/jumpserver-mgmt \
  env='{"JUMPSERVER_URL":"https://jumpserver.example.com","JUMPSERVER_TOKEN":"<token>"}'
```

或者直接编辑你的 MCP 客户端配置文件（下例以 JSON 形式为准；不同客户端的路径不同）：

⚠️ **`env` 里的 token 不要提交进 git 仓**：

```json
{
  "mcpServers": {
    "jumpserver": {
      "type": "stdio",
      "command": "/usr/local/bin/jumpserver-mgmt",
      "env": {
        "JUMPSERVER_URL": "https://jumpserver.example.com",
        "JUMPSERVER_TOKEN": "<token>"
      }
    }
  }
}
```

### 2.4 自检（**接线完先跑这个**）

二进制有 `--doctor`，不依赖 MCP 会话，可以直接从 shell 跑：

```bash
jumpserver-mgmt --doctor    # 管理面接线 + 堡垒机配置体检
```

`doctor` 同时也是子命令形态（`jumpserver-mgmt doctor`，与 `--doctor` 等价），
`--help` 打印用法。退出码沿用 OEKIT `pkg/cli` 的约定：`0` = 正常、`1` = 有问题、
`2` = 用法错误（未知子命令 / 参数不合法）。

⚠️ **`--help` / `-h` / `help` 的退出码是 `0`，不是 `2`** —— 问帮助不是错误路径。
写 `prog --help || fail` 的脚本要注意这个。

---

## 3. 环境变量完整表

| 变量 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `JUMPSERVER_URL` | **是** | **无** | 堡垒机地址，不要带尾部斜杠（代码会 `TrimRight`），也不带 `/api/v1`。⚠️ **管理面必填**：缺了直接启动失败（报错文案带样例值） |
| `JUMPSERVER_TOKEN` | **二选一** | 无 | JumpServer **PrivateToken**，呈现为 `Authorization: Token <key>`。⚠️ **不是 v5 专有**：v4 也注册了 `PrivateTokenAuthentication`（就是 DRF 的 `TokenAuthentication`），所以同一行头在两版上是同一个意思。缺失时服务直接启动失败 |
| `JUMPSERVER_USERNAME` | **二选一** | 无 | 另一种凭据：**用账号密码登录**，堡垒机换成 Bearer token（`POST /api/v1/authentication/auth/`）。给拿不到 PrivateToken 的部署用。v4/v5 都有这个端点 |
| `JUMPSERVER_PASSWORD` | 跟 `JUMPSERVER_USERNAME` 配 | 无 | 上面那个登录的密码。⚠️ **优先读文件**（`~/.config/jumpserver-mgmt/password`，0600），环境变量是退路 —— 长跑进程的环境在 Linux 上是 `/proc` 里谁都能读的 |
| `JUMPSERVER_OTP` | 否 | 无 | base32 的 TOTP 密钥。**只在堡垒机要求二次验证时用得上**：没有它，登录会明说「部署要二次因子、没配 TOTP」而不是给一个看不懂的错 |
| `JUMPSERVER_VERSION` | 否 | `auto` | 堡垒机是 **v4** 还是 **v5**：`v4` / `v5` / `auto`（探测）。⚠️ 它只决定**一件事**：v4 **没有** Chat AI 委派方案 ⇒ 会话只能以 API 凭据的身份跑（= 管理员面）。其余（端点表、认证头、连接令牌体、响应形状）两版一致。详见 §3.5 |
| `JMS_READ_ONLY` | 否 | `false` | **只读模式**：所有写工具在执行前被拒（结构性拒绝，不是约定）。管理面拒掉 29 个写工具。用来「让 agent 只看不动」 |
| `JMS_REQUESTS_PER_SECOND` | 否 | `20` | 客户端对堡垒机的请求速率上限（令牌桶）。堡垒机是单体，失控的 agent 不该拿它练手 |
| `JMS_BURST` | 否 | `40` | 令牌桶的突发额度 |
| `JUMPSERVER_TIMEOUT` | 否 | `90s` | 单次 API 调用超时。连通性检查 50 台机器很慢，这个值是故意宽的 |
| `JMS_SERVER_NAME` | 否 | `jumpserver-mgmt` | 服务在门禁审计留痕里的标识（留痕的 `service` 字段） |
| `JMS_AUDIT` | 否 | `file` | 门禁留痕要不要**落盘**：`file`（每个进程一份 JSONL）或 `off`（只写 stderr） |
| `JMS_AUDIT_DIR` | 否 | `~/.local/state/jumpserver-mgmt/audit` | 留痕文件放哪（目录 0700、文件 0600）。两个服务写同一个目录 |
| `JMS_AUDIT_RETENTION_DAYS` | 否 | `7` | 留痕文件保留多少天。**0 或负数 = 不按时间清**（⚠️ 但**空文件**那条规则照样删：安静超过一小时的空文件与保留期无关）。⚠️ 保留期是**文件级**的：超过保留期**且进程已结束**的文件会被清理；正在运行的进程的文件不删 ⇒ 文件里的行可能比这个窗口更老 |

⚠️ **两种凭据都配了怎么办**：`JUMPSERVER_TOKEN` 赢 —— 它不过期，登录换来的 Bearer token 会过期，
长跑的进程不该靠续命登录活着。两个 `--doctor` 都会报这件事（`api_token` 一行）。

### 3.1 可选：Chat AI 委派凭据

| 变量 | 说明 |
|---|---|
| `JMS_DELEGATION_SECRET` | 堡垒机的 `CHAT_AI_DELEGATION_SECRET`（64 字节）。**别写进 MCP 配置** —— 那等于让能读该目录的每个进程都拿到它 |
| `JMS_DELEGATION_USER_ID` | 委派凭据以哪个用户身份执行。**管理面**不设则自动查 `username=ai` |
| `JMS_DELEGATION_ORG_ID` | 组织 id。不设则自动查 `DEFAULT` |

secret 的**推荐存放位置**（权限 0600）：

```bash
mkdir -p ~/.config/jumpserver-mgmt
chmod 700 ~/.config/jumpserver-mgmt
printf '%s' "$CHAT_AI_DELEGATION_SECRET" > ~/.config/jumpserver-mgmt/delegation.key
chmod 600 ~/.config/jumpserver-mgmt/delegation.key
```

程序会依次找 `JMS_DELEGATION_SECRET` 环境变量 →
`~/.config/jumpserver-mgmt/delegation.key` →
`~/.local/etc/jumpserver-mgmt/delegation.key`。

**委派凭据是什么**：一次性、单次、60 秒内有效、绑定
「方法 + 路径 + query + body」的凭据。它**不额外授权** —— 走的是正常 RBAC，
只是让「不想长期可达」的操作不留长期凭证。

⚠️ **关键性质**：委派请求在堡垒机侧的身份是
`JMS_DELEGATION_USER_ID` 指向的用户（例如 `ai`），**不是** token 的所有者。
所以 `jms_user_sshkey_add` 正是靠这一点，才能把密钥注册到 `ai` 名下
—— `POST /authentication/ssh-key/` 永远给「调用凭据所属的账号」加密钥，
body 里的 `user` 字段会被**静默忽略**。

### 3.2 另一种凭据：账号密码换 Bearer token

`JUMPSERVER_USERNAME` + `JUMPSERVER_PASSWORD`（v4/v5 都有这个端点）：

```
POST /api/v1/authentication/auth/  {"username": "...", "password": "..."}
  → {"token": "<不透明令牌>", "keyword": "Bearer"}
```

- 之后每个请求带 `Authorization: Bearer <token>`。
  ⚠️ **它不是 JWT**：上游发的是一串随机串，存在 Django 的缓存里 ——
  叫它 JWT 会让人去找根本不存在的 claims。
- **它会过期**，所以程序记住它、到期前换、**收到 401 就换一次并重发**。
  ⚠️ **只对「读」重发**（GET/HEAD/OPTIONS）。理由：401 通常是认证层拒绝，
  但上游**视图自己也会回 401**（会话已失效、命令没被接受），两类在网线上分不开 ——
  所以「写」永不因为 401 被重发：写两次的代价（改一次账号、撤一次权限）
  不该由一个启发式来决定。⚠️ 写操作拿到 401 就是**报错给调用方**。
- 部署要二次因子时：登录会答「要码」（HTTP **200** + `{"error":"mfa_required",...}`）
  ⇒ 程序拿 `JUMPSERVER_OTP` 生成 TOTP 码去验，**再重登一次**。
  ⚠️ **这一步靠的是 Django session，不是凭据** ⇒ 登录种下的 session cookie
  必须带到挑战那一步；程序为此给登录路径单独配了一个带 cookie jar 的 HTTP 客户端。

### 3.3 门禁留痕（落盘的那一份）

每次工具调用都留一行痕：调了哪个工具、参数是什么（已脱敏）、门禁每一段的判定与理由、
放行与否、耗时、出参字节数。**这份留痕要落盘**，原因是它记的是「在这里**尝试过**什么」——
而**被门禁拦下的写操作根本没到堡垒机**，所以堡垒机那边没有任何记录。
「谁试过改这个授权但没带确认」这个问题，只有本机的留痕答得了。

| | |
|---|---|
| 落在哪 | `JMS_AUDIT_DIR` 下的 `audit-<日期>-<pid>-<随机>.jsonl`，**每个进程一份** |
| 权限 | 目录 0700、文件 0600 |
| 格式 | 一行一个 JSON：`ts` / `service` / `tool` / `client` / `args` / `allowed` / `refused` / `dry_run` / `would_execute` / `failed` / `stages[]` / `duration_ms` / `out_bytes` / `error` / `delivery` |
| `service` | **服务名**：本服务是 `jumpserver-mgmt`（可用 `JMS_SERVER_NAME` 改） |
| `allowed` 的口径 | **「没有被拒绝」，不是「跑成功了」**。⚠️ **管理面的写操作没带 `confirm` 时，SDK 的输入契约校验先拒掉它** ⇒ 那一行 `allowed=false`、`failed=true`、`stages[]` **为空**（门禁没跑过） |
| `failed` | **这次调用的「活」干砸了**。⚠️ **门禁拒掉一次调用不算失败** —— 那是门禁在干活。⚠️ 管理面的 `jms_status` 属于这一类：它那一格是「**身份探针**没成」（无害，不是「有命令在机器上跑」） |
| `error` | **这次调用为什么以错误收场**：门禁拒的与工具失败的都装在这儿。⚠️ 想知道「这里什么坏了」用 `failed=true` 筛 |
| 脱敏 | **每一列自由文本都过同一套脱敏**：参数、错误、**拒绝理由**、门禁每一段的理由 |
| `args` 截断 | 超过 500 字节会截断，**切在字符边界上** ⇒ 切坏一个多字节字符，会让留痕里的参数**与实际调用的不是同一个字符串** |
| 怎么读 | `tail -f ~/.local/state/jumpserver-mgmt/audit/*.jsonl*`，或用执行面的 `jmsx_audit_query` 工具 |
| 留痕目录为空时 | 写侧与读侧说**同一句话**：没有目录 = **没有留痕文件** |

⚠️ **审计类工具的答复也走脱敏**：`jms_command_audit` / `jms_commands_search` /
`jms_asset_evidence` 把命令原文交回给 agent ⇒ 那条路径上的凭据会**进 agent 的上下文窗口**
（不进留痕，但一样是泄漏）。它与留痕**共用同一份词表**（在 kit 仓的 `pkg/redact`），
**形状各走各的**（答复保留键名与 scheme：`Authorization: Bearer [REDACTED]`；
留痕塌成 `****`）。⚠️ **形状不同不是漂，词表不同才是漂**。

### 3.4 资产上线标准流程（管理面自己的）

见 §4。

### 3.5 兼容 JumpServer v4

本服务**同时支持 v4 与 v5**。这件事值得说清楚，因为「兼容」听起来像是要维护两套代码，
而实际上不是 —— **协议层本来就是同一套**，只有一处真的不同。

#### 两版一致的部分

| | v5 | v4 | 出处 |
|---|---|---|---|
| API 前缀 | `/api/v1/` | `/api/v1/` | `apps/jumpserver/urls.py` |
| 端点表（40+ 条） | ✅ | ✅ **逐条同路径** | 各 app 的 `api_urls.py` |
| 认证头 | `Authorization: Token <key>` | **同一行** | v4 的 `PrivateTokenAuthentication` 就是 DRF 的 `TokenAuthentication` |
| 连接令牌的 `account` | 账号 UUID | **账号 UUID 也行** | v4 的 `get_asset_accounts_by_alias`：见到 UUID 就按 id 查 |
| 资产详情的 `accounts` | 有 | **有** | v4 的 `DetailMixin.accounts` |
| 资产树的 `full_value` | 有 | **有** | v4 的 `Node.full_value` |

⇒ **管理面 97 个工具在 v4 上都能用**，**不用改任何工具**。

#### 唯一真的不同：Chat AI 委派方案

**v4 根本没有 chat_ai 这个应用** ⇒ 后果是**会话的身份更宽**：
委派凭据是「一次性、绑定、60 秒」的窄凭据；v4 上没有它，
于是连接令牌只能用 API 凭据签 ⇒ **会话带的是那个凭据的权限**（本部署上是管理员），
而不是 `ai` 账号那个更窄的面。⚠️ 这是**部署提供给什么**的问题，不是配置能解决的。

#### 怎么告诉程序这是 v4

`JUMPSERVER_VERSION`：
- `auto`（默认）—— 启动后第一次需要时**探测一次**：问**两条**路径，比它们的答案。
  - 一条是 `/api/v1/chat-ai/runtime-store/`（**v5 注册了 chat-ai 前缀，v4 没有**）；
  - 一条是 `/api/v1/zzz-jms-version-probe-404/`（**哪一版都没有**，名字故意取得不可猜）。
  ⇒ **两条答案一样 ⇒ 那条路由不存在 ⇒ v4；答案不同 ⇒ 路由在 ⇒ v5。**
  ⚠️ **为什么不直接看状态码** —— v4 的「没这条路由」处理器**是坏的**（它回 **500**），
  所以 v4 上 `/api/v1/` 下**任何**不存在的路径**长得一模一样** ⇒
  「路由不在」这件事**没法**从状态码读出来，只能**比**。
  ⚠️ 探不到（堡垒机暂时不可达）时**不猜**：按「有这个能力」处理（保窄），
  **30 秒内不再问，之后再问** —— 猜错了的代价是**宽**或**慢**，
  这里选**窄**。
- `v4` / `v5` —— 显式指定，**优先于探测**，而且**一次请求都不用发**。

⚠️ **实测推翻了两条源码推断**（这正是「协议事实要实测，别抄」）：
① v4 的列表端点也是**裸数组**；② `perm_account_invalid` 在 v4 上**不是**
「账号字段不对」，而是**「没人被授权用这个资产」**—— 同一句话两个意思，极易误诊。
⚠️ **v4 没有 PrivateToken 的 API 出口** ⇒ **v4 上只能用「账号密码换 Bearer」这一种凭据**。

---

## 4. 资产上线标准流程

在堡垒机上登记一台新机器，缺一不可：

```
1. 建资产     jms_asset_create   ← 必须带 protocols！否则永远连不上
2. 加账号     jms_account_create ← 没有账号 = 登录报「没有可用的授权账号」
3. 建授权     jms_permission_create  ← accounts 必须 all_accounts=true
4. 建用户     jms_user_create    ← email 必填，否则 400
5. 注册公钥   jms_user_sshkey_add ← 必须 ssh-rsa
6. 归位       jms_asset_move     ← 把资产放进资产树的对应节点
```

第 1 步漏了 protocols、第 2 步漏了账号、第 3 步的 accounts 传了空列表 ——
这三个错误**都不会立刻报错**，而是等到登录时才失败，且症状很像「凭据不对」。
`jumpserver-mgmt --doctor` / `jms_doctor` 会直接把这三类问题列出来。

### 4.1 授权的账号粒度

`jms_permission_create` 的 `accounts` 决定这个授权能登进去用**哪些账号**：

| 传法 | 含义 |
|---|---|
| `all_accounts: true` | 该授权下这些资产的**全部账号**（字面量 `@ALL`） |
| `accounts: ["opsuser"]` | **只有** `opsuser` 这个账号（最小权限） |
| 都不传 | **一个账号都没有** —— 登录必失败 |

⚠️ `accounts` 传的是**账号名**（`"opsuser"`），不是账号 UUID。传 UUID 不报错，
但授权等于什么都没授。

⚠️ `jms_permission_grant` 是**只增不减**的：给一个已经是 `@ALL` 的授权追加
`accounts` 时，`@ALL` 会被保留 —— 追加动作不会把范围悄悄收窄。
**收窄授权是另一件事**，本套工具不提供，因为那是「撤销」而不是「授予」。

### 4.2 资产树

```
建节点     jms_nodes_create   {name, parent}
移动资产   jms_asset_move     {assets, node, action: add|remove}
看节点     jms_nodes_list
```

⚠️ 节点的**可写字段是 `value`**，不是 `name`（`name` 是派生的只读字段）。
直接传 `{"name": "x"}` 会建出一个**没有名字**的节点。

⚠️ 节点里**还有资产时删不掉**：API 答 403
`Deletion failed, the node contains assets`。先把资产移出再删。

---

## 5. 自检

### 5.1 `jumpserver-mgmt --doctor`

```
jumpserver-mgmt doctor
  base_url: https://jumpserver.example.com
  identity: admin (superuser=true)

  [OK  ] api_token                    JUMPSERVER_TOKEN is set
  [OK  ] api_reachable                the bastion answered and accepted the token
  [OK  ] identity                     acting as admin
  [OK  ] delegation                   single-use Chat AI delegation is configured
  [OK  ] delegation_identity          a delegated credential acts as ai (...)
  [FAIL] assets_without_protocols     6 asset(s) have no protocol and can never be connected to: ...
         fix: add a terminal protocol to each listed asset with jms_asset_update
  [WARN] assets_without_accounts      4 asset(s) have no account: ...
         fix: add an account to each listed asset with jms_account_create
  [WARN] permissions_without_accounts 1 permission(s) grant NO account (empty is not @ALL): ...
         fix: grant an account to each listed permission with jms_permission_grant
  [OK  ] permissions_without_members  every permission is held by at least one user or group
verdict: 3 of 10 checks need attention: assets_without_protocols, assets_without_accounts, permissions_without_accounts
```

> ⚠️ 上面是**真实渲染器**（`oekit/pkg/cli` 的 `Report`）的输出：检查项名称列**按内容
> 自适应**，修复建议缩进到**名称列的起点**，结论行紧跟检查项（中间没有空行）。
> 改渲染器时这段样例要跟着重生成。

检查项含义：

| 检查项 | 级别 | 含义 |
|---|---|---|
| `api_token` / `api_reachable` / `identity` | FAIL | 接线断了：地址、凭据、权限 |
| `version` | WARN | **堡垒机是几版**：探测到的与配置说的**不一致**（或**探测不到**）。⚠️ 配置成 v5 而部署是 v4 ⇒ 每次委派都回 401，会话全被踢去慢路径。详见 §3.5 |
| `delegation` | WARN | 没配委派 secret。⚠️ **v4 上是另一种情况**：部署**根本没有**委派方案 ⇒ 这一行说的是「会话以 API 凭据的身份跑」，而不是「去配一个 secret」 |
| `delegation_identity` | OK | 报出委派凭据实际以谁的身份执行 |
| `assets_without_protocols` | **FAIL** | 这些资产**永远连不上**，登录时报「没有可用的授权账号或终端协议」 |
| `assets_without_accounts` | WARN | 同上，只是还没有账号可试 |
| `permissions_without_accounts` | WARN | 该授权**一个账号都没授予**（空 ≠ `@ALL`） |
| `permissions_without_members` | WARN | 该授权**没有人持有** |
| `permissions_with_unknown_accounts` | WARN | 该授权点名的账号**在它授权的资产上根本不存在** —— 这种授权永远用不了 |

> ⚠️ **`web`(Website) 和 `cloud`(Kubernetes) 类资产不参与「无协议/无账号」的检查** ——
> 它们本来就没有终端协议和 SSH 账号。早期版本把它们算进去，一次报出 6 条假警报，
> 而那 6 个服务全都好好地在跑。

同一个检查在 MCP 会话里也能跑：`jms_doctor`（`verbose=true` 会列出全部资产）。

会话内返回的是**统一的诊断形状**（`jms_doctor` / `jms_asset_audit` /
`jms_command_guard` / `jms_security_review` / `jms_login_asset_check` 都一样）：

| 字段 | 含义 |
|---|---|
| `checks[]` | 逐项 `{name, status, detail, fix?}` |
| `findings[]` | 里面 status ≠ ok 的那些 |
| `status` | 总体评级 ok / warn / fail |
| `verdict` | 一句话结论 |
| `advice` | 一句话怎么修 |

CLI 上看到的 `[OK]/[WARN]/[FAIL]` 就是每项 check 的 `status`。

⚠️ **会话内的 `jms_doctor` 不报留痕** —— 它刻意不带配置（注册表不传配置），
「不知道」与「没有」是两种沉默。CLI 的 `--doctor` **报**留痕（`audit_trail` 一行）。

---

## 6. 工具选择速查

### 6.1 维护堡垒机

```
├── 加/删资产                 → jms_asset_create / update / delete
├── 批量登记资产              → jms_assets_bulk_create
├── 加账号                    → jms_account_create / update / delete
├── 加用户                    → jms_user_create / jms_user_password_set
├── 授权                      → jms_permission_create / grant / delete
│                              账号粒度: all_accounts=true 或 accounts=["账号名"]
├── 看授权里都有谁            → jms_permission_get      （列表只给数量）
├── 这个授权实际覆盖哪些资产  → jms_permission_coverage  （展开节点继承）
├── 总体谁权限最大            → jms_access_matrix
├── 两个授权差在哪            → jms_permission_compare
├── 安全态势体检              → jms_security_review
├── 看某人能访问什么          → jms_user_access         （有效权限视图）
├── 看用户组（权限可挂在组上） → jms_user_groups_list
├── 这台机器现在什么状态       → jms_asset_audit         （一个调用给出全部 + 结论）
├── 跑命令前预检会不会被拦     → jms_command_guard
├── 资产树                    → jms_nodes_list / jms_nodes_create / jms_asset_move
├── 堡垒机自己注册了哪些终端  → jms_terminals_list
├── 命令过滤                  → jms_cmdgroup_create → jms_cmdfilter_create
│                              （删组用 jms_cmdgroup_delete）
├── 标签                      → jms_label_attach / jms_label_resources
├── 盘点账号漂移              → jms_gathered_accounts   （present:false 就是漂移）
├── 查角色能干什么            → jms_role_permissions / jms_system_role_permissions
└── 安全审计
    ├── 谁改了配置            → jms_operate_logs
    ├── 谁改了谁的密码        → jms_password_change_logs
    ├── 谁能登某资产          → jms_login_asset_check
    └── 高危操作不留长期凭证  → jms_delegation_call
```

⚠️ **一共 97 个工具**，上表是常用的一批。完整目录跑 `jumpserver-mgmt tools`
（打印名字/风险/确认要求）。

### 6.2 不确定的时候

先跑 `jms_doctor`（会话内）或 `--doctor`（shell）。它会直接告诉你哪一环断了。

---

## 7. 故障排查

| 现象 | 先查 | 原因 |
|---|---|---|
| 工具在会话里调不到 | 重启会话 | runtime 的工具目录是**会话开始时缓存**的 |
| 登录报「没有可用的授权账号或终端协议」 | `jms_doctor` | 资产没 protocols / 没账号 / 授权的 accounts 是空列表 |
| 权限改了不生效 | 重开会话 | 会话是**启动时**快照 |
| 创建用户报 `{"email":["This field is required."]}` | 补 `email` | v5 硬性要求 |
| 创建用户报 `{"username":["User with the same Username already exists."]}` | 换名字 | 重名 |
| `jms_user_sshkey_add` 报「只能给凭据所属账号加密钥」 | 见 §3.1 | v5 没有跨用户端点；把 `JMS_DELEGATION_USER_ID` 指向目标用户 |
| 账号自动化回 403 | 看是哪一族 | **两族要许可证**（不是缺权限）⇒ 加角色没用；反过来，push/gather 的 403 才是**凭据缺权限** |
| `jms_permission_get` 说「没人能访问」 | 取详情 | 列表端点把关联对象返回成 null |
| 资产详情里缺 platform / protocols | 补契约，不要静默丢弃 | 缺字段是当场发现、当场补进 IDL 的 |
| 节点建出来没有名字 | 传 `value` 不是 `name` | 节点的**可写字段是 `value`** |
| 删节点报 403 | 先把资产移出 | 「节点里还有资产」 |
| MCP 进程 exit 137 / 无 stderr | 换一个**新路径** | 受执行策略限制的环境里，覆盖已存在路径的二进制会被杀 |
| `replacement directory ... does not exist` | `./script/restore-go-modcache.sh` | Go module cache 被清空 |
| `go mod tidy` 失败 | 别 tidy | 受限网络下抓不到测试专用依赖，手改 `go.mod` |

---

## 8. 改代码 → 生效

```
改代码 → ./script/build.sh → python3 script/publish-mcp.py
       → mcp update（用 publish 输出的命令）→ 重启会话
```

| 目录 | 是什么 | 谁改 |
|---|---|---|
| 仓库源码目录 | **源码**，唯一真相 | 你 / AI |
| `./bin/` | **构建产物** | `script/build.sh` |
| `~/.local/jumpserver-mgmt-runtime/<stamp>/` | **运行时二进制**，MCP 配置指向这里 | `script/publish-mcp.py` |
| 你的 MCP 客户端配置文件 | **接线**，记录上面那个路径 + 环境变量 | `mcp update` |

**改源码 ≠ 生效。** 已经跑起来的 MCP 进程还持有旧二进制，必须重启会话。

> ⚠️ 发布必须进**新的时间戳目录**。在某些受执行策略限制的环境里，原地覆盖二进制会被杀
> （exit 137、无 stderr）。`publish-mcp.py` 已经这么做了，不要自己写 `cp` 覆盖。

改完记得看工具数：管理面加一个工具应该 **N+1**（97 → 98）。没变说明注册表漏了。

> 开发环境的完整流程、验证要求与常见坑见 [`DEVELOPMENT.md`](DEVELOPMENT.md)。

---

## 9. 安全须知

- **token 不要进 git**。MCP 客户端的配置文件在仓外，正常不会被提交，但别手滑拷进仓里。
- **委派 secret 不要写进 MCP 配置**（见 §3.1）。
- **一切写操作都要 `confirm=true`**，包括删资产、改授权、重置密码 —— 不可逆。
- **删类操作不可逆**：删资产会连它的账号一起删。
- **命令以资产账号身份执行，不提权**。要 root 自己写 `sudo -n`。
- **命令过滤器的范围要核对**：一条挂错资产的过滤器会静默弄坏别人的自动化，
  而且是在**他的机器上**炸，不是你的。跑之前用 `jms_command_guard` 预检。
- ⚠️ **改密码不要用 `jms_account_update`** —— 那会让密钥**经过 agent**；
  用账号自动化的 push（密钥不出堡垒机）。

---

## 10. 给 AI agent 的操作手册

`skills/` 目录里是 agent 选工具和避坑的依据：

| skill | 内容 |
|---|---|
| `jumpserver-ops` | 入口：两个服务分工、工具速查、铁律、koko 协议、v5 坑表 |
| `jms-onboarding` | 资产上线（写操作）：标准流程 + 四个静默失败 |
| `jms-access-review` | 权限审计（只读）：谁能登什么、授权语义、最小权限 |

**改了工具要同步改 `skills/`** —— 那里有文档/代码脱节检查
（`./script/check-skill-drift.sh`）。
