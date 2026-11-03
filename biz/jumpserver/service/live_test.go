package service

// The live check: the coverage rule has to agree with the bastion's own
// effective-access view, on real data.
//
// It lives here rather than in internal/jumpserver/client because the RULE lives here now —
// and because a rule that can only be exercised against a live bastion is a
// rule that is not tested at all on a machine that has no bastion.

import (
	"context"
	"fmt"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/client"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/livetest"
)

// The coverage decision has to consider the node subtree in BOTH directions:
// a permission on a parent node covers a child node's assets, and an asset
// filed in a child node is covered by a permission on the parent.
func TestCoversAssetThroughNodeSubtreeAgainstTheBastion(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()

	// A permission scoped to the "studio" node, and an asset filed in
	// "studio/arm".
	var perm map[string]any
	if err := c.Do(ctx, "POST", "/perms/asset-permissions/", nil, map[string]any{
		"name":      testsupport.Uniq("mcp-test-nodeperm"),
		"users":     client.IDList([]string{"645c04c7-4380-475f-81be-845b7a2f1efc"}),
		"nodes":     client.IDList([]string{"9a5bf262-38c4-44db-a73e-7e1f9ae5f3e2"}), // /DEFAULT/studio
		"accounts":  []string{"@ALL"},
		"is_active": true,
	}, &perm); err != nil {
		t.Fatalf("create permission: %v", err)
	}
	id := fmt.Sprint(perm["id"])
	defer c.Do(ctx, "DELETE", "/perms/asset-permissions/"+id+"/", nil, nil, nil)

	var detail map[string]any
	if err := c.Do(ctx, "GET", "/perms/asset-permissions/"+id+"/", nil, nil, &detail); err != nil {
		t.Fatalf("read permission: %v", err)
	}

	// The tree is what turns a node-scoped permission into a set of assets.
	tree, err := c.LoadNodeTree(ctx)
	if err != nil {
		t.Fatalf("load node tree: %v", err)
	}

	// node-071 lives in /DEFAULT/studio/arm, so a permission on
	// /DEFAULT/studio must reach it.
	var asset map[string]any
	if err := c.Do(ctx, "GET", "/assets/assets/42e1a002-e33f-4c38-ad16-479cadb2df41/", nil, nil, &asset); err != nil {
		t.Fatalf("read asset: %v", err)
	}
	if !CoversAsset(detail, asset, tree.Descendants) {
		t.Error("a permission on /DEFAULT/studio must cover the asset filed in /DEFAULT/studio/arm")
	}

	// An asset outside that subtree must not be covered. It has to be a real
	// asset from a different subtree -- changing the id of the one above
	// would not do it, because node coverage follows the asset's NODE
	// membership, not its identity.
	inSubtree := map[string]bool{}
	for _, nid := range tree.Descendants("9a5bf262-38c4-44db-a73e-7e1f9ae5f3e2") {
		inSubtree[nid] = true
	}
	var outside map[string]any
	assets, err := c.ListAll(ctx, "/assets/assets/")
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	for _, a := range assets {
		inside := false
		for _, n := range jmsapi.AsSlice(a["nodes"]) {
			if m, ok := n.(map[string]any); ok && inSubtree[fmt.Sprint(m["id"])] {
				inside = true
			}
		}
		if !inside {
			outside = a
			break
		}
	}
	if outside == nil {
		t.Skip("no asset outside the studio subtree on this bastion")
	}
	if CoversAsset(detail, outside, tree.Descendants) {
		t.Errorf("a permission on /DEFAULT/studio must not cover %v, which is filed elsewhere",
			outside["name"])
	}
}
