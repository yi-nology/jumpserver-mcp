package handler

// identityOf asks the bastion who the credential acts as.
//
// It reads /users/profile/ — the one route that answers with the CALLER.
// ⚠️ The old probe took the FIRST ROW OF /users/users/ and called that the
// identity; it is only ever right when the caller happens to be the user the
// list orders first (admin on a fresh deployment). A non-admin caller was
// reported as admin — measured on v4 with a probe account (jms_status said
// "admin" while the credential acted as "zz_mcp_probe"). The first row of a
// list is a list fact, not an identity fact.
// ⚠️ /users/users/profile/ and /me/ do NOT exist in v5 (404) — the earlier
// belief that "v5 has no identity endpoint" came from probing only those
// two. /users/profile/ answers 200 there (measured 2026-11-01).
//
// On failure the caller must say "identity unknown" rather than fall back to
// the list heuristic: the contract's `user` field promises 当前身份, and a
// wrong name is worse than a missing one.
import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yi-nology/jumpserver-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
)

// identityOf returns the credential's own username, display name and
// superuser flag. err != nil means the identity could not be established —
// it says why; callers must not invent a name.
func identityOf(ctx context.Context, c *repository.Client) (who, name string, super bool, err error) {
	var raw json.RawMessage
	if gerr := c.GetQuery(ctx, jmsapi.ResUsersProfile, nil, &raw); gerr != nil {
		return "", "", false, fmt.Errorf("identity probe failed: %w", gerr)
	}
	var u map[string]any
	if uerr := json.Unmarshal(raw, &u); uerr != nil {
		return "", "", false, fmt.Errorf("identity probe: cannot read the answer: %w", uerr)
	}
	// A 200 with no username is still a failed probe, not an identity of "".
	// Returning an empty who here used to make callers report auth_ok=true
	// with every identity field blank and the doctor print "acting as " — a
	// silent success that looked like an empty caller. The name is required
	// to answer; without it there is nothing to report, so it is an error.
	who = jmsapi.Str(u["username"])
	if who == "" {
		return "", "", false, fmt.Errorf("identity probe: the answer carried no username")
	}
	super, _ = u["is_superuser"].(bool)
	return who, jmsapi.Str(u["name"]), super, nil
}
