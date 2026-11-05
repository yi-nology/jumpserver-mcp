/*
 * delegation —— 委派：状态、探针、调用回执。
 */
namespace go jmsms

/** 委派调用之后的回执。 */
struct DelegationCallResult {
  /** 堡垒机回的状态码。 */
  1: optional i32 status,
  2: optional string method,
  3: optional string path,
  4: optional string query,
  5: optional string operation_id,
  /** 那张一次性凭据本身（已用掉）。 */
  6: optional string credential,
  /** 回了多少字节。 */
  7: optional i32 bytes,
  /** 响应体（JSON 原文；截断过就看 preview）。 */
  8: optional string body,
  /** 列表响应里有几条。 */
  9: optional i32 count,
  /** 响应体预览（截断后的原文）。 */
  10: optional string preview,
  /** 堡垒机拒绝时回的原文。 */
  11: optional string error_body,
  /** 这次答复该怎么读。 */
  12: optional string note,
  /** 被拦下时的理由（写操作缺 confirm）。 */
  13: optional string blocked,
}

/**
 * 委派凭据的状态（jms_delegation_status）。
 */
struct DelegationStatus {
  /** 配没配。 */
  1: required bool configured,
  2: optional string base_url,
  3: optional string key_id,
  /** 委派请求在堡垒机侧的身份。 */
  4: optional string user_id,
  5: optional string org_id,
  /** 凭据绑定了什么（一条一个约束）。 */
  6: optional list<string> binds,
  7: optional string note,
  /** 没配上的原因。 */
  8: optional string reason,
  /** 怎么配。 */
  9: optional string how_to_configure,
}

/** 委派探针里的一项检查。 */
struct DelegationCheck {
  /** 情形名：valid / replay / operation_mismatch … */
  1: required string case,
  /** 期望什么。 */
  2: required string expect,
  /** 堡垒机回了什么状态码。 */
  3: optional i32 status,
  /** 堡垒机说了什么。 */
  4: optional string core_says,
  /** 请求根本没送到（网络/签名问题）。 */
  5: optional string transport_error,
}

/**
 * 委派凭据的一次探针（jms_delegation_probe）。
 *
 * 它是**只读**的：探针用的凭据是一次性的，探完就作废。
 */
struct DelegationProbe {
  /** 探到的身份。 */
  1: optional string user_id,
  2: optional string org_id,
  /** 一项一项的检查。 */
  3: optional list<DelegationCheck> checks,
  4: optional string note,
}
