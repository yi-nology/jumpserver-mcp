package handler

// truth_test.go guards one thing: the answers follow the LIVE API's facts,
// not its labels. Every fixture here is a shape MEASURED from the real
// bastion on 2026-11-03, and every fake endpoint behaves the way the real
// endpoint behaves — a fake that agrees with the code is a test that agrees
// with nothing.
//
// What was measured (v5, 2026-11-03):
//
//  1. GET /accounts/gathered-accounts/?asset=<id> IGNORES the `asset`
//     parameter and returns the whole org's rows. Only `asset_id=` filters.
//     A tool that filters by `asset=` therefore reads OTHER assets' rows —
//     and jms_gather_apply then PUTs those foreign row ids to
//     /accounts/gathered-accounts/status/, confirming someone else's
//     gather on someone else's asset.
//  2. A job execution against an unreachable host reported
//     is_success=true with recap {"ok":[],"dark":{},"skipped":[],
//     "failures":{}} in one run and is_success=false in the next — the same
//     fact, two different labels. The recap is the only hard evidence:
//     nothing ran anywhere.
//  3. The gather adhoc report for an unreachable host says
//     "Asset Success Count 0 Asset Failure Count 1" while the tool answered
//     "the run completed but reported no new accounts".

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mcp/internal/repository"
	"github.com/yi-nology/jumpserver-mcp/internal/service/policy"
)

// gatherRow is one gathered-account record, in the shape the live API
// returns (nested asset object, status as a {value,label} pair).
type gatherRow struct {
	id        string
	username  string
	assetID   string
	confirmed bool
}

// truthFake serves the endpoints these guards touch, in the measured shape.
type truthFake struct {
	rows    []gatherRow
	putIDs  []string // recorder: every row id PUT to /status/
	exec    map[string]any
	execSet bool
	list    []map[string]any
	report  string // the adhoc gather report, as HTML
}

func (f *truthFake) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path, method := r.URL.Path, r.Method
		switch {
		case path == "/api/v1/assets/assets/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "results": []map[string]any{
				{"id": "TARGET", "name": "zz-probe", "address": "192.0.2.1",
					"platform":  map[string]any{"id": "P1", "name": "Linux"},
					"protocols": []map[string]any{{"name": "ssh", "port": 22}},
					"is_active": true},
			}})
		case path == "/api/v1/accounts/gathered-accounts/" && method == "GET":
			// LIKE THE REAL API (measured 2026-11-03): only asset_id
			// filters; the `asset` parameter is ignored and everything
			// comes back. This is the whole point of the guard — a fake
			// that filtered on `asset=` would agree with the buggy code.
			want := r.URL.Query().Get("asset_id")
			out := []map[string]any{}
			for _, row := range f.rows {
				if want != "" && row.assetID != want {
					continue
				}
				status := map[string]any{"value": "0", "label": "Pending"}
				if row.confirmed {
					status = map[string]any{"value": "1", "label": "Confirm"}
				}
				out = append(out, map[string]any{
					"id":       row.id,
					"username": row.username,
					"asset":    map[string]any{"id": row.assetID, "name": "some-asset"},
					"status":   status,
				})
			}
			// LIKE THE REAL v5 (measured 2026-10-15/11-03): a BARE ARRAY,
			// not the {count, results} envelope v4 uses. The protocol
			// layer reads both — but a fake in the wrong shape is a test
			// measuring the wrong thing.
			json.NewEncoder(w).Encode(out)
		case path == "/api/v1/accounts/gathered-accounts/status/" && method == "PUT":
			var body struct {
				IDs    []string `json:"ids"`
				Status string   `json:"status"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.putIDs = append(f.putIDs, body.IDs...)
			if body.Status == "1" {
				for i := range f.rows {
					for _, id := range body.IDs {
						if f.rows[i].id == id {
							f.rows[i].confirmed = true
						}
					}
				}
			}
			// The real endpoint answers 204 with an empty body.
			w.WriteHeader(http.StatusNoContent)
		case path == "/api/v1/accounts/gather-account-executions/adhoc/" && method == "GET":
			w.Header().Set("Content-Type", "text/html")
			report := f.report
			if report == "" {
				report = "<html><body>Basic Information Task Name Adhoc gather accounts: zz-probe " +
					"Asset Count 1 Asset Success Count 1 Asset Failure Count 0 " +
					"Newly discovered accounts: 1 New account found</body></html>"
			}
			w.Write([]byte(report))
		case path == "/api/v1/ops/jobs/" && method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"id": "J1", "task_id": "T1"})
		case path == "/api/v1/ops/job-executions/T1/" && method == "GET":
			if !f.execSet {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(f.exec)
		case path == "/api/v1/ops/job-executions/" && method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"count": len(f.list), "results": f.list})
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"detail": "no such endpoint: " + method + " " + path})
		}
	})
}

// truthSession drives the gather and job tools against the fake.
func truthSession(t *testing.T, f *truthFake) *mcp.ClientSession {
	t.Helper()
	ts := httptest.NewServer(f.handler())
	t.Cleanup(ts.Close)
	c := repository.NewBase(ts.URL, "test-token", ts.Client())
	return testsupport.NewSession(t, "jms-truth-test", reg.Options{Policy: policy.Table},
		func(r *reg.Registry) {
			RegisterGatherTools(r, c)
			RegisterJobTools(r, c)
		})
}

// callRaw returns (isError, json view, raw text) so a test can assert on
// honest failures too — CallToolJSON refuses error answers, and "the failure
// is visible" is one of the things under test here.
func callRaw(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (bool, map[string]any, string) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	txt := testsupport.ToolText(t, res)
	out := map[string]any{}
	_ = json.Unmarshal([]byte(txt), &out)
	return res.IsError, out, txt
}

// The rows of one gathered-accounts fixture: the target asset has a
// duplicated pending row (each gather run appends one), and ANOTHER asset's
// rows sit right next to them in the org-wide list.
func foreignAndLocalRows() []gatherRow {
	return []gatherRow{
		{id: "T-1", username: "probeuser", assetID: "TARGET", confirmed: false},
		{id: "T-2", username: "probeuser", assetID: "TARGET", confirmed: false},
		{id: "X-1", username: "root", assetID: "OTHER", confirmed: true},
		{id: "X-2", username: "opsuser", assetID: "OTHER", confirmed: true},
		{id: "X-3", username: "foreignpending", assetID: "OTHER", confirmed: false},
	}
}

// jms_gather_apply must act on ONE asset's rows — never another asset's.
//
// Measured 2026-11-03: the tool listed DGX_spark's and node-071's confirmed
// accounts as "already adopted on this asset" for a brand-new asset, and
// would have PUT another asset's pending row ids into the status endpoint
// — a write on records this call was never asked about.
// The LIST tool had its own copy of the query and kept the trap G1 caught:
// `asset=` is silently ignored by the live API, so the listing answered with
// the whole org's rows while its schema promised "only accounts gathered
// from this asset". The fix is two-layer — the parameter is asset_id, AND
// every row naming another asset is dropped here — so this guard fails
// whether the trap is in the parameter name or in a server that ignores it.
func TestGatheredListIsScopedToOneAsset(t *testing.T) {
	f := &truthFake{
		rows: []gatherRow{
			{id: "T-1", username: "mine", assetID: "TARGET", confirmed: false},
			{id: "X-1", username: "theirs", assetID: "OTHER", confirmed: false},
			{id: "X-2", username: "opsuser", assetID: "OTHER", confirmed: true},
		},
	}
	cs := truthSession(t, f)
	isErr, out, txt := callRaw(t, cs, "jms_gathered_accounts",
		map[string]any{"asset": "zz-probe"})
	if isErr {
		t.Fatalf("the listing answers normally: %s", txt)
	}
	items, _ := out["items"].([]any)
	ids := []string{}
	for _, it := range items {
		row := it.(map[string]any)
		ids = append(ids, fmt.Sprint(row["id"]))
		if fmt.Sprint(row["id"]) != "T-1" {
			t.Errorf("the listing answered with row %v from ANOTHER asset (asset= is ignored by the live API):\n%s", row, txt)
		}
	}
	if len(items) != 1 || ids[0] != "T-1" {
		t.Errorf("items = %v, want exactly [T-1] (this asset's one row)", ids)
	}
}

func TestGatherApplyIsScopedToOneAsset(t *testing.T) {
	f := &truthFake{rows: foreignAndLocalRows()}
	cs := truthSession(t, f)

	isErr, out, txt := callRaw(t, cs, "jms_gather_apply",
		map[string]any{"asset": "zz-probe", "confirm": true})
	if isErr {
		t.Fatalf("gathering the target asset's own pending rows must work: %s", txt)
	}
	for _, foreign := range []string{"root", "opsuser", "foreignpending"} {
		if strings.Contains(txt, foreign) {
			t.Errorf("the answer names %q, which belongs to ANOTHER asset's gather rows:\n%s", foreign, txt)
		}
	}
	if len(f.putIDs) == 0 {
		t.Fatalf("nothing was adopted, but the target asset has pending rows")
	}
	allowed := map[string]bool{"T-1": true, "T-2": true}
	for _, id := range f.putIDs {
		if !allowed[id] {
			t.Errorf("PUT /gathered-accounts/status/ carried row id %q, which belongs to ANOTHER asset — that confirms a foreign gather", id)
		}
	}
	adopted, _ := out["adopted"].([]any)
	if len(adopted) == 0 {
		t.Errorf("adopted = %v, want the target asset's pending account", out["adopted"])
	}
}

// The "already confirmed" list says which accounts are ALREADY adopted on
// this asset — one entry per account, decided per account, whatever order
// the rows come in. A duplicated row per account is normal (each gather run
// appends one) and must not become a duplicated claim; a pending row for an
// account whose other row is already confirmed must not be reported as
// freshly adopted.
func TestGatherApplyPartitionsPerAccount(t *testing.T) {
	f := &truthFake{rows: []gatherRow{
		{id: "T-1", username: "dupuser", assetID: "TARGET", confirmed: true},
		{id: "T-2", username: "dupuser", assetID: "TARGET", confirmed: true},
	}}
	cs := truthSession(t, f)
	isErr, out, txt := callRaw(t, cs, "jms_gather_apply",
		map[string]any{"asset": "zz-probe", "confirm": true})
	if isErr {
		t.Fatalf("an already-adopted account must answer, not fail: %s", txt)
	}
	already, _ := out["already_confirmed"].([]any)
	if len(already) != 1 {
		t.Errorf("already_confirmed = %v, want exactly one entry for one account", out["already_confirmed"])
	}

	// pending row FIRST, confirmed row second: the account is already
	// adopted, so nothing may be claimed as newly adopted.
	f2 := &truthFake{rows: []gatherRow{
		{id: "T-3", username: "dupuser", assetID: "TARGET", confirmed: false},
		{id: "T-4", username: "dupuser", assetID: "TARGET", confirmed: true},
	}}
	cs2 := truthSession(t, f2)
	isErr, out2, txt2 := callRaw(t, cs2, "jms_gather_apply",
		map[string]any{"asset": "zz-probe", "confirm": true})
	if isErr {
		t.Fatalf("an already-adopted account must answer, not fail: %s", txt2)
	}
	if adopted, ok := out2["adopted"].([]any); ok && len(adopted) > 0 {
		t.Errorf("adopted = %v, but the account was already adopted before this call", out2["adopted"])
	}
	already2, _ := out2["already_confirmed"].([]any)
	if len(already2) != 1 {
		t.Errorf("already_confirmed = %v, want exactly one entry for one account", out2["already_confirmed"])
	}
}

// jms_gather_run must say the gather FAILED when the report says it failed.
//
// Measured 2026-11-03: the report said "Asset Success Count 0 Asset Failure
// Count 1" (unreachable host) and the tool answered "the run completed but
// reported no new accounts" — a failure painted as an empty success. It must
// also count only THIS asset's gathered rows (the fixture seeds another
// asset's rows on purpose).
func TestGatherRunSaysTheFailureItSaw(t *testing.T) {
	f := &truthFake{
		// The measured live case: the target asset has NO gather rows of
		// its own — only ANOTHER asset's rows sit in the org-wide list.
		rows: []gatherRow{
			{id: "X-1", username: "root", assetID: "OTHER", confirmed: true},
			{id: "X-2", username: "opsuser", assetID: "OTHER", confirmed: true},
			{id: "X-3", username: "foreignpending", assetID: "OTHER", confirmed: false},
		},
		report: "<html><body>Basic Information Task Name Adhoc gather accounts: zz-probe " +
			"Start Date 2026/11/03 10:11:45 End Date 2026/11/03 10:11:45 " +
			"Asset Count 1 Asset Success Count 0 Asset Failure Count 1 Asset not supported 0 " +
			"Duration 0.02s Newly discovered accounts: 0 New account not found</body></html>",
	}
	cs := truthSession(t, f)
	isErr, out, txt := callRaw(t, cs, "jms_gather_run",
		map[string]any{"asset": "zz-probe", "confirm": true})
	if isErr {
		t.Fatalf("the tool answers with the report even on failure: %s", txt)
	}
	note, _ := out["note"].(string)
	if !regexp.MustCompile(`(?i)fail`).MatchString(note) {
		t.Errorf("note = %q, want it to name the failure the report recorded (Asset Failure Count 1)", note)
	}
	for _, foreign := range []string{"root", "opsuser", "foreignpending"} {
		if strings.Contains(txt, foreign) {
			t.Errorf("the answer names %q from ANOTHER asset's gather rows:\n%s", foreign, txt)
		}
	}
	if g, ok := out["gathering"].(float64); ok && int(g) != 0 {
		t.Errorf("gathering = %v, want 0 rows for THIS asset (the seeded rows belong to another asset)", out["gathering"])
	}
}

// A run whose discovery NEVER RAN must not be called a completion either.
//
// Measured 2026-11-03 (10:50): the same adhoc gather against an asset whose
// platform has no remote automation reported
// "Asset Count 1 Asset Success Count 0 Asset Failure Count 0 Asset not
// supported 1" — and the tool answered "the run completed but reported no
// new accounts". Not-supported is not a run that found nothing; it is a run
// that never happened. And when the report's counts cannot be read at all,
// the answer must say it could not read the verdict — NOT assert one.
func TestGatherRunSaysWhenDiscoveryNeverRan(t *testing.T) {
	cases := []struct {
		name   string
		report string
		want   *regexp.Regexp
	}{
		{"not supported is named", "Basic Information Task Name Adhoc gather accounts: zz-probe " +
			"Start Date 2026/11/03 10:50:58 End Date 2026/11/03 10:50:58 " +
			"Asset Count 1 Asset Success Count 0 Asset Failure Count 0 Asset not supported 1 " +
			"Duration 0.01s Newly discovered accounts: 0 New account not found",
			regexp.MustCompile(`(?i)not supported`)},
		// Measured 2026-11-03 11:12 from the sweep's full-answer dump: a
		// report whose run had classified NOTHING yet — scope 1, all three
		// tallies 0. Not an empty result; a run that has not (yet) happened.
		{"nothing classified yet is named", "Basic Information Task Name Adhoc gather accounts: zz-probe " +
			"Start Date 2026/11/03 11:12:17 End Date 2026/11/03 11:12:17 " +
			"Asset Count 1 Asset Success Count 0 Asset Failure Count 0 Asset not supported 0 " +
			"Duration 0.01s Newly discovered accounts: 0 New account not found",
			regexp.MustCompile(`(?i)not happened|before the run finished|never got scheduled`)},
		// A verdict word gone missing is NOT a zero count: with "Asset
		// Failure Count" absent, a naive parser reads "succeeded 1,
		// failed 0" and the answer dresses a possibly-failed run as
		// "already known" (LOW-7). Unreadable must be admitted as such.
		{"failure word gone is unreadable", "Basic Information Task Name Adhoc gather accounts: zz-probe " +
			"Asset Count 1 Asset Success Count 1 Asset not supported 0 " +
			"Duration 0.01s Newly discovered accounts: 0 New account not found",
			regexp.MustCompile(`(?i)no readable counts|could not read`)},
		{"unreadable verdict says so", "Basic Information whatever shape this is",
			regexp.MustCompile(`(?i)no readable counts|could not read`)},
	}
	// Every one of these runs discovered NOTHING. The note must not, in any
	// wording, hand the reader an empty success: those two phrases are what
	// a "ran fine, nothing new" answer looks like. (A mutation that renamed
	// the warning but kept its tail once slipped past a want-only check.)
	forbid := regexp.MustCompile(`(?i)already known|the run completed|no new accounts: every`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &truthFake{report: tc.report}
			cs := truthSession(t, f)
			isErr, out, txt := callRaw(t, cs, "jms_gather_run",
				map[string]any{"asset": "zz-probe", "confirm": true})
			if isErr {
				t.Fatalf("the tool answers with the report even on failure: %s", txt)
			}
			note, _ := out["note"].(string)
			if !tc.want.MatchString(note) {
				t.Errorf("note = %q, want it to match %s", note, tc.want)
			}
			if forbid.MatchString(note) {
				t.Errorf("note = %q, dresses a non-run up as an empty success", note)
			}
		})
	}
}

// jms_job_run must not report success without recap evidence.
//
// Measured 2026-11-03: an execution whose recap was
// {"ok":[],"dark":{},"skipped":[],"failures":{}} — nothing ran on any host —
// carried is_success=true once and is_success=false the next time. The
// upstream label cannot be copied; the recap decides.
func TestJobRunDoesNotClaimSuccessWithoutRecapProof(t *testing.T) {
	f := &truthFake{
		execSet: true,
		exec: map[string]any{
			"id": "T1", "is_finished": true, "is_success": true,
			"time_cost": "1.761272",
			"summary": map[string]any{
				"ok": []any{}, "dark": map[string]any{},
				"skipped": []any{}, "failures": map[string]any{}},
		},
	}
	cs := truthSession(t, f)
	isErr, out, txt := callRaw(t, cs, "jms_job_run",
		map[string]any{"assets": []string{"zz-probe"}, "command": "echo x", "wait": 3, "confirm": true})
	if isErr {
		t.Fatalf("the run answers with the execution record: %s", txt)
	}
	if ok, _ := out["is_success"].(bool); ok {
		t.Errorf("is_success = true, but the recap says no host ran the command:\n%s", txt)
	}
	// Success may only be claimed with recap proof — and where there IS
	// proof, it must be claimed. The guard cuts both ways so a blanket
	// "always false" cannot pass as honesty.
	f2 := &truthFake{
		execSet: true,
		exec: map[string]any{
			"id": "T2", "is_finished": true, "is_success": false, "time_cost": "1.7",
			"summary": map[string]any{
				"ok": []any{"node-071"}, "dark": map[string]any{},
				"skipped": []any{}, "failures": map[string]any{}},
		},
	}
	cs2 := truthSession(t, f2)
	_, out2, txt2 := callRaw(t, cs2, "jms_job_run",
		map[string]any{"assets": []string{"zz-probe"}, "command": "echo x", "wait": 3, "confirm": true})
	if ok, _ := out2["is_success"].(bool); !ok {
		t.Errorf("is_success is not true although the recap names a host that ran ok:\n%s", txt2)
	}
}

// No recap at all ⇒ the verdict is NOT GIVEN, whatever the upstream label
// says. The label is the thing that lied — true for "nothing ran" — and
// even a false label only says the TASK errored: the command may have run
// on every host before the task tripped. IDL JobPlan.is_success promises
// "这一格不给"; this pins the promise (M4: the code used to answer false
// here, which made the IDL a liar).
func TestJobSuccessIsAbsentWithoutAnyRecap(t *testing.T) {
	for _, label := range []bool{true, false} {
		f := &truthFake{
			execSet: true,
			exec:    map[string]any{"id": "T9", "is_finished": true, "is_success": label, "time_cost": "0.5"},
		}
		cs := truthSession(t, f)
		_, out, txt := callRaw(t, cs, "jms_job_run",
			map[string]any{"assets": []string{"zz-probe"}, "command": "echo x", "wait": 3, "confirm": true})
		if _, present := out["is_success"]; present {
			t.Errorf("upstream label is_success=%v and there is no recap: the answer gives is_success=%v, want the field absent (无法确认就不说):\n%s",
				label, out["is_success"], txt)
		}
	}
}

// The same derivation, wherever the record is read: jms_job_status and
// jms_job_list copy the upstream label today. One truth, one reading.
func TestJobStatusAndListReadTheRecapNotTheLabel(t *testing.T) {
	emptyRecap := map[string]any{
		"id": "T1", "is_finished": true, "is_success": true, "time_cost": "1.76",
		"summary": map[string]any{
			"ok": []any{}, "dark": map[string]any{},
			"skipped": []any{}, "failures": map[string]any{}},
	}
	realRecap := map[string]any{
		"id": "T2", "is_finished": true, "is_success": true, "time_cost": "5.16",
		"summary": map[string]any{
			"ok": []any{"node-071"}, "dark": map[string]any{},
			"skipped": []any{}, "failures": map[string]any{}},
	}
	f := &truthFake{execSet: true, exec: emptyRecap,
		list: []map[string]any{emptyRecap, realRecap}}
	cs := truthSession(t, f)

	_, out, txt := callRaw(t, cs, "jms_job_status", map[string]any{"task_id": "T1"})
	if ok, _ := out["is_success"].(bool); ok {
		t.Errorf("jms_job_status claims is_success=true for a recap that lists no host:\n%s", txt)
	}

	_, out2, txt2 := callRaw(t, cs, "jms_job_list", map[string]any{})
	items, _ := out2["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("jms_job_list returned %d rows, want 2: %s", len(items), txt2)
	}
	for _, it := range items {
		row := it.(map[string]any)
		hosts := []any{}
		if s, ok := row["summary"].(map[string]any); ok {
			hosts, _ = s["ok"].([]any)
		}
		claimed, _ := row["is_success"].(bool)
		if claimed && len(hosts) == 0 {
			t.Errorf("jms_job_list claims is_success=true without a single host in the recap: %v", row)
		}
		if !claimed && len(hosts) > 0 {
			t.Errorf("jms_job_list denies success although the recap names a host that ran: %v", row)
		}
	}
}

// The recap's "dark" list is hosts that were UNREACHABLE — a host that did
// not run the command. "Which hosts worked" has to say so instead of
// dropping them on the floor.
func TestJobSummaryKeepsUnreachableHostsVisible(t *testing.T) {
	h := jobSummary(map[string]any{
		"ok": []any{}, "skipped": []any{},
		"failures": map[string]any{},
		"dark":     map[string]any{"rpi4-011": 1},
	})
	if h == nil {
		t.Fatalf("a recap naming an unreachable host must not flatten to nothing")
	}
	if _, present := h.Failed["rpi4-011"]; !present {
		t.Errorf("failed = %v, want the unreachable host to be visible there", h.Failed)
	}
	if empty := jobSummary(map[string]any{
		"ok": []any{}, "skipped": []any{}, "failures": map[string]any{}, "dark": map[string]any{},
	}); empty != nil {
		t.Errorf("an empty recap must read as 'nothing to report', got %v", empty)
	}
}
