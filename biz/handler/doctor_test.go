package handler

import (
	"strings"
	"testing"
)

// The asset-level rules are the ones that produced six false alarms when they
// were wrong: a Website asset is reached over HTTP and a Kubernetes asset over
// its API, so neither ever has a terminal protocol or a login account.
//
// judgeAssets is a pure function now, so those rules can be pinned without a
// bastion.
func TestJudgeAssetsSkipsNonTerminalCategories(t *testing.T) {
	assets := []map[string]any{
		// A host with a protocol and an account: fine.
		{"address": "192.0.2.71", "category": map[string]any{"value": "host"},
			"platform":        map[string]any{"name": "Linux"},
			"protocols":       []any{map[string]any{"name": "ssh"}},
			"accounts_amount": float64(1)},
		// A host with neither: both hazards.
		{"address": "192.0.2.72", "category": map[string]any{"value": "host"},
			"platform":        map[string]any{"name": "Linux"},
			"protocols":       []any{},
			"accounts_amount": float64(0)},
		// A website: no protocol and no account is CORRECT for it.
		{"address": "https://10.0.8.10", "category": map[string]any{"value": "web"},
			"platform":        map[string]any{"name": "Website"},
			"protocols":       []any{},
			"accounts_amount": float64(0)},
		// A kubernetes asset: same story.
		{"address": "https://10.0.24.3:6443", "category": map[string]any{"value": "cloud"},
			"platform":        map[string]any{"name": "Kubernetes"},
			"protocols":       []any{},
			"accounts_amount": float64(0)},
	}
	j := judgeAssets(assets)

	if j.Total != 4 {
		t.Errorf("Total = %d, want 4", j.Total)
	}
	// Only the host without a protocol is a hazard. The website and the
	// kubernetes asset are not hosts, so they are not flagged.
	if len(j.NoProtocol) != 1 || j.NoProtocol[0] != "192.0.2.72" {
		t.Errorf("NoProtocol = %v, want only the host 192.0.2.72", j.NoProtocol)
	}
	if len(j.NoAccount) != 1 || j.NoAccount[0] != "192.0.2.72" {
		t.Errorf("NoAccount = %v, want only the host 192.0.2.72", j.NoAccount)
	}
	if j.ByPlatform["Linux"] != 2 || j.ByPlatform["Website"] != 1 {
		t.Errorf("ByPlatform = %v, want Linux:2 Website:1", j.ByPlatform)
	}
}

// An unknown category is assumed to be a host: flagging a real host is
// cheap, silently skipping one is not.
func TestJudgeAssetsTreatsUnknownCategoryAsHost(t *testing.T) {
	assets := []map[string]any{
		{"address": "192.0.2.73", "platform": map[string]any{"name": "Linux"},
			"protocols": []any{}, "accounts_amount": float64(0)},
	}
	j := judgeAssets(assets)
	if len(j.NoProtocol) != 1 {
		t.Errorf("NoProtocol = %v, want the unknown-category host to be flagged", j.NoProtocol)
	}
}

// The permission-level rules: an empty account list is NOT "@ALL" (it grants
// nothing), a permission nobody holds is useless, and a permission naming an
// account no granted asset has can never be used.
func TestJudgePermissions(t *testing.T) {
	perms := []map[string]any{
		// Grants an account the asset does not have -> can never be used.
		{"name": "bad-acct", "accounts": []any{"root"},
			"assets": []any{"a1"}, "users_amount": float64(1)},
		// Grants nothing: an empty list is not @ALL.
		{"name": "no-acct", "accounts": []any{},
			"assets": []any{"a1"}, "users_amount": float64(1)},
		// Held by nobody.
		{"name": "no-user", "accounts": []any{"opsuser"},
			"assets": []any{"a1"}, "users_amount": float64(0), "user_groups_amount": float64(0)},
		// A healthy permission: grants an account the asset really has.
		{"name": "good", "accounts": []any{"opsuser"},
			"assets": []any{"a1"}, "users_amount": float64(2), "user_groups_amount": float64(0)},
		// @ALL grants every account, so it is never dangling.
		{"name": "all", "accounts": []any{"@ALL"},
			"assets": []any{"a1"}, "users_amount": float64(1)},
	}
	assetAccounts := map[string]map[string]bool{"a1": {"opsuser": true}}

	// The assets each permission reaches are resolved from the DETAIL
	// endpoint: the list endpoint answers with them as null.
	grantedAssets := map[string][]string{
		"bad-acct": {"a1"},
		"no-acct":  {"a1"},
		"no-user":  {"a1"},
		"good":     {"a1"},
		"all":      {"a1"},
	}
	j := judgePermissions(perms, assetAccounts, grantedAssets)
	if j.Total != 5 {
		t.Errorf("Total = %d, want 5", j.Total)
	}
	if len(j.NoAccount) != 1 || j.NoAccount[0] != "no-acct" {
		t.Errorf("NoAccount = %v, want only no-acct", j.NoAccount)
	}
	if len(j.NoUser) != 1 || j.NoUser[0] != "no-user" {
		t.Errorf("NoUser = %v, want only no-user", j.NoUser)
	}
	if len(j.Dangling) != 1 || !strings.Contains(j.Dangling[0], "bad-acct") {
		t.Errorf("Dangling = %v, want only bad-acct", j.Dangling)
	}
}
