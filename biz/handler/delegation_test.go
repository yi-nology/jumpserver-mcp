package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yi-nology/jumpserver-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
	"github.com/yi-nology/jumpserver-mgmt/internal/service/policy"
)

// The delegation tool can call ANY endpoint with ANY method, so the method is
// what decides whether the call changes anything.
//
// It is the tool an operator reaches for when the long-lived token should
// not stay reachable — deletes, permission changes, account resets — which
// is exactly why a write through it has to be confirmed.
func TestDelegationCallBlocksAWriteWithoutConfirmation(t *testing.T) {
	// A client that cannot reach anything: the refusal has to happen before
	// the delegation is set up, so the answer must not depend on it.
	c := repository.NewBase("http://127.0.0.1:1", "t", nil)
	cs := testsupport.NewSession(t, "delegation-test", reg.Options{Policy: policy.Table},
		func(r *reg.Registry) { RegisterDelegationTools(r, c) })

	for _, method := range []string{"POST", "PATCH", "PUT", "DELETE"} {
		out := testsupport.CallToolJSON(t, cs, "jms_delegation_call",
			map[string]any{"method": method, "path": "/assets/assets/"})
		if out["blocked"] == nil {
			t.Errorf("%s was not blocked: %v", method, out)
		}
	}
}

// A read does not need a confirmation — that is the whole point of the rule
// being about the method.
func TestDelegationCallLetsAReadThrough(t *testing.T) {
	c := repository.NewBase("http://127.0.0.1:1", "t", nil)
	cs := testsupport.NewSession(t, "delegation-test", reg.Options{Policy: policy.Table},
		func(r *reg.Registry) { RegisterDelegationTools(r, c) })

	// A GET is not blocked: it fails later, on the delegation not being
	// configured — which is a different answer, and an error rather than a
	// refusal. So this one reads the result itself instead of insisting on
	// a successful call.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_delegation_call",
		Arguments: map[string]any{"method": "GET", "path": "/assets/assets/"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	text := testsupport.ToolText(t, res)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err == nil && out["blocked"] != nil {
		t.Errorf("a read was blocked: %s", text)
	}
}
