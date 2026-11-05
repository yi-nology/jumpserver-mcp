/*
 * evidence —— 证据与会话实录：采过的命令、逐条记录、回放。
 */
namespace go jmsms

/**
 * 取证时间线上的一条命令。
 */
struct EvidenceCommand {
  /** 什么时候敲的。 */
  1: optional string time,
  /** 敲了什么（凭据已遮蔽）。 */
  2: optional string input,
  /** 回了什么（凭据已遮蔽）。 */
  3: optional string output,
  /** 这条命令的风险级别。 */
  4: optional string risk_level,
}

/**
 * 取证时间线上的一次会话。
 *
 * 只有**有审计价值**的会话才会出现在时间线上（error_reason 为空）；
 * 握手半成品被单独收进盲点，不混在有价值的会话里。
 */
struct EvidenceSession {
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
  /** 持续时长。 */
  7: optional string duration,
  /** 这条会话记下的命令。 */
  8: optional list<EvidenceCommand> commands,
  /** 命令条数。 */
  9: optional i32 command_count,
  /** 命令是否被截断。 */
  10: optional bool commands_truncated,
  /** 有录像文件。 */
  11: optional bool has_replay,
  /** 录像字节数。 */
  12: optional i64 replay_bytes,
  /** 录像是否因为凭据被遮蔽。 */
  13: optional bool redacted,
  /** 有交互但没留下命令行。 */
  14: optional bool no_commands,
  /** 录像文本（取了录像时才有）。 */
  15: optional string replay_text,
  /** 录像取不到时的原因。 */
  16: optional string replay_error,
}

/**
 * 一条命令的记录（jms_session_transcript 的一行）。
 *
 * input / output 是**碎片**：堡垒机把一条命令拆成几段存，这里按时间排回去。
 */
struct CommandRecord {
  /** 输入。 */
  1: optional string input,
  /** 输出。 */
  2: optional string output,
  /** 什么时候。 */
  3: optional string time,
  /** 堡垒机记的风险级别。 */
  4: optional string risk_level,
}

/**
 * 一条会话的命令级流水（jms_session_transcript）。
 *
 * transcript 是拼起来的全文；records 是它的碎片版。
 */
struct SessionTranscript {
  /** 会话 id。 */
  1: required string session,
  /** 本次返回了几条。 */
  2: optional i32 count,
  /** 一共多少条。 */
  3: optional i32 total,
  /** 返回数是否少于命中数。 */
  4: optional bool truncated,
  /** 命令记录。 */
  5: optional list<CommandRecord> records,
  6: optional string user,
  7: optional string asset,
  8: optional string address,
  9: optional string account,
  10: optional string first_seen,
  11: optional string last_seen,
  /** 各个风险级别各出现了几次。 */
  12: optional map<string, i32> risk_levels,
  /** 拼起来的全文。 */
  13: optional string transcript,
  /** 全文长度。 */
  14: optional i32 chars,
  /** 全文被截断了。 */
  15: optional bool text_truncated,
  16: optional string note,
  /** 有多个风险级别（被拒的命令会没有输出）。 */
  17: optional string warning,
}

/**
 * 一条会话的录像（jms_session_replay）。
 *
 * 「有录像」不等于「有审计价值」：has_command 才是那句命令在不在的证据。
 */
struct SessionReplay {
  /** 会话 id。 */
  1: required string session,
  2: optional string user,
  3: optional string asset,
  4: optional string account,
  5: optional string date_start,
  6: optional string date_end,
  7: optional string duration,
  /** 有没有录像。 */
  8: optional bool has_replay,
  /** 能不能回放。 */
  9: optional bool can_replay,
  /** 能不能接管这条会话。 */
  10: optional bool can_join,
  /** 录像里有没有命令。 */
  11: optional bool has_command,
  12: optional i32 command_amount,
  /** 录像文件多大。 */
  13: optional i32 replay_size,
  /** 抽出来的终端文本。 */
  14: optional string terminal_text,
  15: optional i32 terminal_text_chars,
  /** 出错原因（v5 里「只录下握手半截」就是它）。 */
  16: optional string error_reason,
  17: optional string note,
  /** 录像下载不下来。 */
  18: optional string replay_error,
  /** 文本里的凭据样式的值被脱敏了。 */
  19: optional bool redacted,
}
