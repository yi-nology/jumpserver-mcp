# 变更记录

本文件遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
本项目遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 变更

### 新增

### 修复

## [0.3.3] - 2026-11-04

### 修复

- ⚠️ **`checksums.txt` 里的路径带目录前缀 ⇒ 校验形同虚设**：
  实测 `v0.3.2` 的 checksums 里写的是
  `./jumpserver-mcp-linux-amd64/jumpserver-mcp-linux-amd64`，
  而**下载页上的资产是扁平的** ⇒ 用户在下载目录跑
  `sha256sum -c checksums.txt` 会报「文件不存在」。
  ⇒ 算哈希时读源文件，只把输出里的路径换成 basename。

### 变更

- 版本号 0.3.2 → 0.3.3。

## [0.3.2] - 2026-11-04

### 修复

- ⚠️ **`v0.3.1` 装出来的二进制 `--version` 说「0.3.0」**：
  版本常量忘了跟着 tag 抬，而**它不在任何测试里、也不在任何守卫里** ⇒
  没人拦。`go install ...@v0.3.1` 装到的是功能正常、但版本号说错的二进制。
  ⇒ 版本常量抬到 `0.3.2`，并加一道守卫（见下）。

- **新增版本守卫**：二进制说的版本必须等于**它是从哪个 tag 构建出来的**
  （`cmd/jumpserver-mcp/version_test.go`）。
  ⇒ 在 **tag 触发**的 CI 里它有牙齿：忘了抬版本常量 ⇒ 发版构建当场红。
  ⇒ 在 main 上它**跳过**并说明原因 —— 否则「版本号先抬、tag 后打」这条正常
  流程会把自己的 CI 弄红，守卫就会被第一个被它烦到的人关掉。

### 变更

- ⚠️ **为什么是 `v0.3.2` 而不是重打 `v0.3.1`**：`v0.3.1` 已经被 Go 的模块代理
  （`proxy.golang.org`）与校验和库（`sum.golang.org`）**收录** ⇒
  **移动或删除那个 tag 会让已经记录了哈希的版本对不上** ⇒ 用户装不上。
  ⇒ 只能往前发一个新的。

- **新增 GitHub Release**（`.github/workflows/release.yml`）：tag 触发 ⇒
  六个平台（linux/darwin/windows × amd64/arm64）交叉编译 + `checksums.txt`。
  ⚠️ README 一直写着「方式二：release 二进制（见 Releases 页）」，
  但**从来没有发过 Release** ⇒ 照着做的人下到的��� 404。
  （CI 先跑测试，**红了就不发**。）

## [0.3.1] - 2026-11-04

### 修复

- ⚠️ **`v0.3.0` 那个 tag 是坏的**（已删除）：它在 CI 还红的时候被打出来 ⇒
  `.gitignore` 的 `coverage.*` 把同名**源码** `biz/jumpserver/handler/coverage.go`
  吞掉 ⇒ 那个 tag 缺文件 ⇒ `go install github.com/yi-nology/jumpserver-mcp/cmd/jumpserver-mcp@v0.3.0`
  **编译失败**。现在 `v0.3.1` 是第一个可用的 tag。
  ⇒ 三个仓的 CI 都加了 `on: push: tags` ⇒ **tag 也会跑 CI**，坏掉的 tag 会当场红。

- **发布自检不再把「缺配置」误判成「二进制坏了」**
  （`script/publish-mcp.py`）：管理面**按设计**没有 `JUMPSERVER_URL` 就不启动
  ⇒ 以前每次发布都报一个假红，而真坏的时候反而没人看了。
  ⇒ 现在：它**自己说了原因**地退出 ⇒ 判 OK（并把原因带出来）；
  没说话就退出（被系统杀掉 / 段错误）⇒ 才是真的起不来；
  panic 仍判 FAIL（它的话里有 `panic:` 与 goroutine 栈 ⇒ 那是崩了）。

### 变更

- **底座版本抬到 `jumpserver-mcp-kit v0.1.1`**：那一版修掉了
  「三条底座守卫其实从未执行过」的问题（`protocolLayerFile` 拿带 `.go` 的
  文件名去比导入路径 ⇒ 永不相等 ⇒ 状态词汇 / 能力监视名单 / 发行版词汇
  三条检查一条都没跑过，而守卫仍报绿）。
  ⚠️ 本仓的守卫依赖底座 ⇒ 底座发了新 tag，本仓**必须**跟着抬 require
  （否则两边 CI 跑的不是同一份底座）。

- **README §3.1 的事实更正**：原文说「没有凭据只能注册一部分工具」——
  **实测是错的**：无凭据时仍然注册**全部 97 个**（注册看工具目录，不看凭据；
  没凭据是**调用时**失败，不是注册时少注册）。
  ⇒ 改成「管理面始终注册 97 个」，并说清**执行面**才是 14/15 那个
  （`jmsx_inventory` 需要 REST client）。

## [0.3.0] - 2026-11-04

首个发布（**这个 tag 后来被删除了** —— 它缺一个源码文件，编译不过；
可用的是 `v0.3.1`，见上一条）。

### 新增

- **97 个工具**，覆盖：资产（列表/详情/创建/改/删/批量登记/移动）、
  账号（增删改、自动改密、账号发现）、授权（创建/授予/删除/覆盖范围/对比/有效权限视图）、
  用户与用户组、标签、命令组与命令过滤（登录/剪贴板/脱敏 ACL）、
  角色与权限、资产树与终端、连通性、审计检索（登录日志/操作日志/密码变更/文件传输）、
  取证时间线与接触历史、委派凭据、批量作业、节点继承与覆盖范围、平台与节点杂项读、
  安全体检。
- **`--doctor` 自检**：接线 + 堡垒机配置体检 —— 报出静默失败
  （资产没有 protocols、资产没有账号、授权一个账号都没授予、授权点名的账号不存在）。
  退出码 `0`/`1`/`2`（问帮助是 `0`）。
- **只读模式**（`JMS_READ_ONLY=1`）：结构性拒掉所有写工具。
- **门禁与留痕接线**：`confirm=true` 规则、风险分级、每次调用一行留痕
  （含脱敏后的参数、门禁每一段的判定与理由、放行与否、耗时、失败标记）。
- **共用底座**：协议层、门禁、输出契约、留痕、脱敏、风险分类、失败分类、共用词汇 ——
  在 [jumpserver-mcp-kit](https://github.com/yi-nology/jumpserver-mcp-kit) 仓。

### 变更

- ⚠️ **改名**：本项目从单体仓 `jms-mcp` 拆成独立仓 `jumpserver-mcp`。
  - 二进制：`jumpserver-mcp`（单体仓时代就叫这个名字，现在它是独立仓的产物；
    产物落在 `./bin/`，发布落在 `~/.local/jumpserver-mcp-runtime/<时间戳>/`）。
  - MCP 服务名：客户端配置里的 key 默认为 `jumpserver`（部署时可改）；
    留痕（审计）里的 `service` 字段是 `jumpserver-mcp`。
  - ⚠️ **老留痕不会自动迁移**：留痕目录从 `~/.local/state/jms-mcp/audit` 改为
    `~/.local/state/jumpserver-mcp/audit`（见 kit 仓的 CHANGELOG）。
