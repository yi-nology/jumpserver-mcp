/*
 * security —— 登录 ACL、脱敏规则、剪贴板 ACL。
 */
namespace go jmsms

/**
 * ACL 用户/资产匹配条件里的一条属性匹配（type=attrs 时用）。
 */
struct ACLAttr {
  /** 匹配什么属性，如 ip。 */
  1: optional string name,
  /** 匹配方式：exact / contains 等。 */
  2: optional string match,
  /** 要匹配的值。 */
  3: optional string value,
}

/**
 * ACL 的作用对象选择器（users / assets 字段的真实形态）。
 *
 * ⚠️ 它**不是**字符串列表：堡垒机的 JSONManyToMany 字段只认
 * {"type":"all"} / {"type":"ids","ids":[…]} / {"type":"attrs","attrs":[…]}
 * 三种形态，传 ["*"] 会得到 400（错误文本只描述选择器形态）。
 */
struct ACLSelector {
  /** 形态：all / ids / attrs。 */
  1: optional string type,
  /** type=ids 时的对象 id 列表。 */
  2: optional list<string> ids,
  /** type=attrs 时的属性匹配列表。 */
  3: optional list<ACLAttr> attrs,
}

/**
 * 登录 ACL 的时间窗条目。
 *
 * ⚠️ id 是**周几**（0=周日 … 6=周六），value 是「、」分隔的
 * HH:MM~HH:MM 时段（如 "09:00~12:00、13:30~18:00"）。
 * 某天的 value 为空 = 那天不放行；整张表为空 = 不限时间。
 */
struct ACLTimePeriod {
  /** 周几（0=周日）。 */
  1: optional i32 id,
  /** 「、」分隔的 HH:MM~HH:MM 时段。 */
  2: optional string value,
}

/**
 * 登录 ACL 的规则（jms_login_acl_* 的 rules 字段）。
 */
struct LoginACLRule {
  /** IP 匹配组：* / 单 IP / CIDR / 区段（如 10.1.1.1-10.1.1.20）。 */
  1: optional list<string> ip_group,
  /** 时间窗（按周几排）。 */
  2: optional list<ACLTimePeriod> time_period,
}

/**
 * 一条登录 ACL（jms_login_acl_create / jms_login_acl_update 的答复）。
 *
 * 它管的是「**谁能登录堡垒机**」（IP、时间窗），不是「谁能进哪台机器」
 * （那是登录资产 ACL / 资产授权的活）。
 */
struct LoginACL {
  /** 它的 id。 */
  1: optional string id,
  /** 名字。 */
  2: optional string name,
  /** 动作：reject / accept / review / notice。 */
  3: optional string action,
  /** 优先级（小的先匹配）。 */
  4: optional i32 priority,
  /** 是否启用（不启用的规则不生效）。 */
  5: optional bool is_active,
  /** 对哪些用户生效。 */
  6: optional ACLSelector users,
  /** 规则本体（IP 组 + 时间窗）。 */
  7: optional LoginACLRule rules,
  /** 复核人（action=review 时的审批人，用户 id）。 */
  8: optional list<string> reviewers,
  /** 备注。 */
  9: optional string comment,
  /** 谁建的。 */
  10: optional string created_by,
  /** 建立时间。 */
  11: optional string date_created,
  /** 最后修改时间。 */
  12: optional string date_updated,
}

/**
 * 一条数据脱敏规则（jms_data_masking_rule_* 的答复/行）。
 *
 * 它管的是「**展示**账号密码等敏感字段时怎么遮」——按字段名模式匹配，
 * 固定字符替换 / 隐藏中间 / 只留前缀 / 只留后缀。
 */
struct DataMaskingRule {
  /** 它的 id。 */
  1: optional string id,
  /** 名字。 */
  2: optional string name,
  /** 动作：reject / accept / review / notice。 */
  3: optional string action,
  /** 优先级。 */
  4: optional i32 priority,
  /** 是否启用。 */
  5: optional bool is_active,
  /** 对哪些用户生效。 */
  6: optional ACLSelector users,
  /** 对哪些资产生效。 */
  7: optional ACLSelector assets,
  /** 对哪些账号生效（如 ["*"]）。 */
  8: optional list<string> accounts,
  /** 字段名模式（如 password）。 */
  9: optional string fields_pattern,
  /** 遮蔽方式：fixed_char / hide_middle / keep_prefix / keep_suffix。 */
  10: optional string masking_method,
  /** 遮蔽用的字符（fixed_char 时用，如 ######）。 */
  11: optional string mask_pattern,
  /** 属于哪个组织。 */
  12: optional string org_id,
  /** 组织名。 */
  13: optional string org_name,
  /** 复核人（用户 id）。 */
  14: optional list<string> reviewers,
  /** 备注。 */
  15: optional string comment,
  /** 谁建的。 */
  16: optional string created_by,
  /** 建立时间。 */
  17: optional string date_created,
  /** 最后修改时间。 */
  18: optional string date_updated,
}

/**
 * 剪贴板 ACL 的操作项。
 */
struct ClipboardOperation {
  /** 操作：copy / paste。 */
  1: optional string value,
  /** 人读的说明。 */
  2: optional string label,
}

/**
 * 一条剪贴板 ACL（jms_clipboard_acl_* 的答复/行）。
 *
 * 它管的是会话里的**剪贴板与文件大小**：允许哪些剪贴板操作、
 * 复制/粘贴文本上限、下载/上传文件大小上限。
 */
struct ClipboardACL {
  /** 它的 id。 */
  1: optional string id,
  /** 名字。 */
  2: optional string name,
  /** 动作：reject / accept（这个家族只认这两个）。 */
  3: optional string action,
  /** 优先级。 */
  4: optional i32 priority,
  /** 是否启用。 */
  5: optional bool is_active,
  /** 对哪些用户生效。 */
  6: optional ACLSelector users,
  /** 对哪些资产生效。 */
  7: optional ACLSelector assets,
  /** 对哪些账号生效。 */
  8: optional list<string> accounts,
  /** 允许的剪贴板操作（copy / paste）。 */
  9: optional list<ClipboardOperation> operations,
  /** 复制文本上限（字节）。 */
  10: optional i32 copy_text_limit,
  /** 粘贴文本上限（字节）。 */
  11: optional i32 paste_text_limit,
  /** 下载文件大小上限（字节）。 */
  12: optional i32 download_file_size_limit,
  /** 上传文件大小上限（字节）。 */
  13: optional i32 upload_file_size_limit,
  /** 属于哪个组织。 */
  14: optional string org_id,
  /** 组织名。 */
  15: optional string org_name,
  /** 复核人（用户 id）。 */
  16: optional list<string> reviewers,
  /** 备注。 */
  17: optional string comment,
  /** 谁建的。 */
  18: optional string created_by,
  /** 建立时间。 */
  19: optional string date_created,
  /** 最后修改时间。 */
  20: optional string date_updated,
}
