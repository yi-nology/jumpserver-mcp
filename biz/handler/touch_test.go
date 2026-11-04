package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mcp/internal/repository"
	"github.com/yi-nology/jumpserver-mcp/internal/service/policy"
)

// The bastion serves two different shapes for the same concept. The asset,
// account and gathered-account endpoints nest the asset as an object:
//
//	{"asset": {"id": "...", "name": "...", "address": "..."}}
//
// while the terminal session and command endpoints flatten it into a label
// plus a separate id:
//
//	{"asset": "name(address)", "asset_id": "..."}
//
// Reading a flat row with nested() yields nil for every attribute. That is
// exactly how the session and command tools ended up reporting no asset at
// all, and no test caught it because the tools were only ever exercised
// against a bastion where "no asset" looked like a plausible answer.
func TestAssetFieldReadsBothShapes(t *testing.T) {
	row := map[string]any{"asset": map[string]any{
		"id": "uuid-1", "name": "dnsbox1", "address": "192.0.2.2",
	}}
	flat := map[string]any{"asset": "node-031(198.51.100.31)", "asset_id": "uuid-2"}

	cases := []struct {
		row  map[string]any
		attr string
		want string
	}{
		{row, "id", "uuid-1"},
		{row, "name", "dnsbox1"},
		{row, "address", "192.0.2.2"},
		{flat, "id", "uuid-2"},
		{flat, "name", "node-031"},
		{flat, "address", "198.51.100.31"},
	}
	for _, c := range cases {
		if got := fmt.Sprint(jmsapi.AssetField(c.row, c.attr)); got != c.want {
			t.Errorf("assetField(%v, %q) = %q, want %q", c.row, c.attr, got, c.want)
		}
	}
}

func TestSplitAssetLabel(t *testing.T) {
	cases := []struct{ in, name, addr string }{
		{"node-031(198.51.100.31)", "node-031", "198.51.100.31"},
		{"prod-tencent(https://10.0.24.3:6443)", "prod-tencent", "https://10.0.24.3:6443"},
		{"gateway(192.0.2.1)", "gateway", "192.0.2.1"},
		{"bare", "bare", ""},
		{"weird(a(b)c)", "weird", "a(b)c"},
	}
	for _, c := range cases {
		name, addr := jmsapi.SplitAssetLabel(c.in)
		if name != c.name || addr != c.addr {
			t.Errorf("splitAssetLabel(%q) = (%q, %q), want (%q, %q)", c.in, name, addr, c.name, c.addr)
		}
	}
}

// A command row carries no asset_id, so it can only be matched by its label.
// A session row carries one, so it must be matched by id -- matching by label
// alone breaks on two assets that share a name.
func TestAssetMatcherFindsRowsOfEitherKind(t *testing.T) {
	m := repository.NewAssetMatcher([]string{"uuid-1"}, []map[string]any{
		{"id": "uuid-1", "name": "dnsbox1", "address": "192.0.2.2"},
	})

	commandRow := map[string]any{"asset": "dnsbox1(192.0.2.2)"}
	if !m.Match(commandRow) {
		t.Error("a command row for the asset did not match")
	}
	sessionRow := map[string]any{"asset": "dnsbox1(192.0.2.2)", "asset_id": "uuid-1"}
	if !m.Match(sessionRow) {
		t.Error("a session row for the asset did not match")
	}
	other := map[string]any{"asset": "node-031(198.51.100.31)", "asset_id": "uuid-9"}
	if m.Match(other) {
		t.Error("a row for a different asset matched")
	}
}

func TestNormDate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026/10/20", "2026/10/20"},
		{"2026-10-20", "2026/10/20"},
		{"2026/10/20 12:00:00 +0800", "2026/10/20"},
		{"nonsense", ""},
	}
	for _, c := range cases {
		if got := normDate(c.in); got != c.want {
			t.Errorf("normDate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAggregateTouchBucketsPerAsset(t *testing.T) {
	sessions := []map[string]any{
		{"id": "s1", "asset": "dnsbox1(192.0.2.2)", "asset_id": "a1",
			"user": "ai(ai)", "account": "opsuser(opsuser)", "remote_addr": "192.0.2.1",
			"protocol": "ssh", "date_start": "2026/10/25 17:00:00 +0800",
			"date_end": "2026/10/25 17:00:05 +0800", "duration": "0:00:05",
			"has_replay": true, "replay_size": 1030},
		{"id": "s2", "asset": "dnsbox1(192.0.2.2)", "asset_id": "a1",
			"user": "Ops User(opsuser)", "account": "root(root)", "remote_addr": "192.0.2.150",
			"protocol": "ssh", "date_start": "2026/10/24 09:00:00 +0800",
			"date_end": "2026/10/24 09:10:00 +0800", "duration": "0:10:00",
			"has_replay": true, "replay_size": 99000},
		{"id": "s3", "asset": "node-031(198.51.100.31)", "asset_id": "a2",
			"user": "ai(ai)", "account": "opsuser(opsuser)", "remote_addr": "192.0.2.1",
			"protocol": "ssh", "date_start": "2026/10/20 08:00:00 +0800",
			"date_end": "2026/10/20 08:00:03 +0800", "duration": "0:00:03",
			"has_replay": false, "replay_size": 0},
	}
	res := aggregateTouch(sessions,
		map[string]int{"s1": 5},
		map[string]string{"s1": "ls -la"},
		nil, "", "")

	if res.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want 3", res.TotalSessions)
	}
	if len(res.Assets) != 2 {
		t.Fatalf("assets = %d, want 2", len(res.Assets))
	}
	// Newest contact first.
	first := res.Assets[0]
	if first.name != "dnsbox1" || first.address != "192.0.2.2" {
		t.Errorf("first asset = %s/%s, want dnsbox1/192.0.2.2", first.name, first.address)
	}
	if first.total != 2 {
		t.Errorf("dnsbox1 sessions = %d, want 2", first.total)
	}
	if len(first.users) != 2 {
		t.Errorf("dnsbox1 users = %v, want two", sortedSet(first.users))
	}
	if first.withReplay != 2 {
		t.Errorf("dnsbox1 sessions with replay = %d, want 2", first.withReplay)
	}
	// The command count rides on the session row.
	var found bool
	for _, r := range first.recent {
		if r.GetSessionID() == "s1" {
			found = true
			if r.GetCommands() != 5 {
				t.Errorf("s1 commands = %v, want 5", r.GetCommands())
			}
			if r.GetLastInput() != "ls -la" {
				t.Errorf("s1 last_input = %v, want ls -la", r.GetLastInput())
			}
		}
	}
	if !found {
		t.Error("session s1 is missing from the dnsbox1 bucket")
	}
	// The user filter must exclude the other person's session.
	only := aggregateTouch(sessions, nil, nil, nil, "Ops User", "")
	if only.TotalSessions != 1 {
		t.Errorf("filtered TotalSessions = %d, want 1", only.TotalSessions)
	}
	// The date filter must drop the older session.
	recent := aggregateTouch(sessions, nil, nil, nil, "", "2026/10/24")
	if recent.TotalSessions != 2 {
		t.Errorf("since-filtered TotalSessions = %d, want 2", recent.TotalSessions)
	}
}

// ---------------------------------------------------------------- wire level

// fakeBastion answers the endpoints the audit tools read, with the flat row
// shapes the real bastion uses for sessions and commands.
func fakeBastion() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/terminal/sessions/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "s1", "user": "ai(ai)", "asset": "node-031(198.51.100.31)",
				"asset_id": "a1", "account": "opsuser(opsuser)", "protocol": "ssh",
				"remote_addr": "192.0.2.1", "duration": "0:00:07",
				"date_start": "2026/10/25 17:10:38 +0800",
				"date_end":   "2026/10/25 17:10:45 +0800",
				"has_replay": true, "can_replay": true, "replay_size": 1030,
				"command_amount": 5, "has_command": false, "is_success": true,
				"error_reason": map[string]any{"value": "", "label": ""},
			}})
		case "/api/v1/terminal/commands/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "c1", "user": "ai(ai)", "asset": "node-031(198.51.100.31)",
				"session": "s1", "account": "opsuser(opsuser)", "input": "ls -la",
				"output": "total 0", "risk_level": map[string]any{"value": 0, "label": "Accept"},
				"timestamp": 1792919445, "timestamp_display": "2026/10/25 17:10:45 +0800",
				"remote_addr": "192.0.2.1",
			}})
		case "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "a1", "name": "node-031", "address": "198.51.100.31",
					"platform": map[string]any{"id": 1, "name": "Linux"}},
				{"id": "a2", "name": "idle-box", "address": "198.51.100.99",
					"platform": map[string]any{"id": 1, "name": "Linux"}},
			})
		case "/api/v1/perms/asset-permissions/":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "p1", "name": "所有", "accounts": []any{"@ALL"},
					"users_amount": float64(2), "assets_amount": float64(3),
					"is_active": true, "date_created": "2026/01/01 00:00:00 +0800"},
				{"id": "p2", "name": "x86", "accounts": []any{"opsuser"},
					"users_amount": float64(1), "assets_amount": float64(2),
					"is_active": true, "date_created": "2026/01/02 00:00:00 +0800"},
			})
		case "/api/v1/audits/login-logs/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "l1", "username": "ai(ai)", "ip": "192.0.2.1", "city": "LAN",
				"backend": "SSH Key", "backend_display": "SSH Key",
				"user_agent": "Go-http-client/1.1",
				"status":     map[string]any{"value": true, "label": "Success"},
				"datetime":   "2026/10/25 17:10:35 +0800",
			}})
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"detail":"not found"}`))
		}
	})
}

// testSession drives the management tools against a fake bastion.
//
// The tools go on through the registry, exactly as in production: a test
// that registered them directly would not exercise the gate.
func testSession(t *testing.T, handler http.Handler) *mcp.ClientSession {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	c := repository.NewBase(ts.URL, "test-token", ts.Client())
	return testsupport.NewSession(t, "jms-test", reg.Options{Policy: policy.Table},
		func(r *reg.Registry) {
			RegisterStatusTools(r, c)
			RegisterAuditTools(r, c)
			RegisterTouchTools(r, c)
			RegisterEvidenceTools(r, c)
			RegisterJobTools(r, c)
			RegisterTransferTools(r, c)
			RegisterAccountAutomationTools(r, c)
			RegisterNetworkTools(r, c)
			RegisterRBACTools(r, c)
			RegisterLoginACLTools(r, c)
			RegisterDataMaskingTools(r, c)
			RegisterClipboardACLTools(r, c)
		})
}

// The session listing must name the asset. Reading the flat row with nested()
// yields null here, which is the bug this test exists to hold shut.
func TestSessionsListNamesTheAsset(t *testing.T) {
	cs := testSession(t, fakeBastion())
	out := testsupport.CallToolJSON(t, cs, "jms_sessions_list", map[string]any{})

	sessions := out["items"].([]any)
	if len(sessions) != 1 {
		t.Fatalf("sessions = %v, want one", sessions)
	}
	s := sessions[0].(map[string]any)
	if s["asset"] != "node-031" {
		t.Errorf("asset = %v, want node-031", s["asset"])
	}
	if s["address"] != "198.51.100.31" {
		t.Errorf("address = %v, want 198.51.100.31", s["address"])
	}
	if s["protocol"] != "ssh" {
		t.Errorf("protocol = %v, want ssh", s["protocol"])
	}
	if s["account"] != "opsuser(opsuser)" {
		t.Errorf("account = %v, want opsuser(opsuser)", s["account"])
	}
}

// Filtering by asset must actually filter. With the id read off the wrong
// field every row is dropped and the tool answers "nothing happened".
func TestSessionsListFiltersByAsset(t *testing.T) {
	cs := testSession(t, fakeBastion())
	out := testsupport.CallToolJSON(t, cs, "jms_sessions_list", map[string]any{"asset": "198.51.100.31"})
	if n := len(out["items"].([]any)); n != 1 {
		t.Errorf("sessions for the asset = %d, want 1", n)
	}
	out = testsupport.CallToolJSON(t, cs, "jms_sessions_list", map[string]any{"asset": "198.51.100.99"})
	if n := len(out["items"].([]any)); n != 0 {
		t.Errorf("sessions for an asset with no history = %d, want 0", n)
	}
	// A filter for the login name has to reach the "name(username)" display,
	// otherwise it matches nothing at all.
	out = testsupport.CallToolJSON(t, cs, "jms_sessions_list", map[string]any{"user": "opsuser"})
	if n := len(out["items"].([]any)); n != 0 {
		t.Errorf("sessions for user opsuser = %d, want 0 (the fake user is ai)", n)
	}
	out = testsupport.CallToolJSON(t, cs, "jms_sessions_list", map[string]any{"user": "ai"})
	if n := len(out["items"].([]any)); n != 1 {
		t.Errorf("sessions for user ai = %d, want 1", n)
	}
}

// The login log names its address "ip" and its timestamp "datetime", unlike
// the operate log which uses "remote_addr". Reading it under the operate-log
// names silently produced nulls.
func TestLoginLogsCarryAddressAndTime(t *testing.T) {
	cs := testSession(t, fakeBastion())
	out := testsupport.CallToolJSON(t, cs, "jms_login_logs", map[string]any{})

	logs := out["items"].([]any)
	if len(logs) != 1 {
		t.Fatalf("logs = %v, want one", logs)
	}
	l := logs[0].(map[string]any)
	if l["remote_addr"] != "192.0.2.1" {
		t.Errorf("remote_addr = %v, want 192.0.2.1", l["remote_addr"])
	}
	if !strings.Contains(fmt.Sprint(l["date_created"]), "2026/10/25") {
		t.Errorf("date_created = %v, want the 2026/10/25 timestamp", l["date_created"])
	}
	if l["backend"] != "SSH Key" {
		t.Errorf("backend = %v, want SSH Key", l["backend"])
	}
}

// The endpoint answers with its own 200-row cap whatever page_size says, so a
// limit that is not applied on this side is simply ignored.
func TestLoginLogsRespectsLimit(t *testing.T) {
	rows := []map[string]any{}
	for i := 0; i < 5; i++ {
		rows = append(rows, map[string]any{
			"id": fmt.Sprintf("l%d", i), "username": "ai(ai)", "ip": "192.0.2.1",
			"backend": "SSH Key", "user_agent": "x",
			"status":   map[string]any{"value": true, "label": "Success"},
			"datetime": "2026/10/25 17:10:35 +0800",
		})
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/audits/login-logs/" {
			json.NewEncoder(w).Encode(rows)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	cs := testSession(t, handler)
	out := testsupport.CallToolJSON(t, cs, "jms_login_logs", map[string]any{"limit": 2})
	if out["count"] != float64(2) {
		t.Errorf("count = %v, want 2", out["count"])
	}
	if out["total"] != float64(5) {
		t.Errorf("total_rows = %v, want 5", out["total"])
	}
}

func TestPermissionsListRespectsLimit(t *testing.T) {
	rows := []map[string]any{}
	for i := 0; i < 5; i++ {
		rows = append(rows, map[string]any{
			"id": fmt.Sprintf("p%d", i), "name": fmt.Sprintf("perm-%d", i),
			"is_active": true, "assets_amount": 1, "users_amount": 1,
			"nodes_amount": 0, "accounts": []string{},
		})
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/perms/asset-permissions/" {
			json.NewEncoder(w).Encode(rows)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	cs := testSession(t, handler)
	out := testsupport.CallToolJSON(t, cs, "jms_permissions_list", map[string]any{"limit": 2})
	if out["count"] != float64(2) {
		t.Errorf("count = %v, want 2", out["count"])
	}
	if out["total"] != float64(5) {
		t.Errorf("total_rows = %v, want 5", out["total"])
	}
}

// The whole point of the touch history: who touched which machine, from where,
// under which account -- with the recording indexed rather than downloaded.
func TestAssetTouchHistoryAggregatesByAsset(t *testing.T) {
	cs := testSession(t, fakeBastion())
	out := testsupport.CallToolJSON(t, cs, "jms_asset_touch_history", map[string]any{})

	if out["assets_touched"] != float64(1) {
		t.Errorf("assets_touched = %v, want 1", out["assets_touched"])
	}
	assets := out["assets"].([]any)
	a := assets[0].(map[string]any)
	if a["asset"] != "node-031" {
		t.Errorf("asset = %v, want node-031", a["asset"])
	}
	if a["address"] != "198.51.100.31" {
		t.Errorf("address = %v, want 198.51.100.31", a["address"])
	}
	if a["sessions"] != float64(1) {
		t.Errorf("sessions = %v, want 1", a["sessions"])
	}
	if a["sessions_with_replay"] != float64(1) {
		t.Errorf("sessions_with_replay = %v, want 1", a["sessions_with_replay"])
	}
	if a["sessions_with_commands"] != float64(1) {
		t.Errorf("sessions_with_commands = %v, want 1", a["sessions_with_commands"])
	}
	recent := a["recent"].([]any)
	r := recent[0].(map[string]any)
	if r["source_ip"] != "192.0.2.1" {
		t.Errorf("source_ip = %v, want 192.0.2.1", r["source_ip"])
	}
	if r["commands"] != float64(1) {
		t.Errorf("commands = %v, want 1", r["commands"])
	}
	if r["last_input"] != "ls -la" {
		t.Errorf("last_input = %v, want ls -la", r["last_input"])
	}
	// The login log has no asset, so the tool has to say so rather than
	// pretend a bastion login belongs to a machine.
	if !strings.Contains(fmt.Sprint(out["note"]), "login log has no asset") {
		t.Errorf("note = %v, want it to explain that logins carry no asset", out["note"])
	}
}

// ---------------------------------------------------------------- evidence

// replayTar builds the archive the replay endpoint serves: a tar holding one
// gzipped .cast.gz member, which is what castToText walks.
func replayTar(t *testing.T, lines []string) []byte {
	t.Helper()
	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	for _, l := range lines {
		gw.Write([]byte(l + "\n"))
	}
	gw.Close()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	tw.WriteHeader(&tar.Header{Name: "session.cast.gz", Mode: 0600, Size: int64(gz.Len())})
	tw.Write(gz.Bytes())
	tw.Close()
	return buf.Bytes()
}

// Two sessions on one asset: one that carries audit value and two commands,
// one flagged replay_unsupported that recorded only a handshake. The evidence
// tool has to report the first and drop the second -- a recording exists for
// both, so has_replay cannot be the test.
func fakeEvidenceBastion(t *testing.T) http.Handler {
	archive := replayTar(t, []string{
		`[0.5, "o", "last login: Tue Oct 25 17:00:00 2026"]`,
		`[1.2, "o", "ls -la\r\ntotal 4\r\n"]`,
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/assets/assets/":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "a1", "name": "dnsbox1", "address": "192.0.2.2",
				"platform": map[string]any{"id": 1, "name": "Linux"},
			}})
		case r.URL.Path == "/api/v1/terminal/sessions/":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "s1", "asset": "dnsbox1(192.0.2.2)", "asset_id": "a1",
					"user": "Ops User(opsuser)", "account": "opsuser(opsuser)",
					"remote_addr": "192.0.2.150", "protocol": "ssh",
					"date_start": "2026/10/25 17:00:00 +0800",
					"date_end":   "2026/10/25 17:30:00 +0800", "duration": "0:30:00",
					"has_replay": true, "replay_size": 1200,
					"error_reason": map[string]any{"value": "", "label": ""}},
				{"id": "s2", "asset": "dnsbox1(192.0.2.2)", "asset_id": "a1",
					"user": "ai(ai)", "account": "opsuser(opsuser)",
					"remote_addr": "192.0.2.1", "protocol": "ssh",
					"date_start": "2026/10/25 18:00:00 +0800",
					"date_end":   "2026/10/25 18:00:04 +0800", "duration": "0:00:04",
					"has_replay": true, "replay_size": 1500,
					"error_reason": map[string]any{"value": "replay_unsupported", "label": "replay unsupported"}},
				// This asset is no longer registered on the bastion: it is in
				// the session trail and in no asset list.
				{"id": "s3", "asset": "gone-box(192.0.2.99)", "asset_id": "a9",
					"user": "Ops User(opsuser)", "account": "root(root)",
					"remote_addr": "192.0.2.150", "protocol": "ssh",
					"date_start": "2026/05/18 11:13:58 +0800",
					"date_end":   "2026/05/18 11:59:30 +0800", "duration": "0:45:32",
					"has_replay": true, "replay_size": 1411,
					"error_reason": map[string]any{"value": "", "label": ""}},
			})
		case r.URL.Path == "/api/v1/terminal/commands/":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "c1", "session": "s1", "user": "Ops User(opsuser)",
					"asset": "dnsbox1(192.0.2.2)", "account": "opsuser(opsuser)",
					"input": "ls -la", "output": "total 4",
					"risk_level": map[string]any{"value": 0, "label": "Accept"},
					"timestamp":  1792919445, "timestamp_display": "2026/10/25 17:30:00 +0800"},
				{"id": "c2", "session": "s1", "user": "Ops User(opsuser)",
					"asset": "dnsbox1(192.0.2.2)", "account": "opsuser(opsuser)",
					"input": "uptime", "output": "up 6 days",
					"risk_level": map[string]any{"value": 0, "label": "Accept"},
					"timestamp":  1792919400, "timestamp_display": "2026/10/25 17:29:15 +0800"},
			})
		case strings.HasSuffix(r.URL.Path, "/replay/download/"):
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(archive)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestAssetEvidenceReportsOnlySessionsWithEvidence(t *testing.T) {
	cs := testSession(t, fakeEvidenceBastion(t))
	out := testsupport.CallToolJSON(t, cs, "jms_asset_evidence", map[string]any{"asset": "192.0.2.2"})

	if out["asset"] != "dnsbox1" {
		t.Errorf("asset = %v, want dnsbox1", out["asset"])
	}
	if out["sessions_on_asset"] != float64(2) {
		t.Errorf("sessions_on_asset = %v, want 2", out["sessions_on_asset"])
	}
	if out["sessions_with_evidence"] != float64(1) {
		t.Errorf("sessions_with_evidence = %v, want 1", out["sessions_with_evidence"])
	}
	if out["sessions_without_evidence"] != float64(1) {
		t.Errorf("sessions_without_evidence = %v, want 1", out["sessions_without_evidence"])
	}
	timeline := out["timeline"].([]any)
	if len(timeline) != 1 {
		t.Fatalf("timeline = %d entries, want 1", len(timeline))
	}
	e := timeline[0].(map[string]any)
	if e["session_id"] != "s1" {
		t.Errorf("session_id = %v, want s1 (the one with no error_reason)", e["session_id"])
	}
	cmds := e["commands"].([]any)
	if len(cmds) != 2 {
		t.Fatalf("commands = %d, want 2", len(cmds))
	}
	// Newest command first.
	if cmds[0].(map[string]any)["input"] != "ls -la" {
		t.Errorf("first command = %v, want ls -la (timestamp 17:30:00)",
			cmds[0].(map[string]any)["input"])
	}
	if e["user"] != "Ops User(opsuser)" {
		t.Errorf("user = %v, want Ops User(opsuser)", e["user"])
	}
	if e["source_ip"] != "192.0.2.150" {
		t.Errorf("source_ip = %v, want 192.0.2.150", e["source_ip"])
	}
}

func TestAssetEvidenceCanPullTheRecording(t *testing.T) {
	cs := testSession(t, fakeEvidenceBastion(t))
	out := testsupport.CallToolJSON(t, cs, "jms_asset_evidence",
		map[string]any{"asset": "192.0.2.2", "include_replay": true})
	e := out["timeline"].([]any)[0].(map[string]any)
	txt, ok := e["replay_text"].(string)
	if !ok {
		t.Fatalf("replay_text missing: %v", e)
	}
	if !strings.Contains(txt, "ls -la") {
		t.Errorf("replay_text = %q, want it to carry the recorded output", txt)
	}
	if out["replays_pulled"] != float64(1) {
		t.Errorf("replays_pulled = %v, want 1", out["replays_pulled"])
	}
}

// The touch history has to separate "a recording exists" from "the session
// carries audit value", otherwise its coverage numbers repeat the same
// illusion that has_replay creates.
func TestTouchHistorySeparatesEvidenceFromReplay(t *testing.T) {
	cs := testSession(t, fakeEvidenceBastion(t))
	out := testsupport.CallToolJSON(t, cs, "jms_asset_touch_history", map[string]any{})
	a := out["assets"].([]any)[0].(map[string]any)
	if a["sessions"] != float64(2) {
		t.Errorf("sessions = %v, want 2", a["sessions"])
	}
	if a["sessions_with_replay"] != float64(2) {
		t.Errorf("sessions_with_replay = %v, want 2 (both have a recording)", a["sessions_with_replay"])
	}
	if a["sessions_with_evidence"] != float64(1) {
		t.Errorf("sessions_with_evidence = %v, want 1", a["sessions_with_evidence"])
	}
	cov := out["coverage"].(map[string]any)
	// The coverage block is global across assets, so it counts the evidence
	// session of the deleted asset too -- two in total, one of them on
	// dnsbox1.
	if cov["sessions_with_evidence"] != float64(2) {
		t.Errorf("coverage.sessions_with_evidence = %v, want 2", cov["sessions_with_evidence"])
	}
	// The command rate is measured against the evidence sessions, so it can
	// never exceed 100%. Dividing the all-sessions command count by the
	// evidence count produced 121% before, because a few handshake leftovers
	// do carry command rows.
	if r := fmt.Sprint(cov["command_rate"]); strings.HasSuffix(r, "%") {
		var n float64
		fmt.Sscanf(r, "%f", &n)
		if n > 100 {
			t.Errorf("command_rate = %v, must not exceed 100%%", r)
		}
	} else {
		t.Errorf("command_rate = %v, want a percentage", cov["command_rate"])
	}
	recent := a["recent"].([]any)[0].(map[string]any)
	if recent["has_evidence"] != false {
		t.Errorf("newest session has_evidence = %v, want false (it is the replay_unsupported one)",
			recent["has_evidence"])
	}
}

// An asset that has been deleted from the bastion still has a trail, and
// "who touched this machine" is still worth answering for it. The evidence
// tool has to fall back to the label the sessions carry instead of failing
// with "no such asset".
func TestAssetEvidenceSurvivesADeletedAsset(t *testing.T) {
	cs := testSession(t, fakeEvidenceBastion(t))
	out := testsupport.CallToolJSON(t, cs, "jms_asset_evidence",
		map[string]any{"asset": "192.0.2.99", "limit": 2})

	if out["asset_deleted_from_bastion"] != true {
		t.Errorf("asset_deleted_from_bastion = %v, want true", out["asset_deleted_from_bastion"])
	}
	if out["asset"] != "gone-box" {
		t.Errorf("asset = %v, want gone-box", out["asset"])
	}
	if out["sessions_with_evidence"] != float64(1) {
		t.Errorf("sessions_with_evidence = %v, want 1", out["sessions_with_evidence"])
	}
	timeline := out["timeline"].([]any)
	if len(timeline) != 1 {
		t.Fatalf("timeline = %d entries, want 1", len(timeline))
	}
	e := timeline[0].(map[string]any)
	if e["session_id"] != "s3" {
		t.Errorf("session_id = %v, want s3", e["session_id"])
	}
	if e["account"] != "root(root)" {
		t.Errorf("account = %v, want root(root)", e["account"])
	}
}

// normLimit normalises a caller-supplied row count against oekit's
// pagination.Policy. The rule that matters is the over-maximum case: it
// falls back to the DEFAULT, not to the maximum. Every limit check in this
// server has always behaved that way, and changing it to clamp would be a
// behaviour change rather than a refactor.
func TestNormLimitFallsBackToDefault(t *testing.T) {
	cases := []struct{ v, def, max, want int }{
		{0, 20, 100, 20},
		{-5, 20, 100, 20},
		{50, 20, 100, 50},
		{100, 20, 100, 100},
		{101, 20, 100, 20}, // over the maximum -> default, not the maximum
		{9999, 50, 200, 50},
	}
	for _, c := range cases {
		if got := repository.NormLimit(c.v, c.def, c.max); got != c.want {
			t.Errorf("normLimit(%d, %d, %d) = %d, want %d", c.v, c.def, c.max, got, c.want)
		}
	}
}

// count / total / truncated must be present on every list tool, so a caller
// never has to learn a per-tool name for "how many came back" and "how many
// matched" -- and never has to guess whether the field exists at all.
func TestListToolsAlwaysCarryCountTotalTruncated(t *testing.T) {
	cs := testSession(t, fakeBastion())
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"jms_login_logs", map[string]any{"limit": 2}},
		{"jms_permissions_list", map[string]any{"limit": 2}},
		{"jms_sessions_list", map[string]any{"limit": 2}},
		{"jms_commands_search", map[string]any{"limit": 2}},
		{"jms_assets_list", map[string]any{"limit": 2}},
	} {
		out := testsupport.CallToolJSON(t, cs, tc.tool, tc.args)
		if _, ok := out["count"]; !ok {
			t.Errorf("%s: count missing", tc.tool)
		}
		if _, ok := out["total"]; !ok {
			t.Errorf("%s: total missing", tc.tool)
		}
		if _, ok := out["truncated"]; !ok {
			t.Errorf("%s: truncated missing", tc.tool)
		}
	}
}
