package router

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yi-nology/jumpserver-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
	"github.com/yi-nology/jumpserver-mgmt/internal/service/policy"
)

// Every tool must name the fields it answers with.
//
// render.Object is the escape hatch: it says "an object" and nothing else,
// so the answer is not checked at all — a tool can quietly stop returning a
// field and nobody finds out.
//
// The management plane used to have 31 of them. They are all on a contract
// now, and this guard is what keeps them there: a new tool gets a contract
// type in the IDL, not the vague shape.
func TestNoToolFallsBackToTheVagueShape(t *testing.T) {
	r := reg.New(&mcp.Implementation{Name: "shape-test", Version: "1"},
		reg.Options{Policy: policy.Table})
	Register(r, repository.NewBase("http://127.0.0.1:1", "t", nil))

	ct, st := mcp.NewInMemoryTransports()
	ss, err := r.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	defer ss.Close()

	cl := mcp.NewClient(&mcp.Implementation{Name: "probe", Version: "1"}, nil)
	cs, err := cl.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer cs.Close()

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(res.Tools) == 0 {
		t.Fatal("no tools listed")
	}
	for _, tool := range res.Tools {
		schema, ok := tool.OutputSchema.(map[string]any)
		if !ok {
			t.Errorf("tool %s declares no output schema", tool.Name)
			continue
		}
		props, _ := schema["properties"].(map[string]any)
		if len(props) == 0 {
			t.Errorf("tool %s declares a shape with no fields of its own: "+
				"its answer is not checked", tool.Name)
		}
	}
}

// The rows of a list must be typed too.
//
// A typed envelope with free-form rows says "here are some objects" — the
// caller still has to guess the field names, and a field that quietly
// disappears from a row is invisible. A typed row says what it carries, and
// the server checks every row before the model sees it.
//
// There is no exception left. jms_label_resources' row used to be one: it
// carries the bastion's labeled-resource record, whose shape was unknown
// because this bastion had no labelled resource at all. Observing one (a
// label created, attached, looked at, removed) showed the resource comes
// over as a display STRING — so the row is ours to shape after all.

// TestEveryListToolTypesItsRows is the second half of the shape rule: the
// envelope is not enough.
func TestEveryListToolTypesItsRows(t *testing.T) {
	r := reg.New(&mcp.Implementation{Name: "rows-test", Version: "1"},
		reg.Options{Policy: policy.Table})
	Register(r, repository.NewBase("http://127.0.0.1:1", "t", nil))

	ct, st := mcp.NewInMemoryTransports()
	ss, err := r.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	defer ss.Close()

	cl := mcp.NewClient(&mcp.Implementation{Name: "probe", Version: "1"}, nil)
	cs, err := cl.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer cs.Close()

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	checked := 0
	for _, tool := range res.Tools {
		schema, ok := tool.OutputSchema.(map[string]any)
		if !ok {
			continue
		}
		props, _ := schema["properties"].(map[string]any)
		items, ok := props["items"].(map[string]any)
		if !ok {
			continue // not a list-shaped answer
		}
		// The row's own schema sits inside the array's items.
		row, ok := items["items"].(map[string]any)
		if !ok {
			t.Errorf("tool %s declares a list of something unnamed", tool.Name)
			continue
		}
		checked++
		rowProps, _ := row["properties"].(map[string]any)
		if len(rowProps) > 0 {
			continue // the row names its fields ✓
		}
		t.Errorf("tool %s answers with rows that name no fields: the caller has "+
			"to guess them, and a row that loses one is invisible", tool.Name)
	}
	if checked == 0 {
		t.Fatal("no list-shaped tools found; the check is looking at nothing")
	}
}
