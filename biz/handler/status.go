package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/yi-nology/jumpserver-mcp/biz/model/jmsms"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oagentkit/guardx/toolfail"
	"github.com/oekit/pkg/convert"

	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/errs"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/render"
	"github.com/yi-nology/jumpserver-mcp/internal/repository"
)

// statusArgs is the (empty) input of jms_status.
type statusArgs struct{}

// The read-only tools: what the bastion holds and what state it is in.
// They are the entry point of an investigation — status first, then the
// lists, then the single-asset view.
func RegisterStatusTools(r *reg.Registry, c *repository.Client) {
	reg.Add(r, &mcp.Tool{
		Name:         "jms_status",
		Description:  "Check JumpServer reachability, API auth, version and asset counts. Start here for a quick look at whether the bastion answers and how big it is; use jms_doctor when you need the deeper check of what is misconfigured.",
		OutputSchema: render.Schema[jmsms.BastionStatus](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ statusArgs) (*mcp.CallToolResult, any, error) {
		out := &jmsms.BastionStatus{
			BaseURL:         c.BaseURL,
			AssetByPlatform: map[string]int32{},
		}

		// Auth check and identity in one call: /users/profile/ answers with
		// the CALLER. ⚠️ The first row of /users/users/ is NOT the caller —
		// see identityOf for the measured failure (a non-admin caller was
		// reported as admin).
		if who, disp, _, idErr := identityOf(ctx, c); idErr == nil {
			out.AuthOk = true
			out.User = jmsapi.FieldStr(who)
			out.Name = jmsapi.FieldStr(disp)
		} else {
			// The identity probe failed. That does not by itself mean the
			// credential is bad — an odd deployment could lack the route, or
			// the caller may lack users.view_user on the list. So auth is
			// settled separately, and by the FAILURE CLASS: only a 401 means
			// the credential was rejected. A 403 (no permission to list
			// users) or a 404/500 is a valid credential reaching a route it
			// cannot use — answering auth_ok=false there would blame the
			// credential for a permission gap and send the operator to
			// rotate a token that is fine.
			var raw json.RawMessage
			err := c.GetQuery(ctx, jmsapi.ResUsers, url.Values{"page_size": {"1"}}, &raw)
			// Did the API actually ANSWER (any HTTP status), or did the
			// request die in transport (connection refused / timeout / DNS)?
			// Only a response proves the token reached the API; transport
			// failure proves nothing about the credential.
			var apiErr *errs.APIError
			responded := errors.As(err, &apiErr)
			switch {
			case err == nil:
				out.AuthOk = true
				out.Error = jms.Opt(idErr.Error())
			case errs.Class(err) == toolfail.ClassAuthentication:
				out.AuthOk = false
				out.Error = jms.Opt("the credential was rejected: " + err.Error())
			case responded:
				// The API answered with a non-auth status (403 / 404 / 5xx):
				// the request reached the API and was NOT rejected as an
				// invalid credential — the route is simply unavailable to it.
				// So the token CAN call the API: auth_ok is genuinely true.
				out.AuthOk = true
				out.Error = jms.Opt(idErr.Error() + " (the credential was not rejected as invalid; " +
					"the users list is unavailable to it)")
			default:
				// Transport failure: the request never reached the API, so
				// whether the token can call it is UNPROVEN. auth_ok is a
				// required bool, so the unknown folds to its conservative
				// value (false = not proven able), and the error says plainly
				// this is NOT a verdict on the credential — asserting
				// "accepted" here would be a fabrication, the same sin as
				// inventing an identity.
				out.AuthOk = false
				out.Error = jms.Opt(idErr.Error() + " (could not reach the bastion; this is NOT a verdict " +
					"on the credential — the token was never tested)")
			}
		}

		if raw := fetchRaw(c, ctx, jmsapi.ResAssets); raw != nil {
			if rows, err := jmsapi.List(raw); err == nil {
				byPlatform := map[string]int32{}
				okCount, errCount := int32(0), int32(0)
				for _, r := range rows {
					var a map[string]any
					if json.Unmarshal(r, &a) != nil {
						continue
					}
					p, _ := jmsapi.Nested(a, "platform", "name").(string)
					if p == "" {
						p = "(none)"
					}
					byPlatform[p]++
					switch jmsapi.Label(a, "connectivity") {
					case "ok":
						okCount++
					case "err":
						errCount++
					}
				}
				out.AssetTotal = jms.OptInt32(int32(len(rows)))
				out.AssetByPlatform = byPlatform
				out.ConnectivityOk = jms.OptInt32(okCount)
				out.ConnectivityErr = jms.OptInt32(errCount)
			}
		}
		return render.Text(out)
	})

	type platformsArgs struct{}
	reg.Add(r, &mcp.Tool{
		Name:         "jms_platforms_list",
		Description:  "List asset platforms (Linux, macOS, Gateway, Website, Kubernetes, MySQL ...) with ids. Use it to find the platform name jms_asset_create expects, and to see which platforms this bastion supports.",
		OutputSchema: render.Schema[render.ListOf[jmsms.PlatformSummary]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ platformsArgs) (*mcp.CallToolResult, any, error) {
		raw := fetchRaw(c, ctx, jmsapi.ResPlatforms)
		if raw == nil {
			return render.Err(fmt.Errorf("cannot list platforms"))
		}
		rows, err := jmsapi.List(raw)
		if err != nil {
			return render.Err(err)
		}
		var out []jmsms.PlatformSummary
		for _, r := range rows {
			var p map[string]any
			if json.Unmarshal(r, &p) == nil {
				out = append(out, jmsms.PlatformSummary{
					ID:   jmsapi.FieldStr(p["id"]),
					Name: jmsapi.FieldStr(p["name"]),
					Type: jmsapi.FieldStr(p["type"]),
				})
			}
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i].GetName() < out[j].GetName()
		})
		return render.Text(render.ListOf[jmsms.PlatformSummary]{
			ListMeta: jms.ListMeta{Count: int32(len(out)), Total: int32(len(out)), Truncated: false},
			Items:    out,
		})
	})

	type assetsListArgs struct {
		Platform     string `json:"platform,omitempty" jsonschema:"filter by platform name, e.g. Linux or Kubernetes"`
		Connectivity string `json:"connectivity,omitempty" jsonschema:"filter by connectivity state: ok, err or -"`
		Search       string `json:"search,omitempty" jsonschema:"substring match on address or name"`
		Limit        int    `json:"limit,omitempty" jsonschema:"max rows to return (default 100)"`
	}
	reg.Add(r, &mcp.Tool{
		Name:         "jms_assets_list",
		Description:  "List managed assets with address, platform and connectivity. Filter by platform, connectivity or a search string. Use it to find a machine's exact name and address before opening a session on it; the execution plane's jmsx_inventory is the same list seen from the other side.",
		OutputSchema: render.Schema[render.ListOf[jms.AssetSummary]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args assetsListArgs) (*mcp.CallToolResult, any, error) {
		rows, err := c.List(ctx, jmsapi.ResAssets)
		if err != nil {
			return render.Err(err)
		}
		limit := args.Limit
		limit = repository.NormLimit(limit, 100, 500)
		out := []jms.AssetSummary{}
		for _, a := range rows {
			plat := fmt.Sprint(jmsapi.Nested(a, "platform", "name"))
			conn := jmsapi.Label(a, "connectivity")
			if args.Platform != "" && !strings.EqualFold(plat, args.Platform) {
				continue
			}
			if args.Connectivity != "" && !strings.EqualFold(conn, args.Connectivity) {
				continue
			}
			if args.Search != "" {
				s := strings.ToLower(args.Search)
				if !strings.Contains(strings.ToLower(jmsapi.Str(a["address"])), s) &&
					!strings.Contains(strings.ToLower(jmsapi.Str(a["name"])), s) {
					continue
				}
			}
			// The shared asset row: the execution plane's inventory
			// produces the same one, so "which machines are there" has
			// one answer whichever server is asked.
			out = append(out, jmsapi.AssetSummary(a))
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i].GetAddress() < out[j].GetAddress()
		})
		// total is how many assets matched the filters, before the cap.
		total := len(out)
		if len(out) > limit {
			out = out[:limit]
		}
		return render.Text(render.ListOf[jms.AssetSummary]{
			ListMeta: jms.ListMeta{Count: int32(len(out)), Total: int32(total), Truncated: len(out) < total},
			Items:    out,
		})
	})

	type assetGetArgs struct {
		Ref string `json:"ref" jsonschema:"asset address, name or UUID, e.g. 198.51.100.3"`
	}
	reg.Add(r, &mcp.Tool{
		Name:         "jms_asset_get",
		Description:  "Full detail for one asset: platform, protocols, accounts, connectivity, last login and verification time. This is the bastion's raw record for that machine — use jms_asset_audit instead when you want the verdict on what is broken with it.",
		OutputSchema: render.Schema[jmsms.AssetDetail](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args assetGetArgs) (*mcp.CallToolResult, any, error) {
		_, meta, err := c.ResolveAssets(ctx, []string{args.Ref})
		if err != nil {
			return render.Err(err)
		}
		if len(meta) == 0 {
			return render.Err(fmt.Errorf("asset not found: %s", args.Ref))
		}
		id := meta[0]["id"].(string)
		var detail map[string]any
		if err := c.Get(ctx, jmsapi.ResAssets, id, &detail); err != nil {
			return render.Err(err)
		}
		return render.Text(assetDetail(detail))
	})

	type accountsArgs struct {
		Asset string `json:"asset,omitempty" jsonschema:"only accounts of this asset (address/name/uuid)"`
	}
	reg.Add(r, &mcp.Tool{
		Name:         "jms_accounts_list",
		Description:  "List asset accounts with username, secret type and privileged flag. Secrets are never returned. Use it to see which logins a machine accepts; jms_asset_audit answers the same question together with the verdict on what else is wrong with the machine.",
		OutputSchema: render.Schema[render.ListOf[jmsms.AccountSummary]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args accountsArgs) (*mcp.CallToolResult, any, error) {
		rows, err := c.List(ctx, jmsapi.ResAccounts)
		if err != nil {
			return render.Err(err)
		}
		var assetIDs map[string]bool
		if args.Asset != "" {
			ids, _, err := c.ResolveAssets(ctx, []string{args.Asset})
			if err != nil {
				return render.Err(err)
			}
			assetIDs = map[string]bool{}
			for _, id := range ids {
				assetIDs[id] = true
			}
		}
		// The row carries the account AND the machine it belongs to: a list
		// of logins without the asset they hang on cannot be acted on.
		out := []jmsms.AccountSummary{}
		for _, a := range rows {
			if assetIDs != nil {
				aid := fmt.Sprint(jmsapi.Nested(a, "asset", "id"))
				if !assetIDs[aid] {
					continue
				}
			}
			out = append(out, jmsms.AccountSummary{
				Username:      jmsapi.Str(a["username"]),
				Name:          jmsapi.FieldStr(a["name"]),
				SecretType:    jmsapi.FieldStr(jmsapi.Label(a, "secret_type")),
				Privileged:    jmsapi.FieldBool(a["privileged"]),
				IsActive:      jmsapi.FieldBool(a["is_active"]),
				Connectivity:  jmsapi.FieldStr(jmsapi.Label(a, "connectivity")),
				DateLastLogin: jmsapi.FieldStr(a["date_last_login"]),
				Asset:         jmsapi.FieldStr(jmsapi.FieldName(a, "asset")),
				Address:       jmsapi.FieldStr(jmsapi.Nested(a, "asset", "address")),
			})
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i].GetAddress() < out[j].GetAddress()
		})
		items := out
		return render.Text(render.ListOf[jmsms.AccountSummary]{
			ListMeta: jms.ListMeta{
				Count:     int32(len(items)),
				Total:     int32(len(rows)),
				Truncated: len(items) < len(rows),
			},
			Items: items,
		})
	})

	type nodesArgs struct{}
	reg.Add(r, &mcp.Tool{
		Name:         "jms_nodes_list",
		Description:  "List asset tree nodes (the grouping folders assets belong to). Use it to find the node path that jms_asset_move wants: each row carries the node's name and its full path in full_value (that is the path to pass on).",
		OutputSchema: render.Schema[render.ListOf[jmsms.NodeSummary]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ nodesArgs) (*mcp.CallToolResult, any, error) {
		rows, err := c.List(ctx, jmsapi.ResNodes)
		if err != nil {
			return render.Err(err)
		}
		var out []jmsms.NodeSummary
		for _, n := range rows {
			out = append(out, jmsms.NodeSummary{
				ID:        jmsapi.Str(n["id"]),
				Name:      jmsapi.Str(n["name"]),
				FullValue: jmsapi.FieldStr(n["full_value"]),
				ParentID:  jmsapi.FieldStr(n["parent_id"]),
				Assets:    jmsapi.FieldInt32(n["assets_amount"]),
			})
		}
		return render.Text(render.ListOf[jmsms.NodeSummary]{
			ListMeta: jms.ListMeta{Count: int32(len(out)), Total: int32(len(rows)), Truncated: false},
			Items:    out,
		})
	})

	type permsArgs struct {
		Limit int `json:"limit,omitempty" jsonschema:"max rows (default 50)"`
	}
	reg.Add(r, &mcp.Tool{
		Name:         "jms_permissions_list",
		Description:  "List asset permissions (who may reach which asset). Use for access-review and least-privilege audits. It reports counts, so use jms_permission_get when you need to know WHO is in one permission.",
		OutputSchema: render.Schema[render.ListOf[jmsms.PermissionSummary]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args permsArgs) (*mcp.CallToolResult, any, error) {
		limit := args.Limit
		limit = repository.NormLimit(limit, 50, 200)
		q := url.Values{"page_size": {convert.AnyToStr(limit)}, "ordering": {"-date_created"}}
		rows, err := c.ListQuery(ctx, jmsapi.ResPermissions, q)
		if err != nil {
			return render.Err(err)
		}
		var out []jmsms.PermissionSummary
		for _, p := range rows {
			// The endpoint ignores page_size and answers with its own cap of 200
			// rows, so the limit has to be applied here.
			if len(out) >= limit {
				break
			}
			// The list endpoint returns assets as null; the counts are what an
			// access review actually needs.
			out = append(out, jmsms.PermissionSummary{
				Name:         jmsapi.Str(p["name"]),
				IsActive:     jmsapi.FieldBool(p["is_active"]),
				AssetsAmount: jmsapi.FieldInt32(p["assets_amount"]),
				UsersAmount:  jmsapi.FieldInt32(p["users_amount"]),
				NodesAmount:  jmsapi.FieldInt32(p["nodes_amount"]),
				Accounts:     asStringSlice(p["accounts"]),
				DateCreated:  jmsapi.FieldStr(p["date_created"]),
			})
		}
		res := render.ListOf[jmsms.PermissionSummary]{
			ListMeta: jms.ListMeta{
				Count:     int32(len(out)),
				Total:     int32(len(rows)),
				Truncated: len(out) < len(rows),
			},
			Items: out,
		}
		if len(out) >= limit && len(rows) > len(out) {
			res.Note = "showing the first " + convert.AnyToStr(len(out)) + " of " +
				convert.AnyToStr(len(rows)) + " permissions"
		}
		return render.Text(res)
	})
}

// fetchRaw reads a collection as raw JSON.
//
// The status tools want the raw envelope rather than decoded rows: they
// count and group, and they must survive a shape they do not understand.
func fetchRaw(c *repository.Client, ctx context.Context, res jmsapi.Resource) json.RawMessage {
	var raw json.RawMessage
	if err := c.GetQuery(ctx, res, url.Values{"page_size": {"500"}}, &raw); err != nil {
		return nil
	}
	return raw
}

// assetDetail turns a bastion asset record into the contract type.
//
// Both jms_asset_get and jms_asset_create answer with it: "an asset" is one
// shape, and the record a create call just wrote is the same shape.
func assetDetail(m map[string]any) *jmsms.AssetDetail {
	// The tree position is the path a human reads, so the node's full_value
	// is what goes in; the leaf name is the fallback.
	nodes := []string{}
	for _, n := range jmsapi.AsSlice(m["nodes"]) {
		nm, ok := n.(map[string]any)
		if !ok {
			continue
		}
		path := jmsapi.Str(nm["full_value"])
		if path == "" {
			path = jmsapi.Str(nm["name"])
		}
		if path != "" {
			nodes = append(nodes, path)
		}
	}
	// The org comes as the id itself on some records and as the whole object
	// on others.
	orgID := jmsapi.Str(m["org_id"])
	if om, ok := m["org_id"].(map[string]any); ok {
		orgID = jmsapi.Str(om["id"])
	}
	orgName := jmsapi.FieldName(m, "org_name")
	if orgName == "" {
		orgName = jmsapi.FieldName(m, "org")
	}
	// What it runs on, how it is reached, what it is tagged with and who may
	// log into it.
	protocols := []*jmsms.ProtocolRef{}
	for _, p := range jmsapi.AsSlice(m["protocols"]) {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		protocols = append(protocols, &jmsms.ProtocolRef{
			Name: jms.Opt(jmsapi.Str(pm["name"])),
			Port: jmsapi.FieldInt32(pm["port"]),
		})
	}
	labels := []string{}
	for _, l := range jmsapi.AsSlice(m["labels"]) {
		lm, ok := l.(map[string]any)
		if !ok {
			continue
		}
		name := jmsapi.Str(lm["name"])
		if name == "" {
			continue
		}
		if v := jmsapi.Str(lm["value"]); v != "" && v != name {
			name += ":" + v
		}
		labels = append(labels, name)
	}
	accounts := []string{}
	for _, a := range jmsapi.AsSlice(m["accounts"]) {
		am, ok := a.(map[string]any)
		if !ok {
			continue
		}
		if u := jmsapi.Str(am["username"]); u != "" {
			accounts = append(accounts, u)
		}
	}
	return &jmsms.AssetDetail{
		ID:             jmsapi.Str(m["id"]),
		Name:           jmsapi.Str(m["name"]),
		Address:        jmsapi.Str(m["address"]),
		AccountsAmount: jmsapi.FieldInt32(m["accounts_amount"]),
		IsActive:       jmsapi.FieldBool(m["is_active"]),
		Comment:        jmsapi.FieldStr(m["comment"]),
		CreatedBy:      jmsapi.FieldStr(jmsapi.FieldName(m, "created_by")),
		DateCreated:    jmsapi.FieldStr(m["date_created"]),
		DateUpdated:    jmsapi.FieldStr(m["date_updated"]),
		DateVerified:   jmsapi.FieldStr(m["date_verified"]),
		DateLastLogin:  jmsapi.FieldStr(m["date_last_login"]),
		OrgID:          jmsapi.FieldStr(orgID),
		OrgName:        jmsapi.FieldStr(orgName),
		NodesDisplay:   nodes,
		Platform:       jmsapi.FieldStr(jmsapi.Nested(m, "platform", "name")),
		Protocols:      protocols,
		Connectivity:   jmsapi.FieldStr(jmsapi.Label(m, "connectivity")),
		Category:       jmsapi.FieldStr(jmsapi.Nested(m, "category", "value")),
		Labels:         labels,
		Accounts:       accounts,
	}
}
