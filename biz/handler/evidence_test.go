package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
)

// fakeBastionWithBlindSpot serves one asset with two evidence-carrying
// sessions: one that left command rows and one that did not.
//
// The second one is the case the audit trail cannot speak for -- a real
// interaction with no command record -- and it is deliberately OLDER than the
// first, so that a limit which keeps the newest sessions would drop it.
func fakeBastionWithBlindSpot(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "a1", "name": "dnsbox1", "address": "192.0.2.2",
				"platform": map[string]any{"id": 1, "name": "Linux"},
			}})
		case "/api/v1/terminal/sessions/":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "s1", "asset": "dnsbox1(192.0.2.2)", "asset_id": "a1",
					"user": "Ops User(opsuser)", "account": "opsuser(opsuser)",
					"remote_addr": "192.0.2.150", "protocol": "ssh",
					"date_start": "2026/05/07 22:23:24 +0800",
					"date_end":   "2026/05/07 23:00:00 +0800", "duration": "0:37:01",
					"has_replay": true, "can_replay": true, "replay_size": 5218,
					"error_reason": map[string]any{"value": "", "label": ""}},
				{"id": "s2", "asset": "dnsbox1(192.0.2.2)", "asset_id": "a1",
					"user": "Ops User(opsuser)", "account": "opsuser(opsuser)",
					"remote_addr": "10.253.253.4", "protocol": "ssh",
					"date_start": "2026/04/16 15:41:16 +0800",
					"date_end":   "2026/04/16 16:11:47 +0800", "duration": "0:30:31",
					"has_replay": true, "can_replay": true, "replay_size": 955,
					"error_reason": map[string]any{"value": "", "label": ""}},
			})
		case "/api/v1/terminal/commands/":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "c1", "session": "s1", "user": "Ops User(opsuser)",
					"asset": "dnsbox1(192.0.2.2)", "account": "opsuser(opsuser)",
					"input": "cat /etc/wireguard/wg0.conf", "output": "[Interface]",
					"risk_level":        map[string]any{"value": 0, "label": "Accept"},
					"timestamp":         1792919445,
					"timestamp_display": "2026/05/07 22:30:01 +0800"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// A session that carried a real interaction but left no command row is a hole
// in the audit trail. It must be reported on its own -- and it must survive a
// limit that keeps the newest sessions, because that is exactly how it would
// otherwise disappear.
func TestAssetEvidenceSurfacesSessionsWithoutCommands(t *testing.T) {
	cs := testSession(t, fakeBastionWithBlindSpot(t))
	out := testsupport.CallToolJSON(t, cs, "jms_asset_evidence",
		map[string]any{"asset": "192.0.2.2", "limit": 1})

	blind, ok := out["sessions_without_commands"].([]any)
	if !ok {
		t.Fatalf("sessions_without_commands missing: %v", out)
	}
	if len(blind) != 1 {
		t.Fatalf("blind spots = %d, want 1: %v", len(blind), blind)
	}
	b := blind[0].(map[string]any)
	if b["session_id"] != "s2" {
		t.Errorf("blind spot session = %v, want s2", b["session_id"])
	}
	// The recording size is what tells an idle shell apart from a missing
	// trail, so it has to be reported.
	if b["replay_bytes"] != float64(955) {
		t.Errorf("replay_bytes = %v, want 955", b["replay_bytes"])
	}
	if b["duration"] != "0:30:31" {
		t.Errorf("duration = %v, want 0:30:31", b["duration"])
	}
	if out["blind_spot_note"] == nil {
		t.Error("blind_spot_note missing")
	}
	// The limit keeps the newest session only; the blind spot is older and
	// must still be found.
	if n := len(out["timeline"].([]any)); n != 1 {
		t.Errorf("timeline = %d entries, want 1", n)
	}
}

// The evidence count must be the real number, not the number that survived
// the limit: reporting the capped figure understates how much of the trail
// exists.
func TestAssetEvidenceCountsAllEvidenceSessions(t *testing.T) {
	cs := testSession(t, fakeBastionWithBlindSpot(t))
	out := testsupport.CallToolJSON(t, cs, "jms_asset_evidence",
		map[string]any{"asset": "192.0.2.2", "limit": 1})
	if out["sessions_with_evidence"] != float64(2) {
		t.Errorf("sessions_with_evidence = %v, want 2 (the real count, not the capped one)",
			out["sessions_with_evidence"])
	}
}
