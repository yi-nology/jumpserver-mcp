/*
 * user —— 用户、用户组，以及口令与 SSH 密钥的写回执。
 */
namespace go jmsms

/**
 * 用户摘要：堡垒机上的一个登录账号。
 */
struct UserSummary {
  /** 登录名。 */
  1: required string username,
  /** uuid。 */
  2: optional string id,
  /** 显示名。 */
  3: optional string name,
  /** 邮箱。 */
  4: optional string email,
  /** 是否启用。 */
  5: optional bool is_active,
  /** 是否超级管理员。 */
  6: optional bool is_superuser,
  /** 是否需要改密码（从没改过或已过期）。 */
  7: optional bool need_update_password,
  /** 是否登记过 SSH 公钥。 */
  8: optional bool has_public_key,
  /** 最后一次登录时间。 */
  9: optional string last_login,
}

/** 改完用户启用状态之后的回执。 */
struct UserActiveResult {
  1: optional string username,
  2: optional bool is_active,
}

/**
 * 一个用户建好之后的回执（jms_user_create）。
 *
 * generated_password 只出现一次：口令是只写的，堡垒机不再返回它。
 */
struct UserCreated {
  1: required string id,
  2: optional string username,
  3: optional string name,
  4: optional string email,
  5: optional bool is_active,
  6: optional bool is_superuser,
  7: optional bool need_update_password,
  /** 读回来核对过。 */
  8: optional bool verified,
  /** 生成的口令（只这一次）。 */
  9: optional string generated_password,
  10: optional string warning,
  /** 下一步该做什么。 */
  11: optional string next_step,
}

/**
 * 改完口令之后的回执（jms_user_password_set）。
 *
 * 「改没改成功」不能只看 200 —— 堡垒机不把口令读回来，所以要比改口令前后的
 * date_password_last_updated。verified 是这个比对的结论。
 */
struct PasswordSet {
  1: required string username,
  2: required string id,
  3: required bool password_changed,
  /** 口令读不回来（永远是 false）。 */
  4: required bool password_readable,
  /** 堡垒机记的口令时间戳。 */
  5: optional string date_password_last_updated,
  /** 核对结论：时间戳动了没有。 */
  6: optional bool verified,
  /** 核不上的原因。 */
  7: optional string note,
}

/**
 * 登记一把 SSH 公钥之后的回执（jms_user_sshkey_add）。
 *
 * ⚠️ 只有 ssh-rsa 会被接受：koko 拒 ed25519，而堡垒机会**默默**接受它
 * （不报错、公钥却没存下来），于是登录时才失败。
 */
struct SSHKeyAdded {
  1: required string username,
  /** key 的 id。 */
  2: optional string key_id,
  /** 公钥类型（ssh-rsa）。 */
  3: optional string key_type,
  /** 钥匙挂在谁名下。 */
  4: optional string owner_user_id,
  /** 写到哪儿了（老字段 public_key 是写不进去的）。 */
  5: optional string path_used,
  /** 写进去了。 */
  6: optional bool written,
  /** 读回来核对过。 */
  7: optional bool verified,
  /** 委派凭据的说明（没有就不出现）。 */
  8: optional string credential,
  /** 还想连不上时看这里。 */
  9: optional string hint,
}

/**
 * 撤掉一把 SSH 公钥之后的回执（jms_user_sshkey_remove）。
 */
struct SSHKeyRemoved {
  1: required string username,
  2: optional string user_id,
  /** 撤掉了几把。 */
  3: optional i32 removed,
  4: optional string note,
  /** 委派凭据的说明（走委派时才有）。 */
  5: optional string credential,
}

/**
 * 一个用户组（jms_user_groups_list 的一行）。
 *
 * permissions 是**哪些授权**经由这个组持有 —— 授权挂在组上时，从用户列表
 * 和授权列表里都看不出来，只有这里能看见。
 */
struct UserGroupSummary {
  1: optional string id,
  2: optional string name,
  /** 成员的名字。 */
  3: optional list<string> members,
  4: optional i32 members_amount,
  /** 经由这个组持有的授权名。 */
  5: optional list<string> permissions,
}
