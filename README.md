# jumpserver-mgmt — JumpServer 堡垒机的**管理面**（MCP + HTTP API）

让 AI agent **经堡垒机**管理你的基础设施：资产、账号、授权、命令过滤、登录 ACL、审计检索 ——
全程留审计录像。

同一个工具目录、两个面：**MCP**（stdio / streamable HTTP）与 **REST**
（脚本、人、非 MCP 的调用方直接调）。

> 📖 **同时支持 JumpServer v4 与 v5。** 协议层是同一套（端点表、认证头、连接令牌体、响应形状
> 逐条对着上游 v4.10.20-lts 源码核过）。v4 没有 Chat AI 委派方案 ⇒ v4 上的会话只能以 API
> 凭据的身份跑（= 管理员面），`--doctor` 会说明白。

| 仓库 | 是什么 | 工具数 |
|---|---|---|
| **`yi-nology/jumpserver-mgmt`**（本仓） | **管理面**：资产 / 账号 / 授权 / 连通性 / 命令过滤 / 标签 / 审计 / 登录策略 | 97 |
| [`yi-nology/jumpserver-exec`](https://github.com/yi-nology/jumpserver-exec) | **执行面**：经堡垒机 koko(2222) 在受管机器上跑命令、传文件 | 15 |
| [`yi-nology/jumpserver-kit`](https://github.com/yi-nology/jumpserver-kit) | 共用底座：协议层 / 门禁 / 输出契约 / 留痕 / 脱敏（**是库，不是服务**） | — |

**AI 访问机器一律经堡垒机**，不接受直连 SSH —— 直连会丢审计录像。

---

## 1. 前置条件

| | 需要什么 |
|---|---|
| 管理面 | 一个 JumpServer 部署（v4 或 v5）+ 一个 **PrivateToken** |

PrivateToken 在堡垒机上拿：v5 是 `设置 → API 授权`，v4 是 `用户 → API 密钥`。

---

## 2. 五分钟装好

### 2.1 安装

```bash
# 方式一：go install（需要 Go 1.26+）
go install github.com/yi-nology/jumpserver-mgmt@latest

# 方式二：release 二进制（见 Releases 页）
curl -LO https://github.com/yi-nology/jumpserver-mgmt/releases/latest/download/jumpserver-mgmt-linux-amd64
chmod +x jumpserver-mgmt-linux-amd64
```

### 2.2 接线（MCP 配置）

```json
{
  "mcpServers": {
    "jumpserver": {
      "command": "jumpserver-mgmt",
      "env": {
        "JUMPSERVER_URL": "https://jumpserver.example.com",
        "JUMPSERVER_TOKEN": "<你的 PrivateToken>"
      }
    }
  }
```

⚠️ `JUMPSERVER_URL` 填部署的根地址，**不要带 `/api/v1`**（客户端自己拼），也不带结尾斜杠。
⚠️ 别把 token 写进命令行参数 —— 进程参数同机可见。走环境变量或 `~/.config/jumpserver-mgmt/` 下的
文件（0600）。

### 2.3 接线（HTTP 面，可选）

同一个工具目录还有第二个面：**REST + MCP over streamable HTTP**。默认关着，
配两个环境变量就开：

```json
"env": {
  "JUMPSERVER_URL": "https://jumpserver.example.com",
  "JUMPSERVER_TOKEN": "<你的 PrivateToken>",
  "JMS_HTTP_ADDR": "127.0.0.1:8731",
  "JMS_HTTP_TOKEN": "<openssl rand -hex 32 生成一个>"
}
```

| 路由 | 干什么 |
|---|---|
| `GET /api/v1/health` | 存活探针（**唯一**不要令牌的路由，只报名字/版本/工具数） |
| `GET /api/v1/tools` | 工具目录：名字、风险、确认要求、入参/出参形状 |
| `POST /api/v1/tools/{name}/invoke` | 调一个工具（Bearer 令牌） |
| `/mcp` | MCP over streamable HTTP（Bearer 令牌） |

⚠️ **没有「不开认证」这回事**：`JMS_HTTP_ADDR` 配了却不配 `JMS_HTTP_TOKEN`，启动即拒。
⚠️ 地址写 `127.0.0.1`，别写 `0.0.0.0` —— 这些路由带着部署自己的凭据去打堡垒机，
暴露到网络需要你自己有代理与 TLS 的故事。
⚠️ **调用结果的读法与 stdio 逐字相同**：拒绝、失败、入参不合形状都是 **HTTP 200 +
`is_error`**（请求没错，结果是拒绝）；请求本身坏才是 400，没有这个工具才是 404。
契约文档在 [`swagger/openapi.json`](swagger/openapi.json)（生成物，与工具同源）。

- **只当服务部署时**用 `jumpserver-mgmt serve http`（只开 HTTP 面，长跑到信号为止）；
- **给其他服务同步调用工具时**用 `serve rpc`（只开 kitex 面，`JMS_RPC_ADDR` +
  `JMS_RPC_TOKEN`）或 `serve all`（HTTP+RPC 常驻；两者都没配拒绝启动；不含 stdio）；
- 默认的 `serve` 是 stdio，配了 `JMS_HTTP_ADDR` / `JMS_RPC_ADDR` 会**顺带**开
  对应的面 —— 一个进程、同一道门禁、同一份留痕。

### 2.4 自检

```bash
jumpserver-mgmt --doctor   # 接线 + 堡垒机配置体检
```

退出码：`0` = 正常，`1` = 有问题（看 `FAIL` 行），`2` = 用法错误。
`--doctor` 还会报出**静默失败**：资产没有 protocols、资产没有账号、授权一个账号都没授予。

装完连不上先看 [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md) 的故障对照表。

---

## 3. 这个服务管什么

**管堡垒机本身**：资产、账号、授权、节点树、命令过滤、登录 ACL、剪贴板 ACL、数据脱敏规则、
批量作业、审计检索（会话 / 命令 / 登录 / 操作日志）、网络拓扑、接触史、文件传输留痕、
取证时间线。

它回答的是「**这个堡垒机现在是什么样子、谁能干什么**」。

契约按域拆成 17 份 thrift（`idl/jmsms/`），每个域一个文件：

| 域 | 管什么 |
|---|---|
| `asset` / `refs` | 资产、节点树、标签、引用关系 |
| `account` / `user` | 账号、用户、SSH 公钥、密码 |
| `permission` / `rbac` / `acl` | 授权、RBAC、登录 ACL、剪贴板 ACL |
| `cmdfilter` / `security` | 命令过滤、登录策略、数据脱敏规则 |
| `automation` / `job` | 批量作业、作业执行 |
| `records` / `audit` | 会话、命令、登录、操作日志的检索 |
| `evidence` / `gather` | 取证时间线、证据收集 |
| `network` / `status` | 网络拓扑、连通性、运行状态 |
| `delegation` / `touch` | 委派凭据、接触面 |

完整工具清单见 [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md) §6。

### 3.1 工具数与凭据无关（执行面才有关）

管理面**始终注册全部 97 个工具**，有没有凭据都一样 —— 注册看的是工具目录，
不看你配没配 `JUMPSERVER_TOKEN`。**没有凭据时，调用会在进门那一刻失败**
（报「没配 URL / 没配 token」），而不是那个工具从清单里消失。

⇒ 所以 `jumpserver-mgmt tools` 列出的数目**永远是 97**；它不变少，
少的是「能用」的那部分。

⚠️ **执行面不一样**：它有一个工具（`jmsx_inventory`，查资产清单）需要 REST
client ⇒ 没有 `JUMPSERVER_TOKEN` 时它**不注册** ⇒ 执行面是 14 而不是 15。

---

## 4. 安全模型（这是本仓的重点）

给 agent 一个能改堡垒机的工具，最大的问题不是功能，是「**谁让它做的、它做了什么、拦没拦住**」。
这四件事本仓都做了：

| 机制 | 做什么 | 在哪 |
|---|---|---|
| **门禁** | 改堡垒机的工具必须先 `confirm`；危险命令（`rm -rf` 等）**先用 shell 解析器把命令解析成 AST**，再按策略表判只读性 —— 不是字符串匹配（解析器在上游 `oagentkit/tools`，策略表在底座 `pkg/risk`） | `pkg/risk` + `pkg/reg` |
| **留痕** | 每次调用一行：调了什么工具、参数是什么、门禁每一段的判定与理由、放行与否、耗时。**被拦下的调用根本没到堡垒机**（堡垒机没有会话可查），所以只有这里查得到 | `pkg/audit`，落盘 JSONL |
| **脱敏** | 参数、错误、拒绝理由里的凭据按**同一份词表**遮蔽，两个出口（答复 / 留痕）逐字节一致 | `pkg/redact` |
| **只读模式** | `JMS_READ_ONLY=true` ⇒ 一切会改东西的工具直接拒绝 | 管理面与执行面都支持 |
| **HTTP 面** | REST 与 MCP over HTTP **共用同一个注册表** ⇒ 同一道门禁、同一套确认、同一份留痕；除 `/api/v1/health` 外全部要 Bearer 令牌（常量时间比较），没有「不开认证」模式 | `pkg/httpapi`（底座） |
| **RPC 面** | kitex/thrift 第三张脸，同一注册表同一扇门：拒绝是 is_error **答案**、token 错是**传输错误**；无匿名（目录也认证） | `pkg/rpcapi` + `pkg/faces`（底座） |

> 上面三个 `pkg/*` 现在住在
> [`jumpserver-kit`](https://github.com/yi-nology/jumpserver-kit) —— 它们是两个服务
> 共用的底座，不认任何一个服务。

`skills/` 下有 3 个给 AI agent 的操作手册（选工具的正确姿势、机器上线怎么走、
「谁能登哪台」怎么查）。**装上 skill 的 agent 才知道怎么正确地用这些工具**。

---

## 5. 已知行为（别当成 bug）

- **`jms_connectivity_check` 对 DB / Redis 平台是假阳性** —— JumpServer 没做真实协议握手。
- **Website / Kubernetes / Redis 平台不支持连通性测试**，工具会直接报错而非返回 `err`。
- **`jms_job_run` 不返回 stdout/stderr** —— JumpServer v5 的 API 根本不暴露执行输出，
  要拿真实输出用执行面的 `jmsx_exec` / `jmsx_run`。
  **但作业实际跑了什么命令、谁跑的、哪台失败**用 `jms_job_audit`。
- **命令以资产账号身份执行**，JumpServer 不提权；要 root 自己加 `sudo -n`。
- **v4 没有委派凭据方案**，会话只能以 API 凭据身份跑（= 管理员面）。

---

## 6. 目录布局

```
.
├── main.go / init.go / server.go / workers.go / shutdown.go
│                                ← 入口层（**仓根**的 package main）：serve / doctor / tools / version
├── cmd/                        ← 子命令（入口层 import 的**普通包**，不是二进制）
│   ├── serve/                  ← 跑服务：stdio MCP（默认）；http/rpc/all 三种子命令选面
│   ├── doctor/                 ← 自检：接线与堡垒机配置
│   └── tools/                  ← 工具目录：名字、风险、确认要求；`tools openapi` 生成 HTTP 契约
├── biz/                        ← 业务
│   ├── handler/                ← 97 个工具的实现（一个域一个文件）
│   ├── model/jmsms/            ← 输出契约的生成物（thriftgo，产物入库）
│   └── router/                 ← 装配：哪个域注册哪些工具
├── internal/                   ← 模块私有
│   ├── config/                 ← 配置（环境变量 → 结构体）
│   ├── repository/             ← 数据访问层：堡垒机的资源
│   │   └── livetest/           ← 打真堡垒机的测试助手
│   ├── service/                ← 领域规则：纯函数，不碰网络，可离线单测
│   │   └── policy/             ← 工具目录的风险侧：改不改堡垒机、要不要 confirm
│   ├── arch/                   ← 机械守卫（见下）
│   ├── middleware/             ← （空）HTTP 中间件：HTTP 面的认证与路由在底座 pkg/httpapi
│   └── rpchandler/             ← （空）RPC 服务端：这一面不对外提供 RPC
├── idl/jmsms/                  ← 输出契约的 thrift 定义（入口 + 按域拆的多份）
├── pkg/                        ← （空）通用库在 kit 仓，不在这里放东西
├── swagger/                    ← HTTP 面的 OpenAPI（生成物：`tools openapi`，守卫逐字节比对）
├── etc/ deploy/ monitoring/ rpcx/ kitex_gen/
│                                ← （空）hz 骨架里的其余层：见各目录里的 README
├── script/                     ← 构建 / 发布 / 生成 / 校验 / 变异自证脚本
├── skills/                     ← 给 AI agent 的操作手册（3 个 skill）
├── docs/                       ← 配置与排障
└── AGENTS.md                   ← 给 AI 的迭代指南（改代码前先读）
```

目录按 **CloudWeGo hz 的标准骨架**摆（`cmd/` `biz/` `idl/` `internal/` `pkg/` `etc/` `script/`）。

⚠️ **入口层在仓根**：一个仓一个二进制，二进制就是仓根那个 `package main` ⇒
`go install github.com/yi-nology/jumpserver-mgmt@latest` 装出来的就是它
（**不是** `…/cmd/jumpserver-mgmt@latest` —— 那个路径没有了）。

⚠️ **`cmd/` 里是子命令，不是二进制**：那些包是入口层 import 的普通包。
而**实现不住在仓根** —— 仓根那个 package 是 `main`，Go **不许 import 一个
main 包** ⇒ 子命令的实现只能在 `cmd/` 下。

⚠️ **空的层不是「忘了做」**：每一层都有自己的 README，写着它为什么空、
什么时候不再是空的。

### 6.1 为什么是三个仓

三个仓回答三个问题：

- **管理面**回答「这个堡垒机现在是什么样子、谁能干什么」；
- **执行面**回答「在那台机器上做这件事，会发生什么」；
- **底座**是前两者**共同**认的东西（同一套协议、同一个门禁、同一份契约、同一个发布节奏）。

放一个仓里的时候，底座是「两个服务都用的 `pkg/`」—— 一旦它伸手进某个服务，
它就不再是共用的，而是那个服务披着共用名字的私有代码。所以它有自己的仓。

拆开的代价是明面上的：底座要有版本，两个服务要各自 `require` 它。
**规矩**（写在底座仓的 README 里）：底座**只在公开 API 变了才发版**；
改了底座 ⇒ 先发 kit 的 tag ⇒ 再更新两个服务的 require ⇒ 再发服务。

### 6.2 守卫（`internal/arch/`）

机械守卫住在测试里，**每个提交都跑**，不靠人记得：

- 工具答复必须是**契约类型**，不是手搓的 map（手搓 map 与契约在 JSON 上分不出来）；
- 凭据词表不许把这个仓自己的参数名误判成凭据（误判 = 留痕不再记录那个参数）；
- `swagger/openapi.json` 与工具目录**逐字节**对得上（`openapi_test.go`；「会红」这件事
  由 `script/mutate-swagger-guard.py` 的 4 条变异机械证明）；
- 两个服务**不得互相 import**；底座**不得依赖任何一个服务**；
- 每个依赖都被 `go.sum` 钉住、都能从 module cache 恢复。

跨仓的那几条（底座必须被两个服务都用、共用词汇必须有人产出）需要三个仓同时在手：
本地会跳过并说明缺哪棵树，**CI 里强制**（`JMS_REQUIRE_SIBLINGS=1` ⇒ 缺树即失败）。

---

## 7. 文档

| 文档 | 内容 |
|---|---|
| [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md) | 环境变量、接线、资产上线流程、自检、工具选择速查、故障对照表 |
| [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) | 构建、发布、迭代流程、常见坑 |
| [`skills/`](skills/) | 给 AI agent 的操作手册（3 个 skill） |
| [`AGENTS.md`](AGENTS.md) | 给 AI 的迭代指南：架构不变量、铁律、验证要求 |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | 怎么参与：跑测试、加工具、提交约定 |
| [`SECURITY.md`](SECURITY.md) | 漏洞报告与凭据处理纪律 |

底座与执行面的文档在各自的仓里：
[`jumpserver-kit`](https://github.com/yi-nology/jumpserver-kit) ·
[`jumpserver-exec`](https://github.com/yi-nology/jumpserver-exec)。

---

## 8. 参与

欢迎提 issue 和 PR。动手前请读 [`CONTRIBUTING.md`](CONTRIBUTING.md) 与
[`AGENTS.md`](AGENTS.md) —— 后者记着这个仓的架构不变量与「已经在哪些地方栽过」的清单，
改代码前扫一眼能省你一轮返工。

⚠️ **别把凭据贴进 issue / PR / 日志**：token、私钥、堡垒机地址都是敏感的。
详见 [`SECURITY.md`](SECURITY.md)。

## 9. 许可

[Apache-2.0](LICENSE)。
