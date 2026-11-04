package service

// The audit-value rules are the ones the agent's manual warns about at
// length, because following them wrong produces a confidently wrong number.
// Nothing tested them: they lived in the tool layer, reachable only by
// calling a tool against a live bastion.

import (
	"testing"
)

func TestSessionErrorReasonReadsEveryShape(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want string
	}{
		{"absent", map[string]any{}, ""},
		{"null", map[string]any{"error_reason": nil}, ""},
		{"labelled object", map[string]any{
			"error_reason": map[string]any{"value": "replay_unsupported", "label": "replay unsupported"},
		}, "replay_unsupported"},
		{"bare string", map[string]any{"error_reason": "replay_unsupported"}, "replay_unsupported"},
		{"empty string", map[string]any{"error_reason": ""}, ""},
	}
	for _, c := range cases {
		if got := SessionErrorReason(c.in); got != c.want {
			t.Errorf("%s: SessionErrorReason = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestHasAuditValueIsNotTheSameAsHavingARecording(t *testing.T) {
	// A session with a recording but no interaction: the case that makes
	// "has_replay" a trap.
	handshake := map[string]any{
		"has_replay":   true,
		"error_reason": map[string]any{"value": "replay_unsupported", "label": "replay unsupported"},
	}
	if HasAuditValue(handshake) {
		t.Error("a session that only recorded a handshake must not count as evidence")
	}
	// A real interaction, with or without a recording.
	real := map[string]any{"has_replay": true}
	if !HasAuditValue(real) {
		t.Error("a session with no error_reason carries audit value")
	}
	// No recording, but real interaction: still evidence.
	noRecording := map[string]any{"has_replay": false}
	if !HasAuditValue(noRecording) {
		t.Error("a session with no error_reason carries audit value even without a recording")
	}
}

func TestPercent(t *testing.T) {
	if got := Percent(1, 2); got != "50%" {
		t.Errorf("Percent(1, 2) = %q, want 50%%", got)
	}
	if got := Percent(0, 10); got != "0%" {
		t.Errorf("Percent(0, 10) = %q, want 0%%", got)
	}
	// A zero denominator is not 100% and not 0%: it is "no denominator".
	if got := Percent(3, 0); got != "-" {
		t.Errorf("Percent(3, 0) = %q, want -", got)
	}
}

func TestCoverageUsesTheRightDenominatorForEachRate(t *testing.T) {
	// The shape of this bastion: 617 sessions, 518 of them handshake
	// leftovers, 91 of the remaining 99 with command rows.
	c := Coverage{
		TotalSessions:        617,
		SessionsWithEvidence: 99,
		SessionsWithCommand:  95,
		EvidenceWithCommand:  91,
		SessionsWithReplay:   614,
		CommandsIndexed:      200,
	}
	f := c.Fields()

	// command_rate divides the evidence-with-command count by the EVIDENCE
	// count. Dividing by the session count instead gives a number that
	// looks like "everything is audited".
	if f.GetCommandRate() != "92%" {
		t.Errorf("command_rate = %v, want 92%% (91 of the 99 sessions with evidence)",
			f.GetCommandRate())
	}
	// The share of all sessions that carried real interaction: 99 of 617.
	if f.GetEvidenceRate() != "16%" {
		t.Errorf("evidence_rate = %v, want 16%% (99 of 617)", f.GetEvidenceRate())
	}
	// The same numerator over the whole session count: the reassuring,
	// misleading number.
	if f.GetCommandRateOfAll() != "15%" {
		t.Errorf("command_rate_of_all = %v, want 15%% (95 of 617)", f.GetCommandRateOfAll())
	}
	// A rate over 100% is the symptom of the wrong denominator; make sure
	// the shape cannot produce one.
	if got := f.GetCommandRate(); len(got) > 0 && got[0] != '-' {
		for _, ch := range got {
			if ch > '9' && ch != '%' {
				t.Errorf("command_rate = %q looks like a rate over 100%%", got)
			}
		}
	}
	// The raw counts stay in the answer: the rates are the summary, the
	// counts are what they were computed from.
	if f.GetSessionsWithEvidence() != 99 {
		t.Errorf("sessions_with_evidence = %v, want 99", f.GetSessionsWithEvidence())
	}
	if f.GetSessionsWithReplay() != 614 {
		t.Errorf("sessions_with_replay = %v, want 614", f.GetSessionsWithReplay())
	}
	if f.GetCommandsIndexed() != 200 {
		t.Errorf("commands_indexed = %v, want 200", f.GetCommandsIndexed())
	}
}

func TestCoverageWithNothingIsNotAZeroPercent(t *testing.T) {
	f := Coverage{}.Fields()
	if f.GetEvidenceRate() != "-" {
		t.Errorf("evidence_rate = %v, want -", f.GetEvidenceRate())
	}
	if f.GetCommandRate() != "-" {
		t.Errorf("command_rate = %v, want -", f.GetCommandRate())
	}
}
