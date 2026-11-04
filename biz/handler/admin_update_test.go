// The update-side gaps the live sweep (script/sweep-write-full.py) found on
// 2026-11-03, each pinned here first so the fix has a guard that goes red
// when the gap comes back.
//
// ① jms_asset_update collapsed the comment's three states into two: an
//
//	explicit "" was treated as "not given", so the note could never be
//	emptied — the same `!= ""` bug the ACL updates were fixed for.
//
// ② jms_job_stop sent `{"ids": [...]}` while the live v5 serializer wants
//
//	`task_id` (HTTP 400: {"error":{"task_id":["This field is required."]}})
//	— the contract test's fake agreed with the CODE (`{"ok": true}` for any
//	body) instead of with the API, so the mismatch slipped through. The
//	fake in this file validates the body the way the real serializer does.
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

// assetReq is one request the fake bastion saw, for wire assertions.
type assetReq struct {
	method string
	path   string
	body   map[string]any
}

// assetFake records every request and serves the two endpoints asset_update
// touches: the resolve list and the PATCH. protoJSON is what the GETs answer
// with for "protocols" (default: ssh on 22); a test that needs an asset with
// no ssh-family protocol sets it.
type assetFake struct {
	mu        sync.Mutex
	reqs      []assetReq
	protoJSON string
}

func (f *assetFake) protos() string {
	if f.protoJSON != "" {
		return f.protoJSON
	}
	return `[{"name":"ssh","port":22}]`
}

func (f *assetFake) snapshot() []assetReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]assetReq{}, f.reqs...)
}

func (f *assetFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body map[string]any
	if r.Method == http.MethodPatch || r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, assetReq{r.Method, r.URL.Path, body})
	f.mu.Unlock()

	switch {
	case r.URL.Path == "/api/v1/assets/assets/" && r.Method == http.MethodGet:
		w.Write([]byte(`{"count":1,"results":[{"id":"A1","name":"zz-probe-asset",` +
			`"address":"192.0.2.1","platform":{"id":"P1","name":"Linux"},` +
			`"protocols":` + f.protos() + `,"is_active":true,` +
			`"comment":"original note"}]}`))
	case r.URL.Path == "/api/v1/assets/assets/A1/" && r.Method == http.MethodGet:
		w.Write([]byte(`{"id":"A1","name":"zz-probe-asset",` +
			`"address":"192.0.2.1","platform":{"id":"P1","name":"Linux"},` +
			`"protocols":` + f.protos() + `,"is_active":true,` +
			`"comment":"original note"}`))
	case r.URL.Path == "/api/v1/assets/assets/A1/" && r.Method == http.MethodPatch:
		w.Write([]byte(`{"id":"A1","name":"zz-probe-asset","address":"192.0.2.1",` +
			`"platform":{"id":"P1","name":"Linux"},"is_active":true}`))
	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"detail":"no such endpoint: ` + r.Method + " " + r.URL.Path + `"}`))
	}
}

// An explicit "" must reach the wire as a present-but-empty key: that is the
// CLEAR direction of the three-state contract (absent / empty / set). A
// `!= ""` gate makes the field unemptyable.
func TestAssetUpdateClearsTheCommentOnAnEmptyString(t *testing.T) {
	fake := &assetFake{}
	cs := writeTestSession(t, fake)
	testsupport.CallToolJSON(t, cs, "jms_asset_update", map[string]any{
		"asset": "zz-probe-asset", "comment": "", "confirm": true,
	})

	reqs := fake.snapshot()
	var patch map[string]any
	for _, rq := range reqs {
		if rq.method == http.MethodPatch {
			patch = rq.body
		}
	}
	if patch == nil {
		t.Fatalf("no PATCH went out (update refused?): %v", reqs)
	}
	v, present := patch["comment"]
	if !present {
		t.Fatalf("PATCH body omits comment: %v — an empty string must CLEAR it, not vanish", patch)
	}
	if v != "" {
		t.Errorf("PATCH body comment = %v, want an empty string (the clear)", v)
	}
}

// The ABSENT direction: omitting the field must not send it at all.
func TestAssetUpdateOmitsTheCommentWhenNoneWasGiven(t *testing.T) {
	fake := &assetFake{}
	cs := writeTestSession(t, fake)
	testsupport.CallToolJSON(t, cs, "jms_asset_update", map[string]any{
		"asset": "zz-probe-asset", "ssh_port": 2222, "confirm": true,
	})

	for _, rq := range fake.snapshot() {
		if rq.method != http.MethodPatch {
			continue
		}
		if _, present := rq.body["comment"]; present {
			t.Errorf("PATCH body carries comment although none was given: %v", rq.body)
		}
	}
}

// The description promises "correcting a wrong port" as a headline repair
// (an HAProxy on 2222 while the asset says 22 reads as an AUTH error). That
// promise means `ssh_port` alone must move the ssh-family ports — and today
// the bare int is silently dropped ("nothing to change").
func TestAssetUpdateSshPortAloneMovesThePort(t *testing.T) {
	fake := &assetFake{}
	cs := writeTestSession(t, fake)
	testsupport.CallToolJSON(t, cs, "jms_asset_update", map[string]any{
		"asset": "zz-probe-asset", "ssh_port": 2222, "confirm": true,
	})

	reqs := fake.snapshot()
	var patch map[string]any
	for _, rq := range reqs {
		if rq.method == http.MethodPatch {
			patch = rq.body
		}
	}
	if patch == nil {
		t.Fatalf("no PATCH went out: ssh_port alone must move the port, "+
			"that is the repair the description promises: %v", reqs)
	}
	protos, _ := patch["protocols"].([]any)
	if len(protos) == 0 {
		t.Fatalf("PATCH body carries no protocols: %v", patch)
	}
	moved := false
	for _, p := range protos {
		m, _ := p.(map[string]any)
		if m["name"] == "ssh" {
			if m["port"] == float64(2222) {
				moved = true
			} else {
				t.Errorf("ssh port = %v, want 2222", m["port"])
			}
		}
	}
	if !moved {
		t.Errorf("no ssh protocol carries the new port: %v", protos)
	}
}

// The flip side of the same promise: when there IS no ssh-family protocol to
// move, the move cannot happen — and an answer of "updated" over a repair
// that never landed is the silent failure this repo forbids. The tool must
// say so, and no PATCH may go out.
func TestAssetUpdateSshPortRefusesLoudlyWhenThereIsNoSuchProtocol(t *testing.T) {
	fake := &assetFake{protoJSON: `[{"name":"mysql","port":3306}]`}
	cs := writeTestSession(t, fake)
	isErr, _, txt := callRaw(t, cs, "jms_asset_update", map[string]any{
		"asset": "zz-probe-asset", "ssh_port": 2222, "confirm": true,
	})
	if !isErr {
		t.Errorf("the tool answered success although there is no ssh protocol to move:\n%s", txt)
	}
	for _, rq := range fake.snapshot() {
		if rq.method == http.MethodPatch {
			t.Errorf("a PATCH went out although the port could not be moved: %v", rq)
		}
	}
}

// The stop endpoint as the live v5 actually validates it: `task_id` is
// REQUIRED (verbatim 400 body captured from the machine on 2026-11-03).
// A fake that answers {"ok":true} to any body would agree with whatever the
// code sends — which is exactly how the wrong field name survived.
func TestJobStopSendsTheFieldTheAPIWants(t *testing.T) {
	fake := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/ops/job-executions/stop/" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["task_id"]; !ok {
			// the live serializer's own words
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"task_id":["This field is required."]}}`))
			return
		}
		w.Write([]byte(`{"ok": true}`))
	})
	cs := writeTestSession(t, fake)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_job_stop",
		Arguments: map[string]any{"task_id": "T1", "confirm": true},
	})
	if err != nil {
		t.Fatalf("call jms_job_stop: %v", err)
	}
	if res.IsError {
		t.Fatalf("jms_job_stop cannot stop anything on a live bastion — "+
			"it sends the wrong body field: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "T1") {
		t.Errorf("the answer does not name the stopped execution: %s", txt)
	}
}
