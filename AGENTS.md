# AGENTS.md — jumpserver-mgmt 迭代指南

> 给 AI agent 的作业说明。**改这个仓的代码前先读完本页**，尤其第 1、2 节。

本仓 = JumpServer 堡垒机的**管理面** MCP 服务：改堡垒机自己的配置 ——
资产 / 账号 / 权限 / 标签 / 命令过滤 / 登录 ACL / 审计检索 / 取证 / 委派 / 批量作业。
**97 个工具**，一个二进制 `jumpserver-mgmt`。
面向人读的完整说明在 [`README.md`](README.md)，
**配置与排障看 [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md)**。

---

## 0. 先认位置：本仓只是三个仓里的一个

| 仓 | 是什么 | 干什么 |
|---|---|---|
| [`yi-nology/jumpserver-mgmt`](https://github.com/yi-nology/jumpserver-mgmt)（本仓） | MCP server `jumpserver` | **管理面** — 改堡垒机自己的配置 | 
| [`yi-nology/jumpserver-exec`](https://github.com/yi-nology/jumpserver-exec) | MCP server `jumpserver-exec` | **执行面** — 经堡垒机在**受管机器**上跑命令、传文件 |
| [`yi-nology/jumpserver-kit`](https://github.com/yi-nology/jumpserver-kit) | 共用底座（**库**） | 协议层 / 门禁 / 输出契约 / 留痕 / 脱敏 / 风险分类 / 共用词汇 / 守卫的机械 |

**共享纪律（铁律、验证要求、提交约定、底座怎么发版）在 kit 仓的 AGENTS.md 里**：
👉 <https://github.com/yi-nology/jumpserver-kit/blob/main/AGENTS.md>

⚠️ **只读本仓的 agent 也要拿到底线** ⇒ §1 把最关键的铁律**原文重复**了一遍。
⚠️ **底座在 kit 仓**：本仓 `require github.com/yi-nology/jumpserver-kit`，
协议层（`pkg/jmsapi`）、门禁（`pkg/reg`）、结果整形与输出契约（`pkg/render`）、
留痕（`pkg/audit`）、脱敏（`pkg/redact`）、风险分类（`pkg/risk`）、
失败分类（`pkg/errs`）、共用词汇（`model/jms`）**都不在本仓**。

**目录约定**（CloudWeGo hz 的标准骨架）：

```
main.go / init.go / server.go / workers.go / shutdown.go
                             # 入口层（**仓根**的 package main）：serve / doctor / tools / version
cmd/                        # 子命令（入口层 import 的**普通包**，不是二进制）
├── serve/                  #   默认命令：以 stdio 提供 MCP 服务
├── doctor/                 #   自检：接线与堡垒机配置
└── tools/                  #   工具目录：名字、风险、确认要求
biz/
├── handler/                # 97 个工具的实现（一个域一个文件，只做解码/编排/渲染）
├── model/jmsms/            # 契约的生成物（thriftgo slim 模板，产物入库）
└── router/                 # 装配：哪个域注册哪些工具
internal/
├── config/                 # 配置（环境变量 → 结构体）
├── repository/             # 数据访问层（资产解析、授权展开、账号归属）
│   └── livetest/           #   打真堡垒机的那套（由环境变量 gate 住）
├── service/                # 领域规则（纯函数，可离线单测）
│   └── policy/             #   工具目录的**风险侧**：每个工具改不改堡垒机、要不要确认
├── arch/                   # 机械守卫（判据；守卫体在 kit 的 pkg/arch）
├── middleware/             # （空）HTTP 中间件：这一面没有 HTTP
└── rpchandler/             # （空）RPC 服务端：这一面不对外提供 RPC
idl/jmsms/                  # 契约的 thrift 定义：入口 jmsms.thrift + 按域拆的多份
pkg/                        # （空）通用库在 kit 仓，不在这里放东西
etc/ swagger/ deploy/ monitoring/ rpcx/ kitex_gen/
                             # （空）hz 骨架里的其余层：见各目录里的 README
script/                     # 构建 / 发布 / 生成 / 校验 / 技能漂移检查
skills/                     # 给 AI 的操作手册
```

⚠️ **入口层在仓根**：一个仓一个二进制，二进制就是仓根那个 `package main` ⇒
`go install github.com/yi-nology/jumpserver-mgmt@latest` 装出来的就是它
（**不是** `…/cmd/jumpserver-mgmt@latest` —— 那个路径没有了）。

⚠️ **`cmd/` 里是子命令，不是二进制**：那些包是入口层 import 的普通包。
而**实现不住在仓根** —— 仓根那个 package 是 `main`，Go **不许 import 一个
main 包** ⇒ 子命令的实现只能在 `cmd/` 下。

⚠️ **空的层不是「忘了做」**：每一层都有自己的 README，写着它为什么空、
什么时候不再是空的。

hz 的 `router.go` / `router_gen.go` 在这里没有对应物：本仓没有 HTTP 路由，
`biz/router` 就是它们的对应物。

**分层的方向是硬的，有守卫盯着**（判据在本仓 `internal/arch/`，守卫体在 kit 的 `pkg/arch`）：
- **本仓不得 import 执行面**（另一个服务）；**底座不得依赖任何服务**。
- **入口层（仓根 + `cmd/`）是最外层，谁都不许 import**。
- **IDL 目录名 == 生成包名 == 模型目录名**：`idl/jmsms/` 生成的包必须叫 `jmsms`，
  放在叫 `jmsms` 的目录里。契约按域拆成同目录多份 thrift：入口是 `jmsms.thrift`
  （include 各域文件），**每一份**都声明 `namespace go jmsms` —— 拆的是文件，
  不是词汇，生成物仍是一个包、类型名不变（跨文件引用要写 `域名.类型名`，
  且 `gen-idl.sh` 必须带 `-r`，否则 include 的文件没有生成物）。

---

## 1. 铁律（原文重复 —— 完整版在 kit 仓的 AGENTS.md）

> 完整版：<https://github.com/yi-nology/jumpserver-kit/blob/main/AGENTS.md> §2
> 下面这几条是**只读本仓也要拿到底线**的。

1. **改源码 ≠ 生效。** 必须走完：`./script/build.sh` → `python3 script/publish-mcp.py`
   → `mcp update` → **重启会话**。只改代码就汇报「已完成」是错的。
   ⚠️ **`publish-mcp.py` 不会改 MCP 配置** —— 配置还指着旧的运行时目录，
   于是「真调」调的是**旧二进制**，而你会以为是新代码坏了。
   `mcp get <server>` 看 `command` 字段，确认它指向刚发布的那一版，再真调。

2. **不要原地覆盖二进制。** 沙箱按路径执行策略，覆盖已存在路径的二进制会被 SIGKILL
   （exit 137、无 stderr）。发布必须进新的时间戳目录 —— `publish-mcp.py` 已经这么做了，
   别自己写 `cp` 覆盖。

3. **构建报 `replacement directory ... does not exist`** ⇒ Go module cache 被清空了。
   跑 `./script/restore-go-modcache.sh`，**不要**去改 go.mod 的 replace 路径。

4. **别跑 `go mod tidy`** —— 它要抓测试专用依赖（x/tools、x/term），沙箱下必失败。
   改依赖版本就手工同步 `go.mod` 里的 `require`。

5. **构建用 `GOPROXY=off`**（脚本已经设了），**且不要带 `-mod=mod`** ——
   只接受 `readonly`/`vendor`，带了会直接报错。

6. **源码树里不要留构建产物。** 产物落在 `./bin/`（发布后落在
   `~/.local/jumpserver-mgmt-runtime/<stamp>/`）。历史上编辑器备份文件
   （`main.go.1281232157774820236`）、swap 文件（`.!6188!xxx.go`）、
   零字节标记（`*-go-tmp-umask`）把源目录搞得像垃圾场，**已清理，别再产生**。

7. **大改之后要有一双没参与过的眼睛。** 改动 ≥ 300 行，或动了**协议 / 并发 /
   权限身份 / 失败路径 / 存量行为**其中任何一项，就必须起一个**独立 verifier 子代理**
   审一遍，提示词用 [`script/review-prompt.md`](https://github.com/yi-nology/jumpserver-kit/blob/main/script/review-prompt.md)
   里的模板。
   - **为什么不是「自己再读一遍」**：自己写的代码自己看不出盲区。本仓有过一次
     独立审阅挑出 3 个 HIGH：超时根本不是超时（还会把同机其他会话一起卡死）、
     兜底路径在最需要时失效、计划里的成本数字错 18 倍。**三个都不会自己暴露**。
   - **审阅结论要贴进汇报**：修了哪几条、哪几条判定为不属实。**没贴就等于没审。**
   - ⚠️ **审阅起不来时的退路**：子代理审阅会长时间零输出（两轮实测）⇒
     **别干等**，改成**按审阅清单逐条自己核**。**审阅的价值是「问对问题」，不是「谁来问」** ——
     本仓账号自动化那一轮，自审就核出了一处真缺陷
     （上游 snapshot 没有 `account_amount` 这个键，而**编出来的测试 fixture
     里偏偏有它** ⇒ 实现去读它 ⇒ 测试全绿、真机上永远读到空）。

8. **用 `reg.Add` 注册，不要用 `mcp.AddTool`。** `pkg/reg`（在 kit 仓）是**唯一**的
   注册入口，也是门禁所在。用 `mcp.AddTool` 就绕过了门禁，
   而门禁是「写操作必须 confirm」这条规则的唯一执行者。

9. **声明输出形状（`OutputSchema`），而且答案本身必须是契约类型。**
   服务端在返回前按 schema 校验，形状不对直接报错；机械守卫
   （`internal/arch/management_test.go` 的 `TestTheAnswerTypesAreFoundMechanically`）盯着「不许手拼 map 的答案或行」。
   ⚠️ **契约类型写了没人产出 = 契约不存在**（`biz/model/jmsms/contract_test.go` 盯着）。

10. **底座改动会波及执行面** ⇒ **改底座前先发 kit 的 tag**，再更新两个服务仓的
    `require`。⚠️ **不许在本仓的 `go.mod` 里写 `replace`**（那会让 `go.mod`
    与机器绑定）；本地联调用**不入库**的 `go.work`。

11. **AI 访问机器一律经堡垒机 koko(2222)**，不接受直连 SSH —— 直连会丢审计录像。
    （本服务不直连机器，但它给执行面签的连接令牌与委派凭据决定着会话身份，
    这条一样要守。）

12. **别把「编译过」说成「已完成」。** 汇报时给出：编译结果、测试结果、工具数变化、
    发布路径、自检结果、以及「**需要重启会话才生效**」这句话。


## 2. 管理面的实现要点（`biz/`）

### 2.1 分层：谁负责什么

- 底层是 JumpServer REST API，**v4 与 v5 都支持**。**传输、端点表与响应形状在 kit 仓的
  `pkg/jmsapi/`**（两个服务共用一份），本仓的 `internal/repository/` 是它之上的
  **领域仓储**（资产解析、授权展开、账号归属），`internal/service/` 是再上面的**领域规则**
  （纯函数，可离线单测），`biz/handler/` 只做解码/编排/渲染。
  ⚠️ **响应形状的助手（`Label` / `Display` / `Nested` / `AsSlice` / `List` / `Truncate` /
  `FieldName` / `AssetField` / `AssetSummary` / `SplitAssetLabel`）属于**协议**，
  一律去 `jmsapi` 取，**别在 `internal/repository` 里再写一份** ——
  曾经两面各写一份，结果管理面修好了 UTF-8 截断、执行面没修（按字节切，多字节字符被切坏）。
  截断的实现在 kit 的 `pkg/textx` **只此一份**（rune 边界那三行出过三次 UTF-8 事故），
  `jmsapi.Truncate` / 留痕的 `clip`/`truncate` 都是它的薄壳，只是后缀语义不同。
  ⚠️ **取线上字段用 `jmsapi` 的那组转换函数，不要直接 `convert.AnyToStr`**：
  `convert.AnyToStr(nil)` 返回的是字符串 `"<nil>"` —— 它长得像个值，过得了
  schema 校验，最后以一个不存在的名字送到模型面前。`jmsapi.Str` / `FieldStr` /
  `FieldBool` / `FieldInt32` 把「没有」和「有」分开（线上数字是 float64，
  进契约前要转 int32）。

### 2.2 v4 与 v5：差异只有一处

⚠️ **v4 与 v5 的差异只有一处**：v4 **没有** Chat AI 委派方案 ⇒ 委派在 v4 上不可用，
会话只能以 API 凭据的身份跑（= 管理员面）。**端点表、认证头、连接令牌体、响应形状两版一致**
（逐条对着上游 v4.10.20-lts 源码核过，见 kit 的 `pkg/jmsapi/version.go` 的注释与
`skills/jumpserver-ops/references/version-compat.md`）。

⚠️ **2026-10-31 起有真机了**（v4.10.20）：端点表 49 条、版本探测、
连接令牌（**账号 UUID**）、登录凭据（v4 没有 PrivateToken 的 API 出口 ⇒ 用账号密码
换 Bearer）、执行面真跑命令，**全部实测通过**（门控真调：
`internal/repository/livetest/v4_test.go`）。
⚠️ **新增的 8 个端点（文件传输留痕 / 账号自动化 / 网络拓扑）也在 v4 真机上核过**
（2026-11-01，无认证探测全部回 401 = 路由存在）⇒ **两版路径逐条一致**。
⚠️ **实测推翻了两条源码推断**：① v4 的列表端点也是**裸数组**（我曾按源码推断成
标准信封）；② `perm_account_invalid` 在 v4 上**不是**「账号字段错」，而是
「**没人被授权用这个资产**」—— 同一句话两个意思，极易误诊。
⇒ **读源码不等于跑真机**：这两条都是只有真机才能发现的。

### 2.3 账号自动化：分四族，两族要许可证

⚠️ **账号自动化分四族，两族要许可证**（2026-11-01 真机实测）：

| 族 | 端点前缀 | 社区版 |
|---|---|---|
| **推送账号** | `push-account-*` | ✅ 可用 |
| **账号发现** | `gather-account-*` | ✅ 可用 |
| 自动改密 | `change-secret-*` | ❌ **403** |
| 账号检查 | `check-account-*` | ❌ **403** |

⚠️ 那两个 403 **不是缺权限**：上游是
`permission_classes = [RBACPermission, IsValidLicense]` ⇒ **功能未授权**
（`settings.XPACK_LICENSE_IS_VALID` 为假）⇒ 角色里给了权限也没用。
⇒ **工具遇到 403 要说清是哪种**：让人去「加角色」是白费力气（那角色本来就有）。
⚠️ **方向要选对**：本工具能到的族（push/gather）是**核心功能**、不需许可证
⇒ 它们的 403 是**凭据缺权限**；说成「要买许可证」会把人带偏。
⇒ **许可证判断要做成查表，且默认「不涉及许可证」** —— 猜错的方向会让人
去买不需要的东西。

⚠️ **`push-account` 能达成同样的目标**（密钥不出堡垒机）：堡垒机生成密码 →
推到目标机 → 存进自家 vault ⇒ 密钥全程不经 agent。
⇒ **改密码不要用 `jms_account_update`** —— 那会让密钥**经过 agent**，
正是这个能力要避免的。
⚠️ ⚠️ **但 `push` 不是「轮换密码」**：`PushAccountAutomation.gen_nonlocal_accounts`
只给**机器上还没有该名字的账号**新建（`create_usernames = set(usernames) -
set(已有账号)`）⇒ **已存在的账号它不动** ⇒ **轮换已有账号的密码是
`change-secret` 的活（要许可证，本部署用不了）**。
⇒ **别把 push 说成「改密码」** —— 描述写错 ⇒ agent 会以为跑一次 push 就轮换了密码
（真机上就是这个误会：目标机已有 `opsuser` ⇒ 没有可推的账号 ⇒ 执行 failed）。
⇒ 同理 **`jms_account_update` 会让密钥经过 agent**；能用自动化就别用它改密码。
⚠️ **`push-account.save()` 里 `if not XPACK_LICENSE_IS_VALID: self.is_periodic = False`**
⇒ 社区版能**手动触发**、不能周期执行 ⇒ 计划建成 `is_periodic=false`，
由 agent 显式触发。

⚠️ **账号自动化的执行是异步的，答复只说「已受理」**：
POST `*-executions/` ⇒ 201 `{"task": "<celery task id>"}` ⇒ 后台跑。
⚠️ **那个 task id 就是 execution id**（上游用 `current_task.request.id` 建 execution）
⇒ 拿它就能直接查那次执行。
⚠️ **失败原因 API 拿不到**：execution 的 serializer **不暴露 `result`/`summary`**
⇒ 失败的执行只说「failed」⇒ 工具必须说清「原因不可得，去看 records」，
否则调用方会以为「failed = 没做事」。
⚠️ **每个账号的结果在 `*-records/`**（不是 executions）⇒ executions 是「跑了几次」，
records 是「每个账号成没成」。
⚠️ ⚠️ **但只有 push 有 records**：上游**没有注册** `gather-account-records`
（真机实测 **404**）⇒ **gather 的结果是「发现的账号」**，
在 `/accounts/gathered-accounts/`（= 端点表里的 `ResGathered`，
`jms_gathered_accounts` 已经读它）。
⇒ **端点表里的路径不能按名字推**：`gather-account-records` 看着该有，
实际上没有 ⇒ 写工具前**要真机打一次**（或对上游 `urls.py` 核）。
⚠️ **真机实测**：gather 触发后 execution 状态是 `running`，
`gathered-accounts` 里出现了 4 条真实发现 ⇒ **gather 是真能用的**（ansible 队列通）。

⚠️ **两族的 `accounts` 字段意思不一样**（同名字段、不同语义）：
- **push** = **要在机器上创建的账号名**（上游标了 required）；
- **gather** = **不参与执行**（gather 用「资产上已有的账号」登录发现）
  ⇒ **别给 gather 传 accounts**：传了等于没传，还会让调用方以为
  「要发现哪些账号」⇒ 那是**跑完之后**的结果（`gathered-accounts`）。
⚠️ **「连一条 record 都没有」本身就是信息**：堡垒机是**开始动某台机器时**才建
record 的 ⇒ 一条都没有 ⇒ **失败在准备阶段，一台机器都没到**（多半是堡垒机自己的
ansible/celery 组件没起）⇒ 答复要说这一点，否则调用方会跑去检查目标机
（查错了地方）。

### 2.4 审计面有四个域，各有各的字段形状

照搬别的域会静默读出 null：

| 域 | 端点 | 时间字段 | 方向/状态字段 | 资产字段 |
|---|---|---|---|---|
| 登录日志 | `/audits/login-logs/` | `datetime` | `status`（标签对象） | —— |
| 操作日志 | `/audits/operate-logs/` | `datetime` | —— | —— |
| 密码变更 | `/audits/password-change-logs/` | —— | —— | —— |
| **文件传输** | `/audits/ftp-logs/` | `date_start` | `operate`（标签对象） | **扁平 `name(address)`**，无 id |

⚠️ `/audits/ftp-logs/` 的名字是**历史遗留**（当年只做 FTP），它也记 SFTP。
⚠️ 这个端点在 **v4 与 v5 上路径一致**（v4 真机无认证探测：回 **401** = 路由存在，
而不存在的路径回 404）。

⚠️ **审计留痕里的资产名是「传输当时的快照」** ⇒ **地址才是身份**：
机器改名后旧名被新机器复用 ⇒ 按名字匹配会把**另一台机器**的历史算到它头上。
⇒ 匹配**地址优先**；名字只在记录没有地址时才用。
⚠️ 但**资产已从堡垒机删除**时，它只能**按名字问**（地址不在列表里了）⇒ 兜底路径
必须**认名字** ⇒ 否则「问一台已退役的机器」永远得到「从没被传过文件」。
⇒ 兜底时按**引用的形状**决定比哪半（地址形状比地址，其余比名字），并在 note 里说明。

⚠️ **假堡垒机的数据要用真机读到的字节，别自己编形状。** 审计类端点**每个的字段名都不一样**：
`/audits/login-logs/` 是 `ip`/`datetime` 且 `status` 是标签对象；
`/audits/operate-logs/` 是 `remote_addr`/`datetime`；
`/audits/ftp-logs/` 是 `remote_addr`/`date_start` 且 `operate` 是标签对象
——**照搬登录日志的字段名会静默读出 null**（`jms_login_logs` 早就踩过这个，
注释里写着）。⇒ 真调一次，把真字节抄进测试的假堡垒机。

### 2.5 文件传输留痕：与执行面自己算的 checksum 是两份不同的东西

- **文件传输留痕是执行面文件操作的「堡垒机侧记录」**：
  `jmsx_put_file` / `jmsx_get_file` / `jmsx_relay` 全都经堡垒机，堡垒机自己记了
  每一次传输（`jms_file_transfers` 读它）。
- ⚠️ **它与执行面自己算的 checksum 是两份不同的东西**：checksum 是**传输当时的自证**
  （「这些字节到了」），堡垒机的留痕是**记录**（「这次传输被尝试过、是谁、对哪台机器、成没成」）
  ⇒ **谁也替代不了谁**。

### 2.6 取数与过滤（协议层的规矩在 kit 仓，这里是它的管理面实例）

⚠️ **端点的过滤器不能信：不被声明的参数被静默忽略** —— 完整规则在 kit 的
AGENTS.md §4.1（协议层）。**管理面的实例**：`jms_file_transfers` 就是**本地过滤**的
（服务端只用来缩小窗口）。⚠️ 这条不是洁癖：服务端过滤一旦没生效，答复**看起来完整**
（`total` 照样算、`truncated=false`）⇒ 「没传过文件」这个**取证假阴性**就是这么来的。
⇒ **本地比较前先把日期归一化**（`normDate`：ISO `2026-10-20` ⇒ `2026/10/20`）——
记录的时间戳是 `2026/10/28 00:05:33 +0800`，直接拿 `2026-10-20` 去比字符串，
**`/` 排在 `-` 之后 ⇒ 每一行都通过** ⇒ 返回了请求日期之前的记录。
⇒ **读不出来的日期要报错**（`cannot read since=...`），不能当成「没有记录」。

⚠️ **列表端点把关联对象返回成 null**（`perms/asset-permissions/` 的 assets/users、
`acls/command-filter-acls/` 的全部）—— 凡是「这个授权覆盖这个资产吗」的问题，
**必须取详情**，否则会得出「没人能访问」的假结论。

### 2.7 差异写在「能力」上，不写在「版本」上（管理面这一侧）

（完整规则在 kit 的 AGENTS.md §4.1；这里是本仓这一侧要做的。）

- **管理面只贡献自己的策略**：`UnknownPresented`（本仓的委派工具在未知态是**拒绝**的）
  与 `UnknownRefused`。判定与句子都在 kit 的 `pkg/jmsapi`。
- ✅ **三条结构断言都有机械守卫**（kit 的 `pkg/arch`）：
  | 断言 | 守卫 |
  |---|---|
  | 「控制流里不许出现版本号」 | `TestTheVersionDoesNotLeaveTheProtocolLayer` |
  | 「措辞只有一份」 | `TestTheCapabilityWordingIsNotRewrittenElsewhere`（**跨仓** ⇒ 要三棵树同时在手） |
  | 「每个签委派凭据的地方都要先问能力」 | `TestADelegationCallSiteConsultsTheVersion`（+ `script/mutate-version-guard.py` 的 **7 条变异**钉住它） |

### 2.8 `internal/service/` 装的是「判断」，不是「取数」

现在里面是三组规则，都是纯函数、都有离线单测（不用连堡垒机）：
- `diagnosis.go` —— 五个诊断类工具共用的词汇：评级（fail 压过 warn）、findings 子集、
  advice 拼接、标准结论句式。**以前它住在 handler 里、一条单测都没有**，
  只能靠调工具对着真堡垒机验。
- `audit.go` —— 审计价值的判定：`error_reason` 为空 = 真有交互。
  ⚠️ **「有录像」不等于「有审计价值」**：本机 617 个会话里 518 个是
  `replay_unsupported`（只录下握手半截），按录像数算覆盖率会得出「99.5% 都被审计了」。
  `Coverage` 把三个比率的不同分母钉死：`command_rate` 的分母是**有证据的会话数**，
  不是会话数（否则会算出超过 100%）。
- `commandfilter.go` —— 命令过滤器与命令组的匹配口径（每行作为独立词出现在输入任意位置即命中，
  「rm」命中「rm -rf」「sudo rm」，不命中「rmdir」；regex 组按正则；ignore_case 决定是否折叠大小写），
  以及作用域口径（空列表 = 对所有人，非空 = 只对列出的那些）。
  ⚠️ 这组规则决定「这条破坏性命令会不会被拦下」——**假阴性不是一句话说错，是命令真跑了**。

### 2.9 输出形状是契约，不是随手拼的 map

- 列表类工具统一用 `render.ListOf[jmsms.XxxSummary]`（信封是 `count`/`total`/`truncated`/`items`/
  `note`），**行也有具名类型** —— 信封有类型而行没有，等于让调用方去猜行里的
  字段名，而少一个字段是看不见的。服务器在返回前按 schema 校验每一行。
- ⚠️ **形状已知的工具要直接产出契约类型**（`jmsms.XxxResult`），不要现拼 map：
  map 的键名写错编译期不报错，而契约类型写错编译期就红。管理面 97 个工具
  现在全部有具名契约（`render.Object` 与 `render.List` 都已删除），
  **而且答案本身就是那个类型**（`internal/arch/management_test.go` 的 `TestTheAnswerTypesAreFoundMechanically` 盯着）。
- ⚠️ **一个工具一个契约类型**：工具的几种答法（演练 / 被拦 / 跑了、列全部 /
  查单个）共用同一个类型，区别只在填了哪些字段。两种答法形状真的不同
  （`jms_system_role_permissions`：不点名答角色清单、点名答权限路径），
  就让**行类型取并集**，并在 `note` 里写明这次答的是哪一种 ——
  契约声明着一种形状、答复给另一种，是比缺字段更坏的谎。
- ⚠️ **组合答复类型**住在 `biz/model/jmsms/outputs.go`（手写）：
  诊断类工具的答复 = 共享的 `jms.Diagnosis`（用 `service.Diagnosis` 装配）
  + 该工具自己的语境字段。信封放不下的语境（列表类工具的「按类型分布」、
  「这是哪个角色」）写进 `note`，别塞进计数字段。
- ⚠️ **契约缺字段时补契约，不要静默丢弃**：这轮改造里，账号列表丢了
  `asset`/`address`、资产详情丢了 `platform`/`protocols`/`connectivity`、
  连通性报告丢了「`-` 状态」的资产 —— 都是当场发现、当场补进 IDL 的。
  一个被丢掉的字段，调用方看不到自己少知道了什么。
- ⚠️ **工具层不许出现 API 路径字面量，也不许调裸传输**（`c.Do` / `c.ListAll`）。
  有一道机械守卫盯着：`biz/handler/protocol_guard_test.go` 扫源码，违规就红。
  要访问堡垒机就去问**资源**（`jmsapi.ResPermissions` 等），网关负责拼 URL。

### 2.10 `internal/service/policy/policy.go` 是危险分级中枢

决定哪些操作要 `confirm`、要写什么提示。门禁本身在 kit 的 `pkg/reg/`
（guardx/invoke 链 + permgate 能力判定）。
⚠️ **`confirm` 必须是入参里的必填字段**（不带 `omitempty`）⇒
没带 confirm 的调用会被 SDK 的 schema 校验先拒掉，报的是
`required: missing properties: ["confirm"]`。门禁是第二道防线（schema 一旦放松就顶上）。
⚠️ **代价：这类调用原本不进留痕** ⇒ kit 的注册表在 SDK 的服务端上挂了一层中间件
（`pkg/reg/rejected.go`），记下「**工具压根没跑到**就被 SDK 拒掉」的调用。

### 2.11 委派凭据（Chat AI 委派）

- `internal/repository/delegation.go` + `biz/handler/delegation.go`：
  **Chat AI 委派凭据**机制。一个凭据只授权一次调用（一个 method/path/query/body）、
  一次有效、60 秒内过期。它**不额外授权** —— 调用者仍走正常 RBAC。
  用它做「不想长期可达」的操作（删除、改权限）。
- ⚠️ **委派请求在堡垒机侧的身份是 `JMS_DELEGATION_USER_ID` 指向的用户（例如 `ai`，
  即你自己的那个账号），不是 token 的所有者（admin）**。想「以某个用户身份」操作
  （注册 SSH 公钥）只能靠它。

### 2.12 其余实现要点

- `biz/handler/onboarding.go`：账号/用户的创建与改密、权限明细、用户有效权限、
  接线体检（`jms_doctor`）。
- `biz/handler/scope.go`：资产树（建节点/移动资产）、用户组、已注册终端。
- 授权的 `accounts` 传**账号名**（`["opsuser"]`），不是 UUID；`@ALL` 才是「全部账号」。
- ⚠️ **`web`(Website) / `cloud`(Kubernetes) 类资产本来就没有协议和账号** ——
  把它们报成「永远连不上」是误报（曾因此报出 6 条假警报）。
- 工具名统一 `jms_` 前缀；写操作统一要 `confirm`（由策略表统一执行）。
- ⚠️ **审计留痕的「资产已删除」要能答**：`jms_asset_evidence` 与
  `jms_file_transfers` 都**不能**因为机器退役了就答「no such asset」——
  机器没了恰恰是最该问「谁动过它」的时候 ⇒ 按记录自带的**扁平标签**
  （`name(address)`）兜底匹配，而不是直接报错。



## 3. 管理面专属铁律

1. **工具数是 97，被测试钉住。**
   `biz/router/policy_test.go` 的 `TestToolCount` 直接写死
   `if got := len(r.Specs()); got != 97` ⇒ 加一个工具**必须**改这个数（连同
   README 与 skills 里引用的数字）。⚠️ 这个数**是给 agent 的契约**，不是内部计数：
   「这个 server 给我哪些工具」变了，就是变了。
   ⚠️ 同一个测试还查**目录与注册表一致**：策略表里有、注册表里没有（或反过来）都红。

2. **契约域是 17 个。** `idl/jmsms/` 下按域拆的 thrift：
   `account` / `asset` / `automation` / `cmdfilter` / `delegation` / `evidence` /
   `gather` / `job` / `network` / `permission` / `rbac` / `records` / `refs` /
   `security` / `status` / `touch` / `user`（外加入口 `jmsms.thrift`）。
   ⚠️ **每一份**都声明 `namespace go jmsms` —— 拆的是文件，不是词汇。
   ⚠️ 改了 IDL 要跑 `./script/gen-idl.sh` 重新生成 `biz/model/jmsms/`（产物入库）。
   ⚠️ **契约类型写了没人产出 = 契约不存在** ——
   `biz/model/jmsms/contract_test.go` 盯着（守卫身在 kit 的 `pkg/contractscan`）。

3. **doctor 的行为是契约。**
   - `jumpserver-mgmt --doctor` 与 `jumpserver-mgmt doctor` 等价；
     退出码沿用 OEKIT `pkg/cli` 的约定：`0` = 正常、`1` = 有问题（看 `FAIL` 行）、
     `2` = 用法错误。⚠️ **`--help` 的退出码是 0**，不是 2 —— 问帮助不是错误路径。
   - 检查项：`api_token` / `api_reachable` / `identity` / `version` /
     `delegation` / `delegation_identity` / `assets_without_protocols` /
     `assets_without_accounts` / `permissions_without_accounts` /
     `permissions_without_members` / `permissions_with_unknown_accounts`。
   - ⚠️ **`web`(Website) 和 `cloud`(Kubernetes) 类资产不参与「无协议/无账号」的检查** ——
     它们本来就没有终端协议和 SSH 账号。早期版本把它们算进去，一次报出 6 条假警报，
     而那 6 个服务全都好好地在跑。
   - ⚠️ **能力是三态的**：探测失败时 doctor 的 `delegation` 一行**只字不提「没测出来」**
     这件事已经出过一次（详见 kit 的 AGENTS.md §4.1）⇒ **两个 doctor 必须按三态措辞**。
   - ⚠️ **会话内的 `jms_doctor` 不报留痕** —— 它刻意不带配置（注册表不传配置），
     「不知道」与「没有」是两种沉默。CLI 的 `--doctor` **报**留痕（`audit_trail` 一行）。
   - ⚠️ `audit_trail` 检查会**在留痕目录里建一个临时文件再删掉**（探测可写性）⇒
     **诊断带一点写副作用**；不可写的留痕目录现在让 `--doctor` **退出码变成 1**
     （原来 0）—— 这是有意的（假绿灯更糟），但拿 doctor 退出码做健康门禁的部署要知道这件事。

4. **身份/status/ACL/RBAC 那一轮的教训（别再犯）：**
   - ⚠️ **探针失败时不许捏造身份**：不可达/失败时**不能**编一个「凭据已被接受」，
     也不能让 doctor 的失败分支捏造一个零值身份。
   - ⚠️ **清空门不能退回 `!= ""`**：那会把「空的」与「没设」混成一样。
   - ⚠️ **空 username 不算成功**。
   - ⚠️ **同名角色不跨 scope 合并**：两个 scope 里同名的角色是两个角色。
   - ⚠️ **401 不等于 `auth_ok=true`**。
   - ✅ 这些都有机械守卫：`script/mutate-identity-guard.py`（6 条变异：不可达时捏造
     「凭据已被接受」/ doctor 失败分支捏造零值身份 / 清空门退回 `!= ""` /
     空 username 当成功 / 同名角色跨 scope 被合并 / 401 当 `auth_ok=true`）。
     ⚠️ 它用 **overlay**，所以 **`-overlay` 必须写在子命令之后**
     （`go test -overlay=f.json ...`）：写成 `go -overlay f.json test ...` 会被 go 工具
     回「flag provided but not defined」—— **退出码非零，但与「变异被抓」完全无法区分**
     ⇒ 每条变异都会判成「canary 没炸」。这坑真踩过一次（6/6 全误判）。
   - ⚠️ **三种强度盯同一批修复**：单测看**代码**、变异脚本看**守卫**、
     黑盒探针（`script/fake-bastion.py` 让 `/users/profile/` 与列表第一行
     **故意不一致**，再像 MCP 客户端一样直驱二进制）看**交付二进制的对外答复**。
     真机上 profile 与列表第一行恰好都是 admin、分不出身份来自哪条路 ——
     只有假堡垒机能让两者分歧，所以黑盒那一格不可由前两种替代。

5. **答复的脱敏走「同一件事的两半」的形状那一半。**
   本仓的 `biz/handler/redact.go` 负责**形状**（保留键名与 scheme：
   `Authorization: Bearer [REDACTED]`、`{"password":"[REDACTED]"}`），
   **词表**在 kit 的 `pkg/redact.IsCredentialKey`（**只加那一份**）。
   ⚠️ **要加词就加 kit 那一份** —— 别往答复路径里另加一条词表：
   那会让同一个词在两条路径上**一个抹一个不抹**。
   ⚠️ **形状不同不是漂，词表不同才是漂**：这一条曾经漂过 —— 答复路径自己带一份词表，
   与留痕那份差 **14 条**（`passphrase`/`credential`/`cookie`/`kubeconfig`/
   `id_token`/`refresh_token`/…），而且**完全不认 JSON 形态** ⇒
   `{"password":"x"}` 原样进 agent 的上下文窗口（实测 12/12 全漏）。
   ⚠️ 答复路径**非标量**的值要**整字段**抹（`{"secret":{"value":"x"}}` ⇒ 整块抹掉）——
   只抹片段会**留下内层的凭据**并把 JSON 改坏。
   ⚠️ 答复路径的脱敏**幂等**（同一段文本脱敏两次结果相同）。

---

## 4. 验证要求（交付前必须过）

| 检查 | 怎么过 |
|---|---|
| 编译 | `./script/build.sh` 无错（产物落在 `./bin/jumpserver-mgmt`） |
| 单元测试 | 同脚本内置 `go test ./...`，必须全绿（脚本会打印失败日志，不吞掉） |
| 启动 | `./script/bootstrap.sh` 能起一个进程（stdio 上说 JSON-RPC） |
| 工具数 | **97**（`biz/router/policy_test.go` 钉住） |
| 目录一致 | 策略表与注册表必须一一对应（同上） |
| 契约 | 契约类型必须真被产出（`biz/model/jmsms/contract_test.go`）；**答案本身必须是契约类型**（`internal/arch/management_test.go` 的 `TestTheAnswerTypesAreFoundMechanically`） |
| 分层 | 底座不得依赖任何服务；不得 import 执行面；IDL/包/模型三名对齐（`internal/arch/`） |
| **凭据** | 明文凭据不许进仓（底座 `pkg/arch/credential.go` + 本仓 `internal/arch/management_test.go` 的 `TestThePlaintextCredentialsAreNotInTheRepository`）⇒ 形状 + 语境两半都认；`jms:allow-credential` 是**行级**豁免且**被记账** |
| 工具不许调裸传输 | `biz/handler/protocol_guard_test.go` 扫源码 |
| 自检 | `./bin/jumpserver-mgmt --doctor` 退出码 0 |
| 真调 | 重启会话后实际调一次新工具；**重启前**可用 JSON-RPC 直接驱动已发布二进制（参考 `script/verify-mcp-launch.py`） |

⚠️ **改了输出形状一定要真调一次**（读工具对真堡垒机，写工具对假堡垒机）。
schema 校验只在答案被返回时才跑，所以「编译过 + 单元测试绿」**不等于**
形状对 —— `jms_permission_grant` 的契约与答案不符，是真调时才发现的。

⚠️ **别把「编译过」说成「已完成」。** 汇报时给出：编译结果、测试结果、工具数变化、
发布路径、自检结果、以及「**需要重启会话才生效**」这句话。

---

## 5. 沙箱速查

| 现象 | 原因 | 修法 |
|---|---|---|
| exit 137、无 stderr | 覆盖写二进制，路径被执行策略拒了 | 发布到新时间戳目录（`script/publish-mcp.py`） |
| `replacement directory ... does not exist` | Go module cache 被清 | `./script/restore-go-modcache.sh` |
| 依赖解析要去下载（`go: downloading ...`） | module cache 里少了那个模块 | `./script/restore-go-modcache.sh` |
| `missing go.sum entry for module ...` | go.sum 少了那个精确版本的条目 | `GOFLAGS=-mod=mod GOSUMDB=off GOPROXY=off go build ./...` |
| `go mod tidy` 失败 | 要抓测试专用依赖（x/tools、x/term），沙箱下拿不到 | 别 tidy，手改 go.mod 的 require |
| `-mod may only be set to readonly or vendor` | 模块模式只接受 readonly/vendor | 去掉这个 flag（构建脚本已去掉） |
| 源码树里冒出 `.go.1234567890` 之类文件 | 编辑器/工具留下的备份 | 清理掉，别提交 |
| **底座改了编译不过** | `require` 指向的 kit 版本还没发 | 先发 kit 的 tag，再更新本仓的 require |

---

## 6. 提交约定

- 提交前确认 `git status` 里**没有**构建产物、备份文件、二进制（`./bin/` 不入库）。
- 提交信息用中文，说明**改了什么行为**（不是「更新代码」）。
- ⚠️ **`go.work` / `go.work.sum` 不入库**（见 `.gitignore`）：它是本机的联调接线。
- 不要提交 `~/.minimax/mcp.json` 里的 token —— 那是仓外的运行时配置。
- **改了工具要同步改 `skills/`** —— 那里的工具表是 agent 选工具的依据。
  仓里有一道检查（`./script/check-skill-drift.sh`）：文档提到的工具 vs 代码注册的工具，
  有差异就说明脱节了。

---

## 7. 相关文档

| 文档 | 内容 |
|---|---|
| [`README.md`](README.md) | **对外入口**：定位、前置条件、五分钟装好、安全模型、已知行为 |
| [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md) | 环境变量、接线、资产上线流程、自检、97 个工具速查、故障对照表 |
| [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) | 构建、发布、迭代流程、验证要求、常见坑（**改代码前看这个**） |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | 怎么参与：跑测试、加工具的规矩、提交约定 |
| [`SECURITY.md`](SECURITY.md) | 漏洞报告渠道、凭据处理纪律 |
| [`CHANGELOG.md`](CHANGELOG.md) | 变更记录（Keep a Changelog） |
| [`docs/JUMPSERVER-API-NOTES.md`](docs/JUMPSERVER-API-NOTES.md) | 管理面细节 |
| [`skills/`](skills/) | **给 AI 的操作手册**（jumpserver-ops / jms-onboarding / jms-access-review） |
| **kit 仓的 AGENTS.md** | <https://github.com/yi-nology/jumpserver-kit/blob/main/AGENTS.md> —— **共享纪律**（铁律、底座怎么发版、跨仓守卫） |
| **kit 仓的 ARCHITECTURE.md** | <https://github.com/yi-nology/jumpserver-kit/blob/main/docs/ARCHITECTURE.md> —— 三仓的边界与依赖方向 |

**AI 访问机器一律经堡垒机 koko(2222)**，不接受直连 SSH —— 直连会丢审计录像。

