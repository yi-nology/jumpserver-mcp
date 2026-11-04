# 共享参考：JumpServer v4 与 v5 的差异

> ⚠️ **本文件里的每一条，最初都是对着上游 v4.10.20-lts 的源码读出来的**；
> **2026-10-31 起有一台真 v4.10.20 堡垒机（`192.0.2.21`）**，端点表、版本探测、
> 连接令牌、登录凭据、执行面**全部实测通过**（见 §6；门控真调在
> `internal/repository/livetest/v4_test.go`）。
> ⚠️ 但**实测推翻了两条源码推断** —— 说明「读源码」与「跑真机」不是一回事，
> 这也正是本仓那条老教训：*协议事实要实测，别抄*。
>
> 对照来源：`apps/jumpserver/urls.py`（API 前缀）、各 app 的 `api_urls.py`（端点注册）、
> `apps/authentication/backends/drf.py`（认证类）、`apps/authentication/models/connection_token.py`
> （连接令牌）、`apps/assets/serializers/asset/common.py`（资产与账号形状）、
> `apps/accounts/models/account.py`（alias 语义）。
>
> ⚠️ **2026-10-31 起这些结论都有真机背书**（见 §6）：本仓起了一台 v4.10.20 堡垒机，
> 端点表、版本探测、连接令牌、登录凭据、执行面全部**实测通过**。⚠️ 而实测**推翻了两条**
> 源码推断（列表端点的形状、`perm_account_invalid` 的含义）—— 都记在 §5.1 与 §6.2。

---

## 0. 一句话结论

**两版几乎一样，只有一样不同：v4 没有 Chat AI 委派方案。**

端点表、认证头、连接令牌体、响应形状 —— 逐条同路径、逐条同形状。
所以**管理面 97 个工具 + 执行面 15 个工具在 v4 上都能用**，**不用改任何工具**。

⚠️ 差异的方向是**不对称**的，这是最要紧的一条：

- **把 v4 说成 v5**（默认 `auto` 而探测失败时就会这样）⇒ 执行面每次会话都去发委派凭据
  ⇒ v4 回 **401** ⇒ 而 401 在那条路上被读成「连接令牌机制不可用」
  ⇒ **每次会话都被踢去慢十倍的 koko 终端路径**。
- **把 v5 说成 v4** ⇒ 委派被关掉 ⇒ 会话以 API 凭据的身份跑（= 管理员面，本部署上是管理员）
  ⇒ **比该有的宽**，但功能正常。

⇒ 猜错的那一面是「慢」，猜错的那一面是「宽」。**宁可慢，不要宽**。

---

## 1. 认证：PrivateToken 两版通用

```
Authorization: Token <PrivateToken>
```

v4 的 `PrivateTokenAuthentication` 就是 DRF 的 `TokenAuthentication`（`model = PrivateToken`），
**同一个头在两版上是同一个意思**。⚠️ 「v5 砍掉了 AccessKey/Bearer」说的是 v5，
不是 v4 —— 别把它当成版本差异。

另一种凭据（**两版都有**这个端点）：

```
POST /api/v1/authentication/auth/   {"username": "...", "password": "..."}
  → 200 {"token": "<不透明令牌>", "keyword": "Bearer"}
  → 200 {"error": "mfa_required", "msg": ...}  ← 部署要二次因子时（注意是 200，不是 401）
  → 400 {"error": "..."}                  ← 凭据不对

POST /api/v1/authentication/mfa/challenge/  {"type": "otp", "code": "<TOTP>"}
```

⚠️ **Bearer token 会过期**（PrivateToken 不会）⇒ 长跑进程必须能续。
⚠️ **收到 401 换一次重发，只对「读」重发**：401 通常是认证层拒绝（没被执行），
但上游视图自己也会回 401，两类分不开 ⇒ 写操作永不因为 401 被重发（写两次的代价太大）。
⚠️ **MFA 靠的是 Django session，不是凭据**：挑战端点从 `request.session` 读
`user_id`/`auth_password`/`auth_mfa`（`MFAChallengeVerifyApi.initial()` → `get_user_from_session()`，
没有 session 就 401）⇒ **登录种下的 session cookie 必须带到挑战那一步**。
⚠️ 而**普通客户端不能带 cookie**：带有效 session cookie 的请求会被 Django 认成 session 用户，
那就不是 token 属主了 ⇒ 两条路径必须用两个 HTTP 客户端。

---

## 2. 连接令牌：body 两版同形

```json
{"asset": "<资产UUID>", "account": "<账号UUID>", "protocol": "ssh", "connect_method": "web_cli"}
```

⚠️ **第三方 v4 资料说「token API 要账号的 alias 而不是 UUID」—— 那句话只对动态账号成立。**

v4 的 `ConnectionToken.get_asset_accounts_by_alias`：

```python
if is_uuid(alias): kwargs = {'id': alias}
else:               kwargs = {'name': alias}
```

而 v4 的 `Account.alias` 对普通账号**就等于它的 id**（`return str(self.id)`，
除非 username 以 `@` 开头）。⇒ **UUID 在 v4 上是被接受的 alias**，
本仓现在发的 UUID 两版都通。

⚠️ 只有**动态账号**（`@USER` / `@INPUT`）在两版上不同：v4 要那个字面量。
本仓挑的是**真实账号**（按 UUID 选），所以碰不到这个差别。

---

## 3. 唯一真的不同：Chat AI 委派方案

**v4 根本没有 `chat_ai` 这个应用**：

- 没有 `X-JMS-AI-Delegation` 认证类；
- 没有 `chat-ai/` API 前缀（v5 的 `apps/jumpserver/urls.py` 里有，v4 的没有）；
- ⇒ **v4 上不存在更窄的凭据**。

后果两条：

1. **会话的身份更宽**：连接令牌只能用 API 凭据签 ⇒ 会话带的是**那个凭据**的权限
   （本部署 = 管理员），而不是 `ai` 那个更窄的面。⚠️ 这是部署提供给什么的问题，
   **配置改不动** —— 两个 `--doctor` 的 `identity` 一行会说明白。
2. **不关掉就连累快路径**：委派请求**不带** API token ⇒ v4 回 401 ⇒ 被读成
   「连接令牌机制不可用」⇒ 每次会话都走慢十倍的终端路径。
   ⇒ 程序在 v4 上**根本不发起委派**（`pkg/jmsapi` 的版本档把它关掉）。

---

## 4. 怎么知道这是 v4

程序**问**堡垒机**两条**路径，然后**比**它们的答案：

| 路径 | 是什么 |
|---|---|
| `GET /api/v1/chat-ai/runtime-store/` | **v5 注册了 chat-ai 前缀，v4 没有** |
| `GET /api/v1/zzz-jms-version-probe-404/` | **哪一版都没有**（名字故意取得不可猜） |

| 两条的答案 | 结论 |
|---|---|
| **一样** | **v4**（chat-ai 那条路由不存在，它的答案和「哪条都不存在」一样） |
| **不一样** | **v5**（路由在，只是这次调用被拒或方法不对） |
| 连不上 | **不知道** ⇒ 按「有这个能力」处理（保窄），**30 秒内不再问，之后再问** |

⚠️ **为什么不看状态码就行** —— 因为 **v4 的「没这条路由」处理器是坏的**：它回 **500**
（渲染 404 页面时自己崩了），所以 v4 上 `/api/v1/` 下**任何**不存在的路径**长得一模一样**
⇒ 「路由不在」这件事**没法**从状态码读出来，只能**比**。

⚠️ **真机实测（两条路径的答案在两版上都不一样，这正是比对的依据）**：

```
# v5（2026-10-30，http://192.0.2.41）
GET /api/v1/chat-ai/runtime-store/       -> 401 {"code":"not_authenticated"}
GET /api/v1/zzz-jms-version-probe-404/  -> 404

# v4（2026-10-31，真 v4.10.20-ce，192.0.2.21）
GET /api/v1/chat-ai/runtime-store/       -> 500
GET /api/v1/zzz-jms-version-probe-404/  -> 500
```

那个 401 **不是 bug**：v5 的 runtime-store 视图用 AccessKey **签名**认证
（`SignatureAuthentication` + `IsServiceAccount`），所以它**不收**本仓带的 PrivateToken ——
但它**仍然不是 v4 的答案**，而这正是探测要问的：**「这条路径是不是注册过的路由」**。
⚠️ 而 v4 那两条**都是 500** —— 它的 404 处理器崩了。

⚠️ **显式配置压过探测**（`JUMPSERVER_VERSION=v4|v5`）：操作员可能知道探测看不见的事
（部署在会藏 API 前缀的反代后面），而且**显式配置一次请求都不用发**。

⚠️ 两个 `--doctor` 的 `version` 一行会报**版本**与**能力**（委派方案 present/absent/unknown），
**两者不一致时是 WARN**。
⚠️ 这也意味着：**探测答错时，doctor 那一行是唯一的信号** —— 在 v5 上看到 `v4 (detected)`
就该显式写 `JUMPSERVER_VERSION=v5`。

## 4.5 「差异」在本仓怎么维护（改代码前必读）

差异**不写在版本上，写在能力上**，而且**只有一份清单**：`pkg/jmsapi/capability.go` 的
`capabilityDecls`。那一行里写着四件事：

| 字段 | 内容 | 谁用 |
|---|---|---|
| `Versions` | 哪一版**有**这个能力 | —— |
| `SchemeCalls` | 这个能力怎么被交出去（函数名） | `internal/arch` 的守卫**从清单里读** |
| `Cost` | 没有它要付什么代价（**一份措辞**） | 两个 doctor、三个委派工具、执行面**共用** |

⇒ **代码问能力，不問版本**：`api.State(ctx, jmsapi.CapDelegation)`。
⚠️ **控制流里不许出现版本号** —— 版本是操作员填的（`JUMPSERVER_VERSION`），是**输入**不是问题。
（散文里可以出现版本号：跟人解释时说「这台是 v4」比说「这台没有委派能力」好懂。）

⚠️ **能力是三态的（present / absent / unknown），没有「折叠布尔」**：`State` 是事实，
`Present`（已确认有）与 `MayPresent`（没确认缺）是两个不同的决策。
⚠️ 报告面**必须**按三态措辞：探测失败时要**说「没测出来」**，不能只报「委派已配置」
（那会把「这个部署到底有没有这个能力」整个吞掉 —— 真发生过）。

⇒ ⚠️ **「部署有没有」与「这里配没配」是两个问题**：能力是部署的（配不出来），
配置是本机的（能配）。合成一个答案时，答复只能说一句含混的话。

⇒ **加一个版本差异 = 加一行清单**。不是去工具层加版本判断、不是去改三处 doctor 的措辞、
不是去改守卫的名单（⚠️ **自己维护的名单会悄悄失效**：改了函数名守卫就不报了 ⇒
以为有人守，其实瞎了）。

---

## 5. 哪些东西两版一致（照抄 v5 的坑表即可）

| | 一致？ | 出处 |
|---|---|---|
| API 前缀 `/api/v1/` | ✅ | 两版的 `apps/jumpserver/urls.py` |
| 端点表（`pkg/jmsapi/api.go` 全部 40+ 条） | ✅ 逐条同路径 | 各 app 的 `api_urls.py` |
| 认证头 `Authorization: Token <key>` | ✅ | v4 的 `drf.py` |
| 资产详情的 `accounts` | ✅ | v4 的 `DetailMixin.accounts` |
| 账号字段 id/name/username/is_active/privileged | ✅ | v4 的 `BaseAccountSerializer.Meta.fields` |
| ⚠️ 账号**没有** `alias` 字段 | ✅ 两版都没有 | v4 的 `BaseAccountSerializer` 只列了上面那五个 |
| `platform` 是嵌套对象 | ✅ | v4 的 `AssetSerializer` |
| 资产树的 `full_value` | ✅ | v4 的 `Node.full_value` |
| 列表端点把关联对象返回成 null | ✅（v5 坑表 §3） | v4 同源的血缘 |

⇒ **v5 坑表（`v5-api-traps.md`）在 v4 上照用** —— 写接口的字段形状、
授权的 `accounts` 传账号名、PATCH 是替换不是追加、节点的 `value` 是可写字段 ……
**这些都不是版本差异**，是这套 API 的形状。

### 5.1 「实测过的 v5 事实」逐条在 v4 上复核过（2026-10-30）

这些原本是「v5 的事实」，看着像版本差异。**逐条对着 v4.10.20-lts 源码核过，结论是：不是差异。**

| 事实 | v4 上的依据 |
|---|---|
| 标签 / 角色 / 内容类型端点返**裸数组** | ✅ **实测：v4 也是裸数组**，与 v5 一致。（⚠️ 我曾按源码推断「v4 返标准信封」——**错的**；真机一跑就见分晓。好在 `pkg/jmsapi.List` 两种形状都吃，所以代码从没受影响，只有文档说错过） |
| **创建用户强制 email**（缺了 400） | v4 的 `User.email = EncryptCharField(max_length=128, unique=True)` ⇒ **没有** `blank=True` ⇒ 同样必填 |
| **作业 API 不返 stdout** | v4 的 `JobExecutionSerializer` 没有 output/stdout 字段 ⇒ 同样不返 |
| 命令过滤器的**多对多信封**（`{"type":"ids","ids":[…]}`） | v4 的 `CommandFilterACLViewSet` 同样有独立的 `list` serializer ⇒ 同样形状 |
| **SSH 公钥没有跨用户端点**（只能注册给凭据属主） | v4 的 `SSHkeyViewSet.get_queryset()` = `request.user.ssh_keys.all()` ⇒ 同样只认调用者 |
| PATCH 关联字段是**替换**不是追加 | 同源血缘，两版一致 |

⇒ **结论**：v4 的差异面比「版本号」窄得多 —— 真的只有**委派**那一条。

---

## 6. 实测状态（2026-10-31，真 v4.10.20）

⚠️ **本仓曾长期只有「源码级核对」** —— 没有 v4 堡垒机可测。2026-10-31 起有一台
（`192.0.2.21`，v4.10.20-ce），下面每一项都是**在真机上证过的**。

| | 状态 |
|---|---|
| 端点表 49 条在真 v4 上的应答 | ✅ **实测全通**（门控真调：`internal/repository/livetest/v4_test.go`） |
| 版本探测在真 v4 上判成 v4 | ✅ **实测**（两条路径**答案一样**：chat-ai → 500，必然缺失路径 → 500） |
| 委派方案在真 v4 上不存在 | ✅ **实测** |
| 连接令牌用**账号 UUID** | ✅ **实测成功**（账号名 / `@USER` 都被拒 ⇒ 与 v5 一致） |
| 登录凭据（账号密码换 Bearer） | ✅ **实测可用**（⚠️ v4 **没有** PrivateToken 的 API 出口 ⇒ v4 上只能用这个形态） |
| 执行面真跑命令（令牌 → koko → 目标机） | ✅ **实测**（输出与退出码都对） |
| MFA 登录流 | ⏳ **未实测**（⚠️ 它依赖 Django 的 session cookie；上游源码已核） |
| 资产/账号/授权的**写入形状** | ✅ **实测一致**（`platform` 给 pk、`protocols` 给字典数组 —— 与 v5 一样） |

### 6.1 装一个 v4 时踩到的四件事（都不是代码问题）

| 现象 | 真因 | 处理 |
|---|---|---|
| `POST /api/v1/authentication/auth/` 答 **500** `Server internal error` | 口令太弱：v4 抛 `PasswordTooSimple`，而它自己的异常处理器访问不存在的 `.detail` ⇒ 处理器自己崩了 ⇒ 500 | 把 admin 口令设成强口令（`docker exec jms_core … manage.py shell` 里 `set_password`） |
| koko 反复 `401 Authentication credentials were not provided` | **组件注册窗口**：`SECURITY_SERVICE_ACCOUNT_REGISTRATION=auto` 时**只有 core 启动后 300 秒**允许注册；core 跑迁移十几分钟，koko 起来时窗口早过了 | 配置里加 `SECURITY_SERVICE_ACCOUNT_REGISTRATION=true`，并 `--force-recreate` 容器（env 是**创建时**注入的，`restart` 不重读） |
| koko 拿到的 `BootstrapToken` 是 `****` | `koko.yml` 用 `env_file: ${CONFIG_SAFE_FILE}`（**脱敏副本**）。⚠️ 官方做法是**整行删掉**排除的键（`DB_HOST DB_PORT DB_PASSWORD …`），**不是**抹成 `****` —— 而 `BOOTSTRAP_TOKEN` 是组件之间要共享的，抹掉它 koko 就注册不上 | 按官方的 `gen_safe_config` 生成脱敏副本 |
| 换连接令牌报 `perm_account_invalid: Account not found` | ⚠️ **不是账号字段错**，是**没人被授权用这个资产**（`account.actions` 为空）。同一句话两个意思 | 建资产授权（授权里带上账号名） |

### 6.2 一条极易误诊的坑

`perm_account_invalid` 在 v4 上**同时**表示两件事：
「账号字段不对」与「账号存在但没人被授权」。后者才是常见的那一个，
而它的报错文本指向的是前者 ⇒ 排查方向会被带偏十分钟。

⚠️ 与本仓自己的规矩同源：**资产必须被授权，否则连不上**（见 `docs/CONFIGURATION.md` §4.1）。
⚠️ 同样的坑还有第二层：koko 拿到令牌后去连**目标机**，目标机认证失败时它回的是
「ssh: unable to authenticate」—— **看起来像网络/密钥问题，其实可能是资产里存的
账号密钥不对**。

### 6.3 想再起一台？脚本已经在那儿

`script/stand-up-v4.sh <host>` —— 上面四个坑都填好了（注册窗口、safe config、
强制重建、下载并发），装的是 core/koko/web + mysql/redis（celery/lion 关掉，小机器也跑得动）。
装完按本地那半的提示设一个强口令，然后：

```
JMS_TEST_V4_URL=http://<host> JMS_TEST_V4_USERNAME=admin JMS_TEST_V4_PASSWORD='<强口令>' \
  go test ./internal/repository/livetest/ -run V4 -v
```

### 6.4 进一台 v4：凭据与账号（2026-11-03 在 `192.0.2.21` 实测）

⚠️ **SSH 账号 ≠ 堡垒机账号**：`192.0.2.21` 的 SSH 是 `opsuser`（免密），
但 **JumpServer 里没有 `opsuser` 这个用户** —— 只有 `admin` + `[Core]-…` + `[KoKo]-…`。
⇒ 拿 SSH 的凭据去登录堡垒机，**必然**是 `password_failed`。

⚠️ **admin 口令不记在任何地方**（`stand-up-v4.sh` 是参数传入的，仓里故意不记）⇒
要临时进一台 v4，只有两条路：**问操作员**，或**重设**（同一机制，脚本里已有）：

```
docker cp setpw.py <host>:/tmp/ && ssh <host> 'docker cp /tmp/setpw.py jms_core:/tmp/setpw.py && \
  docker exec -e ADMIN_PASSWORD=... jms_core bash -c \
  "cd /opt/jumpserver/apps && python3 manage.py shell < /tmp/setpw.py"'
```

⚠️ **v4 的超管不是 `role` 字段说了算**：`is_superuser` 是**派生属性** = `system_roles`
里挂了 **SystemAdmin** 内置角色 ⇒ 只设 `u.role = 'Admin'` 仍然 **403**（列表类接口全拒，
但 `/users/profile/` 会 200 —— 看着像"能用了"）。组模型叫 **`UserGroup`**（没有 `users.models.Group`）。

⚠️ **v4 的认证头是 `Bearer`**：手工探测用 `Authorization: Token <t>` 会得到
`401 Invalid token`（那是 v5 的形状）⇒ v4 只能用 `Authorization: Bearer <t>`。

⚠️ **v4 上没有任何 koko 公钥登记** ⇒ 执行面（`jmsx_exec`）要真跑命令，得先给堡垒机
用户写 `public_key`（否则 koko 的 SSH 认证过不去）；资产侧的账号与它是两回事。

⚠️ **版本探测的绝对状态码会飘**：2026-11-03 从**客户端真正走的那条路**
（`http://192.0.2.21`）量到 **500/500，连测 6 次全 500**（与 §6 的记录一致）；
但同一台机、同一容器（没重启）、从宿主机用 `127.0.0.1`/容器 IP 打，**有一个窗口量到过 404**
（机制没查清）⇒ **绝不能读单次状态码**，只能**比两条**（答案相同即 v4）。

⚠️ **要建临时超管（验完即删）**：见上面那条 `manage.py shell` 机制 ——
`u.role='Admin'` + `u.system_roles.set(admin.system_roles.all())` + `u.set_password(...)`
+ 加 `Default` 组；用完 `User.objects.filter(username=...).delete()` 连带清干净。
⚠️ 临时超管**能列一切但连不上机器**：还要单独给它一条**资产授权**（带上账号名），
否则签连接令牌会 `perm_account_invalid`（见 §6.2）。
