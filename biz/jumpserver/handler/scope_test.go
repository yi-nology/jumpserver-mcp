package handler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/oekit/pkg/slicex"
	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/service"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"

	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/livetest"

	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/client"
)

// The asset tree.
//
// A node's writable field is `value`; `name` is derived and read-only, so a
// node created with {"name": ...} comes back with an EMPTY name. And a node
// that still holds assets cannot be deleted -- the API answers 403 "Deletion
// failed, the node contains assets". Both are the kind of thing that only
// shows up when you try it, which is why they are pinned by a test.

// TestNodeLifecycle creates a node, files an asset into it, takes the asset
// back out and removes the node.
func TestNodeLifecycle(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()
	assetID, _ := livetest.TestAsset(t)

	name := testsupport.Uniq("mcp-test-node")
	var created map[string]any
	if err := c.Post(ctx, jmsapi.ResNodes, map[string]any{"value": name, "full_value": "/DEFAULT/" + name}, &created); err != nil {
		t.Fatalf("create node: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("create node returned no id: %v", created)
	}
	defer func() {
		// A node holding assets cannot be deleted, so empty it first.
		_ = c.Put(ctx, jmsapi.ResNodes, id, "assets/remove", map[string]any{"assets": []string{assetID}}, nil)
		_ = c.Delete(ctx, jmsapi.ResNodes, id)
	}()

	var back map[string]any
	if err := c.Get(ctx, jmsapi.ResNodes, id, &back); err != nil {
		t.Fatalf("read node back: %v", err)
	}
	if got := fmt.Sprint(back["name"]); got != name {
		t.Errorf("node name = %q, want %q (the writable field is `value`, not `name`)", got, name)
	}

	// File the asset into the node and read the count back.
	if err := c.Put(ctx, jmsapi.ResNodes, id, "assets/add", map[string]any{"assets": []string{assetID}}, nil); err != nil {
		t.Fatalf("add asset to node: %v", err)
	}
	var afterAdd map[string]any
	_ = c.Get(ctx, jmsapi.ResNodes, id, &afterAdd)
	if n, _ := afterAdd["assets_amount"].(float64); n != 1 {
		t.Errorf("assets_amount = %v after adding one, want 1", afterAdd["assets_amount"])
	}

	// Take it back out, so the node can be deleted.
	if err := c.Put(ctx, jmsapi.ResNodes, id, "assets/remove", map[string]any{"assets": []string{assetID}}, nil); err != nil {
		t.Fatalf("remove asset from node: %v", err)
	}
	var afterRemove map[string]any
	_ = c.Get(ctx, jmsapi.ResNodes, id, &afterRemove)
	if n, _ := afterRemove["assets_amount"].(float64); n != 0 {
		t.Errorf("assets_amount = %v after removing, want 0", afterRemove["assets_amount"])
	}
}

// A permission can be scoped to named accounts instead of @ALL. The accounts
// are ACCOUNT NAMES, not ids -- passing an id silently grants nothing.
func TestPermissionWithNamedAccounts(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()
	assetID, _ := livetest.TestAsset(t)

	name := testsupport.Uniq("mcp-test-perm")
	var created map[string]any
	if err := c.Post(ctx, jmsapi.ResPermissions, map[string]any{
		"name":      name,
		"users":     client.IDList([]string{"645c04c7-4380-475f-81be-845b7a2f1efc"}), // opsuser
		"assets":    client.IDList([]string{assetID}),
		"accounts":  []string{"opsuser"},
		"is_active": true,
	}, &created); err != nil {
		t.Fatalf("create permission: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("create permission returned no id: %v", created)
	}
	defer c.Delete(ctx, jmsapi.ResPermissions, id)

	var back map[string]any
	if err := c.Get(ctx, jmsapi.ResPermissions, id, &back); err != nil {
		t.Fatalf("read permission back: %v", err)
	}
	accts := asStringSlice(back["accounts"])
	if len(accts) != 1 || accts[0] != "opsuser" {
		t.Errorf("accounts = %v, want [opsuser]", accts)
	}

	// The accounts sub-resource expands the names into the real accounts.
	var expanded []map[string]any
	if err := c.ListSubInto(ctx, jmsapi.ResPermissions, id, "accounts", &expanded); err != nil {
		t.Fatalf("expand accounts: %v", err)
	}
	if len(expanded) == 0 {
		t.Error("the named account did not resolve to an account")
	}
}

// Granting named accounts to a permission that already grants @ALL must not
// narrow it: that would be a revoke disguised as a grant.
func TestGrantKeepsAllAccounts(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()
	assetID, _ := livetest.TestAsset(t)

	name := testsupport.Uniq("mcp-test-all")
	var created map[string]any
	if err := c.Post(ctx, jmsapi.ResPermissions, map[string]any{
		"name":      name,
		"users":     client.IDList([]string{"645c04c7-4380-475f-81be-845b7a2f1efc"}),
		"assets":    client.IDList([]string{assetID}),
		"accounts":  []string{service.AllAccounts},
		"is_active": true,
	}, &created); err != nil {
		t.Fatalf("create permission: %v", err)
	}
	id, _ := created["id"].(string)
	defer c.Delete(ctx, jmsapi.ResPermissions, id)

	// This is what the tool does: read the current set, then merge.
	var current map[string]any
	if err := c.Get(ctx, jmsapi.ResPermissions, id, &current); err != nil {
		t.Fatalf("read permission: %v", err)
	}
	currentAccts := asStringSlice(current["accounts"])
	merged := currentAccts
	if !slicex.Contains(currentAccts, service.AllAccounts) {
		merged = mergeIDs(currentAccts, []string{"opsuser"})
	}
	if err := c.Patch(ctx, jmsapi.ResPermissions, id, map[string]any{"accounts": merged}, nil); err != nil {
		t.Fatalf("patch accounts: %v", err)
	}

	var after map[string]any
	_ = c.Get(ctx, jmsapi.ResPermissions, id, &after)
	accts := asStringSlice(after["accounts"])
	if !slicex.Contains(accts, service.AllAccounts) {
		t.Errorf("accounts = %v, want @ALL to survive the grant", accts)
	}
}

// The read-only scope tools must work against the live API.
func TestScopeReadTools(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()

	var groups []map[string]any
	if err := c.ListInto(ctx, jmsapi.ResGroups, &groups); err != nil {
		t.Fatalf("list user groups: %v", err)
	}
	t.Logf("user groups: %d", len(groups))

	var terminals []map[string]any
	if err := c.ListInto(ctx, jmsapi.ResTerminals, &terminals); err != nil {
		t.Fatalf("list terminals: %v", err)
	}
	if len(terminals) == 0 {
		t.Error("expected at least the bastion's own component terminals")
	}
	t.Logf("terminals: %d", len(terminals))
}

// The doctor must keep reporting the hazards it is meant to catch, including
// the new one: a permission naming an account no granted asset has.
func TestDoctorChecksUnknownAccountGrant(t *testing.T) {
	c := livetest.LiveClient(t)
	report, err := Doctor(context.Background(), c, false)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	checks := report.Checks
	found := false
	for _, ck := range checks {
		if ck.Name == "permissions_with_unknown_accounts" {
			found = true
			switch ck.Status {
			case jms.Status_ok, jms.Status_warn, jms.Status_fail:
			default:
				t.Errorf("unexpected status %v", ck.Status)
			}
		}
	}
	if !found {
		t.Error("the doctor does not check for permissions naming unknown accounts")
	}
}

// A node path that does not exist has to fail with a clear message rather
// than an empty id.
func TestResolveNodePathsRejectsUnknown(t *testing.T) {
	c := livetest.LiveClient(t)
	_, err := c.ResolveNodePaths(context.Background(), []string{"/DEFAULT/definitely-not-a-node"})
	if err == nil {
		t.Fatal("expected an error for an unknown node path")
	}
	if !strings.Contains(err.Error(), "no such node") {
		t.Errorf("error = %v, want it to name the problem", err)
	}
}

// os is used by the live-test helpers; keep the import meaningful.
