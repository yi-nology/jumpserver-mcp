package service

// The audit-value rules.
//
// These are the rules the agent's manual spends a whole section on, because
// getting them wrong produces a confidently wrong number:
//
//   - a session that merely HAS A RECORDING is not evidence. On this bastion
//     518 of 617 sessions are flagged replay_unsupported: they recorded the
//     bytes of a half-finished koko handshake and nothing else. Counting
//     them as audited says "99.5% of the fleet is covered" and it is false.
//   - the session's error_reason is the criterion: empty means the session
//     carried real interaction.
//
// The rules live here, away from the tools, because three tools need them
// (jms_asset_touch_history, jms_asset_evidence, and the session views) and
// because a rule that three places spell out is a rule that will be spelled
// out differently.

import (
	"github.com/oekit/pkg/convert"
	"github.com/yi-nology/jumpserver-kit/model/jms"

	"github.com/yi-nology/jumpserver-mgmt/biz/model/jmsms"
)

// SessionErrorReason reads a session's error_reason.
//
// The field is a labelled object ({value, label}) on the sessions that have
// one, a bare string on others, and absent on the sessions that were fine —
// all three shapes occur on the same endpoint.
func SessionErrorReason(s map[string]any) string {
	e, ok := s["error_reason"]
	if !ok || e == nil {
		return ""
	}
	if mm, ok := e.(map[string]any); ok {
		return convert.AnyToStr(mm["value"])
	}
	return convert.AnyToStr(e)
}

// HasAuditValue reports whether a session carries real audit value.
//
// Empty error_reason means the session had real interaction. Anything else
// means the session failed in a way that left no interaction behind — most
// often a half-finished handshake.
//
// It is NOT the same question as "does this session have a recording", and
// the two answers disagree on most of this bastion's sessions.
func HasAuditValue(s map[string]any) bool {
	return SessionErrorReason(s) == ""
}

// Percent renders a share as a rounded percentage string; "-" when the
// denominator is zero.
func Percent(part, whole int) string {
	if whole == 0 {
		return "-"
	}
	return convert.Float64ToStr(float64(part)*100/float64(whole), 0) + "%"
}

// Coverage is how much of a set of sessions is really auditable.
//
// The rates use different denominators on purpose, and the difference is the
// whole point:
//
//   - evidence_rate is the share of ALL sessions that carried real
//     interaction. It is the honest "how much of the fleet was actually
//     used" number, and it looks alarming because the handshake leftovers
//     dominate the denominator.
//   - command_rate is the share of the sessions that carried real
//     interaction that ALSO have command rows. Its denominator is the
//     evidence count, NOT the session count. Dividing one by the other
//     yields a rate over 100%, because a few handshake leftovers do have
//     command rows.
//   - command_rate_of_all is the same numerator over the whole session
//     count: the number that looks reassuring and is mostly the debug
//     leftovers.
type Coverage struct {
	// TotalSessions is every session seen.
	TotalSessions int
	// SessionsWithEvidence are the sessions that carried real interaction.
	SessionsWithEvidence int
	// SessionsWithCommand are the sessions that have command rows.
	SessionsWithCommand int
	// EvidenceWithCommand are the sessions that have BOTH real interaction
	// and command rows.
	EvidenceWithCommand int
	// SessionsWithReplay are the sessions that have a recording file.
	SessionsWithReplay int
	// CommandsIndexed is how many command rows the audit log holds.
	CommandsIndexed int
}

// Fields renders the coverage as the answer's fields.
//
// It returns the contract type, not a map: the coverage is part of the
// touch-history answer, and a map here would put the field names back into
// the code by hand — where a renamed field shows up as a missing value
// instead of a compile error.
func (c Coverage) Fields() jmsms.TouchCoverage {
	return jmsms.TouchCoverage{
		SessionsWithEvidence: jms.OptInt32(int32(c.SessionsWithEvidence)),
		EvidenceRate:         jms.Opt(Percent(c.SessionsWithEvidence, c.TotalSessions)),
		EvidenceWithCommand:  jms.OptInt32(int32(c.EvidenceWithCommand)),
		CommandRate:          jms.Opt(Percent(c.EvidenceWithCommand, c.SessionsWithEvidence)),
		SessionsWithCommand:  jms.OptInt32(int32(c.SessionsWithCommand)),
		CommandRateOfAll:     jms.Opt(Percent(c.SessionsWithCommand, c.TotalSessions)),
		SessionsWithReplay:   jms.OptInt32(int32(c.SessionsWithReplay)),
		CommandsIndexed:      jms.OptInt32(int32(c.CommandsIndexed)),
	}
}
