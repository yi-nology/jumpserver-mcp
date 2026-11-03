package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/policy"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/client"
)

// The write tools answer with a contract too, and they cannot be checked
// against the real bastion without changing it — so they are checked against
// a fake one that answers the way the bastion answers.
//
// This is where the contract earns its keep. The server validates an answer
// against the tool's declared schema BEFORE the model sees it, so an answer
// whose shape does not match fails the call right here — loudly, in a test,
// instead of quietly reaching the agent as a different shape than promised.
//
// Two real bugs were found this way: jms_permission_grant answered an
// object where the contract promised a boolean, and jms_job_run answered
// the risk class as a number where the contract promised the word.

// fakeBastionForWrites answers the endpoints the write tools touch.
func fakeBastionForWrites(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path, method := r.URL.Path, r.Method
		switch {
		case path == "/api/v1/assets/assets/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "results": []map[string]any{
				{"id": "A1", "name": "node-071", "address": "192.0.2.71",
					"platform":  map[string]any{"id": "P1", "name": "Linux"},
					"protocols": []map[string]any{{"name": "ssh", "port": 22}},
					"is_active": true},
			}})
		case path == "/api/v1/assets/assets/A1/" && method == "PATCH":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "A1", "name": "node-071", "address": "192.0.2.71",
				"platform":  map[string]any{"id": "P1", "name": "Linux"},
				"protocols": []map[string]any{{"name": "ssh", "port": 2222}},
				"is_active": true,
			})
		case path == "/api/v1/assets/nodes/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "results": []map[string]any{
				{"id": "N1", "name": "studio", "full_value": "/DEFAULT/studio", "assets_amount": 1},
			}})
		case path == "/api/v1/assets/nodes/" && method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"id": "N1"})
		case path == "/api/v1/assets/nodes/N1/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "N1", "name": "arm", "full_value": "/DEFAULT/arm", "assets_amount": 0,
			})
		case path == "/api/v1/perms/asset-permissions/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "results": []map[string]any{
				{"id": "M1", "name": "ai-全部资产", "users_amount": 2, "assets_amount": 3,
					"accounts": []string{"opsuser"}},
			}})
		case path == "/api/v1/perms/asset-permissions/M1/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "M1", "name": "ai-全部资产", "users_amount": 2, "assets_amount": 3,
				"users":    []map[string]any{{"id": "U1", "name": "opsuser"}},
				"accounts": []string{"opsuser"},
			})
		case path == "/api/v1/perms/asset-permissions/M1/" && method == "PATCH":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "M1", "name": "ai-全部资产", "users_amount": 3, "assets_amount": 3,
				"accounts": []string{"opsuser"},
			})
		case path == "/api/v1/users/users/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "results": []map[string]any{
				{"id": "U1", "username": "opsuser", "name": "opsuser", "is_active": true},
			}})
		case path == "/api/v1/users/users/U1/" && method == "PATCH":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "U1", "username": "opsuser", "name": "opsuser", "is_active": false,
			})
		case path == "/api/v1/ops/jobs/" && method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"id": "J1", "task_id": "T1"})
		case path == "/api/v1/perms/asset-permissions/" && method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"id": "M9", "name": "new-perm"})
		case path == "/api/v1/accounts/accounts/" && method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"id": "AC1", "username": "opsuser", "name": "opsuser"})
		case path == "/api/v1/accounts/accounts/AC1/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "AC1", "username": "opsuser", "name": "opsuser",
				"secret_type":  map[string]any{"value": "password", "label": "Password"},
				"connectivity": map[string]any{"value": "ok", "label": "OK"},
				"privileged":   false,
				"is_active":    true,
				"asset":        map[string]any{"name": "node-071", "address": "192.0.2.71"},
			})
		case path == "/api/v1/users/users/" && method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"id": "U9"})
		case path == "/api/v1/users/users/U9/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "U9", "username": "newuser", "name": "New User",
				"email": "newuser@example.com", "is_active": true,
				"is_superuser": false, "need_update_password": true,
			})
		case path == "/api/v1/users/users/U1/password/" && method == "PUT":
			json.NewEncoder(w).Encode(map[string]any{"id": "U9"})
		case path == "/api/v1/labels/labels/" && method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"id": "L9", "name": "env", "value": "prod"})
		case path == "/api/v1/labels/labels/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "results": []map[string]any{
				{"id": "L9", "name": "env", "value": "prod"},
			}})
		case path == "/api/v1/acls/command-groups/" && method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"id": "G9", "name": "destructive"})
		case path == "/api/v1/acls/command-groups/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "results": []map[string]any{
				{"id": "G9", "name": "destructive"},
			}})
		case path == "/api/v1/assets/nodes/N1/assets/add/" && method == "PUT":
			json.NewEncoder(w).Encode(map[string]any{})
		case path == "/api/v1/ops/job-executions/stop/" && method == "POST":
			// Validate the body the way the LIVE serializer does (captured
			// 2026-11-03): `task_id` is required. A fake that answers ok to
			// any body agrees with the code instead of with the API — that
			// is how `{"ids": [...]}` survived here while the real machine
			// 400ed every call.
			var stopBody map[string]any
			_ = json.NewDecoder(r.Body).Decode(&stopBody)
			if _, ok := stopBody["task_id"]; !ok {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"task_id": []string{"This field is required."}}})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case path == "/api/v1/assets/assets/" && method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"id": "A9", "name": "new-box", "platform": 11})
		case path == "/api/v1/users/users/U1/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "U1", "username": "opsuser", "name": "opsuser",
				"date_password_last_updated": "2026/10/26 10:00:00 +0800",
			})
		case path == "/api/v1/ops/job-executions/T1/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "T1", "is_finished": true, "is_success": true,
				"material": "shell:uptime", "time_cost": "3.20",
				"date_start":    "2026/10/26 09:00:00 +0800",
				"date_finished": "2026/10/26 09:00:03 +0800",
				"summary": map[string]any{
					"ok": []any{"node-071"}, "failures": map[string]any{}, "skipped": []any{}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"detail": "no such endpoint: " + method + " " + path})
		}
	})
}

// writeTestSession drives the write tools against a fake bastion.
func writeTestSession(t *testing.T, handler http.Handler) *mcp.ClientSession {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	c := client.NewBase(ts.URL, "test-token", ts.Client())
	return testsupport.NewSession(t, "jms-contract-test", reg.Options{Policy: policy.Table},
		func(r *reg.Registry) {
			RegisterUserTools(r, c)
			RegisterUserWriteTools(r, c)
			RegisterAccountWriteTools(r, c)
			RegisterPermissionTools(r, c)
			RegisterAssetWriteTools(r, c)
			RegisterScopeTools(r, c)
			RegisterFilterTools(r, c)
			RegisterLabelRoleTools(r, c)
			RegisterBulkTools(r, c)
			RegisterJobTools(r, c)
		})
}

// Every write tool answers with a shape its contract promises.
//
// The call itself is the assertion: the server checks the answer against the
// declared schema, and a mismatch fails the call.
func TestWriteToolsAnswerTheirOwnContract(t *testing.T) {
	cs := writeTestSession(t, fakeBastionForWrites(t))

	cases := []struct {
		name string
		tool string
		args map[string]any
	}{
		{
			name: "changing an asset",
			tool: "jms_asset_update",
			args: map[string]any{
				"asset": "192.0.2.71", "protocols": []string{"ssh:2222"}, "confirm": true,
			},
		},
		{
			name: "creating a node",
			tool: "jms_nodes_create",
			args: map[string]any{"name": "arm", "confirm": true},
		},
		{
			name: "granting a permission",
			tool: "jms_permission_grant",
			args: map[string]any{
				"permission": "ai-全部资产", "users": []string{"opsuser"}, "confirm": true,
			},
		},
		{
			name: "disabling a user",
			tool: "jms_user_set_active",
			args: map[string]any{"user": "opsuser", "active": false, "confirm": true},
		},
		{
			name: "running a batch job",
			tool: "jms_job_run",
			args: map[string]any{
				"assets": []string{"192.0.2.71"}, "command": "uptime", "confirm": true,
			},
		},
		{
			name: "asking a batch job for its status",
			tool: "jms_job_status",
			args: map[string]any{"task_id": "T1"},
		},
	}
	for _, tc := range cases {
		out := testsupport.CallToolJSON(t, cs, tc.tool, tc.args)
		if len(out) == 0 {
			t.Errorf("%s: the answer is empty", tc.name)
		}
	}
}

// The changed set of a permission is a set of ids, not a yes/no.
//
// It used to be a yes/no in the contract while the tool answered with the
// merged set — so the contract and the answer disagreed, and the contract
// was the one that was wrong: "now this permission covers whom" is the
// question the caller is actually asking.
func TestPermissionGrantSaysWhatItGranted(t *testing.T) {
	cs := writeTestSession(t, fakeBastionForWrites(t))
	out := testsupport.CallToolJSON(t, cs, "jms_permission_grant", map[string]any{
		"permission": "ai-全部资产", "users": []string{"opsuser"}, "confirm": true,
	})

	changed, ok := out["changed"].(map[string]any)
	if !ok {
		t.Fatalf("changed = %v, want the set that was written", out["changed"])
	}
	users, ok := changed["users"].([]any)
	if !ok || len(users) == 0 {
		t.Fatalf("the changed set does not say which users: %v", changed)
	}
	if out["permission"] != "ai-全部资产" {
		t.Errorf("permission = %v, want the permission's name", out["permission"])
	}
}

// The risk of a batch job is a word, like everywhere else.
//
// The enum carries its own wire form (CommandClass marshals as its name), so
// this is "READ" rather than the number behind it. The test pins it: the
// contract promises a string, and a number here would fail every call that
// runs a batch job.
func TestJobRunSpeaksTheRiskAsAWord(t *testing.T) {
	cs := writeTestSession(t, fakeBastionForWrites(t))
	out := testsupport.CallToolJSON(t, cs, "jms_job_run", map[string]any{
		"assets": []string{"192.0.2.71"}, "command": "uptime", "confirm": true,
	})

	risk, ok := out["risk"].(string)
	if !ok {
		t.Fatalf("risk = %v (%T), want the word READ", out["risk"], out["risk"])
	}
	if risk != "READ" {
		t.Errorf("risk = %q, want READ", risk)
	}
}

// The write tools that came onto a contract in the last round are exercised
// here too: the fake bastion answers the way the bastion answers, and the
// answer is checked against the tool's own schema before the caller sees it.
//
// A typed contract is checked by the compiler, but the VALUES still have to
// come from the right place — a field filled from the wrong response is
// wrong in a way no schema can catch.
func TestMoreWriteToolsAnswerTheirOwnContract(t *testing.T) {
	cs := writeTestSession(t, fakeBastionForWrites(t))
	cases := []struct {
		name string
		tool string
		args map[string]any
	}{
		{
			name: "creating a permission",
			tool: "jms_permission_create",
			args: map[string]any{
				"name": "new-perm", "users": []string{"opsuser"},
				"all_accounts": true, "confirm": true,
			},
		},
		{
			name: "creating an account",
			tool: "jms_account_create",
			args: map[string]any{
				"asset": "192.0.2.71", "account": "opsuser",
				"secret": "s3cret", "confirm": true,
			},
		},
		{
			name: "creating a user",
			tool: "jms_user_create",
			args: map[string]any{
				"user": "newuser", "email": "newuser@example.com", "confirm": true,
			},
		},
		{
			name: "setting a password",
			tool: "jms_user_password_set",
			args: map[string]any{"user": "opsuser", "password": "s3cret", "confirm": true},
		},
		{
			name: "creating a label",
			tool: "jms_label_create",
			args: map[string]any{"name": "env", "value": "prod", "confirm": true},
		},
		{
			name: "creating a command group",
			tool: "jms_cmdgroup_create",
			args: map[string]any{"name": "destructive", "commands": []string{"rm"}, "confirm": true},
		},
		{
			name: "moving an asset into a node",
			tool: "jms_asset_move",
			args: map[string]any{
				"assets": []string{"192.0.2.71"}, "node": "/DEFAULT/studio",
				"action": "add", "confirm": true,
			},
		},
		{
			name: "creating assets in bulk",
			tool: "jms_assets_bulk_create",
			args: map[string]any{
				"assets": []map[string]any{
					{"name": "new-box", "address": "192.0.2.99", "platform": "Linux",
						"protocols": []string{"ssh"}},
				},
				"confirm": true,
			},
		},
		{
			name: "stopping a batch job",
			tool: "jms_job_stop",
			args: map[string]any{"task_id": "T1", "confirm": true},
		},
	}
	for _, tc := range cases {
		out := testsupport.CallToolJSON(t, cs, tc.tool, tc.args)
		if len(out) == 0 {
			t.Errorf("%s: the answer is empty", tc.name)
		}
	}
}

// The permission's accounts are always a list — including the @ALL case.
//
// It used to be the string "@ALL" in one branch and a list of names in the
// other, so the same field meant two different things depending on how the
// permission was created.
func TestPermissionAccountsAreAlwaysAList(t *testing.T) {
	cs := writeTestSession(t, fakeBastionForWrites(t))
	out := testsupport.CallToolJSON(t, cs, "jms_permission_create", map[string]any{
		"name": "new-perm", "users": []string{"opsuser"},
		"all_accounts": true, "confirm": true,
	})
	accounts, ok := out["accounts"].([]any)
	if !ok {
		t.Fatalf("accounts = %v (%T), want a list", out["accounts"], out["accounts"])
	}
	if len(accounts) != 1 || accounts[0] != "@ALL" {
		t.Errorf("accounts = %v, want [\"@ALL\"]", accounts)
	}
}
