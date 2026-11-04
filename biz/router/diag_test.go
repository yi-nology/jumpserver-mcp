package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oekit/pkg/convert"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mcp/biz/handler"
	"github.com/yi-nology/jumpserver-mcp/internal/repository"
	"github.com/yi-nology/jumpserver-mcp/internal/service/policy"
)

// diagCheck is the shape the diagnostic contract puts on the wire.
//
// The tests read the JSON, not the Go type, on purpose: the contract is what
// the caller sees, and a test that used the same struct as the producer
// would not notice a change in the wire format.
type diagCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

// The diagnostic tools answer in ONE shape, and a caller has to be able to
// read any of them the same way: what was examined, what is not right, how
// bad it is, and what to do about it.
//
// That contract is worth a test of its own, because it is exactly the kind
// of thing that drifts: each tool was written separately, and the day one of
// them grows a "notes" field again, a caller that reads the five of them
// the same way breaks on that one.
//
// The test drives all five against a fake bastion and checks the shape
// itself: the shared fields are present, every status is one of the three,
// the findings are exactly the checks that are not ok, and the grade is the
// worst of them.
func TestDiagnosticToolsShareOneShape(t *testing.T) {
	cs := diagSession(t, fakeDiagBastion(true))
	cases := []struct {
		tool string
		args map[string]any
	}{
		{"jms_doctor", map[string]any{}},
		{"jms_asset_audit", map[string]any{"asset": "203.0.113.1"}},
		{"jms_command_guard", map[string]any{"command": "rm -rf /tmp/x"}},
		{"jms_security_review", map[string]any{}},
		{"jms_login_asset_check", map[string]any{"asset": "203.0.113.1"}},
	}
	for _, tc := range cases {
		out := testsupport.CallToolJSON(t, cs, tc.tool, tc.args)
		assertDiagShape(t, tc.tool, out)
	}
}

// The old fields are gone on purpose: the answers used to carry a boolean
// "healthy", a string list "problems" and a free-floating "note", and a
// caller could not tell which of them was authoritative. Their disappearance
// is the breaking part of this change, so it is pinned here.
func TestDiagnosticToolsDropTheOldFields(t *testing.T) {
	cs := diagSession(t, fakeDiagBastion(true))

	doctor := testsupport.CallToolJSON(t, cs, "jms_doctor", map[string]any{})
	if _, ok := doctor["healthy"]; ok {
		t.Error("the doctor still answers with a boolean 'healthy'; the status field replaces it")
	}

	audit := testsupport.CallToolJSON(t, cs, "jms_asset_audit", map[string]any{"asset": "203.0.113.1"})
	if _, ok := audit["problems"]; ok {
		t.Error("the asset audit still answers with a string list 'problems'; the checks field replaces it")
	}
	if _, ok := audit["healthy"]; ok {
		t.Error("the asset audit still answers with a boolean 'healthy'")
	}

	guard := testsupport.CallToolJSON(t, cs, "jms_command_guard", map[string]any{"command": "rm -rf /tmp/x"})
	if _, ok := guard["note"]; ok {
		t.Error("the command guard still answers with a free-floating 'note'")
	}
	// The four-value enum is gone: the verdict is a sentence now and the
	// grade lives in the status.
	if v, _ := guard["verdict"].(string); v == "blocked" || v == "warned" ||
		v == "explicitly_allowed" || v == "not_filtered" {
		t.Errorf("the command guard still answers with the old enum verdict %q", v)
	}

	review := testsupport.CallToolJSON(t, cs, "jms_security_review", map[string]any{})
	if _, ok := review["healthy"]; ok {
		t.Error("the security review still answers with a boolean 'healthy'")
	}
}

// The asset audit has to say the same thing about a broken machine as before
// -- but in the shared shape, and with the fix attached to each finding.
func TestAssetAuditGradesAndFixesTheBrokenMachine(t *testing.T) {
	cs := diagSession(t, fakeDiagBastion(true))

	ok := testsupport.CallToolJSON(t, cs, "jms_asset_audit", map[string]any{"asset": "203.0.113.1"})
	if ok["status"] != "ok" {
		t.Errorf("a healthy machine grades %v, want ok", ok["status"])
	}
	if len(diagChecksOf(t, "jms_asset_audit", ok)) == 0 {
		t.Error("the audit of a healthy machine carries no checks")
	}

	broken := testsupport.CallToolJSON(t, cs, "jms_asset_audit", map[string]any{"asset": "203.0.113.2"})
	if broken["status"] != "fail" {
		t.Errorf("a machine without protocol, account or permission grades %v, want fail", broken["status"])
	}
	checks := diagChecksOf(t, "jms_asset_audit", broken)
	want := map[string]string{
		"terminal_protocol": "fail",
		"accounts":          "fail",
		"permissions":       "fail",
	}
	for _, ck := range checks {
		if want[ck.Name] == "" {
			continue
		}
		if ck.Status != want[ck.Name] {
			t.Errorf("check %q = %q, want %q", ck.Name, ck.Status, want[ck.Name])
		}
		if ck.Fix == "" {
			t.Errorf("check %q is %q but carries no fix", ck.Name, ck.Status)
		}
	}
	// Every failure has to name the tool that fixes it: "what to do" is the
	// point of the advice, and a tool name is what an agent can act on.
	for _, ck := range checks {
		if ck.Status != "fail" {
			continue
		}
		if !strings.Contains(ck.Fix, "jms_") {
			t.Errorf("check %q says %q, which names no tool to fix it with", ck.Name, ck.Fix)
		}
	}
	if advice, _ := broken["advice"].(string); advice == "" {
		t.Error("the audit of a broken machine carries no advice")
	}
}

// The guard's grade is the whole point of the tool: a command the bastion
// refuses has to grade fail, and a command nothing covers has to grade ok.
func TestCommandGuardGradesTheRefusal(t *testing.T) {
	cs := diagSession(t, fakeDiagBastion(true))

	blocked := testsupport.CallToolJSON(t, cs, "jms_command_guard", map[string]any{"command": "rm -rf /tmp/x"})
	if blocked["status"] != "fail" {
		t.Errorf("a rejected command grades %v, want fail", blocked["status"])
	}
	if verdict, _ := blocked["verdict"].(string); verdict == "" {
		t.Error("the guard has no verdict")
	}

	allowed := testsupport.CallToolJSON(t, cs, "jms_command_guard", map[string]any{"command": "ls -la"})
	if allowed["status"] != "ok" {
		t.Errorf("an uncovered command grades %v, want ok", allowed["status"])
	}
}

// "This bastion has no command filters at all" was a free-floating note; it
// is a check now, so that a caller reading the checks sees it.
func TestCommandGuardReportsAnUnfilteredBastion(t *testing.T) {
	cs := diagSession(t, fakeDiagBastion(false))
	out := testsupport.CallToolJSON(t, cs, "jms_command_guard", map[string]any{"command": "rm -rf /tmp/x"})
	if out["status"] != "ok" {
		t.Errorf("an unfiltered bastion grades %v, want ok", out["status"])
	}
	found := false
	for _, ck := range diagChecksOf(t, "jms_command_guard", out) {
		if ck.Name == "command_filters" {
			found = true
		}
	}
	if !found {
		t.Error("the guard does not report that the bastion has no command filters")
	}
}

// The doctor keeps its own rule -- a WARN is "needs attention" for the exit
// code -- but it now says so in the shared shape.
func TestDoctorKeepsItsWarnings(t *testing.T) {
	cs := diagSession(t, fakeDiagBastion(true))
	out := testsupport.CallToolJSON(t, cs, "jms_doctor", map[string]any{})
	checks := diagChecksOf(t, "jms_doctor", out)

	byName := map[string]diagCheck{}
	for _, ck := range checks {
		byName[ck.Name] = ck
	}
	if ck, ok := byName["assets_without_protocols"]; !ok {
		t.Error("the doctor no longer reports the assets without a protocol")
	} else if ck.Status != "fail" {
		t.Errorf("assets_without_protocols = %q, want fail", ck.Status)
	}
	if ck, ok := byName["assets_without_accounts"]; !ok {
		t.Error("the doctor no longer reports the assets without an account")
	} else if ck.Status != "warn" {
		t.Errorf("assets_without_accounts = %q, want warn", ck.Status)
	}
	// Every finding names the tool that fixes it.
	for _, ck := range checks {
		if ck.Status != "ok" && ck.Fix == "" {
			t.Errorf("check %q is %q but carries no fix", ck.Name, ck.Status)
		}
	}
}

// The security review grades a superuser as a warning, not as a failure:
// having one is normal for a bastion, having five is not.
func TestSecurityReviewGradesSuperusersAsWarn(t *testing.T) {
	cs := diagSession(t, fakeDiagBastion(true))
	out := testsupport.CallToolJSON(t, cs, "jms_security_review", map[string]any{})
	if out["status"] != "warn" {
		t.Errorf("a bastion with one superuser grades %v, want warn", out["status"])
	}
	byName := map[string]diagCheck{}
	for _, ck := range diagChecksOf(t, "jms_security_review", out) {
		byName[ck.Name] = ck
	}
	if ck, ok := byName["superusers"]; !ok {
		t.Error("the security review does not report the superusers")
	} else if ck.Status != "warn" {
		t.Errorf("superusers = %q, want warn", ck.Status)
	}
	// A credential that was never rotated is worth a look, and the tool
	// promises to report it -- so it has to be one of the checks.
	if ck, ok := byName["password_never_rotated"]; !ok {
		t.Error("the security review does not report the credentials that were never rotated")
	} else if ck.Status != "warn" {
		t.Errorf("password_never_rotated = %q, want warn", ck.Status)
	}
}

// ---------------------------------------------------------------- helpers

// assertDiagShape checks the shared contract of one answer.
func assertDiagShape(t *testing.T, tool string, out map[string]any) {
	t.Helper()
	for _, key := range []string{"checks", "findings", "status", "verdict", "advice"} {
		if _, ok := out[key]; !ok {
			t.Errorf("%s: the answer has no %q", tool, key)
		}
	}
	status, _ := out["status"].(string)
	switch status {
	case "ok", "warn", "fail":
	default:
		t.Errorf("%s: status = %q, want one of ok/warn/fail", tool, status)
	}
	if verdict, _ := out["verdict"].(string); verdict == "" {
		t.Errorf("%s: the verdict is empty", tool)
	}
	if advice, _ := out["advice"].(string); advice == "" {
		t.Errorf("%s: the advice is empty", tool)
	}

	checks := diagChecksOf(t, tool, out)
	if len(checks) == 0 {
		t.Errorf("%s: the answer carries no checks", tool)
	}
	worst := "ok"
	for _, ck := range checks {
		switch ck.Status {
		case "ok":
		case "warn":
			if worst != "fail" {
				worst = "warn"
			}
		case "fail":
			worst = "fail"
		default:
			t.Errorf("%s: check %q has status %q", tool, ck.Name, ck.Status)
		}
		if ck.Detail == "" {
			t.Errorf("%s: check %q has no detail", tool, ck.Name)
		}
	}
	if worst != status {
		t.Errorf("%s: status = %q, but the worst check is %q", tool, status, worst)
	}

	// The findings are exactly the checks that are not ok.
	findings := diagChecksOf(t, tool, out, "findings")
	notOK := 0
	for _, ck := range checks {
		if ck.Status != "ok" {
			notOK++
		}
	}
	if len(findings) != notOK {
		t.Errorf("%s: %d findings, but %d checks are not ok", tool, len(findings), notOK)
	}
}

// diagChecksOf reads a list of checks out of an answer.
func diagChecksOf(t *testing.T, tool string, out map[string]any, key ...string) []diagCheck {
	t.Helper()
	name := "checks"
	if len(key) > 0 {
		name = key[0]
	}
	raw, ok := out[name].([]any)
	if !ok {
		t.Fatalf("%s: %q is not a list", tool, name)
	}
	checks := []diagCheck{}
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("%s: %q holds a non-object", tool, name)
		}
		checks = append(checks, diagCheck{
			Name:   convert.AnyToStr(m["name"]),
			Status: convert.AnyToStr(m["status"]),
			Detail: convert.AnyToStr(m["detail"]),
			Fix:    convert.AnyToStr(m["fix"]),
		})
	}
	return checks
}

// diagSession wires the five diagnostic tools against a fake bastion.
func diagSession(t *testing.T, bastion http.Handler) *mcp.ClientSession {
	t.Helper()
	ts := httptest.NewServer(bastion)
	t.Cleanup(ts.Close)

	c := repository.NewBase(ts.URL, "test-token", ts.Client())
	return testsupport.NewSession(t, "jms-diag-test", reg.Options{Policy: policy.Table},
		func(r *reg.Registry) {
			handler.RegisterDoctorTool(r, c)
			handler.RegisterAuditViewTools(r, c)
			handler.RegisterCommandGuard(r, c)
			handler.RegisterSecurityReview(r, c)
			handler.RegisterSecurityTools(r, c)
		})
}

// fakeDiagBastion answers the endpoints the diagnostic tools read.
//
// It holds one healthy machine (a protocol, an account, a permission that
// grants that account) and one broken machine (no protocol, no account, no
// permission), plus a command filter that rejects "rm".
func fakeDiagBastion(withFilter bool) http.Handler {
	assets := []map[string]any{
		{
			"id": "a1", "name": "good-box", "address": "203.0.113.1",
			"platform": map[string]any{"name": "Linux"},
			"category": map[string]any{"value": "host"},
			"type":     map[string]any{"value": "linux"},
			"protocols": []any{
				map[string]any{"name": "ssh", "port": float64(22)},
			},
			"nodes": []any{}, "labels": []any{},
			"is_active": true, "accounts_amount": float64(1),
			"connectivity":    map[string]any{"value": "ok"},
			"comment":         "",
			"date_verified":   "2026/10/20 10:00:00 +0800",
			"date_last_login": "2026/10/25 12:00:00 +0800",
		},
		{
			"id": "a2", "name": "broken-box", "address": "203.0.113.2",
			"platform":  map[string]any{"name": "Linux"},
			"category":  map[string]any{"value": "host"},
			"type":      map[string]any{"value": "linux"},
			"protocols": []any{}, "nodes": []any{}, "labels": []any{},
			"is_active": true, "accounts_amount": float64(0),
			"connectivity":    map[string]any{"value": "err"},
			"comment":         "",
			"date_verified":   "",
			"date_last_login": "",
		},
	}
	accounts := []map[string]any{
		{
			"id": "acc1", "username": "opsuser", "name": "opsuser",
			"secret_type": map[string]any{"value": "password", "label": "Password"},
			"privileged":  false, "is_active": true,
			"connectivity":    map[string]any{"value": "ok"},
			"asset":           map[string]any{"id": "a1"},
			"date_last_login": "2026/10/25 12:00:00 +0800",
		},
	}
	perms := []map[string]any{
		{"id": "p1", "name": "ai-all", "accounts": []any{"@ALL"},
			"users_amount": float64(1), "assets_amount": float64(1),
			"is_active": true, "date_created": "2026/01/01 00:00:00 +0800"},
	}
	permDetail := map[string]any{
		"id": "p1", "name": "ai-all", "accounts": []any{"@ALL"},
		"users":       []any{map[string]any{"name": "ai"}},
		"user_groups": []any{},
		"assets":      []any{map[string]any{"id": "a1"}},
		"nodes":       []any{},
		"is_active":   true,
	}
	users := []map[string]any{
		{"id": "u1", "username": "admin", "is_superuser": true, "is_active": true,
			"last_login": "2026/10/25 10:00:00 +0800", "need_update_password": false},
		{"id": "u2", "username": "ai", "is_superuser": false, "is_active": true,
			"last_login": "2026/10/25 11:00:00 +0800", "need_update_password": true},
	}
	logs := []map[string]any{
		{"id": "l1", "username": "admin", "ip": "192.0.2.1", "city": "LAN",
			"status":   map[string]any{"value": true, "label": "Success"},
			"datetime": "2026/10/25 10:00:00 +0800"},
	}
	groups := []map[string]any{
		{"id": "g1", "name": "rm-group", "type": map[string]any{"value": "command"},
			"content": "rm", "ignore_case": true},
	}
	filters := []map[string]any{}
	filterDetail := map[string]any{}
	if withFilter {
		filters = append(filters, map[string]any{
			"id": "f1", "name": "no-rm", "is_active": true, "priority": float64(10),
		})
		filterDetail = map[string]any{
			"id": "f1", "name": "no-rm", "action": "reject", "priority": float64(10),
			"is_active": true, "users": []any{}, "assets": []any{}, "accounts": []any{},
			"command_groups": []any{map[string]any{"id": "g1"}},
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/users/profile/":
			// The caller, as /users/profile/ answers. The doctor probes
			// reachability AND identity here now (not the users LIST, which
			// needs users.view_user). Must be present or the reachability
			// probe 404s and the doctor stops before the asset checks.
			json.NewEncoder(w).Encode(map[string]any{
				"id": "u1", "username": "admin", "name": "Administrator",
				"is_superuser": true,
			})
		case "/api/v1/users/users/":
			json.NewEncoder(w).Encode(users)
		case "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode(assets)
		case "/api/v1/assets/assets/a1/":
			json.NewEncoder(w).Encode(assets[0])
		case "/api/v1/assets/assets/a2/":
			json.NewEncoder(w).Encode(assets[1])
		case "/api/v1/assets/nodes/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/v1/accounts/accounts/":
			json.NewEncoder(w).Encode(accounts)
		case "/api/v1/perms/asset-permissions/":
			json.NewEncoder(w).Encode(perms)
		case "/api/v1/perms/asset-permissions/p1/":
			json.NewEncoder(w).Encode(permDetail)
		case "/api/v1/acls/command-filter-acls/":
			json.NewEncoder(w).Encode(filters)
		case "/api/v1/acls/command-filter-acls/f1/":
			json.NewEncoder(w).Encode(filterDetail)
		case "/api/v1/acls/command-groups/":
			json.NewEncoder(w).Encode(groups)
		case "/api/v1/acls/login-acls/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/v1/acls/login-asset-acls/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/v1/acls/login-asset/check/":
			json.NewEncoder(w).Encode(map[string]any{"need_review": false})
		case "/api/v1/audits/login-logs/":
			json.NewEncoder(w).Encode(logs)
		case "/api/v1/settings/security/block-ip/":
			json.NewEncoder(w).Encode([]map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"detail":"not found"}`))
		}
	})
}
