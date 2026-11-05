/*
 * records —— 各类流水与摘要：会话、终端、命令审计、登录、操作、口令变更、文件传输。
 */
namespace go jmsms

/**
 * 会话摘要：一次终端会话。
 *
 * jms_sessions_list 给得全（时长、录像、命令条数），jms_sessions_live 只给
 * 正在开着的那些 —— 所以除 id 外都是可选的。
 */
struct SessionSummary {
  /** 会话 id。 */
  1: required string id,
  /** 谁开的。 */
  2: optional string user,
  /** 登的是哪台机器。 */
  3: optional string asset,
  /** 机器地址。 */
  4: optional string address,
  /** 用的账号。 */
  5: optional string account,
  /** 用的协议。 */
  6: optional string protocol,
  /** 会话类型。 */
  7: optional string type,
  /** 来源地址。 */
  8: optional string remote_addr,
  /** 开始时间。 */
  9: optional string date_start,
  /** 结束时间。 */
  10: optional string date_end,
  /** 持续时长。 */
  11: optional string duration,
  /** 是否已结束。 */
  12: optional bool is_finished,
  /** 这次登录从哪来。 */
  13: optional string login_from,
  /** 资产的 uuid。 */
  14: optional string asset_id,
  /** 录像能不能取（不是「录像有没有内容」）。 */
  15: optional bool can_replay,
  /** 这条会话记下了几条命令。 */
  16: optional i32 command_amount,
  /**
   * 会话为什么没有留下有效记录。
   *
   * 空串 = 真有交互；非空（最常见的是 replay_unsupported）= 只录下了
   * 半截 koko 握手。**判断一个会话有没有审计价值，看这个字段，不要看
   * has_replay** —— 本机 617 个会话里 518 个是握手半成品，只看 has_replay
   * 会得出「99.5% 都被审计了」的错误结论。
   */
  17: optional string error_reason,
  /** 有录像文件（不等于录像有内容，见 error_reason）。 */
  18: optional bool has_replay,
  /** 会话是否成功。 */
  19: optional bool is_success,
  /** 录像的字节数。 */
  20: optional i64 replay_size,
}

/**
 * 一条会话被强制结束之后的回执（jms_session_terminate）。
 */
struct SessionTerminated {
  1: required string terminated,
  /** 堡垒机怎么回的。 */
  2: optional string response,
}

/**
 * 命令审计里的一条命令（jms_commands_search 的一行）。
 *
 * input 是**敲进去的那部分**；输出在 jms_command_audit 那一行里。
 */
struct CommandSearchSummary {
  1: optional string id,
  2: optional string user,
  3: optional string asset,
  4: optional string address,
  5: optional string account,
  6: optional string session,
  7: optional string remote_addr,
  8: optional string input,
  /** 堡垒机记的风险级别。 */
  9: optional string risk_level,
  10: optional string date_start,
}

/**
 * 命令审计里的一条命令，连同它的输出（jms_command_audit 的一行）。
 */
struct CommandAuditSummary {
  1: optional string when,
  2: optional string session,
  3: optional string user,
  4: optional string asset,
  5: optional string address,
  6: optional string account,
  7: optional string input,
  8: optional string output,
  9: optional string risk_level,
}

/** 一次登录的记录（jms_login_logs 的一行）。 */
struct LoginLogSummary {
  1: optional string username,
  2: optional string backend,
  3: optional string remote_addr,
  4: optional bool success,
  5: optional string user_agent,
  6: optional string date_created,
}

/** 一个系统级角色（jms_system_role_permissions 的一行）。 */

/** 一次配置变更的记录（jms_operate_logs 的一行）。 */
struct OperateLogSummary {
  1: optional string datetime,
  /** 谁改的。 */
  2: optional string user,
  /** 做了什么（create / update / delete）。 */
  3: optional string action,
  4: optional string resource_type,
  5: optional string resource,
  6: optional string remote_addr,
}

/** 一次口令变更的记录（jms_password_change_logs 的一行）。 */
struct PasswordChangeSummary {
  1: optional string datetime,
  /** 口令属于谁。 */
  2: optional string user,
  /** 是谁改的（≠ user 说明是别人替它改的）。 */
  3: optional string change_by,
  4: optional string remote_addr,
  /** 自助改密（true）还是管理员改的。 */
  5: optional bool self_service,
}

/**
 * 一个已注册的终端（jms_terminals_list 的一行）。
 *
 * 组件终端（[KoKo]-… 之类）是堡垒机自己的管道，不是人用的；用
 * is_component 认得出来。
 */
struct TerminalSummary {
  1: optional string name,
  2: optional string type,
  3: optional string remote_addr,
  4: optional bool is_active,
  5: optional bool is_alive,
  /** 上面有没有人在用。 */
  6: optional bool session_online,
  7: optional string comment,
  /** 是不是堡垒机自己的组件终端。 */
  8: optional bool is_component,
}

/**
 * 一次文件传输的记录（jms_file_transfers 的一行）。
 *
 * 堡垒机替每一次上传/下载留痕：**谁**、从**哪**、往**哪台机器**的**哪个账号**、
 * 传了**哪个文件**、**成没成**。
 *
 * ⚠️ 这条记录是**执行面文件传输的唯一事实来源** —— jmsx_put_file /
 * jmsx_get_file / jmsx_relay 都经堡垒机，堡垒机自己记了每一次，
 * 而它们自己只在传输当时算过 checksum（那是**自证**，不是**记录**）。
 */
struct FileTransferRecord {
  /** 这条记录的 id。 */
  1: optional string id,
  /** 谁传的（堡垒机用户）。 */
  2: optional string user,
  /** 来源地址。 */
  3: optional string remote_addr,
  /** 目标资产。 */
  4: optional string asset,
  /** 目标机器地址。 */
  5: optional string address,
  /** 用的账号。 */
  6: optional string account,
  /** 方向：upload / download / mkdir / rmdir / delete / rename / rename_dir / symlink。 */
  7: optional string operate,
  /** 文件名（远端路径）。 */
  8: optional string filename,
  /** 开始时间。 */
  9: optional string date_start,
  /** 这次传输成没成。 */
  10: optional bool is_success,
  /** 堡垒机那边有没有留下这个文件的内容。 */
  11: optional bool has_file,
  /** 这次传输属于哪个会话。 */
  12: optional string session,
}
