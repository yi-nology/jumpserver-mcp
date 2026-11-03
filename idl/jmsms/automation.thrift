/*
 * automation —— 账号自动化：定义、执行、记录。
 */
namespace go jmsms

/**
 * 账号自动化计划（jms_account_automations_list 的一行）。
 *
 * 计划 = 「给哪些资产、推/收哪些账号、用什么密钥策略」的一份配置。
 * 触发它 = 堡垒机自己去目标机上执行，**密钥全程不经调用方**。
 */
struct AccountAutomation {
  /** 计划的 id（触发与删除都用它）。 */
  1: optional string id,
  /** 计划名。 */
  2: optional string name,
  /** 种类：push_account / gather_account。 */
  3: optional string type,
  /** 要操作的账号名。 */
  4: optional list<string> accounts,
  /** 挂了多少个资产。 */
  5: optional i32 assets_amount,
  /** 挂了多少个资产树节点。 */
  6: optional i32 nodes_amount,
  /** 跑过几次。 */
  7: optional i32 executed_amount,
  /** 密钥类型：password / ssh_key。 */
  8: optional string secret_type,
  /**
   * 密钥策略：random（堡垒机自己生成）/ specific（用指定的）。
   *
   * random 才是「密钥不经调用方」的那一种。
   */
  9: optional string secret_strategy,
  /** 生成的密钥长度。 */
  10: optional i32 password_length,
  /** 是不是周期执行。 */
  11: optional bool is_periodic,
  /** 周期（小时）；不是周期执行时为 0。 */
  12: optional i32 interval,
  /** 周期表达式（cron）；不是周期执行时为空。 */
  13: optional string crontab,
  /** 启用了吗。 */
  14: optional bool is_active,
  /** 改完之后要不要验一下连得上。 */
  15: optional bool check_conn_after_change,
  /** 最近一次执行时间。 */
  16: optional string last_execution_date,
  /** 创建时间。 */
  17: optional string date_created,
  /** 更新时间。 */
  18: optional string date_updated,
  /** 备注。 */
  19: optional string comment,
  /** 谁建的。 */
  20: optional string created_by,
}

/**
 * 账号自动化的一次执行（jms_account_automation_executions 的一行）。
 */
struct AccountAutomationExecution {
  /** 这次执行的 id。 */
  1: optional string id,
  /** 触发它的计划名。 */
  2: optional string automation,
  /** 触发它的计划 id。 */
  3: optional string automation_id,
  /** 怎么触发的：manual / timing。 */
  4: optional string trigger,
  /** 状态：pending / running / success / failed / terminated。 */
  5: optional string status,
  /** 开始时间。 */
  6: optional string date_start,
  /** 结束时间。 */
  7: optional string date_finished,
  /** 跑了多久。 */
  8: optional string duration,
  /** 快照里的计划名。 */
  9: optional string snapshot_name,
  /**
   * 快照里有多少个账号。
   *
   * ⚠️ 上游的 snapshot 给的是 `accounts`（**数组**，元素是**账号 id**），
   * **没有** `account_amount` 这个键 ⇒ 读它永远是空。
   */
  10: optional i32 snapshot_account_amount,
  /** 快照里的资产数。 */
  11: optional i32 snapshot_asset_amount,
  /** 快照里的节点数。 */
  12: optional i32 snapshot_node_amount,
}

/**
 * 一次推送的结果（jms_account_automation_records 的一行）。
 *
 * ⚠️ 这是**每个账号**一行，不是每次执行一行。
 */
struct AccountAutomationRecord {
  /** 记录的 id。 */
  1: optional string id,
  /** 目标资产名。 */
  2: optional string asset,
  /** 目标机器地址。 */
  3: optional string address,
  /** 账号名。 */
  4: optional string account,
  /** 这次操作成没成。 */
  5: optional bool is_success,
  /** 状态。 */
  6: optional string status,
  /** 什么时候做完的。 */
  7: optional string date_finished,
  /** 错误信息（上游有就带）。 */
  8: optional string error,
}

/**
 * 触发一次账号自动化的结果（jms_account_automation_run 的）。
 */
struct AccountAutomationRun {
  /** 堡垒机的任务 id（celery 的）。 */
  1: optional string task,
  /** 触发的计划名。 */
  2: optional string automation,
  /** 触发的计划 id。 */
  3: optional string automation_id,
  /**
   * 这次执行要看哪儿。
   *
   * ⚠️ 执行是**异步**的（堡垒机起一个 celery 任务），所以这个答复
   * 说的是「已受理」，不是「已完成」——**结果要另查**。
   */
  4: optional string execution_hint,
}
