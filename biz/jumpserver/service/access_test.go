package service

import (
	"strings"
	"testing"
)

// These tests are the point of the package: the rules used to be reachable
// only by calling a tool against a live bastion, and the case that matters
// most — a grant that names an account the asset does not have — is exactly
// the one a live bastion rarely has. Everything here runs offline.

func TestCoversAssetByDirectName(t *testing.T) {
	perm := map[string]any{"assets": []any{map[string]any{"id": "a1"}}}
	asset := map[string]any{"id": "a1"}
	if !CoversAsset(perm, asset, nil) {
		t.Fatal("a permission naming the asset does not cover it")
	}
	other := map[string]any{"id": "a2"}
	if CoversAsset(perm, other, nil) {
		t.Fatal("a permission naming another asset covers this one")
	}
}

func TestCoversAssetThroughTheNodeSubtree(t *testing.T) {
	// A permission on /DEFAULT/studio reaches the assets filed under
	// /DEFAULT/studio/arm. The asset only lists its own leaf node, so
	// comparing node ids directly would miss it.
	perm := map[string]any{"nodes": []any{map[string]any{"id": "parent"}}}
	asset := map[string]any{
		"id":    "a1",
		"nodes": []any{map[string]any{"id": "child"}},
	}
	descendants := func(id string) []string {
		if id == "parent" {
			return []string{"parent", "child"}
		}
		return []string{id}
	}
	if !CoversAsset(perm, asset, descendants) {
		t.Fatal("a permission on the parent node does not cover the asset in the child node")
	}
	if CoversAsset(perm, asset, func(string) []string { return []string{"parent"} }) {
		t.Fatal("covered an asset whose node is not below the permission's node")
	}
}

func TestCoversAssetNeedsANodeToBeReachable(t *testing.T) {
	perm := map[string]any{"nodes": []any{map[string]any{"id": "n1"}}}
	asset := map[string]any{"id": "a1"}
	if CoversAsset(perm, asset, func(string) []string { return []string{"n1"} }) {
		t.Fatal("an asset filed in no node was reported as covered")
	}
}

func TestCoversAssetWithoutATreeIsOnlyTheDirectName(t *testing.T) {
	perm := map[string]any{"nodes": []any{map[string]any{"id": "n1"}}}
	asset := map[string]any{
		"id":    "a1",
		"nodes": []any{map[string]any{"id": "n1"}},
	}
	if CoversAsset(perm, asset, nil) {
		t.Fatal("without a tree, node coverage must not be claimed")
	}
}

func TestJudgeGrantWithNoAccount(t *testing.T) {
	// The silent failure: the permission exists, the asset exists, and the
	// login still fails.
	g := JudgeGrant(nil, []string{"opsuser"})
	if g.Usable {
		t.Fatal("a grant with no account was judged usable")
	}
	if !strings.Contains(g.Note, "NO account") {
		t.Errorf("note = %q, want it to say the grant hands out no account", g.Note)
	}
}

func TestJudgeGrantWithAllAccounts(t *testing.T) {
	g := JudgeGrant([]string{AllAccounts}, []string{"root", "opsuser"})
	if !g.Usable {
		t.Fatal("a grant of every account was judged unusable")
	}
	if len(g.Accounts) != 2 {
		t.Errorf("usable accounts = %v, want both", g.Accounts)
	}
}

func TestJudgeGrantWithAccountsTheAssetDoesNotHave(t *testing.T) {
	// The case a live bastion rarely has, and the one that matters: the
	// grant names an account that does not exist on the asset, so the
	// permission can never be used.
	g := JudgeGrant([]string{"nosuch"}, []string{"opsuser"})
	if g.Usable {
		t.Fatal("a grant naming an account the asset does not have was judged usable")
	}
	if !strings.Contains(g.Note, "does not have") {
		t.Errorf("note = %q, want it to name the accounts that are missing", g.Note)
	}
}

func TestJudgeGrantWithAnAccountTheAssetHas(t *testing.T) {
	g := JudgeGrant([]string{"opsuser"}, []string{"opsuser", "root"})
	if !g.Usable {
		t.Fatal("a grant naming a real account was judged unusable")
	}
	if len(g.Accounts) != 1 || g.Accounts[0] != "opsuser" {
		t.Errorf("usable accounts = %v, want [opsuser]", g.Accounts)
	}
	if g.Note != "" {
		t.Errorf("note = %q, want empty for a working grant", g.Note)
	}
}

func TestExpectsTerminalAccess(t *testing.T) {
	// A Website and a Kubernetes asset have no terminal protocol and no
	// login account, and neither is a problem: reporting them as broken
	// produced six false alarms.
	for _, category := range []string{"web", "cloud"} {
		asset := map[string]any{"category": map[string]any{"value": category}}
		if ExpectsTerminalAccess(asset) {
			t.Errorf("category %q was expected to be driven through a terminal", category)
		}
	}
	for _, category := range []string{"host", "database", ""} {
		asset := map[string]any{"category": map[string]any{"value": category}}
		if !ExpectsTerminalAccess(asset) {
			t.Errorf("category %q was not expected to be driven through a terminal", category)
		}
	}
}

func TestJudgeAccounts(t *testing.T) {
	all := JudgeAccounts([]map[string]any{
		{"username": "opsuser", "is_active": true},
		{"username": "root", "is_active": true},
	})
	if all.AllDisabled || len(all.Disabled) != 0 {
		t.Errorf("all-active accounts judged as %+v", all)
	}
	if all.Total != 2 {
		t.Errorf("Total = %d, want 2", all.Total)
	}

	some := JudgeAccounts([]map[string]any{
		{"username": "opsuser", "is_active": true},
		{"username": "root", "is_active": false},
	})
	if some.AllDisabled {
		t.Error("one disabled account among two was judged as all disabled")
	}
	if len(some.Disabled) != 1 || some.Disabled[0] != "root" {
		t.Errorf("Disabled = %v, want [root]", some.Disabled)
	}

	none := JudgeAccounts([]map[string]any{
		{"username": "opsuser", "is_active": false},
		{"username": "root", "is_active": false},
	})
	if !none.AllDisabled {
		t.Error("every account disabled was not judged as all disabled")
	}

	// No account at all is not "all disabled": there is nothing to be
	// disabled, and the account check says so separately.
	empty := JudgeAccounts(nil)
	if empty.AllDisabled {
		t.Error("an asset with no accounts was judged as all disabled")
	}
}

func TestContains(t *testing.T) {
	if !Contains([]string{"a", "b"}, "b") {
		t.Error("Contains missed a value that is there")
	}
	if Contains([]string{"a", "b"}, "c") {
		t.Error("Contains found a value that is not there")
	}
}
