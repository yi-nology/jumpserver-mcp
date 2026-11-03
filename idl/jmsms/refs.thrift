/*
 * refs —— 各域共用的引用形状：引用一个用户/资产/动作/协议/身份。
 */
namespace go jmsms

/**
 * 引用：一条记录里提到的另一个对象（用户、用户组、节点）。
 *
 * 这些字段全都可空：它们直接来自堡垒机的返回体，那边的 null 也要能
 * 留在答案里，不能被契约逼成空串。
 */
struct Ref {
  /** uuid。 */
  1: optional string id,
  /** 名字。 */
  2: optional string name,
}

/**
 * 资产引用：多带地址与平台，供「这个人能碰到哪些机器」这类视图用。
 */
struct AssetRef {
  1: optional string id,
  2: optional string name,
  3: optional string address,
  4: optional string platform,
  5: optional string connectivity,
  6: optional bool is_active,
  /** 这个资产所在的树节点。 */
  7: optional list<string> nodes,
  /** 哪些授权让它可达。 */
  8: optional list<string> granted_by,
}

/** 动作：一条授权允许做什么（connect / upload / modify 之类）。 */
struct ActionRef {
  1: optional string value,
  2: optional string label,
}

/** 协议引用：资产声明的协议（名字与端口）。 */
struct ProtocolRef {
  1: optional string name,
  2: optional i32 port,
}

/**
 * 这次调用是以谁的身份做的（jms_doctor 的一小块）。
 */
struct IdentityRef {
  1: optional string username,
  2: optional bool is_superuser,
}
