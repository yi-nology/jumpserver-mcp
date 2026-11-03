package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
)

// fakeBastionWithAutomation serves the account automation families.
//
// ⚠️ The rows are the BYTES the real bastion answered with (read off
// http://192.0.2.41 on 2026/11/01 by creating and triggering a push
// plan), not a shape invented here. Three details of that shape are
// load-bearing:
//
//   - `secret_type` / `secret_strategy` are LABELLED OBJECTS
//     {"value":...,"label":...}, not plain strings;
//   - the LIST shape carries assets_amount / nodes_amount instead of the
//     arrays, while the DETAIL shape carries the arrays -- both appear
//     here so a tool that reads only one of them is caught;
//   - `password_rules` is a nested object, and only its `length` is
//     worth reporting.
func fakeBastionWithAutomation(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "a1", "name": "node-071", "address": "192.0.2.71",
				"platform": map[string]any{"id": 1, "name": "Linux"},
			}})
		case r.URL.Path == "/api/v1/accounts/push-account-automations/":
			if r.Method == http.MethodPost {
				// The real 201 body of a created plan.
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]any{
					"id":   "f12ebc80-58a4-466e-ab34-bcbd4ee1a837",
					"name": "selftest-push", "type": "push_account",
					"interval": 24, "crontab": "",
					"accounts": []string{"opsuser"}, "nodes": []any{},
					"assets": []map[string]any{{
						"id": "a1", "name": "node-071",
					}},
					"periodic_display": "-", "executed_amount": 0,
					"secret_type":     map[string]any{"value": "password", "label": "Password"},
					"secret_strategy": map[string]any{"value": "random", "label": "Randomly generate"},
					"password_rules": map[string]any{
						"length": 16, "lowercase": true, "uppercase": true,
						"digit": true, "symbol": true, "exclude_symbols": "",
					},
					"ssh_key_change_strategy": map[string]any{
						"value": "set_jms", "label": "Replace (only replace keys pushed by this system)",
					},
					"params":      map[string]any{},
					"org_id":      "00000000-0000-0000-0000-000000000002",
					"org_name":    "DEFAULT",
					"is_periodic": false, "is_active": true, "check_conn_after_change": true,
					"date_created":        "2026/11/01 16:19:34 +0800",
					"date_updated":        "2026/11/01 16:19:34 +0800",
					"last_execution_date": nil,
					"comment":             "selftest",
					"created_by":          "Administrator",
				})
				return
			}
			// The LIST shape: counts instead of arrays.
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":   "f12ebc80-58a4-466e-ab34-bcbd4ee1a837",
					"name": "selftest-push", "type": "push_account",
					"interval": 24, "crontab": "",
					"accounts":         []string{"opsuser"},
					"periodic_display": "-", "executed_amount": 2,
					"secret_type":     map[string]any{"value": "password", "label": "Password"},
					"secret_strategy": map[string]any{"value": "random", "label": "Randomly generate"},
					"password_rules": map[string]any{
						"length": 20, "lowercase": true, "uppercase": true,
						"digit": true, "symbol": true, "exclude_symbols": "",
					},
					"ssh_key_change_strategy": map[string]any{
						"value": "set_jms", "label": "Replace",
					},
					"params":        map[string]any{},
					"assets_amount": 1, "nodes_amount": 0,
					"is_periodic": false, "is_active": true, "check_conn_after_change": true,
					"date_created":        "2026/11/01 16:19:34 +0800",
					"date_updated":        "2026/11/01 16:19:34 +0800",
					"last_execution_date": "2026/11/01 17:00:00 +0800",
					"comment":             "selftest",
					"created_by":          "Administrator",
				},
				{
					// ⚠️ A SECOND plan whose name CONTAINS the first one's
					// name. Without it, a prefix/substring match finds the
					// same plan an exact match finds -- and the difference
					// between "runs this plan" and "runs the wrong one"
					// cannot be seen. For a push, the wrong plan installs a
					// password on the wrong machines.
					"id":   "aaaa1111-2222-3333-4444-555566667777",
					"name": "selftest-push-weekly", "type": "push_account",
					"interval": 168, "crontab": "",
					"accounts":         []string{"root"},
					"periodic_display": "-", "executed_amount": 0,
					"secret_type":     map[string]any{"value": "password", "label": "Password"},
					"secret_strategy": map[string]any{"value": "random", "label": "Randomly generate"},
					"password_rules": map[string]any{
						"length": 12, "lowercase": true, "uppercase": true,
						"digit": true, "symbol": true, "exclude_symbols": "",
					},
					"ssh_key_change_strategy": map[string]any{
						"value": "set_jms", "label": "Replace",
					},
					"params":        map[string]any{},
					"assets_amount": 2, "nodes_amount": 0,
					"is_periodic": false, "is_active": true, "check_conn_after_change": true,
					"date_created":        "2026/11/01 16:19:34 +0800",
					"date_updated":        "2026/11/01 16:19:34 +0800",
					"last_execution_date": nil,
					"comment":             "selftest weekly",
					"created_by":          "Administrator",
				},
			})
		case r.URL.Path == "/api/v1/accounts/push-account-executions/":
			if r.Method == http.MethodPost {
				// The real 201 body of a triggered run: a celery task id.
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]any{
					"task": "e3710b47-aabe-4042-8331-8e8a416c70f2",
				})
				return
			}
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "e3710b47-aabe-4042-8331-8e8a416c70f2",
					"automation": map[string]any{
						"id":   "f12ebc80-58a4-466e-ab34-bcbd4ee1a837",
						"name": "selftest-push",
					},
					"trigger":       map[string]any{"value": "manual", "label": "Manual trigger"},
					"date_start":    "2026/11/01 16:19:43 +0800",
					"date_finished": "2026/11/01 16:19:43 +0800",
					"snapshot": map[string]any{
						"type":        map[string]any{"value": "push_account", "label": "Account Push"},
						"name":        "selftest-push",
						"comment":     "selftest",
						"accounts":    []string{"a5a54b34-5738-4a67-8b87-ad9bc6bbfd94"},
						"node_amount": 0, "asset_amount": 1,
					}, "status": map[string]any{"value": "failed", "label": "Failed"},
					"duration": "0.05",
				},
			})
		case r.URL.Path == "/api/v1/accounts/push-account-records/":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "r1", "asset": "node-071(192.0.2.71)",
					"account": "opsuser", "is_success": false,
					"status": "failed", "date_finished": "2026/11/01 16:19:44 +0800",
					"error": "permission denied",
				},
			})
		case r.URL.Path == "/api/v1/accounts/gather-account-automations/":
			json.NewEncoder(w).Encode([]map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// The plan list must name the plan and say what it does.
//
// Every field here sits where reading the wrong one yields a silent null:
// the secret type and strategy are labelled objects, the password length
// is nested inside password_rules, and the asset count only exists in the
// LIST shape.
func TestAccountAutomationsListNamesThePlan(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	out := testsupport.CallToolJSON(t, cs, "jms_account_automations_list",
		map[string]any{"family": "push"})

	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %v, want two rows", items)
	}
	p := items[0].(map[string]any)
	if p["name"] != "selftest-push" {
		t.Errorf("name = %v, want selftest-push", p["name"])
	}
	if p["id"] != "f12ebc80-58a4-466e-ab34-bcbd4ee1a837" {
		t.Errorf("id = %v, want the plan id", p["id"])
	}
	if p["secret_type"] != "password" {
		t.Errorf("secret_type = %v, want password", p["secret_type"])
	}
	if p["secret_strategy"] != "random" {
		t.Errorf("secret_strategy = %v, want random", p["secret_strategy"])
	}
	if p["password_length"] != float64(20) {
		t.Errorf("password_length = %v, want 20", p["password_length"])
	}
	if p["assets_amount"] != float64(1) {
		t.Errorf("assets_amount = %v, want 1", p["assets_amount"])
	}
	if p["executed_amount"] != float64(2) {
		t.Errorf("executed_amount = %v, want 2", p["executed_amount"])
	}
	if p["is_periodic"] != false {
		t.Errorf("is_periodic = %v, want false", p["is_periodic"])
	}
	if p["last_execution_date"] != "2026/11/01 17:00:00 +0800" {
		t.Errorf("last_execution_date = %v, want the timestamp", p["last_execution_date"])
	}
}

// A family that names nothing must be refused.
//
// The family picks the COLLECTION, so defaulting it would answer a
// question about gather with push's data -- and the caller would never
// know the two were mixed up.
func TestAccountAutomationsRefusesAnUnknownFamily(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_account_automations_list",
		Arguments: map[string]any{"family": "sideways"},
	})
	if err != nil {
		t.Fatalf("call jms_account_automations_list: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an unknown family was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "push") {
		t.Errorf("the refusal does not say what is accepted: %s", txt)
	}
}

// An empty family must be refused too -- not defaulted.
func TestAccountAutomationsRefusesAMissingFamily(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_account_automations_list",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call jms_account_automations_list: %v", err)
	}
	if !res.IsError {
		t.Fatalf("a missing family was accepted: %s", testsupport.ToolText(t, res))
	}
}

// An empty plan list must say so.
//
// "No plans" and "the answer is empty" are different claims, and a caller
// that reads an empty list as "there are none" is right -- but one that
// reads it as "the tool found nothing" would be wrong about why.
func TestAccountAutomationsListSaysSoWhenThereAreNone(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	out := testsupport.CallToolJSON(t, cs, "jms_account_automations_list",
		map[string]any{"family": "gather"})
	if n := len(out["items"].([]any)); n != 0 {
		t.Fatalf("items = %v, want none", out["items"])
	}
	note, _ := out["note"].(string)
	if !strings.Contains(note, "gather") {
		t.Errorf("the note does not say which family is empty: %v", out["note"])
	}
}

// Creating a plan must ask for a RANDOM secret.
//
// ⚠️ This is the whole point of the family: with `specific` the secret
// would have to pass through the caller -- the agent -- which is exactly
// what this capability exists to avoid. So the strategy is not a choice
// the caller makes; it is what the tool sends.
func TestAccountAutomationCreateAsksForARandomSecret(t *testing.T) {
	var seenBody map[string]any
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "a1", "name": "node-071", "address": "192.0.2.71",
				"platform": map[string]any{"id": 1, "name": "Linux"},
			}})
		case r.URL.Path == "/api/v1/accounts/push-account-automations/":
			json.NewDecoder(r.Body).Decode(&seenBody)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"id": "f12ebc80", "name": "selftest-push", "type": "push_account",
				"accounts":        []string{"opsuser"},
				"secret_type":     map[string]any{"value": "password", "label": "Password"},
				"secret_strategy": map[string]any{"value": "random", "label": "Randomly generate"},
				"password_rules": map[string]any{
					"length": 16, "lowercase": true, "uppercase": true,
					"digit": true, "symbol": true, "exclude_symbols": "",
				},
				"is_periodic": false, "is_active": true,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	out := testsupport.CallToolJSON(t, cs, "jms_account_automation_create", map[string]any{
		"family": "push", "name": "selftest-push",
		"accounts": []string{"opsuser"}, "assets": []string{"192.0.2.71"},
		"confirm": true,
	})

	if seenBody["secret_strategy"] != "random" {
		t.Errorf("secret_strategy = %v, want random", seenBody["secret_strategy"])
	}
	// The assets are named by address and go over the wire as IDS.
	assets, _ := seenBody["assets"].([]any)
	if len(assets) != 1 || assets[0] != "a1" {
		t.Errorf("assets = %v, want the resolved id", seenBody["assets"])
	}
	// A plan the agent triggers itself is NOT a periodic plan: a periodic
	// one would run on the bastion's own schedule, which nobody asked for.
	if seenBody["is_periodic"] != false {
		t.Errorf("is_periodic = %v, want false", seenBody["is_periodic"])
	}
	if out["name"] != "selftest-push" {
		t.Errorf("name = %v, want selftest-push", out["name"])
	}
}

// The password length must be one the bastion accepts.
//
// ⚠️ The upstream rule field is 8..36, and a length outside it is refused
// by the API with a message that says nothing about WHICH field was
// wrong.
func TestAccountAutomationCreateRefusesABadSecretLength(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	for _, bad := range []int{4, 40} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "jms_account_automation_create",
			Arguments: map[string]any{
				"family": "push", "name": "x", "accounts": []string{"opsuser"},
				"secret_length": bad, "confirm": true,
			},
		})
		if err != nil {
			t.Fatalf("call jms_account_automation_create: %v", err)
		}
		if !res.IsError {
			t.Errorf("a secret length of %d was accepted", bad)
		}
	}
}

// Triggering a plan must say that the work was ACCEPTED, not that it
// finished.
//
// ⚠️ The execution is asynchronous: the bastion starts a celery task and
// answers with its id. An answer that reads "done" would make the caller
// check the target machines for a password that is not there yet.
func TestAccountAutomationRunSaysTheWorkWasAccepted(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	out := testsupport.CallToolJSON(t, cs, "jms_account_automation_run", map[string]any{
		"family": "push", "plan": "selftest-push", "confirm": true,
	})

	if out["task"] != "e3710b47-aabe-4042-8331-8e8a416c70f2" {
		t.Errorf("task = %v, want the bastion's task id", out["task"])
	}
	if out["automation"] != "selftest-push" {
		t.Errorf("automation = %v, want selftest-push", out["automation"])
	}
	hint, _ := out["execution_hint"].(string)
	if !strings.Contains(hint, "background") {
		t.Errorf("the answer does not say the run is asynchronous: %v", out["execution_hint"])
	}
}

// Triggering a plan must go to the EXECUTION collection, not the plan
// one.
//
// The two are different collections with different verbs: posting to the
// plans collection would CREATE another plan instead of running this one.
func TestAccountAutomationRunPostsToTheExecutionCollection(t *testing.T) {
	var postedPath string
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.URL.Path == "/api/v1/accounts/push-account-automations/":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "f12ebc80", "name": "selftest-push", "type": "push_account"},
			})
		case r.URL.Path == "/api/v1/accounts/push-account-executions/":
			postedPath = r.URL.Path
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"task": "t1"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	testsupport.CallToolJSON(t, cs, "jms_account_automation_run", map[string]any{
		"family": "push", "plan": "f12ebc80", "confirm": true,
	})
	if postedPath != "/api/v1/accounts/push-account-executions/" {
		t.Errorf("the run was posted to %q, want the execution collection", postedPath)
	}
}

// A plan that does not exist must be refused, not run by accident.
//
// ⚠️ The name match is EXACT on purpose: a prefix match could trigger the
// wrong plan, and for a push that installs a password on the wrong
// machines. The fixture holds a second plan whose name CONTAINS this
// one's, so a substring match finds two and an exact match finds one.
func TestAccountAutomationRunRefusesAnUnknownPlan(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_account_automation_run",
		Arguments: map[string]any{
			"family": "push", "plan": "no-such-plan", "confirm": true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_account_automation_run: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an unknown plan was accepted: %s", testsupport.ToolText(t, res))
	}
}

// Two plans with the SAME name must be refused, not resolved to one of
// them.
//
// ⚠️ Picking the first would run a plan the caller did not name -- and for
// a push that installs a password on machines nobody asked about.
func TestAccountAutomationRunRefusesAnAmbiguousPlanName(t *testing.T) {
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.URL.Path == "/api/v1/accounts/push-account-automations/":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "p1", "name": "same-name", "type": "push_account"},
				{"id": "p2", "name": "same-name", "type": "push_account"},
			})
		case r.URL.Path == "/api/v1/accounts/push-account-executions/":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"task": "t1"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_account_automation_run",
		Arguments: map[string]any{
			"family": "push", "plan": "same-name", "confirm": true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_account_automation_run: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an ambiguous plan name was resolved instead of refused: %s",
			testsupport.ToolText(t, res))
	}
}

// A 403 must be reported as what it is.
//
// ⚠️ On this deployment the push and gather families are CORE features
// (measured: 200), so a 403 on them means the CREDENTIAL lacks the
// permission -- and saying "this needs a licence" would send the caller
// to buy something they do not need.
//
// (The licence-gated families upstream are change-secret and check-account,
// which this tool does not reach at all.)
func TestAccountAutomationsSayWhatA403Means(t *testing.T) {
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"detail": "You do not have permission to perform this action.",
			"code":   "permission_denied",
		})
	}))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_account_automations_list",
		Arguments: map[string]any{"family": "push"},
	})
	if err != nil {
		t.Fatalf("call jms_account_automations_list: %v", err)
	}
	if !res.IsError {
		t.Fatalf("a 403 was not reported as an error: %s", testsupport.ToolText(t, res))
	}
	txt := testsupport.ToolText(t, res)
	if !strings.Contains(txt, "lacks the permission") {
		t.Errorf("the refusal does not say the credential lacks the permission: %s", txt)
	}
	if strings.Contains(txt, "licence") || strings.Contains(txt, "license") {
		t.Errorf("the refusal blames a licence for a core feature: %s", txt)
	}
}

// The gather plan must NOT sync the discovered accounts unless asked.
//
// ⚠️ `is_sync_account` makes the bastion CREATE the accounts it discovers.
// That is a configuration change nobody asked for, and the upstream
// default is false (report only) -- so the tool must not turn it on by
// accident, and must not turn it on silently either.
func TestAccountAutomationCreateGatherDoesNotSyncByDefault(t *testing.T) {
	var seenBody map[string]any
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.URL.Path == "/api/v1/accounts/gather-account-automations/":
			json.NewDecoder(r.Body).Decode(&seenBody)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"id": "g1", "name": "selftest-gather", "type": "gather_accounts",
				"accounts": []string{"opsuser"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	testsupport.CallToolJSON(t, cs, "jms_account_automation_create", map[string]any{
		"family": "gather", "name": "selftest-gather",
		"accounts": []string{"opsuser"}, "confirm": true,
	})
	if v, ok := seenBody["is_sync_account"]; ok {
		t.Errorf("is_sync_account was sent as %v; the default must be the upstream one (report only)", v)
	}

	// Asked for explicitly, it goes over.
	testsupport.CallToolJSON(t, cs, "jms_account_automation_create", map[string]any{
		"family": "gather", "name": "selftest-gather",
		"accounts": []string{"opsuser"}, "sync_account": true, "confirm": true,
	})
	if seenBody["is_sync_account"] != true {
		t.Errorf("is_sync_account = %v, want true when asked for", seenBody["is_sync_account"])
	}
}

// The gather family has NO per-account result record.
//
// ⚠️ Upstream registers no "gather-account-records" collection -- asking
// for it answers 404 (measured on the live bastion). The gather family
// produces DISCOVERED ACCOUNTS instead, which jms_gathered_accounts
// reads.
//
// ⇒ The tool must say where the gather results ARE, instead of hitting a
// collection that does not exist.
func TestAccountAutomationRecordsRefusesGatherAndSaysWhereToLook(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_account_automation_records",
		Arguments: map[string]any{"family": "gather"},
	})
	if err != nil {
		t.Fatalf("call jms_account_automation_records: %v", err)
	}
	if !res.IsError {
		t.Fatalf("the gather family was answered with a records list: %s",
			testsupport.ToolText(t, res))
	}
	txt := testsupport.ToolText(t, res)
	if !strings.Contains(txt, "jms_gathered_accounts") {
		t.Errorf("the refusal does not say where the gather results are: %s", txt)
	}
}

// The gather plan must NOT take accounts, and must not send them.
//
// ⚠️ A gather logs in with the accounts the ASSETS already have and
// reports what it finds -- `accounts` on a gather plan is not "the
// accounts to look for". Upstream requires the field for push only.
//
// ⇒ Making it required for gather would make the caller name accounts
// that have nothing to do with what a gather does.
func TestAccountAutomationCreateGatherNeedsNoAccounts(t *testing.T) {
	var seenBody map[string]any
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.URL.Path == "/api/v1/accounts/gather-account-automations/":
			json.NewDecoder(r.Body).Decode(&seenBody)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"id": "g1", "name": "selftest-gather", "type": "gather_accounts",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// No accounts at all: it must work.
	testsupport.CallToolJSON(t, cs, "jms_account_automation_create", map[string]any{
		"family": "gather", "name": "selftest-gather", "confirm": true,
	})
	if _, ok := seenBody["accounts"]; ok {
		t.Errorf("accounts was sent for a gather plan: %v", seenBody["accounts"])
	}
}

// A push plan still needs the accounts it is going to create.
//
// ⚠️ Upstream marks the field required for push, and without it the
// bastion has nothing to create.
func TestAccountAutomationCreatePushStillNeedsAccounts(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_account_automation_create",
		Arguments: map[string]any{
			"family": "push", "name": "selftest-push", "confirm": true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_account_automation_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("a push plan without accounts was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "accounts") {
		t.Errorf("the refusal does not say what is missing: %s", txt)
	}
}

// The plan a name refers to must be THE plan, not the first one whose
// name contains it.
//
// ⚠️ The fixture holds "selftest-push" and "selftest-push-weekly". A
// substring match finds both; picking the wrong one would run the weekly
// plan (two other assets, a different account) instead of the one asked
// for.
func TestAccountAutomationRunPicksTheNamedPlanAndNotAnother(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	out := testsupport.CallToolJSON(t, cs, "jms_account_automation_run", map[string]any{
		"family": "push", "plan": "selftest-push", "confirm": true,
	})
	if out["automation"] != "selftest-push" {
		t.Errorf("automation = %v, want selftest-push", out["automation"])
	}
	if out["automation_id"] != "f12ebc80-58a4-466e-ab34-bcbd4ee1a837" {
		t.Errorf("automation_id = %v, want the id of selftest-push", out["automation_id"])
	}
}

// A failed execution must say that the REASON is not available.
//
// ⚠️ The upstream execution serializer exposes neither `result` nor
// `summary`, so a failed execution says only "failed". A caller that
// reads "failed" as "the bastion knows why and did not say" would go
// looking for the reason in the wrong place.
func TestAccountAutomationExecutionsSayTheReasonIsNotAvailable(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	out := testsupport.CallToolJSON(t, cs, "jms_account_automation_executions",
		map[string]any{"family": "push"})

	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v, want one row", items)
	}
	e := items[0].(map[string]any)
	if e["status"] != "failed" {
		t.Errorf("status = %v, want failed", e["status"])
	}
	if e["trigger"] != "manual" {
		t.Errorf("trigger = %v, want manual", e["trigger"])
	}
	if e["automation"] != "selftest-push" {
		t.Errorf("automation = %v, want selftest-push", e["automation"])
	}
	if e["automation_id"] != "f12ebc80-58a4-466e-ab34-bcbd4ee1a837" {
		t.Errorf("automation_id = %v, want the plan id", e["automation_id"])
	}
	if e["snapshot_asset_amount"] != float64(1) {
		t.Errorf("snapshot_asset_amount = %v, want 1", e["snapshot_asset_amount"])
	}
	// ⚠️ The count comes from the LENGTH of the accounts array: upstream
	// has no "account_amount" key, so a reader that looks for one reads
	// nothing at all.
	if e["snapshot_account_amount"] != float64(1) {
		t.Errorf("snapshot_account_amount = %v, want 1 (the length of the accounts array)", e["snapshot_account_amount"])
	}
	if e["snapshot_node_amount"] != float64(0) {
		t.Errorf("snapshot_node_amount = %v, want 0", e["snapshot_node_amount"])
	}
	note, _ := out["note"].(string)
	if !strings.Contains(note, "does not expose the reason") {
		t.Errorf("the note does not say the reason is unavailable: %v", out["note"])
	}
}

// A failed execution with NO per-account record must say so.
//
// ⚠️ The bastion creates one record per account as it starts on that
// account, so NO record at all means the run never reached a machine --
// which points at the bastion's own components (ansible/celery), not at
// the target. Without that, the caller reads "failed" and goes to check
// the machines, which is the wrong place to look.
//
// ⚠️ The fixture has an empty records collection, so this is the case
// that has to be distinguished from "there are records".
func TestAccountAutomationExecutionsSayWhenItFailedBeforeAnyMachine(t *testing.T) {
	// A bastion whose records are EMPTY -- the case that has to be told
	// apart from "there are records, go read them".
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/v1/accounts/push-account-automations/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/v1/accounts/push-account-executions/":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "e3710b47", "automation": map[string]any{
						"id": "f12ebc80", "name": "selftest-push",
					},
					"trigger":       map[string]any{"value": "manual", "label": "Manual trigger"},
					"date_start":    "2026/11/01 16:19:43 +0800",
					"date_finished": "2026/11/01 16:19:43 +0800",
					"snapshot": map[string]any{
						// ⚠️ The real shape: `accounts` is an ARRAY and there
						// is NO "account_amount" key. A fixture that invents
						// one would hide a reader that looks for it.
						"name":         "selftest-push",
						"accounts":     []string{"a5a54b34-5738-4a67-8b87-ad9bc6bbfd94"},
						"node_amount":  0,
						"asset_amount": 1,
					},
					"status":   map[string]any{"value": "failed", "label": "Failed"},
					"duration": "0.05",
				},
			})
		case "/api/v1/accounts/push-account-records/":
			json.NewEncoder(w).Encode([]map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	out := testsupport.CallToolJSON(t, cs, "jms_account_automation_executions",
		map[string]any{"family": "push"})

	note, _ := out["note"].(string)
	if !strings.Contains(note, "before it reached any machine") {
		t.Errorf("the note does not say the run failed before reaching a machine: %v", out["note"])
	}
}

// The per-account records must name the machine and the outcome.
//
// This is where the detail behind a failed execution lives.
func TestAccountAutomationRecordsNameTheMachineAndTheOutcome(t *testing.T) {
	cs := testSession(t, fakeBastionWithAutomation(t))
	out := testsupport.CallToolJSON(t, cs, "jms_account_automation_records",
		map[string]any{"family": "push"})

	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v, want one row", items)
	}
	r := items[0].(map[string]any)
	if r["asset"] != "node-071" {
		t.Errorf("asset = %v, want node-071", r["asset"])
	}
	if r["address"] != "192.0.2.71" {
		t.Errorf("address = %v, want 192.0.2.71", r["address"])
	}
	if r["account"] != "opsuser" {
		t.Errorf("account = %v, want opsuser", r["account"])
	}
	if r["is_success"] != false {
		t.Errorf("is_success = %v, want false", r["is_success"])
	}
	if r["error"] != "permission denied" {
		t.Errorf("error = %v, want permission denied", r["error"])
	}
}
