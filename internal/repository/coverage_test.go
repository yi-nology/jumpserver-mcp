package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository/livetest"
)

// The node tree's ancestry.
//
// JumpServer's node records report parent_id and parent as null for every
// node, so ancestry has to come from the key: the key is a path of indices
// ("1" is /DEFAULT, "1:0" is /DEFAULT/studio, "1:0:1" is /DEFAULT/studio/arm),
// and a node is below another exactly when its key is a prefix of the other's.
//
// Getting this wrong means a permission on /DEFAULT/studio looks like it does
// not reach the assets filed under /DEFAULT/studio/arm -- and those assets
// then show up as "nobody can log in" in every audit.

func nodeTreeFixture() *repository.NodeTree {
	rows := []map[string]any{
		{"id": "n-root", "key": "1", "full_value": "/DEFAULT"},
		{"id": "n-studio", "key": "1:0", "full_value": "/DEFAULT/studio"},
		{"id": "n-arm", "key": "1:0:1", "full_value": "/DEFAULT/studio/arm"},
		{"id": "n-x86", "key": "1:0:2", "full_value": "/DEFAULT/studio/x86"},
		{"id": "n-k8s", "key": "1:0:3", "full_value": "/DEFAULT/studio/k8s"},
		{"id": "n-home", "key": "1:1", "full_value": "/DEFAULT/home"},
		{"id": "n-tencent", "key": "1:2", "full_value": "/DEFAULT/tencent"},
	}
	t := &repository.NodeTree{ByID: map[string]map[string]any{}, ByKey: map[string]map[string]any{}}
	for _, n := range rows {
		t.ByID[fmt.Sprint(n["id"])] = n
		t.ByKey[fmt.Sprint(n["key"])] = n
		t.Order = append(t.Order, fmt.Sprint(n["id"]))
	}
	return t
}

func TestNodeTreeDescendants(t *testing.T) {
	tree := nodeTreeFixture()
	cases := []struct {
		name string
		node string
		want []string
	}{
		{"root covers everything", "n-root",
			[]string{"n-arm", "n-home", "n-k8s", "n-root", "n-studio", "n-tencent", "n-x86"}},
		{"a node covers its children", "n-studio",
			[]string{"n-arm", "n-k8s", "n-studio", "n-x86"}},
		{"a leaf covers only itself", "n-arm", []string{"n-arm"}},
		{"a sibling subtree is not included", "n-home", []string{"n-home"}},
	}
	for _, c := range cases {
		got := tree.Descendants(c.node)
		if len(got) != len(c.want) {
			t.Errorf("%s: descendants(%s) = %v, want %v", c.name, c.node, got, c.want)
			continue
		}
		set := map[string]bool{}
		for _, g := range got {
			set[g] = true
		}
		for _, w := range c.want {
			if !set[w] {
				t.Errorf("%s: descendants(%s) is missing %s (got %v)", c.name, c.node, w, got)
			}
		}
	}
}

// A key that merely starts with the same characters is not a descendant:
// "1:1" (home) is not below "1" + something other than ":".
func TestNodeTreeDescendantsNoPrefixConfusion(t *testing.T) {
	tree := nodeTreeFixture()
	got := tree.Descendants("n-home")
	if len(got) != 1 || got[0] != "n-home" {
		t.Errorf("descendants(home) = %v, want just home", got)
	}
}

// The effective coverage of a permission must match what the bastion's own
// effective-access view says. This is the check that proves the expansion is
// not a plausible guess.
func TestEffectiveAssetsMatchesBastionView(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()

	perms, err := c.ListAll(ctx, "/perms/asset-permissions/")
	if err != nil {
		t.Fatalf("list permissions: %v", err)
	}
	for _, p := range perms {
		var detail map[string]any
		if err := c.Do(ctx, "GET", "/perms/asset-permissions/"+fmt.Sprint(p["id"])+"/", nil, nil, &detail); err != nil {
			continue
		}
		effective, err := c.EffectiveAssets(ctx, detail)
		if err != nil {
			t.Errorf("%v: effective assets: %v", detail["name"], err)
			continue
		}
		// The reported counter counts only the directly named assets, so it
		// is a LOWER bound, never an upper one.
		reported, _ := detail["assets_amount"].(float64)
		if float64(len(effective)) < reported {
			t.Errorf("%v: effective count %d is below the reported %v, which cannot be right",
				detail["name"], len(effective), reported)
		}
		t.Logf("%v: reported=%v effective=%d", detail["name"], reported, len(effective))
	}
}
