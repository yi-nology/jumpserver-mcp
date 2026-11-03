/*
 * account —— 资产上的登录账号，以及它的写回执。
 */
namespace go jmsms

/**
 * 账号摘要：资产上的一个登录账号。
 *
 * secret_type 只说账号用的是密码还是密钥，**永远不含密钥本身**。
 */
struct AccountSummary {
  /** 登录名。 */
  1: required string username,
  /** 显示名。 */
  2: optional string name,
  /** 口令类型：password / ssh_key。 */
  3: optional string secret_type,
  /** 是否特权账号（能 sudo）。 */
  4: optional bool privileged,
  /** 是否启用。 */
  5: optional bool is_active,
  /** 连通性：ok / err / -。 */
  6: optional string connectivity,
  /** 最后一次登录时间。 */
  7: optional string date_last_login,
  /** 这台账号挂在哪台资产上（资产名）。 */
  8: optional string asset,
  /** 那台资产的地址。 */
  9: optional string address,
}

/**
 * 一个账号建好之后的回执（jms_account_create）。
 */
struct AccountCreated {
  1: required string id,
  2: optional string username,
  3: optional string name,
  4: optional string asset,
  5: optional string address,
  6: optional string secret_type,
  7: optional bool privileged,
  8: optional bool is_active,
  9: optional string connectivity,
  /** 读回来核对过。 */
  10: optional bool verified,
  11: optional string note,
}

/**
 * 一个账号改过之后的回执（jms_account_update）。
 *
 * 口令是只写的：secret_written 为 true，secret_readable 永远是 false ——
 * 堡垒机不会把口令读回来。
 */
struct AccountUpdated {
  1: required string id,
  2: optional string username,
  3: optional bool is_active,
  4: optional string secret_type,
  5: optional string asset,
  /** 口令写进去了。 */
  6: optional bool secret_written,
  /** 口令读不回来（永远是 false）。 */
  7: optional bool secret_readable,
}

/**
 * 一个账号删掉之后的回执（jms_account_delete）。
 *
 * 只删堡垒机里的记录，机器上那个用户还在。
 */
struct AccountDeleted {
  1: required string deleted,
  2: optional string asset,
  3: optional string note,
}
