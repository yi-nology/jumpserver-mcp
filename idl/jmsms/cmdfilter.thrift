/*
 * cmdfilter —— 命令过滤：命令组、过滤器与它们的写回执。
 */
namespace go jmsms

/**
 * 一条命中的命令过滤器（jms_command_guard 的一行）。
 */
struct MatchedFilter {
  /** 过滤器名。 */
  1: required string filter,
  /** 它做什么：reject / warning / notify_and_warn / accept。 */
  2: optional string action,
  /** 优先级（数字越小越先评估）。 */
  3: optional i32 priority,
  /** 命中的命令组。 */
  4: optional string command_group,
  /** 组里命中的那条命令。 */
  5: optional string matched_command,
  /** 这个过滤器管的是谁/哪台机器。 */
  6: optional string scope,
}

/**
 * 一个命令组建好之后的回执（jms_cmdgroup_create）。
 *
 * 命令组本身**不拦任何命令** —— 它要跟命令过滤器绑在一起才有用。
 */
struct CmdGroupCreated {
  1: required string id,
  2: required string name,
  /** 组里的命令。 */
  3: optional list<string> commands,
  /** 下一步。 */
  4: optional string next,
}

/**
 * 一个命令过滤器建好之后的回执（jms_cmdfilter_create）。
 *
 * ⚠️ 作用域要看清：过滤器指错资产会**静悄悄地弄坏别人的自动化**，
 * 而且是坏在他们的机器上，不是你的。
 */
struct CmdFilterCreated {
  1: required string id,
  2: required string name,
  /** reject / accept / warning / notify_and_warn。 */
  3: required string action,
  /** 绑了几个命令组。 */
  4: optional i32 command_groups,
  /** 罩着几个用户。 */
  5: optional i32 users,
  /** 罩着几台资产。 */
  6: optional i32 assets,
  7: optional string note,
}

/**
 * 命令过滤器 / 命令组删掉之后的回执。
 *
 * 删掉过滤器 = 它拦过的命令又能跑了。
 */
struct FilterDeleted {
  /** 删掉的是过滤器。 */
  1: optional string deleted_filter,
  /** 删掉的是命令组。 */
  2: optional string deleted_command_group,
}

/**
 * 一个命令组（jms_cmdgroup_list 的一行）。
 *
 * ⚠️ commands 是**一整块文本**（堡垒机把命令按行拼在一个字段里），
 * 不是数组 —— 要按行拆得自己 split。
 */
struct CommandGroupSummary {
  1: optional string id,
  2: optional string name,
  3: optional string type,
  /** 匹配时是否忽略大小写。 */
  4: optional bool ignore_case,
  5: optional string comment,
  6: optional string commands,
}

/**
 * 一个命令过滤器（jms_cmdfilter_list 的一行）。
 *
 * users / assets / accounts 是**几个**（计数），不是成员名单。
 */
struct CommandFilterSummary {
  1: optional string id,
  2: optional string name,
  /** reject / accept / warning / notify_and_warn。 */
  3: optional string action,
  4: optional i32 priority,
  5: optional bool is_active,
  /** 罩着几个用户。 */
  6: optional i32 users,
  /** 罩着几台资产。 */
  7: optional i32 assets,
  /** 罩着几个账号。 */
  8: optional i32 accounts,
  /** 绑了哪些命令组（id）。 */
  9: optional list<string> command_groups,
  10: optional string comment,
}
