# 安全策略（jumpserver-mgmt）

## 报告漏洞

**别开公开 issue，也别发公开 PR。**

请用 GitHub 的 [Private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidelines/reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
（Security → Report a vulnerability），或直接联系维护者。

请一并给出：受影响版本、复现步骤、你观察到的行为、你认为的影响面。

## 这个项目碰什么敏感东西

它本身**不存**任何凭据，但它**经手**这些：

| | 说明 |
|---|---|
| **堡垒机 API token** | 管理面用它调 JumpServer 的 API。权限通常是**管理员级** |
| **堡垒机地址与资产清单** | 暴露了内网拓扑与机器命名 |
| **审计检索的答复** | `jms_command_audit` / `jms_commands_search` / `jms_asset_evidence` 把**命令原文**交回给 agent ⇒ 那条路径上的凭据会**进 agent 的上下文窗口**（不进留痕，但一样是泄漏） |
| **委派凭据** | `JMS_DELEGATION_SECRET` 是堡垒机的 64 字节密钥；它签出来的一次性凭据能以 `ai` 账号的身份调 API |
| **命令参数与输出** | 留痕文件里可能有命令、路径、主机名、偶尔的凭据 |
| **会话录像** | 走堡垒机自己，不经过本项目 |

## 使用者的纪律

- ⚠️ **别把 token / 私钥贴进 issue、PR、聊天、日志**；
- ⚠️ **token 不要进 git**（MCP 客户端的配置文件在仓外，正常不会被提交，但别手滑拷进仓里）；
- ⚠️ **别把 `JUMPSERVER_PASSWORD` 塞进环境变量当首选** ——
  **优先读文件**（`~/.config/jumpserver-mgmt/password`，0600），
  环境变量是退路（长跑进程的环境在 Linux 上是 `/proc` 里谁都能读的）；
- ⚠️ **委派 secret 不要写进 MCP 配置**（那是让能读该目录的每个进程都拿到它）；
- ⚠️ **先跑 `--doctor` 再接生产**：它会报出静默失败（资产没有账号、授权一个账号都没授予）；
- ⚠️ **留痕目录有保留期**（默认 7 天，`JMS_AUDIT_RETENTION_DAYS`）⇒「昨天发生了什么」可能已被清理；
- ⚠️ **`JMS_READ_ONLY=true`** 让这个 server 只读 —— 给只需要查询的 agent 用这个模式；
- ⚠️ **一切写操作都要 `confirm=true`**，包括删资产、改授权、重置密码 —— 不可逆；
- ⚠️ **删类操作不可逆**：删资产会连它的账号一起删；
- ⚠️ **命令过滤器的范围要核对**：一条挂错资产的过滤器会静默弄坏别人的自动化，
  而且是在**他的机器上**炸，不是你的。跑之前用 `jms_command_guard` 预检。

## 留痕（audit trail）本身也是数据

留痕文件（默认 `~/.local/state/jumpserver-mgmt/audit`，一进程一文件，0600）里记着
「谁在这台机器上试图跑什么」。**它也是敏感数据**：把它当审计资料保管，
别整目录丢进共享盘或日志系统。

## 依赖与供应链

本仓依赖 `github.com/yi-nology/jumpserver-kit`（共用底座，Apache-2.0）、
`oagentkit/*`、`oekit/*`（MIT）、`modelcontextprotocol/go-sdk`（MIT→Apache-2.0 过渡）、
`google/jsonschema-go`（MIT）。构建**不写 `replace`**（依赖按普通方式解析，`go.sum` 钉住内容）。

⚠️ **底座在 [kit 仓](https://github.com/yi-nology/jumpserver-kit)** ——
它的漏洞会同时落在两个服务上；报告时注明你看到的是哪个服务的行为。

发现依赖有问题 ⇒ 按上面的私密渠道报，别在公开 issue 里贴 exploit。
