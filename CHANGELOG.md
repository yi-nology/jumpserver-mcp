# 变更记录

本文件遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
本项目遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 变更

### 新增

### 修复

## [0.1.0] - 2026-11-04

首个发布。

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
