/*
 * job —— 批量作业：计划、执行、审计。
 */
namespace go jmsms
include "refs.thrift"

/**
 * 批量作业的答复。
 *
 * 演练时没有 task_id —— 作业根本没起。
 */
struct JobPlan {
  /** 命令的风险分级。 */
  1: optional string risk,
  2: optional string command,
  /** 用哪个账号跑。 */
  3: optional string runas,
  4: optional bool dry_run,
  /** 目标机器。 */
  5: optional list<refs.AssetRef> targets,
  6: optional i32 target_cnt,
  /** 作业 id（真跑起来才有）。 */
  7: optional string task_id,
  /**
   * 真的执行了吗（演练与被拦下时是 false）。
   *
   * ⚠️ 这一格说的是「派发出去了吗」——派发 ≠ 在机器上跑过：目标没有可用
   * 账号或不可达时，任务照常建、executed 也是 true，跑没跑到看 summary
   * 与 note。
   */
  8: optional bool executed,
  /** 被拦下时的理由（被拦下时才有）。 */
  9: optional string blocked,
  /** 跑完了吗。 */
  10: optional bool is_finished,
  /**
   * 命令在目标上跑成功了吗。
   *
   * ⚠️ 判定看 summary（每台机器的 recap），**不看上游的 is_success** ——
   * 上游那一格说的是「任务自己没出错」：对一台不可达的机器，recap 全空
   * （一台都没跑）上游照样报过 true（2026-11-03 实测，同一事实两种字面）。
   * 本仓口径：recap 里至少一台 ok 且没有失败/不可达 ⇒ true；recap 在但
   * 没有一台成功、或有成功也有失败/不可达 ⇒ false；recap 压根没有 ⇒ 这一格
   * 不给（无法确认就不说 —— 上游的 false 也只是「任务自己出错」，命令可能
   * 在任务出错前就在每台上跑完过）。
   */
  11: optional bool is_success,
  /** 每个目标的结果。 */
  12: optional JobAuditHosts summary,
  /** 耗时。 */
  13: optional string time_cost,
  /** 一句说明（为什么没跑、还在跑、…）。 */
  14: optional string note,
  /**
   * 这次演练若真跑，会不会跑。
   *
   * 与执行面同一个口径：只有带确认才需要确认的作业，确认了就跑。
   * 演练与「被拦下」在留痕里必须分得开 —— 所以这一栏与 dry_run 一起给。
   */
  15: optional bool would_execute,
}

/**
 * 一次批量作业的结果。
 *
 * summary 不在这里：它是堡垒机的自由结构
 * （{"dark":{},"failures":{},"ok":[...],"skipped":[...]}），thrift 表达不了
 * 「任意 JSON」，硬写成一个类型只会在某台机器上炸。按本仓的规矩，动态的部分
 * 留在 map 里，键名写在这里：
 *
 *   summary.dark      目标没有回应的机器
 *   summary.failures  每台机器的失败详情
 *   summary.ok        成功执行的目标
 *   summary.skipped   被跳过的目标
 */
struct JobExecutionResult {
  1: optional string task_id,
  2: optional bool is_finished,
  /** 口径同 JobPlan.is_success（按 recap 判，不看上游标签）。 */
  3: optional bool is_success,
  4: optional string material,
  5: optional string date_start,
  6: optional string date_finished,
  /** 耗时。 */
  7: optional string time_cost,
  /** 每个目标的结果。 */
  8: optional JobAuditHosts summary,
  /** 成功的主机。 */
  9: optional list<string> succeeded,
  /** 失败的主机与原因。 */
  10: optional map<string, string> failed,
  /** 跳过的主机。 */
  11: optional list<string> skipped,
  /** 有没有输出可读（v5 的 API 不给作业输出，所以永远是 false）。 */
  12: optional bool stdout_available,
  /** 一句说明。 */
  13: optional string note,
}

/**
 * 一次批量执行里的一台机器（jms_job_audit 的 hosts）。
 *
 * failures 的值是**目标机器自己给的自由文本**（所以被脱敏），键名写在这里：
 *
 *   ok        跑成了的目标
 *   failures  失败的目标 → 原因
 *   skipped   被跳过的目标
 */
struct JobAuditHosts {
  1: optional list<string> ok,
  /**
   * 失败/不可达的目标 → 原因（原因来自目标机器，是自由文本，已脱敏；
   * recap 里 dark（连不上）的目标也在这里，原因写作 unreachable）。
   */
  2: optional map<string, string> failed,
  3: optional list<string> skipped,
}

/** 一次批量作业的执行记录（jms_job_audit 的一行）。 */
struct JobAuditExecution {
  1: optional string id,
  /** 谁跑的。 */
  2: optional string creator,
  /** 什么时候建的记录。 */
  3: optional string created,
  /** 实际下发的命令（凭据样式的值已脱敏）。 */
  4: optional string command,
  5: optional bool success,
  6: optional bool finished,
  7: optional string time_cost,
  /** 关联的执行 id。 */
  8: optional string task_id,
  9: optional string started,
  10: optional string ended,
  /** 每台机器的结果。 */
  11: optional JobAuditHosts hosts,
}

/** 一个批量作业的定义（jms_job_audit 的一行）。 */
struct JobAuditDefinition {
  1: optional string name,
  2: optional string created_by,
  3: optional string last_run,
  /** 堡垒机给的是 "True"/"False" 这样的词，不是布尔。 */
  4: optional string periodic,
  5: optional string interval,
  /** 命令的头一段。 */
  6: optional string command_head,
  /**
   * 作业自己的统计（堡垒机给的自由结构，键名是 total / success）。
   *
   * 只有数字型的键会进来 —— 别的键值类型说不清，所以宁可不报，也不给它
   * 一个猜出来的形状。
   */
  7: optional map<string, i32> summary,
}

/**
 * 批量作业的审计（jms_job_audit）。
 *
 * 两个计数说的是**两件事**：succeeded / failed 描述整批匹配到的执行，
 * count / total 描述这次返回了几条。截断的 failed 列表在 failed_truncated 里。
 */
struct JobAudit {
  /** 本次返回了几条执行。 */
  1: required i32 count,
  /** 匹配到了几次。 */
  2: required i32 total,
  3: required bool truncated,
  /** 匹配到的执行里成功了几次。 */
  4: required i32 succeeded,
  /** 失败了几次。 */
  5: required i32 failed,
  /** 执行记录。 */
  6: optional list<JobAuditExecution> executions,
  /** 失败的执行记录（只_failed=true 时有）。 */
  7: optional list<JobAuditExecution> failed_executions,
  /** 失败的列表被截断了。 */
  8: optional bool failed_truncated,
  /** 作业定义。 */
  9: optional list<JobAuditDefinition> definitions,
  /** 有多少条执行没有每台机器的明细。 */
  10: optional i32 without_host_detail,
  /** 有多少条执行没记下命令。 */
  11: optional i32 without_command,
  12: optional string note,
}

/**
 * 一个批量作业停掉之后的回执（jms_job_stop）。
 */
struct JobStopped {
  1: required string stopped,
  /** 堡垒机怎么回的。 */
  2: optional string response,
}

/**
 * 一次批量作业的执行（jms_job_list 的一行）。
 *
 * summary 是堡垒机的自由结构（ok / failures / skipped），这里用
 * JobAuditHosts 那个形状收着 —— 两个工具读的是同一份数据，不该有两个说法。
 */
struct JobExecutionSummary {
  1: optional string task_id,
  /** 口径同 JobPlan.is_success（按 recap 判，不看上游标签）。 */
  2: optional bool is_success,
  3: optional JobAuditHosts summary,
  4: optional string date_start,
  5: optional string material,
}
