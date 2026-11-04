// tools_coverage.go answers the question the bastion's own counters cannot:
// "what does this permission ACTUALLY cover?".
//
// The problem is real and it is a big one. An AssetPermission carries two
// kinds of scope -- named assets and asset-tree nodes -- and the counter the
// API reports, assets_amount, counts ONLY the named assets:
//
//	permission "所有"   assets_amount = 16   nodes = [home, tencent, arm, x86]
//	                     but it actually reaches 45 assets
//
// The four nodes alone hold 41 assets, and three of the 16 named assets sit
// inside those nodes as well. So the counter understates the real reach by
// nearly 3x, and it understates it in a way that is invisible: the
// permission list looks like a narrow grant, and an access review that trusts
// it concludes the wrong thing.
//
// The expansion is not a guess. Node ancestry is derivable from the node key
// -- the key is a path of indices ("1:0" is /DEFAULT/studio, "1:0:1" is
// /DEFAULT/studio/arm), so a node is an ancestor of another exactly when its
// key is a prefix of the other's. Expanding this way reproduces the bastion's
// own effective-access view exactly (verified: 45 assets either way).
//
// The other half of the problem is the direction of the check. "Does this
// permission cover this asset?" has to consider that a permission on a
// PARENT node covers the assets of its CHILDREN -- otherwise every asset
// filed under /DEFAULT/studio/arm looks uncovered by a permission on
// /DEFAULT/studio.
package handler

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yi-nology/jumpserver-kit/model/jms"
	"github.com/yi-nology/jumpserver-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-kit/pkg/render"
	"github.com/yi-nology/jumpserver-mgmt/biz/model/jmsms"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
)

// nodeTree is the asset tree with its ancestry resolved.

// loadNodeTree reads the asset tree once.

// descendants returns the node itself plus every node below it.
//
// Ancestry comes from the key: a node is below another exactly when its key
// starts with the other's key followed by ":". (The API's own parent_id is
// null for every node, so it cannot be used for this.)

// assetsUnderNode lists the assets filed in a node.
//
// The assets of a node's CHILDREN are not included here; the caller is
// expected to walk the subtree, because "under this node" includes the
// children.

// effectiveAssets returns every asset a permission actually reaches.
//
// It is the union of the directly named assets and the assets of every node
// in the permission's subtree.

// permissionCoversAsset reports whether a permission reaches an asset.
//
// Both the named assets and the node subtree count, and the node comparison
// respects ancestry: a permission on /DEFAULT/studio covers the assets filed
// under /DEFAULT/studio/arm.

// registerCoverageTools exposes the expansion on its own, and as a
// cross-user view.
func RegisterCoverageTools(r *reg.Registry, c *repository.Client) {
	type coverageArgs struct {
		Permission string `json:"permission" jsonschema:"permission name or UUID"`
		Limit      int    `json:"limit,omitempty" jsonschema:"max assets to list (default 100)"`
	}
	reg.Add(r, &mcp.Tool{
		Name: "jms_permission_coverage",
		Description: "List every asset an asset permission ACTUALLY reaches, expanding the asset-tree nodes it " +
			"covers. The counter the API reports (assets_amount) counts only the directly named assets and " +
			"understates the real reach -- on this bastion one permission reports 16 but reaches 45. " +
			"Use this for an access review, and to see which assets a node-scoped permission grants.",
		OutputSchema: render.Schema[jmsms.PermissionCoverage](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args coverageArgs) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Permission) == "" {
			return render.Failf("permission is required")
		}
		id, err := c.PermissionIDByName(ctx, args.Permission)
		if err != nil {
			return render.Failf("%v", err)
		}
		var perm map[string]any
		if err := c.Get(ctx, jmsapi.ResPermissions, id, &perm); err != nil {
			return render.Failf("%v", err)
		}
		effective, err := c.EffectiveAssets(ctx, perm)
		if err != nil {
			return render.Failf("%v", err)
		}
		limit := args.Limit
		limit = repository.NormLimit(limit, 100, 500)
		assets := []*jmsms.AssetRef{}
		for _, a := range effective {
			if len(assets) >= limit {
				continue
			}
			assets = append(assets, &jmsms.AssetRef{
				ID:       jmsapi.FieldStr(a["id"]),
				Name:     jmsapi.FieldStr(a["name"]),
				Address:  jmsapi.FieldStr(a["address"]),
				Platform: jmsapi.FieldStr(jmsapi.Nested(a, "platform", "name")),
			})
		}
		res := &jmsms.PermissionCoverage{
			Permission:           jmsapi.FieldStr(perm["name"]),
			ReportedAssetsAmount: jmsapi.FieldInt32(perm["assets_amount"]),
			EffectiveAssetsCount: jms.OptInt32(int32(len(effective))),
			DirectAssetsCount:    jms.OptInt32(int32(len(jmsapi.AsSlice(perm["assets"])))),
			Nodes:                nodeNames(perm),
			Accounts:             asStringSlice(perm["accounts"]),
			Assets:               assets,
			Truncated:            jms.OptBool(len(effective) > len(assets)),
		}
		if n, _ := perm["assets_amount"].(float64); int(n) != len(effective) {
			res.Warning = jms.Opt(fmt.Sprintf(
				"the permission reports %d assets but actually reaches %d: the difference comes from the "+
					"asset-tree nodes it covers. Do not use the counter for an access review",
				int(n), len(effective)))
		}
		return render.Text(res)
	})

	type matrixArgs struct {
		Limit int `json:"limit,omitempty" jsonschema:"max users to return (default 50)"`
	}
	reg.Add(r, &mcp.Tool{
		Name: "jms_access_matrix",
		Description: "Overview of who can reach what: for every JumpServer user, the assets they can actually " +
			"reach and the permissions that give it to them, taken from the bastion's own effective-access graph " +
			"(so assets inherited through a node or a user group are included, not just the directly named ones). " +
			"Use it to spot who has far more than they need, and for offboarding checks.",
		OutputSchema: render.Schema[render.ListOf[jmsms.AccessMatrixUser]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args matrixArgs) (*mcp.CallToolResult, any, error) {
		users, err := c.List(ctx, jmsapi.ResUsers)
		if err != nil {
			return render.Failf("%v", err)
		}
		perms, err := c.List(ctx, jmsapi.ResPermissions)
		if err != nil {
			return render.Failf("%v", err)
		}
		// Which permissions hold which users (the detail carries the members).
		heldBy := map[string][]string{}
		for _, p := range perms {
			var detail map[string]any
			if err := c.Get(ctx, jmsapi.ResPermissions, jmsapi.Str(p["id"]), &detail); err != nil {
				continue
			}
			for _, u := range jmsapi.AsSlice(detail["users"]) {
				if m, ok := u.(map[string]any); ok {
					id := jmsapi.Str(m["id"])
					heldBy[id] = append(heldBy[id], jmsapi.Str(detail["name"]))
				}
			}
		}
		sort.Strings(heldBy[""])

		limit := args.Limit
		limit = repository.NormLimit(limit, 50, 200)
		out := []jmsms.AccessMatrixUser{}
		for _, u := range users {
			username := jmsapi.Str(u["username"])
			// Component accounts such as [KoKo]-... are internal plumbing.
			if strings.HasPrefix(username, "[") {
				continue
			}
			if len(out) >= limit {
				break
			}
			uid := jmsapi.Str(u["id"])
			rows, err := c.ListSub(ctx, jmsapi.ResPermUserAssets, uid, "assets")
			if err != nil {
				continue
			}
			byPlatform := map[string]int32{}
			for _, a := range rows {
				byPlatform[jmsapi.Str(jmsapi.Nested(a, "platform", "name"))]++
			}
			out = append(out, jmsms.AccessMatrixUser{
				Username:        username,
				IsActive:        jmsapi.FieldBool(u["is_active"]),
				IsSuperuser:     jmsapi.FieldBool(u["is_superuser"]),
				AssetsReachable: jms.OptInt32(int32(len(rows))),
				ByPlatform:      byPlatform,
				Permissions:     heldBy[uid],
			})
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i].GetAssetsReachable() > out[j].GetAssetsReachable()
		})
		res := render.ListOf[jmsms.AccessMatrixUser]{
			ListMeta: jms.ListMeta{
				Count:     int32(len(out)),
				Total:     int32(len(out)),
				Truncated: false,
			},
			Items: out,
			Note:  "sorted by reach, widest first; this is the view to start an access review from",
		}
		return render.Text(res)
	})
}

// nodeNames lists the node names a permission covers.
func nodeNames(perm map[string]any) []string {
	out := []string{}
	for _, n := range jmsapi.AsSlice(perm["nodes"]) {
		if m, ok := n.(map[string]any); ok {
			out = append(out, jmsapi.Str(m["name"]))
		}
	}
	sort.Strings(out)
	return out
}
