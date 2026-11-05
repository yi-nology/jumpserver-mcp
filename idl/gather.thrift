/*
 * gather —— 账号采集与落地（jms_gather_*）。
 */
namespace go jmsms

/**
 * 一次账号采集的发起回执（jms_gather_run）。
 *
 * 采集是堡垒机那边异步跑的（Celery + 自动化），所以这里说的是「发起了」，
 * 不是「采完了」—— 用 jms_gathered_accounts 看结果。
 */
struct GatherRunResult {
  1: required string asset,
  2: optional string asset_id,
  /** 采到几个账号。 */
  3: optional i32 gathering,
  /** 其中哪些是新的（用户名）。 */
  4: optional list<string> newly_found,
  /** 堡垒机那边跑的报告。 */
  5: optional string report,
  6: optional string note,
}

/**
 * 采纳采集结果之后的回执（jms_gather_apply）。
 */
struct GatherApplyResult {
  1: required string asset,
  /** 采纳了哪些（用户名）。 */
  2: optional list<string> adopted,
  /** 没采纳的（用户名）。 */
  3: optional list<string> not_adopted,
  /** 本来就已经是确认状态的（用户名）。 */
  4: optional list<string> already_confirmed,
  5: optional string note,
  6: optional string warning,
  /** 这次要采纳哪些（读回失败时也有）。 */
  7: optional list<string> requested,
}

/**
 * 一个被采集到的账号（jms_gathered_accounts 的一行）。
 *
 * present=false 表示**这台机器上已经没有这个用户了** —— 那是最便宜的
 * 账号漂移检查。
 */
struct GatheredAccountSummary {
  1: optional string id,
  2: optional string username,
  3: optional string asset,
  4: optional string address,
  /** pending / confirm / rejected / offline。 */
  5: optional string status,
  /** 采集时这个账号还在不在。 */
  6: optional bool present,
  /** 远端（被采集的那台机器）上还在不在。 */
  7: optional bool remote_present,
  8: optional string date_updated,
  9: optional string date_last_login,
  /** 采集的细节（截断过的自由文本）。 */
  10: optional string detail,
}
