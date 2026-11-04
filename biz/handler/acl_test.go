// The ACL write-face tests. The fixtures are the REAL probe bytes taken
// from the v5 machine (created is_active:false, read back, deleted): the
// label pairs ({\"value\":\"reject\",\"label\":\"Reject\"}), the selector
// shape ({\"type\":\"all\"}) and the operation pairs are the wire's own,
// not shapes this test invented -- an invented shape would pass while the
// real machine says something else.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yi-nology/jumpserver-kit/pkg/testsupport"
)

// Real POST/GET bytes from the v5 machine, one per family.
const (
	probeLoginACL = `{"rules":{"ip_group":["192.0.2.1"],"time_period":[]},"id":"5f26cb06-7e0c-4025-acfd-41697e93e2ed","priority":99,"action":{"value":"reject","label":"Reject"},"reviewers":[],"name":"mcp-shape-probe-loginacl","users":{"type":"all"},"is_active":false,"date_created":"2026/11/01 21:42:01 +0800","date_updated":"2026/11/01 21:42:01 +0800","created_by":"Administrator","comment":"shape probe, safe to delete"}`

	probeMasking = `{"id":"1795ec23-9b08-420c-a485-6a143545fc7e","name":"mcp-shape-probe-masking","priority":99,"action":{"value":"reject","label":"Reject"},"reviewers":[],"users":{"type":"all"},"assets":{"type":"all"},"accounts":["*"],"org_id":"00000000-0000-0000-0000-000000000002","fields_pattern":"password","masking_method":{"value":"fixed_char","label":"Fixed character replacement"},"mask_pattern":"######","org_name":"DEFAULT","is_active":false,"date_created":"2026/11/01 21:42:01 +0800","date_updated":"2026/11/01 21:42:01 +0800","comment":"shape probe, safe to delete","created_by":"Administrator"}`

	probeClipboard = `{"id":"ce9b4376-4acd-4b4d-94c6-cbeef095f084","name":"mcp-shape-probe-clipboard","priority":99,"action":{"value":"reject","label":"Reject"},"reviewers":[],"users":{"type":"all"},"assets":{"type":"all"},"accounts":["*"],"org_id":"00000000-0000-0000-0000-000000000002","operations":[{"value":"copy","label":"Copy (RDP, VNC)"},{"value":"paste","label":"Paste (RDP, VNC)"}],"copy_text_limit":1024,"paste_text_limit":1024,"download_file_size_limit":0,"upload_file_size_limit":0,"org_name":"DEFAULT","is_active":false,"date_created":"2026/11/01 21:42:01 +0800","date_updated":"2026/11/01 21:42:01 +0800","comment":"shape probe, safe to delete","created_by":"Administrator"}`
)

// aclReq is one request the fake bastion saw, for wire assertions.
type aclReq struct {
	method string
	path   string
	body   map[string]any
}

// fakeBastionWithACL serves the three ACL families and records every
// request. A refused validation must leave the log EMPTY -- the whole
// point of checking before the request goes out is that it does not go
// out.
type fakeBastionWithACL struct {
	mu       sync.Mutex
	reqs     []aclReq
	rows     map[string][]string // family path -> JSON rows
	capacity map[string]int      // family path -> rows for create to echo
}

func (f *fakeBastionWithACL) log(r *http.Request, body map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, aclReq{method: r.Method, path: r.URL.Path, body: body})
}

func (f *fakeBastionWithACL) snapshot() []aclReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]aclReq{}, f.reqs...)
}

func (f *fakeBastionWithACL) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body map[string]any
	if r.Method == http.MethodPost || r.Method == http.MethodPatch {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	f.log(r, body)

	p := strings.TrimSuffix(r.URL.Path, "/")
	// /api/v1/acls/<family>[/<id>]
	var family, id string
	for _, fam := range []string{"login-acls", "data-masking-rules", "clipboard-acls"} {
		prefix := "/api/v1/acls/" + fam
		if p == prefix {
			family = fam
		} else if strings.HasPrefix(p, prefix+"/") {
			family = fam
			id = strings.TrimPrefix(p, prefix+"/")
		}
	}
	if family == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	rows := f.rows[family]

	switch r.Method {
	case http.MethodGet:
		if id != "" {
			// The single row, by id -- the delete-before read.
			for _, row := range rows {
				var m map[string]any
				_ = json.Unmarshal([]byte(row), &m)
				if m["id"] == id {
					w.Write([]byte(row))
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		joined := strings.Join(rows, ",")
		w.Write([]byte(`{"count":` + itoaTest(len(rows)) +
			`,"next":null,"previous":null,"results":[` + joined + `]}`))
	case http.MethodPost:
		w.WriteHeader(http.StatusCreated)
		if len(rows) > 0 {
			w.Write([]byte(rows[0]))
		}
	case http.MethodPatch:
		if len(rows) > 0 {
			w.Write([]byte(rows[0]))
		}
	case http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func fakeACL(t *testing.T) *fakeBastionWithACL {
	t.Helper()
	return &fakeBastionWithACL{rows: map[string][]string{
		"login-acls":         {probeLoginACL},
		"data-masking-rules": {probeMasking},
		"clipboard-acls":     {probeClipboard},
	}}
}

// A refused validation must not put anything on the wire.
func assertNoRequests(t *testing.T, f *fakeBastionWithACL) {
	t.Helper()
	if got := f.snapshot(); len(got) != 0 {
		t.Errorf("the refusal still sent %d request(s): %v", len(got), got)
	}
}

// The create body must carry the SELECTOR shape, not a bare list: the
// server answers 400 to ["*"] and its error names no field.
func TestLoginACLCreateSendsTheSelectorWire(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	out := testsupport.CallToolJSON(t, cs, "jms_login_acl_create", map[string]any{
		"name":    "probe",
		"action":  "ACCEPT", // normalisation: trimmed + lowercased
		"users":   map[string]any{"type": "ids", "ids": []any{"u-1", "u-2"}},
		"rules":   map[string]any{"ip_group": []any{"10.0.0.0/8"}},
		"confirm": true,
	})

	reqs := fake.snapshot()
	if len(reqs) != 1 {
		t.Fatalf("requests = %v, want exactly the one POST", reqs)
	}
	body := reqs[0].body
	if body["action"] != "accept" {
		t.Errorf("action = %v, want accept (normalised)", body["action"])
	}
	users, ok := body["users"].(map[string]any)
	if !ok {
		t.Fatalf("users = %T, want the selector object (a bare list is a 400 on the real machine)", body["users"])
	}
	if users["type"] != "ids" {
		t.Errorf("users.type = %v, want ids", users["type"])
	}
	if ids, ok := users["ids"].([]any); !ok || len(ids) != 2 {
		t.Errorf("users.ids = %v, want the two ids", users["ids"])
	}

	// The reply flattens the LABEL PAIR back to the value, because the
	// value is what round-trips into an update.
	if out["action"] != "reject" {
		t.Errorf("action = %v, want reject (label pair flattened)", out["action"])
	}
	if out["name"] != "mcp-shape-probe-loginacl" {
		t.Errorf("name = %v, want the created row's name", out["name"])
	}
}

// ip_group must be stated explicitly: defaulting it to '*' would match
// every IP, and that default is exactly what this refusal prevents.
func TestLoginACLCreateRefusesAMissingIPGroup(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_login_acl_create",
		Arguments: map[string]any{
			"name":    "probe",
			"action":  "reject",
			"rules":   map[string]any{"ip_group": []any{}},
			"confirm": true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_login_acl_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an empty ip_group was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "ip_group") {
		t.Errorf("the refusal does not name the missing field: %s", txt)
	}
	assertNoRequests(t, fake)
}

// The action vocabulary is the serializer's own exclusion list; a value
// outside it must be refused here where the error can list what IS valid.
func TestLoginACLCreateRefusesAnUnknownAction(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_login_acl_create",
		Arguments: map[string]any{
			"name":    "probe",
			"action":  "sideways",
			"rules":   map[string]any{"ip_group": []any{"*"}},
			"confirm": true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_login_acl_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an unknown action was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "notice") {
		t.Errorf("the refusal does not list the accepted actions: %s", txt)
	}
	assertNoRequests(t, fake)
}

// time_period ids are WEEKDAYS (0=Sunday … 6=Saturday). An out-of-range
// id is refused with that key stated -- the server's 400 names no field.
func TestLoginACLCreateRefusesABadWeekday(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_login_acl_create",
		Arguments: map[string]any{
			"name":   "probe",
			"action": "accept",
			"rules": map[string]any{
				"ip_group": []any{"*"},
				"time_period": []any{
					map[string]any{"id": 7, "value": "09:00~18:00"},
				},
			},
			"confirm": true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_login_acl_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("time_period id 7 was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "weekday") {
		t.Errorf("the refusal does not say the id is a weekday: %s", txt)
	}
	assertNoRequests(t, fake)
}

// A selector type outside all/ids/attrs is refused naming the FIELD.
func TestLoginACLCreateRefusesABadSelector(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_login_acl_create",
		Arguments: map[string]any{
			"name":    "probe",
			"action":  "reject",
			"users":   map[string]any{"type": "everyone"},
			"rules":   map[string]any{"ip_group": []any{"*"}},
			"confirm": true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_login_acl_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("selector type 'everyone' was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "users") {
		t.Errorf("the refusal does not name the field: %s", txt)
	}
	assertNoRequests(t, fake)
}

// Update by NAME: the lookup walks the list, then PATCHes the id. Only
// the fields given may appear in the body -- a PATCH that re-sends the
// whole record would overwrite what the caller left out.
func TestLoginACLUpdateByNAMEPatchesOnlyWhatWasGiven(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	testsupport.CallToolJSON(t, cs, "jms_login_acl_update", map[string]any{
		"rule":      "mcp-shape-probe-loginacl",
		"is_active": true,
		"confirm":   true,
	})

	reqs := fake.snapshot()
	if len(reqs) != 2 {
		t.Fatalf("requests = %v, want list-then-patch", reqs)
	}
	if reqs[0].method != http.MethodGet || reqs[1].method != http.MethodPatch {
		t.Fatalf("request order = %v, want GET (lookup) then PATCH", reqs)
	}
	if !strings.HasSuffix(reqs[1].path, "/5f26cb06-7e0c-4025-acfd-41697e93e2ed/") {
		t.Errorf("PATCH path = %s, want the resolved id", reqs[1].path)
	}
	if _, leaked := reqs[1].body["action"]; leaked {
		t.Errorf("PATCH body carries action although none was given: %v", reqs[1].body)
	}
	if _, leaked := reqs[1].body["rules"]; leaked {
		t.Errorf("PATCH body carries rules although none was given: %v", reqs[1].body)
	}
	if reqs[1].body["is_active"] != true {
		t.Errorf("PATCH body is_active = %v, want true", reqs[1].body["is_active"])
	}
}

// An explicit empty comment CLEARS the note. A bare `string` collapsed
// "absent" and "empty" (the `!= ""` gate read "" as "not given"), so the note
// could never be emptied — the three-state split (absent / empty / set) that
// a pointer restores. This test holds the CLEAR direction shut.
func TestLoginACLUpdateClearsTheCommentOnAnEmptyString(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	testsupport.CallToolJSON(t, cs, "jms_login_acl_update", map[string]any{
		"rule":    "mcp-shape-probe-loginacl",
		"comment": "",
		"confirm": true,
	})

	reqs := fake.snapshot()
	if len(reqs) != 2 {
		t.Fatalf("requests = %v, want list-then-patch", reqs)
	}
	// The key must be PRESENT with an empty value — that is what "clear it"
	// means. Omitting the key would leave the old note in place.
	v, present := reqs[1].body["comment"]
	if !present {
		t.Fatalf("PATCH body omits comment: %v — an empty string must CLEAR it, not vanish", reqs[1].body)
	}
	if v != "" {
		t.Errorf("PATCH body comment = %v, want an empty string (the clear)", v)
	}
}

// Omitting comment must leave it alone (the "absent" side of the three-state
// split) — the pointer is nil, so nothing goes on the wire.
func TestLoginACLUpdateOmitsTheCommentWhenNotGiven(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	testsupport.CallToolJSON(t, cs, "jms_login_acl_update", map[string]any{
		"rule":      "mcp-shape-probe-loginacl",
		"is_active": true,
		"confirm":   true,
	})
	reqs := fake.snapshot()
	if _, present := reqs[1].body["comment"]; present {
		t.Errorf("PATCH body carries comment although none was given: %v", reqs[1].body)
	}
}

// The same CLEAR direction for the masking rule's clearable fields
// (comment + mask_pattern + fields_pattern): an explicit empty string must
// reach the wire as a present-but-empty key. Symmetric with the login guard
// above — a `!= ""` gate on any of these would make the field unemptyable.
func TestDataMaskingRuleUpdateClearsItsFieldsOnAnEmptyString(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	testsupport.CallToolJSON(t, cs, "jms_data_masking_rule_update", map[string]any{
		"rule":           "mcp-shape-probe-masking",
		"comment":        "",
		"mask_pattern":   "",
		"fields_pattern": "",
		"confirm":        true,
	})

	reqs := fake.snapshot()
	if len(reqs) != 2 {
		t.Fatalf("requests = %v, want list-then-patch", reqs)
	}
	for _, key := range []string{"comment", "mask_pattern", "fields_pattern"} {
		v, present := reqs[1].body[key]
		if !present {
			t.Errorf("PATCH body omits %s: %v — an empty string must CLEAR it, not vanish", key, reqs[1].body)
			continue
		}
		if v != "" {
			t.Errorf("PATCH body %s = %v, want an empty string (the clear)", key, v)
		}
	}
}

// The same CLEAR direction for the clipboard rule's comment.
func TestClipboardACLUpdateClearsTheCommentOnAnEmptyString(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	testsupport.CallToolJSON(t, cs, "jms_clipboard_acl_update", map[string]any{
		"rule":    "mcp-shape-probe-clipboard",
		"comment": "",
		"confirm": true,
	})

	reqs := fake.snapshot()
	if len(reqs) != 2 {
		t.Fatalf("requests = %v, want list-then-patch", reqs)
	}
	v, present := reqs[1].body["comment"]
	if !present {
		t.Fatalf("PATCH body omits comment: %v — an empty string must CLEAR it, not vanish", reqs[1].body)
	}
	if v != "" {
		t.Errorf("PATCH body comment = %v, want an empty string (the clear)", v)
	}
}

// An update that changes nothing must say so instead of PATCHing an
// empty body (which the server reads as "change nothing" or rejects).
func TestLoginACLUpdateRefusesNothingToChange(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_login_acl_update",
		Arguments: map[string]any{"rule": "5f26cb06-7e0c-4025-acfd-41697e93e2ed", "confirm": true},
	})
	if err != nil {
		t.Fatalf("call jms_login_acl_update: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an empty update was accepted: %s", testsupport.ToolText(t, res))
	}
	assertNoRequests(t, fake)
}

// An unknown rule name must list what IS available -- the agent then
// retries with the right name instead of guessing again.
func TestLoginACLUpdateSpellsOutTheAvailableRules(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_login_acl_update",
		Arguments: map[string]any{"rule": "no-such-rule", "comment": "x", "confirm": true},
	})
	if err != nil {
		t.Fatalf("call jms_login_acl_update: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an unknown rule was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "mcp-shape-probe-loginacl") {
		t.Errorf("the refusal does not list the available rules: %s", txt)
	}
}

// Delete reads BEFORE deleting: the answer has to say WHAT was deleted.
func TestLoginACLDeleteReadsBeforeItDeletes(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	out := testsupport.CallToolJSON(t, cs, "jms_login_acl_delete", map[string]any{
		"rule":    "5f26cb06-7e0c-4025-acfd-41697e93e2ed",
		"confirm": true,
	})

	reqs := fake.snapshot()
	if len(reqs) != 2 {
		t.Fatalf("requests = %v, want read-then-delete", reqs)
	}
	if reqs[0].method != http.MethodGet || reqs[1].method != http.MethodDelete {
		t.Fatalf("request order = %v, want GET (the before-read) then DELETE", reqs)
	}
	// The answer is the deleted row, flattened: action label pair -> value.
	if out["action"] != "reject" {
		t.Errorf("action = %v, want reject", out["action"])
	}
	if out["id"] != "5f26cb06-7e0c-4025-acfd-41697e93e2ed" {
		t.Errorf("id = %v, want the deleted rule's id", out["id"])
	}
}

// When the before-read FAILS (the row cannot be fetched right before the
// delete), the answer must still name the id that was deleted — not an
// all-nil rule that reads as "an empty rule was deleted". The delete itself
// goes through, so the only certain fact is the id; that is what the answer
// carries.
func TestLoginACLDeleteAnswersTheIDWhenTheBeforeReadFails(t *testing.T) {
	// The underlying fake answers GET-by-id 404 (nothing to read back) but
	// DELETE 204 (it really is deleted).
	base := fakeACL(t)
	fake := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"detail":"gone"}`))
			return
		}
		base.ServeHTTP(w, r)
	})
	cs := testSession(t, fake)
	out := testsupport.CallToolJSON(t, cs, "jms_login_acl_delete", map[string]any{
		"rule":    "5f26cb06-7e0c-4025-acfd-41697e93e2ed",
		"confirm": true,
	})

	if out["id"] != "5f26cb06-7e0c-4025-acfd-41697e93e2ed" {
		t.Errorf("id = %v, want the deleted rule's id even when the before-read failed", out["id"])
	}
	// It must not look like a populated rule either: nothing else is known.
	if _, present := out["action"]; present {
		t.Errorf("action = %v, want absent: the row could not be read back", out["action"])
	}
}

// The masking reply carries masking_method as a LABEL PAIR on the wire;
// the contract flattens it to the value ("fixed_char") because the value
// is what an update accepts back.
func TestDataMaskingRuleFlattensTheLabelPairs(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	out := testsupport.CallToolJSON(t, cs, "jms_data_masking_rules", map[string]any{})

	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %d, want the one probe rule", len(items))
	}
	first := items[0].(map[string]any)
	if first["masking_method"] != "fixed_char" {
		t.Errorf("masking_method = %v, want fixed_char (label pair flattened)", first["masking_method"])
	}
	if first["action"] != "reject" {
		t.Errorf("action = %v, want reject", first["action"])
	}
	// accounts comes back as a plain string list.
	accounts, ok := first["accounts"].([]any)
	if !ok || len(accounts) != 1 || accounts[0] != "*" {
		t.Errorf("accounts = %v, want [\"*\"]", first["accounts"])
	}
}

// masking_method has its own vocabulary; a wrong value is refused with
// the vocabulary listed.
func TestDataMaskingRuleCreateRefusesAnUnknownMethod(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_data_masking_rule_create",
		Arguments: map[string]any{
			"name":           "probe",
			"action":         "reject",
			"accounts":       []any{"*"},
			"fields_pattern": "password",
			"masking_method": "blur",
			"confirm":        true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_data_masking_rule_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("masking_method 'blur' was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "keep_suffix") {
		t.Errorf("the refusal does not list the accepted methods: %s", txt)
	}
	assertNoRequests(t, fake)
}

// fields_pattern names the field to mask; without it the rule matches
// nothing, so it must be required rather than defaulted.
func TestDataMaskingRuleCreateRequiresAFieldsPattern(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_data_masking_rule_create",
		Arguments: map[string]any{
			"name":           "probe",
			"action":         "reject",
			"accounts":       []any{"*"},
			"masking_method": "fixed_char",
			"confirm":        true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_data_masking_rule_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("a rule with no fields_pattern was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "fields_pattern") {
		t.Errorf("the refusal does not name the missing field: %s", txt)
	}
	assertNoRequests(t, fake)
}

// The clipboard family takes ONLY reject/accept -- "review" is not in
// its serializer's vocabulary even though login/masking accept it.
func TestClipboardACLCreateRefusesReview(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_clipboard_acl_create",
		Arguments: map[string]any{
			"name":       "probe",
			"action":     "review",
			"accounts":   []any{"*"},
			"operations": []any{"copy"},
			"confirm":    true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_clipboard_acl_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("clipboard action 'review' was accepted: %s", testsupport.ToolText(t, res))
	}
	assertNoRequests(t, fake)
}

// operations are copy and/or paste. File-size ceilings are the
// *_file_size_limit fields, NOT operations -- "upload" must be refused.
func TestClipboardACLCreateRefusesAnUnknownOperation(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_clipboard_acl_create",
		Arguments: map[string]any{
			"name":       "probe",
			"action":     "accept",
			"accounts":   []any{"*"},
			"operations": []any{"upload"},
			"confirm":    true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_clipboard_acl_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("operation 'upload' was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "copy") {
		t.Errorf("the refusal does not list the accepted operations: %s", txt)
	}
	assertNoRequests(t, fake)
}

// The clipboard reply keeps the operation PAIR (value + label) -- the
// label is the human side of the fact and the value round-trips.
func TestClipboardACLReplyKeepsTheOperationValue(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	out := testsupport.CallToolJSON(t, cs, "jms_clipboard_acls", map[string]any{})

	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %d, want the one probe rule", len(items))
	}
	ops, ok := items[0].(map[string]any)["operations"].([]any)
	if !ok || len(ops) != 2 {
		t.Fatalf("operations = %v, want copy + paste", items[0].(map[string]any)["operations"])
	}
	first := ops[0].(map[string]any)
	if first["value"] != "copy" {
		t.Errorf("operations[0].value = %v, want copy", first["value"])
	}
}

// A selector that SAYS ids but carries none is refused naming the field:
// the server would read it as "matches nothing", which is never what the
// caller meant.
func TestLoginACLCreateRefusesEmptySelectorIDs(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_login_acl_create",
		Arguments: map[string]any{
			"name":    "probe",
			"action":  "reject",
			"users":   map[string]any{"type": "ids", "ids": []any{}},
			"rules":   map[string]any{"ip_group": []any{"*"}},
			"confirm": true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_login_acl_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("type=ids with no ids was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "users") {
		t.Errorf("the refusal does not name the field: %s", txt)
	}
	assertNoRequests(t, fake)
}

// Same for attrs: type=attrs with no attrs, and an attr without value.
func TestLoginACLCreateRefusesIncompleteAttrs(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jms_login_acl_create",
		Arguments: map[string]any{
			"name":   "probe",
			"action": "reject",
			"users": map[string]any{"type": "attrs", "attrs": []any{
				map[string]any{"name": "ip", "match": "exact"},
			}},
			"rules":   map[string]any{"ip_group": []any{"*"}},
			"confirm": true,
		},
	})
	if err != nil {
		t.Fatalf("call jms_login_acl_create: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an attr without value was accepted: %s", testsupport.ToolText(t, res))
	}
	assertNoRequests(t, fake)
}

// accounts and operations must be stated: an empty list matches nothing,
// and "nothing" is never what the caller meant to govern.
func TestClipboardACLCreateRequiresItsLists(t *testing.T) {
	fake := fakeACL(t)
	cs := testSession(t, fake)
	for _, args := range []map[string]any{
		{"name": "p", "action": "accept", "accounts": []any{}, "operations": []any{"copy"}, "confirm": true},
		{"name": "p", "action": "accept", "accounts": []any{"*"}, "operations": []any{}, "confirm": true},
	} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "jms_clipboard_acl_create", Arguments: args,
		})
		if err != nil {
			t.Fatalf("call jms_clipboard_acl_create: %v", err)
		}
		if !res.IsError {
			t.Fatalf("an empty required list was accepted: %s", testsupport.ToolText(t, res))
		}
	}
	assertNoRequests(t, fake)
}
