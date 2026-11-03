/*
 * permission —— 授权：谁能对什么做什么，以及授权变更的回执。
 */
namespace go jmsms
include "refs.thrift"

/**
 * 权限矩阵里的一行：一个用户能碰到多少资产。
 */
struct AccessMatrixUser {
  /** 用户名。 */
  1: required string username,
  /** 是否启用。 */
  2: optional bool is_active,
  /** 是否超级管理员。 */
  3: optional bool is_superuser,
  /** 他能碰到几台资产。 */
  4: optional i32 assets_reachable,
  /** 按平台分的资产数。 */
  5: optional map<string, i32> by_platform,
  /** 授予他访问权的授权名。 */
  6: optional list<string> permissions,
}

/**
 * 授权摘要：一条资产授权在列表里长什么样。
 *
 * accounts 要么是账号名，要么是 ["@ALL"]。
 * **空列表不是「全部账号」，是「一个账号都没给」** —— 那种授权登录时
 * 会失败于「没有可用的授权账号或终端协议」。
 */
struct PermissionSummary {
  /** 授权名。 */
  1: required string name,
  /** 是否启用。 */
  2: optional bool is_active,
  /** 这条授权覆盖几台资产。 */
  3: optional i32 assets_amount,
  /** 几个用户持有它。 */
  4: optional i32 users_amount,
  /** 覆盖几个资产树节点。 */
  5: optional i32 nodes_amount,
  /** 这条授权授出的账号名；["@ALL"] 表示全部账号。 */
  6: optional list<string> accounts,
  /** 创建时间。 */
  7: optional string date_created,
}

/**
 * 一条授权「授给了谁、能授什么」的说明。
 *
 * 这是 jms_asset_audit 里那份视图：站在某台机器前面说「这些授权覆盖了它，
 * 各授了什么账号」。
 */
struct PermissionGrant {
  /** 授权名。 */
  1: required string permission,
  /** 持有这条授权的人（用户组写作 "group:名字"）。 */
  2: optional list<string> users,
  /** 这条授权授出的账号名；["@ALL"] 表示这台资产的全部账号。 */
  3: optional list<string> accounts,
  /** 一句话说清这条授权能不能用（谁都能看懂的那种）。 */
  4: optional string note,
}

/**
 * 一条资产授权的详情。
 *
 * assets_amount 只数**直接点名**的资产；节点继承的资产不算，所以
 * effective_assets_count 才是它真正覆盖的范围。
 */
struct PermissionDetail {
  1: optional string id,
  2: optional string name,
  3: optional bool is_active,
  4: optional bool is_valid,
  /** 持有它的用户。 */
  5: optional list<refs.Ref> users,
  /** 持有它的用户组。 */
  6: optional list<refs.Ref> user_groups,
  /** 直接点名的资产。 */
  7: optional list<refs.Ref> assets,
  /** 覆盖的树节点。 */
  8: optional list<refs.Ref> nodes,
  /** 它授出的账号名；["@ALL"] 表示全部账号。 */
  9: optional list<string> accounts,
  10: optional i32 users_amount,
  11: optional i32 assets_amount,
  12: optional i32 nodes_amount,
  13: optional list<refs.ActionRef> actions,
  14: optional string date_created,
  15: optional string date_expired,
  16: optional string created_by,
  17: optional string comment,
  /** 真正覆盖的资产数（节点继承算进去）。 */
  18: optional i32 effective_assets_count,
  /** 直接点名的资产数。 */
  19: optional i32 direct_assets_count,
  /** 对不上时的提醒（一句话；多条用 "; " 连起来）。 */
  20: optional string warning,
}

/**
 * 一条授权的覆盖面：它真正能碰到哪些资产。
 */
struct PermissionCoverage {
  /** 授权名。 */
  1: optional string permission,
  /** 堡垒机自己报的资产数（只数直接点名的）。 */
  2: optional i32 reported_assets_amount,
  /** 真正覆盖的资产数。 */
  3: optional i32 effective_assets_count,
  /** 直接点名的资产数。 */
  4: optional i32 direct_assets_count,
  /** 覆盖的树节点名。 */
  5: optional list<string> nodes,
  /** 它授出的账号名。 */
  6: optional list<string> accounts,
  /** 覆盖的资产。 */
  7: optional list<refs.AssetRef> assets,
  /** 资产清单是否被截断。 */
  8: optional bool truncated,
  /** 对不上时的提醒（堡垒机报的数与真正覆盖的数不一致）。 */
  9: optional string warning,
}

/** 比较两条授权时的一侧。 */
struct PermissionSide {
  1: optional string permission,
  2: optional list<string> users,
  3: optional list<string> accounts,
  /** 真正覆盖的资产数。 */
  4: optional i32 effective_assets,
  /** 堡垒机自己报的资产数。 */
  5: optional i32 reported_assets_amount,
}

/**
 * 两条授权的对比：换过去会多给什么、少给什么。
 */
struct PermissionComparison {
  1: optional PermissionSide left,
  2: optional PermissionSide right,
  /** 只有左边覆盖的资产。 */
  3: optional list<string> only_left,
  /** 只有右边覆盖的资产。 */
  4: optional list<string> only_right,
  /** 两边都覆盖的资产。 */
  5: optional list<string> both,
  /** 换过去会**多给**什么。 */
  6: optional string warning_left,
  /** 换过去会**少给**什么。 */
  7: optional string warning_right,
}

/**
 * 一个人能碰到什么。
 */
struct UserAccess {
  1: optional string user_id,
  2: optional string username,
  /** 这次返回了几条。 */
  3: optional i32 count,
  /** 过滤后命中了几条。 */
  4: optional i32 total,
  5: optional bool truncated,
  /** 按平台分的资产数。 */
  6: optional map<string, i32> by_platform,
  /** 他能碰到的资产。 */
  7: optional list<refs.AssetRef> assets,
}

/**
 * 一次授权里被写进去的那几项。
 *
 * v5 的 PATCH 是**整体替换**相关对象，所以写之前要把现有的成员读回来合并。
 * 这里回的就是合并后的那份 —— 也就是「现在这条授权到底授给了谁」。
 */
struct PermissionGrantChange {
  /** 合并后的用户 id。 */
  1: optional list<string> users,
  /** 合并后的资产 id。 */
  2: optional list<string> assets,
  /** 合并后的账号名；["@ALL"] 表示全部账号。 */
  3: optional list<string> accounts,
}

/** 改完授权之后的回执。 */
struct PermissionGrantResult {
  /** 授权名。 */
  1: optional string permission,
  /** 写进去的那几项（合并后的完整集合）。 */
  2: optional PermissionGrantChange changed,
  /** 几个用户持有它。 */
  3: optional i32 users_amount,
  /** 覆盖几台资产。 */
  4: optional i32 assets_amount,
  /** 它授出的账号。 */
  5: optional list<string> accounts,
  6: optional string note,
}

/**
 * 一条授权建好之后的回执（jms_permission_create）。
 *
 * accounts 为空 = **一个账号都没授**（不是「全部账号」）—— 那时 warning
 * 会说清楚，否则登录会报「没有可用的授权账号或终端协议」。
 */
struct PermissionCreated {
  1: required string permission_id,
  2: required string name,
  /** 授给了几个用户。 */
  3: required i32 users,
  /** 覆盖几台资产。 */
  4: required i32 assets,
  /** 覆盖几个树节点。 */
  5: required i32 nodes,
  /** 授出的账号；["@ALL"] 表示全部账号。 */
  6: optional list<string> accounts,
  7: optional string note,
  8: optional string warning,
}

/**
 * 一条授权删掉之后的回执（jms_permission_delete）。
 *
 * 不可逆：删掉之后，持有它的人全都进不去了。
 */
struct PermissionDeleted {
  1: required string deleted,
  2: required string id,
}
