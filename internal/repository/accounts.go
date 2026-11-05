// Part of the management plane's repository over the JumpServer API.
//
// The transport and the parsing of the two response shapes v5 mixes live in
// jms-kit/pkg/jmsapi; what is here is the questions the tools ask and the
// resources they get back.
package repository

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/oekit/pkg/convert"
	"github.com/yi-nology/jumpserver-kit/pkg/jmsapi"
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
	// The report is HTML, not JSON, so it goes through the binary fetch —
	// same reason as DownloadReplayText: the credential's own header, and a
	// refused credential reported as itself.
	//
	// Streamed through a scratch file rather than held as one []byte: the
	// report size is not bounded by the API, and pulls run alongside the
	// evidence replay downloads — one less whole-body buffer in RAM when
	// several run together. HTMLToText needs the whole document anyway, so
	// the file is read back in one go; the win is the transfer never
	// doubles the peak.
	tmp, err := os.CreateTemp("", "jms-gather-*.html")
	if err != nil {
		return "", fmt.Errorf("scratch file for gather report: %w", err)
	}
	path := tmp.Name()
	defer func() { _ = tmp.Close(); _ = os.Remove(path) }()
	if _, err := c.GetBinaryTo(ctx, string(jmsapi.ResGatherAdhoc)+"?asset_id="+url.QueryEscape(assetID), tmp); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read gather report: %w", err)
	}
	return jmsapi.HTMLToText(raw), nil
}
