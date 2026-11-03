/*
 * asset —— 资产与树节点、平台、标签，以及它们的写回执。
 */
namespace go jmsms
include "refs.thrift"

/**
 * 资产树节点摘要。
 *
 * parent_id 恒为 null —— 堡垒机 v5 的节点接口不给父节点，所以调用方
 * 不能靠它判断层级，只能看 full_value（/DEFAULT/studio/arm）。
 */
struct NodeSummary {
  /** uuid。 */
  1: required string id,
  /** 节点名。 */
  2: required string name,
  /** 完整路径，形如 /DEFAULT/studio。 */
  3: optional string full_value,
  /** 父节点 id；v5 恒为 null。 */
  4: optional string parent_id,
  /** 这个节点下有几台资产。 */
  5: optional i32 assets,
}

/** 改完资产之后的回执。 */
struct AssetUpdated {
  1: optional string id,
  2: optional string name,
  3: optional string address,
  4: optional string platform,
  /** 声明的协议；注意这里是对象数组（名字 + 端口），不是字符串。 */
  5: optional list<refs.ProtocolRef> protocols,
  6: optional bool is_active,
}

/** 建完树节点之后的回执。 */
struct NodeCreated {
  1: optional string id,
  2: optional string name,
  3: optional string full_value,
  4: optional i32 assets_amount,
  5: optional bool verified,
  6: optional string next_step,
}

/**
 * 一台资产的详情（jms_asset_get / jms_asset_create）。
 *
 * ⚠️ 这份记录里有一半是**堡垒机自己的原始结构**，不是我们的形状 ——
 * platform / category / connectivity / accounts / protocols 那些都是堡垒机
 * 的说法。所以它们留在 map 里，键名写在这里：
 *
 *   platform              {id, name, type}
 *   category              {value, label}
 *   connectivity          {value, label}
 *   type                  {value, label}
 *   protocols             [{name, port}]
 *   accounts              [账号原始记录]
 *   labels                [标签原始记录]
 *   nodes                 [节点原始记录]
 *   directory_services / gathered_info / auto_config / spec_info   原始结构
 *
 * 契约管住的是**我们自己的那一半**（稳定的那几个字段），堡垒机的原始结构
 * 照旧原样透传 —— 把动态的东西塞进契约，契约就不再是契约。
 */
struct AssetDetail {
  1: required string id,
  2: required string name,
  3: required string address,
  /** 这台资产上有几个账号。 */
  4: optional i32 accounts_amount,
  /** 是否启用。 */
  5: optional bool is_active,
  6: optional string comment,
  7: optional string created_by,
  8: optional string date_created,
  9: optional string date_updated,
  10: optional string date_verified,
  11: optional string date_last_login,
  12: optional string org_id,
  13: optional string org_name,
  /** 资产树上的位置（人看的路径）。 */
  14: optional list<string> nodes_display,
  /** 平台，如 Linux / macOS / Gateway。 */
  15: optional string platform,
  /** 终端协议（名字 + 端口）。 */
  16: optional list<refs.ProtocolRef> protocols,
  /** 连通性：ok / err / -。 */
  17: optional string connectivity,
  /** 平台大类：host / web / cloud —— 决定它是不是走终端接入。 */
  18: optional string category,
  /** 资产上的标签名。 */
  19: optional list<string> labels,
  /** 这台机器上的账号名。 */
  20: optional list<string> accounts,
  /** 一句提示（下一步该做什么）。 */
  21: optional string note,
}

/**
 * 一台资产删掉之后的回执（jms_asset_delete）。
 *
 * 不可逆：资产上的账号也一起没了。
 */
struct AssetDeleted {
  1: required string deleted,
  2: required string id,
  3: optional string address,
}

/**
 * 资产挪位置之后的回执（jms_asset_move）。
 */
struct AssetMoved {
  /** add 还是 remove。 */
  1: required string action,
  /** 节点路径。 */
  2: required string node,
  /** 动了哪几台。 */
  3: optional list<string> assets,
  /** 这个节点下现在有几台。 */
  4: optional i32 node_assets_amount,
  5: optional string note,
}

/**
 * 一个标签建好之后的回执（jms_label_create）。
 */
struct LabelCreated {
  1: required string id,
  2: required string name,
  3: required string value,
}

/**
 * 标签挂上/摘下之后的回执（jms_label_attach）。
 *
 * 挂标签是**加**：已有的成员不会被替换（底层是整体 PUT，工具先读回来再合并）。
 */
struct LabelAttached {
  1: required string label,
  2: optional string label_id,
  /** 挂上了哪几个。 */
  3: optional list<string> tagged,
  /** 摘下了哪几个。 */
  4: optional list<string> detached,
  /** 这个标签现在罩着几个资源（一句话）。 */
  5: optional string resources_now_tagged,
}

/**
 * 一个标签删掉之后的回执（jms_label_delete）。
 */
struct LabelDeleted {
  1: required string deleted,
}

/**
 * 批量建资产之后的回执（jms_assets_bulk_create）。
 *
 * 每一台**独立**建，所以一台坏的不影响别的（results 里逐台说）。
 */
struct AssetsBulkCreated {
  /** 要建几台。 */
  1: required i32 requested,
  /** 建成了几台。 */
  2: required i32 created,
  /** 没建成的几台。 */
  3: required i32 failed,
  4: required i32 count,
  5: required i32 total,
  6: required bool truncated,
  /** 逐台的结果。 */
  7: optional list<AssetRow> results,
  /** 下一步。 */
  8: optional string next,
  /**
   * 这一批**有一部分没成**时的说明。
   *
   * ⚠️ 逐台的结果在 results[] 里（每台自己说），但**调用级**也要说一句话
   * —— 否则留痕只会写「这次调用成功了」，而其实 10 台里有 3 台没建成。
   */
  9: optional string error,
}

/**
 * 批量建资产时的一台（jms_assets_bulk_create 的一行）。
 */
struct AssetRow {
  /** 资产名。 */
  1: optional string name,
  /** 地址。 */
  2: optional string address,
  /** 建成了没有。 */
  3: optional bool ok,
  /** 没建成的原因。 */
  4: optional string error,
  /** 建出来的资产 id。 */
  5: optional string id,
  /** 平台（堡垒机回的是它的 id）。 */
  6: optional string platform,
}

/** 一个标签（jms_labels_list 的一行）。 */
struct LabelSummary {
  1: optional string id,
  2: optional string name,
  3: optional string value,
  4: optional string color,
  5: optional string comment,
  /** 罩着几个资源。 */
  6: optional i32 res_count,
}

/**
 * 一个被打上标签的资源（jms_label_resources 的一行）。
 *
 * resource 是堡垒机给的**显示串**（"名字(地址)"），所以 name / address
 * 是从它里拆出来的 —— 直接照抄 resource 的话，name 会是同一串而 address
 * 是空的。
 */
struct LabelResourceSummary {
  /** 标签的显示名。 */
  1: optional string label,
  /** 资源类型（Asset / User …），不是数字。 */
  2: optional string res_type,
  /** 资源 id。 */
  3: optional string res_id,
  /** 堡垒机给的原始显示串。 */
  4: optional string resource,
  /** 资源名。 */
  5: optional string name,
  /** 资源地址。 */
  6: optional string address,
}

/** 一个资产平台（jms_platforms_list 的一行）。 */
struct PlatformSummary {
  1: optional string id,
  2: optional string name,
  3: optional string type,
}
