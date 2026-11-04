package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
)

// The RBAC fixtures are the BYTES the real bastion answered with (read off
// http://192.0.2.41 on 2026/11/01), not shapes invented here. What is
// worth copying byte for byte:
//
//   - a catalogue row carries `content_type` as a NESTED object
//     {"id":30,"app_label":"accounts","model":"account"} — reading it flat
//     yields "" and a row that says "some capability somewhere";
//   - permission ids are NUMBERS (127), role ids are UUID strings — one
//     id reader for both is a bug waiting for one of them;
//   - the list is an ENVELOPE {"count",...,"results":[...]} when the
//     request carries paging parameters, and a BARE ARRAY when it does
//     not. The walk pages (limit=1000), so it sees the envelope; but the
//     same endpoint answering a bare array is the same data, and the
//     envelope count is the only "did I see it all" signal there is.
const (
	fxOrgRoles = `[{"id":"00000000-0000-0000-0000-000000000005","name":"OrgAdmin","display_name":"Organization Admin","scope":{"value":"org","label":"Organization"},"users_amount":1,"builtin":true,"date_created":"2025/05/17 19:02:16 +0800","date_updated":"2026/10/26 08:41:23 +0800","created_by":"System","updated_by":"System","comment":""},{"id":"00000000-0000-0000-0000-000000000006","name":"OrgAuditor","display_name":"Organization Auditor","scope":{"value":"org","label":"Organization"},"users_amount":0,"builtin":true,"date_created":"2025/05/17 19:02:16 +0800","date_updated":"2026/10/26 08:41:23 +0800","created_by":"System","updated_by":"System","comment":""},{"id":"00000000-0000-0000-0000-000000000007","name":"OrgUser","display_name":"Organization Operator","scope":{"value":"org","label":"Organization"},"users_amount":2,"builtin":true,"date_created":"2025/05/17 19:02:16 +0800","date_updated":"2026/10/26 08:41:23 +0800","created_by":"System","updated_by":"System","comment":""}]`

	fxSysRoles = `[{"id":"00000000-0000-0000-0000-000000000001","name":"SystemAdmin","display_name":"System Admin","scope":{"value":"system","label":"System"},"users_amount":2,"builtin":true,"date_created":"2025/05/17 19:02:16 +0800","date_updated":"2026/10/26 08:41:22 +0800","created_by":"System","updated_by":"System","comment":""},{"id":"00000000-0000-0000-0000-000000000002","name":"SystemAuditor","display_name":"System Auditor","scope":{"value":"system","label":"System"},"users_amount":1,"builtin":true,"date_created":"2025/05/17 19:02:16 +0800","date_updated":"2026/10/26 08:41:22 +0800","created_by":"System","updated_by":"System","comment":""},{"id":"00000000-0000-0000-0000-000000000004","name":"SystemComponent","display_name":"System Component","scope":{"value":"system","label":"System"},"users_amount":5,"builtin":true,"date_created":"2025/05/17 19:02:16 +0800","date_updated":"2026/10/26 08:41:23 +0800","created_by":"System","updated_by":"System","comment":""},{"id":"00000000-0000-0000-0000-000000000003","name":"SystemUser","display_name":"System Operator","scope":{"value":"system","label":"System"},"users_amount":2,"builtin":true,"date_created":"2025/05/17 19:02:16 +0800","date_updated":"2026/10/26 08:41:22 +0800","created_by":"System","updated_by":"System","comment":""}]`

	// Three real catalogue rows and one real system-scope row.
	fxPermsRows = `[
{"id":127,"name":"Can add Account","content_type":{"id":30,"app_label":"accounts","model":"account"},"codename":"add_account"},
{"id":194,"name":"Can add Account backup plan","content_type":{"id":43,"app_label":"accounts","model":"backupaccountautomation"},"codename":"add_backupaccountautomation"},
{"id":198,"name":"Can add account check automation","content_type":{"id":44,"app_label":"accounts","model":"checkaccountautomation"},"codename":"add_checkaccountautomation"},
{"id":417,"name":"Can add Access key","content_type":{"id":91,"app_label":"authentication","model":"accesskey"},"codename":"add_accesskey"}]`
)

// fakeBastionWithRBAC serves the permission catalogue and the role lists.
//
// Which role holds which permission is the fixture's join input:
// OrgAdmin holds 127+194, OrgUser holds 127, SystemAdmin holds 417+127,
// and NOBODY holds 198 — the "held by nobody" row is a finding the matrix
// must still show, not noise to drop.
func fakeBastionWithRBAC(t *testing.T) http.Handler {
	t.Helper()
	perms := map[string]string{
		"127": `{"id":127,"name":"Can add Account","content_type":{"id":30,"app_label":"accounts","model":"account"},"codename":"add_account"}`,
		"194": `{"id":194,"name":"Can add Account backup plan","content_type":{"id":43,"app_label":"accounts","model":"backupaccountautomation"},"codename":"add_backupaccountautomation"}`,
		"198": `{"id":198,"name":"Can add account check automation","content_type":{"id":44,"app_label":"accounts","model":"checkaccountautomation"},"codename":"add_checkaccountautomation"}`,
		"417": `{"id":417,"name":"Can add Access key","content_type":{"id":91,"app_label":"authentication","model":"accesskey"},"codename":"add_accesskey"}`,
	}
	holds := map[string][]string{
		"00000000-0000-0000-0000-000000000005": {"127", "194"}, // OrgAdmin
		"00000000-0000-0000-0000-000000000007": {"127"},        // OrgUser
		"00000000-0000-0000-0000-000000000001": {"417", "127"}, // SystemAdmin
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/rbac/org-roles/":
			w.Write([]byte(fxOrgRoles))
		case "/api/v1/rbac/system-roles/":
			w.Write([]byte(fxSysRoles))
		case "/api/v1/rbac/permissions/":
			q := r.URL.Query()
			if role := q.Get("role"); role != "" {
				rows := ""
				for i, id := range holds[role] {
					if i > 0 {
						rows += ","
					}
					rows += perms[id]
				}
				w.Write([]byte(`{"count":` + itoaTest(len(holds[role])) +
					`,"next":null,"previous":null,"results":[` + rows + `]}`))
				return
			}
			// The catalogue. scope=system is the superset (4 rows here);
			// scope=org is the smaller set (the first three).
			rows := []string{perms["127"], perms["194"], perms["198"]}
			count := 265
			if q.Get("scope") == "system" {
				rows = append(rows, perms["417"])
				count = 385
			}
			joined := ""
			for i, row := range rows {
				if i > 0 {
					joined += ","
				}
				joined += row
			}
			w.Write([]byte(`{"count":` + itoaTest(count) +
				`,"next":null,"previous":null,"results":[` + joined + `]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func itoaTest(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestRBACPermissionsReadsTheNestedContentType(t *testing.T) {
	cs := testSession(t, fakeBastionWithRBAC(t))
	out := testsupport.CallToolJSON(t, cs, "jms_rbac_permissions", map[string]any{})

	items := out["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("items = %d, want the three org rows", len(items))
	}
	first := items[0].(map[string]any)
	if first["codename"] != "add_account" {
		t.Errorf("codename = %v, want add_account", first["codename"])
	}
	// The nested content_type is the whole point of the fixture: read flat
	// it would be "" and the row would say "some capability somewhere".
	if first["app_label"] != "accounts" || first["model"] != "account" {
		t.Errorf("content_type = %v/%v, want accounts/account", first["app_label"], first["model"])
	}
	// The numeric id has to come out as the string form of the NUMBER.
	if first["id"] != "127" {
		t.Errorf("id = %v, want 127", first["id"])
	}
}

func TestRBACPermissionsSearchesLocally(t *testing.T) {
	cs := testSession(t, fakeBastionWithRBAC(t))

	out := testsupport.CallToolJSON(t, cs, "jms_rbac_permissions",
		map[string]any{"search": "backup"})
	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("search backup: items = %d, want 1", len(items))
	}
	if items[0].(map[string]any)["codename"] != "add_backupaccountautomation" {
		t.Errorf("wrong row matched: %v", items[0])
	}

	out = testsupport.CallToolJSON(t, cs, "jms_rbac_permissions",
		map[string]any{"codename": "add_account"})
	items = out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("codename filter: items = %d, want exactly 1 (exact match)", len(items))
	}
	if out["note"] != nil && out["note"] != "" {
		t.Errorf("note = %v, want empty on a match", out["note"])
	}

	out = testsupport.CallToolJSON(t, cs, "jms_rbac_permissions",
		map[string]any{"search": "no-such-capability"})
	if out["total"] != float64(0) {
		t.Errorf("total = %v, want 0", out["total"])
	}
	if out["note"] == nil || out["note"] == "" {
		t.Errorf("a zero-match answer must say so in note, got %v", out["note"])
	}
}

func TestRBACRoleMatrixJoinsHoldersAcrossScopes(t *testing.T) {
	cs := testSession(t, fakeBastionWithRBAC(t))
	out := testsupport.CallToolJSON(t, cs, "jms_rbac_role_matrix", map[string]any{})

	roles := out["roles"].([]any)
	if len(roles) != 7 {
		t.Fatalf("roles = %d, want 7 (3 org + 4 system)", len(roles))
	}
	byName := map[string]map[string]any{}
	for _, r := range roles {
		rm := r.(map[string]any)
		byName[rm["name"].(string)] = rm
	}
	if got := byName["OrgAdmin"]["permission_count"]; got != float64(2) {
		t.Errorf("OrgAdmin permission_count = %v, want 2", got)
	}
	if got := byName["SystemAdmin"]["scope"]; got != "system" {
		t.Errorf("SystemAdmin scope = %v, want system", got)
	}
	if got := byName["OrgAdmin"]["users_amount"]; got != float64(1) {
		t.Errorf("OrgAdmin users_amount = %v, want 1", got)
	}

	entries := out["entries"].([]any)
	if len(entries) != 4 {
		t.Fatalf("entries = %d, want 4 catalogue rows", len(entries))
	}
	holdersOf := map[string][]any{}
	for _, e := range entries {
		em := e.(map[string]any)
		holdersOf[em["codename"].(string)] = em["holders"].([]any)
	}
	// add_account is held on both sides of the scope line. Holders are
	// labelled name@scope so an org role and a system role that share a name
	// stay two distinct holders instead of collapsing into one.
	h := holdersOf["add_account"]
	want := map[string]bool{"OrgAdmin@org": true, "OrgUser@org": true, "SystemAdmin@system": true}
	if len(h) != len(want) {
		t.Fatalf("add_account holders = %v, want %d distinct name@scope holders", h, len(want))
	}
	for _, got := range h {
		g, _ := got.(string)
		if !want[g] {
			t.Errorf("add_account holder %q not expected (want name@scope, e.g. OrgAdmin@org)", g)
		}
	}
	// add_checkaccountautomation is held by NOBODY and must still appear:
	// "this permission is held by nobody" is a finding, not noise.
	if len(holdersOf["add_checkaccountautomation"]) != 0 {
		t.Errorf("add_checkaccountautomation holders = %v, want empty (kept, not dropped)",
			holdersOf["add_checkaccountautomation"])
	}
}

// An org role and a system role that SHARE A NAME both holding the same
// permission must show as TWO holders. Keying the dedup on the bare name
// merged them into one row and silently dropped one — the exact thing
// name@scope exists to prevent.
func TestRBACRoleMatrixKeepsSameNameRolesFromDifferentScopesDistinct(t *testing.T) {
	// One permission (id 127), held by an org role named "Admin" AND a
	// system role named "Admin". Same name, different scope.
	perm := `{"id":127,"name":"Can add Account","content_type":{"id":30,"app_label":"accounts","model":"account"},"codename":"add_account"}`
	orgRoles := `[{"id":"org-1","name":"Admin","scope":{"value":"org","label":"Organization"},"users_amount":1,"builtin":true}]`
	sysRoles := `[{"id":"sys-1","name":"Admin","scope":{"value":"system","label":"System"},"users_amount":1,"builtin":true}]`
	fake := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/rbac/org-roles/":
			w.Write([]byte(orgRoles))
		case "/api/v1/rbac/system-roles/":
			w.Write([]byte(sysRoles))
		case "/api/v1/rbac/permissions/":
			q := r.URL.Query()
			if q.Get("role") != "" {
				w.Write([]byte(`{"count":1,"next":null,"previous":null,"results":[` + perm + `]}`))
				return
			}
			count := 265
			if q.Get("scope") == "system" {
				count = 385
			}
			w.Write([]byte(`{"count":` + itoaTest(count) +
				`,"next":null,"previous":null,"results":[` + perm + `]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	cs := testSession(t, fake)
	out := testsupport.CallToolJSON(t, cs, "jms_rbac_role_matrix", map[string]any{})

	var holders []any
	for _, e := range out["entries"].([]any) {
		em := e.(map[string]any)
		if em["codename"] == "add_account" {
			holders = em["holders"].([]any)
		}
	}
	// Two holders, one per scope — not one collapsed "Admin".
	if len(holders) != 2 {
		t.Fatalf("add_account holders = %v, want 2 (Admin@org + Admin@system); "+
			"merging by bare name drops one", holders)
	}
	got := map[string]bool{}
	for _, h := range holders {
		s, _ := h.(string)
		got[s] = true
	}
	if !got["Admin@org"] || !got["Admin@system"] {
		t.Errorf("holders = %v, want Admin@org and Admin@system both present", holders)
	}
}

func TestRBACRoleMatrixFiltersByScope(t *testing.T) {
	cs := testSession(t, fakeBastionWithRBAC(t))
	out := testsupport.CallToolJSON(t, cs, "jms_rbac_role_matrix",
		map[string]any{"scope": "org"})

	roles := out["roles"].([]any)
	if len(roles) != 3 {
		t.Fatalf("roles = %d, want 3 (org only)", len(roles))
	}
	entries := out["entries"].([]any)
	// The org catalogue has 3 rows; the system-only row (add_accesskey)
	// must not leak into an org-scoped matrix.
	for _, e := range entries {
		if e.(map[string]any)["codename"] == "add_accesskey" {
			t.Errorf("add_accesskey (system-only) leaked into an org matrix: %v", e)
		}
	}
	if len(entries) != 3 {
		t.Errorf("entries = %d, want 3 (the org catalogue)", len(entries))
	}
}
