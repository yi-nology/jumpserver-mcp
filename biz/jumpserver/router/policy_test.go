package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/handler"
	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/policy"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/client"
)

// The catalogue and the registration must agree.
//
// Two tables describe the same 76 tools: the registration (what the server
// offers) and the policy (what each tool does to the bastion, and what it
// says when it is called without a confirmation). They drift apart easily —
// a new tool that nobody classified would be treated as an unclassified write
// and refused, and a tool that was classified but never registered would
// leave a rule nobody can reach.
//
// The check is mechanical on purpose: it is the kind of thing that has to be
// true at every commit, not something a reviewer should have to remember.

func newRegistry(t *testing.T) *reg.Registry {
	t.Helper()
	r := reg.New(&mcp.Implementation{Name: "jms-policy-test", Version: "1"},
		reg.Options{Policy: policy.Table})
	c := client.NewBase("http://127.0.0.1:1", "t", http.DefaultClient)
	Register(r, c)
	return r
}

func TestEveryRegisteredToolIsClassified(t *testing.T) {
	r := newRegistry(t)
	for _, spec := range r.Specs() {
		p, ok := policy.Table[spec.Name]
		if !ok {
			t.Errorf("tool %s is registered but not classified in the policy table", spec.Name)
			continue
		}
		if p.Risk != spec.Risk {
			t.Errorf("tool %s: policy risk %s, registered risk %s", spec.Name, p.Risk, spec.Risk)
		}
		if (p.Confirm != "") != (spec.Confirm != nil) {
			t.Errorf("tool %s: policy confirm %q, registered confirm %v",
				spec.Name, p.Confirm, spec.Confirm)
		}
	}
}

// Every tool must say what its answer looks like.
//
// The declaration is what the server checks the answer against before the
// model sees it, so a tool that declares nothing is a tool whose shape can
// drift without anybody noticing — and shape drift is the failure mode that
// reads as "the bastion has no users" when it means "the tool forgot to
// return them".
func TestEveryToolDeclaresItsOutputShape(t *testing.T) {
	r := newRegistry(t)
	for _, spec := range r.Specs() {
		if !r.HasOutputSchema(spec.Name) {
			t.Errorf("tool %s declares no output schema; its answer is unchecked", spec.Name)
		}
	}
}

func TestEveryClassifiedToolIsRegistered(t *testing.T) {
	r := newRegistry(t)
	registered := map[string]bool{}
	for _, spec := range r.Specs() {
		registered[spec.Name] = true
	}
	for name := range policy.Table {
		if !registered[name] {
			t.Errorf("tool %s is classified in the policy table but never registered", name)
		}
	}
}

// The tool count is the number the skills and the README quote. A change
// here is a change to the contract with the agent.
func TestToolCount(t *testing.T) {
	r := newRegistry(t)
	if got := len(r.Specs()); got != 97 {
		t.Errorf("tool count = %d, want 97", got)
	}
}

// A tool that changes the bastion has to say so, and has to carry the
// sentence it answers with.
//
// The sentence is what the model reads when it calls the tool without a
// confirmation, so it cannot be empty: an empty one would leave the caller
// with a refusal that says nothing about what to do.
func TestEveryWriteToolCarriesItsConfirmationSentence(t *testing.T) {
	for name, p := range policy.Table {
		if p.Risk == jms.Risk_read {
			if p.Confirm != "" {
				t.Errorf("read tool %s carries a confirmation sentence", name)
			}
			continue
		}
		if p.Confirm == "" {
			// A conditional confirmation has no sentence to refuse with —
			// the tool decides when the flag is needed. It still needs a
			// line for the catalogue, or the rule is invisible.
			if !p.Conditional {
				t.Errorf("write tool %s has no confirmation sentence", name)
			} else if p.Hint == "" {
				t.Errorf("write tool %s has a conditional confirmation but no hint for the catalogue", name)
			}
			continue
		}
		if p.Confirm != "" && !strings.Contains(p.Confirm, "confirm=true") {
			t.Errorf("tool %s: confirmation sentence does not mention the flag: %q", name, p.Confirm)
		}
	}
}

// The confirmation check has to be structural, not a habit.
//
// A tool that can be registered without passing the gate is a tool whose
// confirmation is a suggestion. The registry is the only door, and this
// checks that the door actually refuses: a write tool called without
// confirm=true must not reach its body.
func TestWriteToolIsRefusedWithoutConfirmation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c := client.NewBase(ts.URL, "t", ts.Client())
	cs := testsupport.NewSession(t, "jms-gate-test", reg.Options{Policy: policy.Table},
		func(r *reg.Registry) {
			handler.RegisterAssetWriteTools(r, c)
		})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_asset_delete",
		Arguments: map[string]any{"asset": "10.0.0.1"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("the delete went through without a confirmation")
	}
	text := ""
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	// The refusal must name the confirmation. Which wording it carries
	// depends on who caught it: the SDK's schema validation catches a
	// missing required field before the gate does, and the gate's own
	// sentence is the fallback. Both say the same thing to the caller.
	if !strings.Contains(text, "confirm") {
		t.Errorf("refusal does not mention the confirmation:\n%s", text)
	}
}

// The same call WITH the confirmation must reach the tool — otherwise the
// gate would be a wall, not a gate.
func TestWriteToolRunsWithConfirmation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_, _ = w.Write([]byte(`[{"id":"a1","name":"box","address":"10.0.0.1"}]`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := client.NewBase(ts.URL, "t", ts.Client())
	cs := testsupport.NewSession(t, "jms-gate-test", reg.Options{Policy: policy.Table},
		func(r *reg.Registry) {
			handler.RegisterAssetWriteTools(r, c)
		})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_asset_delete",
		Arguments: map[string]any{"asset": "10.0.0.1", "confirm": true},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		text := ""
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text += tc.Text
			}
		}
		t.Fatalf("the confirmed delete was refused:\n%s", text)
	}
}

// The read-only mode refuses the write tools without touching them.
//
// This is the mode an operator turns on when an agent should look but not
// touch. It is a mode of the gate, not a convention in the handlers.
func TestReadOnlyModeRefusesWrites(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c := client.NewBase(ts.URL, "t", ts.Client())
	cs := testsupport.NewSession(t, "jms-readonly-test", reg.Options{Policy: policy.Table, ReadOnly: true},
		func(r *reg.Registry) {
			handler.RegisterAssetWriteTools(r, c)
			handler.RegisterStatusTools(r, c)
		})

	// The write is refused even though it carries the confirmation.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_asset_delete",
		Arguments: map[string]any{"asset": "10.0.0.1", "confirm": true},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("the read-only mode let a write through")
	}

	// The read still works.
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_platforms_list",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatal("the read-only mode refused a read")
	}
}

// The catalogue is part of what the agent sees, so it has to be renderable
// and complete.
func TestCatalogueRenders(t *testing.T) {
	r := newRegistry(t)
	b, err := json.Marshal(r.Specs())
	if err != nil {
		t.Fatalf("marshal specs: %v", err)
	}
	var specs []jms.ToolSpec
	if err := json.Unmarshal(b, &specs); err != nil {
		t.Fatalf("unmarshal specs: %v", err)
	}
	if len(specs) != 97 {
		t.Fatalf("specs = %d, want 97", len(specs))
	}
	for _, s := range specs {
		if s.Name == "" || s.Description == "" {
			t.Errorf("tool %q has no name or no description", s.Name)
		}
	}
}
