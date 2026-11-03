/*
 * network —— 网络：区域、网关、路径、连通性。
 */
namespace go jmsms

/** 连通性检查里的一台机器。 */
struct ConnectivityRow {
  1: optional string asset,
  2: optional string address,
  3: optional string platform,
}

/** 连通性检查的计数。 */
struct ConnectivitySummary {
  1: optional i32 ok,
  2: optional i32 err,
  /** 状态是 '-' 的：平台没有连通性测试，不是「连不上」。 */
  3: optional i32 not_tested,
}

/**
 * 连通性检查的答复。
 *
 * platforms_without_test 列出**测不了**的平台（Website / Kubernetes 之类
 * 不支持连通性测试的）—— 它们不连不上，只是没有「连不上」这回事。
 */
struct ConnectivityReport {
  /** 连上的。 */
  1: optional list<ConnectivityRow> ok,
  /** 连不上的。 */
  2: optional list<ConnectivityRow> err,
  /** 测不了的平台。 */
  3: optional list<string> platforms_without_test,
  /** 计数。 */
  4: optional ConnectivitySummary summary,
  /** 测了几台。 */
  5: optional i32 tested,
  /** 等了多久。 */
  6: optional i32 waited_seconds,
  /** 没测的部分与原因。 */
  7: optional string note,
  /** 状态是 '-' 的资产：平台根本没有连通性测试，不是「连不上」。 */
  8: optional list<ConnectivityRow> not_tested,
}

/**
 * 一个网络区域（jms_net_zones_list 的一行）。
 *
 * 区域 = 一组资产 + 能到它们的那几台网关。
 */
struct NetZone {
  /** 区域的 id。 */
  1: optional string id,
  /** 区域名。 */
  2: optional string name,
  /** 这个区域里的网关（名字）。 */
  3: optional list<string> gateways,
  /** 这个区域里的网段（CIDR）。 */
  4: optional list<string> cidrs,
  /** 这个区域里有多少台资产。 */
  5: optional i32 assets_amount,
  /**
   * 是不是自动归区。
   *
   * true = 堡垒机按网段自动把资产划进来。
   */
  6: optional bool auto_assign,
  /** 备注。 */
  7: optional string comment,
}

/**
 * 一台网关（jms_net_gateways_list 的一行）。
 *
 * ⚠️ 「网关」是**平台名**，不是角色：里面既有真正的跳板，也有路由器、
 * 网管设备。要判断它是不是跳板，看它有没有 zone、有没有账号。
 */
struct NetGateway {
  /** 它的 id。 */
  1: optional string id,
  /** 名字。 */
  2: optional string name,
  /** 地址。 */
  3: optional string address,
  /** 属于哪个区域（没归区时为空）。 */
  4: optional string zone,
  /** 连通性：ok / err。 */
  5: optional string connectivity,
  /** 有几个账号。 */
  6: optional i32 accounts_amount,
  /** 在资产树里的位置。 */
  7: optional list<string> nodes_display,
  /** 最后一次连通性验证的时间。 */
  8: optional string date_verified,
  /** 最后一次登录的时间。 */
  9: optional string date_last_login,
  /** 备注（常常写着这台设备是什么）。 */
  10: optional string comment,
}

/**
 * 到一台机器的路由上的一跳（jms_net_path 的一步）。
 */
struct NetPathStep {
  /** 这一跳是什么：gateway（网关）/ target（目标机）。 */
  1: optional string kind,
  /** 名字。 */
  2: optional string name,
  /** 地址。 */
  3: optional string address,
  /** 连通性：ok / err / 未知。 */
  4: optional string connectivity,
  /** 说明这一跳的作用。 */
  5: optional string note,
}

/**
 * 一台机器的网络归属与到达路径（jms_net_path 的答复）。
 */
struct NetPath {
  /** 目标机器。 */
  1: optional string asset,
  /** 目标地址。 */
  2: optional string address,
  /** 目标属于哪个区域（没归区时为空）。 */
  3: optional string zone,
  /** 该区域里的网关（名字）。 */
  4: optional list<string> zone_gateways,
  /** 到达路径。 */
  5: optional list<NetPathStep> steps,
  /** 一句话结论。 */
  6: optional string verdict,
  /** 该区域里的其它机器（数量）。 */
  7: optional i32 zone_assets_amount,
}
