/*
 * touch —— 摸底：最近动过的、按资产、按用户、覆盖率。
 */
namespace go jmsms

/**
 * 接触历史里的一条近期会话。
 */
struct TouchRecent {
  /** 会话 id。 */
  1: optional string session_id,
  /** 开始时间。 */
  2: optional string time,
  /** 结束时间。 */
  3: optional string end,
  /** 谁开的。 */
  4: optional string user,
  /** 用的账号。 */
  5: optional string account,
  /** 来源地址。 */
  6: optional string source_ip,
  /** 协议。 */
  7: optional string protocol,
  /** 持续时长。 */
  8: optional string duration,
  /** 有录像文件。 */
  9: optional bool replay,
  /** 录像字节数。 */
  10: optional i64 replay_bytes,
  /** 这条会话的命令条数。 */
  11: optional i32 commands,
  /** 最后一次敲的命令。 */
  12: optional string last_input,
  /** 会话为什么没有留下有效记录（空 = 有交互）。 */
  13: optional string error_reason,
  /** 这条会话有没有审计价值。 */
  14: optional bool has_evidence,
}

/**
 * 接触历史里的一台资产。
 */
struct TouchAsset {
  /** 资产名。 */
  1: required string asset,
  /** 地址。 */
  2: optional string address,
  /** 资产 uuid。 */
  3: optional string asset_id,
  /** 这台资产上开过几次会话。 */
  4: optional i32 sessions,
  /** 首次接触时间。 */
  5: optional string first_contact,
  /** 末次接触时间。 */
  6: optional string last_contact,
  /** 碰过它的人。 */
  7: optional list<string> users,
  /** 用过的账号。 */
  8: optional list<string> accounts,
  /** 来源地址。 */
  9: optional list<string> source_ips,
  /** 用过的协议。 */
  10: optional list<string> protocols,
  /** 其中有录像的会话数。 */
  11: optional i32 sessions_with_replay,
  /** 其中有命令记录的会话数。 */
  12: optional i32 sessions_with_commands,
  /** 其中有审计价值的会话数。 */
  13: optional i32 sessions_with_evidence,
  /** 有审计价值的会话里，有命令记录的那些。 */
  14: optional i32 evidence_with_commands,
  /** 最近的会话。 */
  15: optional list<TouchRecent> recent,
}

/**
 * 接触历史里「被多于一人碰过」的资产。
 */
struct TouchMultiUser {
  /** 资产名。 */
  1: required string asset,
  /** 碰过它的人。 */
  2: optional list<string> users,
  /** 这台资产上开过几次会话。 */
  3: optional i32 sessions,
}

/**
 * 接触历史里排在最前面的人。
 */
struct TouchTopUser {
  /** 用户名。 */
  1: required string user,
  /** 他开过几次会话。 */
  2: optional i32 sessions,
  /** 他碰到过几台资产。 */
  3: optional i32 assets,
}

/**
 * 接触历史的覆盖率口径。
 *
 * sessions_with_evidence 才是「真的有人在用」的会话数；分母换成它，
 * command_rate 才有意义（拿全部会话当分母会把握手半成品算进去）。
 */
struct TouchCoverage {
  /** 有审计价值的会话数。 */
  1: optional i32 sessions_with_evidence,
  /** 有审计价值的会话占比。 */
  2: optional string evidence_rate,
  /** 有审计价值且有命令记录的会话数。 */
  3: optional i32 evidence_with_command,
  /** 有审计价值的会话里，有命令记录的比例。 */
  4: optional string command_rate,
  /** 全部会话里有命令记录的数。 */
  5: optional i32 sessions_with_command,
  /** 全部会话里有命令记录的比例（看着吓人，多半是调试残留）。 */
  6: optional string command_rate_of_all,
  /** 有录像的会话数。 */
  7: optional i32 sessions_with_replay,
  /** 索引到的命令条数。 */
  8: optional i32 commands_indexed,
}
