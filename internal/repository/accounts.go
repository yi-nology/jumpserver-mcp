// Part of the management plane's repository over the JumpServer API.
//
// The transport and the parsing of the two response shapes v5 mixes live in
// jms-kit/pkg/jmsapi; what is here is the questions the tools ask and the
// resources they get back.
package repository

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/oekit/pkg/convert"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
)

// AccountID finds the id of the account with this username on an asset.
//
// The lookup lives in pkg/jmsapi because the execution plane needs the same
// thing: a connection token's body carries the account's UUID, not its name.
func (c *Client) AccountID(ctx context.Context, assetID, username string) (string, error) {
	accts, err := c.Client.AssetAccounts(ctx, assetID)
	if err != nil {
		return "", err
	}
	names := []string{}
	for _, a := range accts {
		names = append(names, a.Username)
		if strings.EqualFold(a.Username, username) {
			return a.ID, nil
		}
	}
	return "", fmt.Errorf("no account %q on that asset (have: %s)", username, strings.Join(names, ", "))
}

func (c *Client) GatheredRows(ctx context.Context, assetID string) ([]map[string]any, error) {
	// ⚠️ The filter parameter is `asset_id`, NOT `asset` — measured
	// 2026-11-03: the endpoint IGNORES `asset=` and returns the whole
	// org's rows (only `asset_id=` filters). With the wrong name this
	// returned OTHER assets' gathered rows and jms_gather_apply then
	// confirmed foreign rows on foreign assets.
	q := url.Values{"page_size": {"500"}, "asset_id": {assetID}}
	rows, err := c.ListQuery(ctx, jmsapi.ResGathered, q)
	if err != nil {
		return nil, err
	}
	// The parameter is the REQUEST; this filter is the VERIFICATION. The
	// endpoint silently ignored `asset=` once already — the same silence
	// with `asset_id=` would reopen the cross-asset hole with no red guard
	// anywhere. Every row carries its asset id (measured), so a row that
	// says it belongs to another asset is dropped here no matter what the
	// server did with the query. A row that names no asset passes: it
	// cannot be attributed, and dropping data on a guess is the worse lie.
	out := rows[:0]
	for _, m := range rows {
		if id := jmsapi.Str(jmsapi.Nested(m, "asset", "id")); id != "" && id != assetID {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (c *Client) GatheredUsernames(ctx context.Context, assetID string) map[string]bool {
	out := map[string]bool{}
	rows, err := c.GatheredRows(ctx, assetID)
	if err != nil {
		return out
	}
	for _, m := range rows {
		if u := convert.AnyToStr(m["username"]); u != "" {
			out[u] = true
		}
	}
	return out
}

func (c *Client) GatherAdhoc(ctx context.Context, assetID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		c.BaseURL+"/api/v1"+string(jmsapi.ResGatherAdhoc)+"?asset_id="+assetID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Token "+c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d from the adhoc gather endpoint: %s",
			resp.StatusCode, jmsapi.Truncate(string(raw), 200))
	}
	return jmsapi.HTMLToText(raw), nil
}
