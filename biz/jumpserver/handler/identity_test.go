package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/policy"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/client"
)

// identityMode selects how the fake bastion answers the two probes.
type identityMode int

const (
	// profile answers /users/profile/ with the caller (the happy path).
	identityHappy identityMode = iota
	// profile 404s: the probe failed.
	identityProbe404
	// profile answers 200 but with NO username — a silent-empty success.
	identityNoUsername
	// the users LIST answers 403 (no users.view_user) while the profile
	// probe also fails: auth must NOT be blamed for a permission gap.
	identityList403
	// the users LIST answers 401 (the credential is genuinely rejected).
	identityList401
)

// fakeBastionWithIdentity answers the identity probe with THE CALLER while
// the users LIST orders a different user first — the exact shape that made
// jms_status name admin while a probe account was calling (measured on a
// real v4, 2026-11-01). If a test only ever shows both as the same user,
// the first-row heuristic passes and the bug stays invisible.
func fakeBastionWithIdentity(t *testing.T, mode identityMode) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/users/profile/":
			switch mode {
			case identityProbe404, identityList403, identityList401:
				// The probe fails, so status falls back to the users LIST to
				// settle auth — and in identityList403/401 that list answers
				// 403 / 401.
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"detail":"not found"}`))
			case identityNoUsername:
				// 200, but the answer carries no username. Reporting this as
				// an identity of "" is the silent-success bug: it must be a
				// failed probe instead.
				json.NewEncoder(w).Encode(map[string]any{
					"id": "u-probe", "name": "Probe User", "is_superuser": false,
				})
			default:
				json.NewEncoder(w).Encode(map[string]any{
					"id": "u-probe", "username": "zz_probe", "name": "Probe User",
					"is_superuser": false,
				})
			}
		case "/api/v1/users/users/":
			if mode == identityList403 {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"detail":"you do not have permission"}`))
				return
			}
			if mode == identityList401 {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"detail":"invalid token"}`))
				return
			}
			// The LIST's first row is the admin — deliberately NOT the caller.
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "u-admin", "username": "admin", "name": "Administrator",
					"is_superuser": true},
				{"id": "u-probe", "username": "zz_probe", "name": "Probe User",
					"is_superuser": false},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"detail":"not found"}`))
		}
	})
}

// identitySession registers just the two tools under test on their own fake.
func identitySession(t *testing.T, handler http.Handler) *mcp.ClientSession {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	c := client.NewBase(ts.URL, "test-token", ts.Client())
	return testsupport.NewSession(t, "jms-test", reg.Options{Policy: policy.Table},
		func(r *reg.Registry) {
			RegisterStatusTools(r, c)
			RegisterOnboardingTools(r, c)
		})
}

// The status identity is the credential's own user, NOT whoever the users
// list happens to order first.
func TestStatusNamesTheCallerNotTheFirstRow(t *testing.T) {
	cs := identitySession(t, fakeBastionWithIdentity(t, identityHappy))
	out := testsupport.CallToolJSON(t, cs, "jms_status", map[string]any{})

	if out["user"] != "zz_probe" {
		t.Errorf("user = %v, want zz_probe (the caller); the list's first row "+
			"was admin — a first-row identity is what this test holds shut", out["user"])
	}
	if out["name"] != "Probe User" {
		t.Errorf("name = %v, want Probe User", out["name"])
	}
}

// When the identity probe fails, the tools say WHY the identity is missing.
// They must not fall back to the list's first row: "admin" would be a lie.
func TestStatusSaysIdentityUnknownRatherThanGuessing(t *testing.T) {
	cs := identitySession(t, fakeBastionWithIdentity(t, identityProbe404))
	out := testsupport.CallToolJSON(t, cs, "jms_status", map[string]any{})

	if got, _ := out["user"].(string); got != "" {
		t.Errorf("user = %v, want empty: the identity was not established", got)
	}
	if got, _ := out["name"].(string); got != "" {
		t.Errorf("name = %v, want empty: the identity was not established", got)
	}
	errText, _ := out["error"].(string)
	if !strings.Contains(errText, "identity probe") {
		t.Errorf("error = %q, want it to say the identity probe failed", errText)
	}
	// The credential itself is fine (the list answered) — auth and identity
	// are two questions and the answer keeps them apart.
	if ok, _ := out["auth_ok"].(bool); !ok {
		t.Errorf("auth_ok = %v, want true: the users list answered, only the "+
			"identity probe failed", ok)
	}
}

// The doctor's identity row has the same failure mode and the same fix.
func TestDoctorNamesTheCallerNotTheFirstRow(t *testing.T) {
	cs := identitySession(t, fakeBastionWithIdentity(t, identityHappy))
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "jms_doctor", Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call jms_doctor: %v", err)
	}
	if res.IsError {
		t.Fatalf("jms_doctor errored: %s", testsupport.ToolText(t, res))
	}
	text := testsupport.ToolText(t, res)
	if !strings.Contains(text, "acting as zz_probe") {
		t.Errorf("doctor does not name the caller (zz_probe): %s", text)
	}
	if strings.Contains(text, "acting as admin") {
		t.Errorf("doctor named the list's first row (admin) instead of the caller: %s", text)
	}

	// The structured identity block carries the same fact.
	out := testsupport.CallToolJSON(t, cs, "jms_doctor", map[string]any{})
	if ident, ok := out["identity"].(map[string]any); ok {
		if ident["username"] != "zz_probe" {
			t.Errorf("identity.username = %v, want zz_probe", ident["username"])
		}
	} else {
		t.Errorf("identity = %v, want the IdentityRef block", out["identity"])
	}
}

// A 200 with no username is a FAILED probe, not an identity of "". Reporting
// it as a silent success made jms_status answer auth_ok=true with a blank
// user/name and the doctor print "acting as " — a caller named as nobody.
func TestStatusTreatsEmptyUsernameAsAFailedProbe(t *testing.T) {
	cs := identitySession(t, fakeBastionWithIdentity(t, identityNoUsername))
	out := testsupport.CallToolJSON(t, cs, "jms_status", map[string]any{})

	if got, _ := out["user"].(string); got != "" {
		t.Errorf("user = %q, want empty: a probe answer with no username is not an identity", got)
	}
	if got, _ := out["name"].(string); got != "" {
		t.Errorf("name = %q, want empty: a probe answer with no username is not an identity", got)
	}
	errText, _ := out["error"].(string)
	if !strings.Contains(errText, "no username") {
		t.Errorf("error = %q, want it to say the probe answer carried no username", errText)
	}
}

// A 403 on the fallback users LIST is a permission gap, not a bad
// credential. auth_ok must stay true — blaming the credential would send the
// operator to rotate a token that is fine. Only a real 401 reads as auth
// failure.
func TestStatusDoesNotBlameTheCredentialForAPermissionGap(t *testing.T) {
	cs := identitySession(t, fakeBastionWithIdentity(t, identityList403))
	out := testsupport.CallToolJSON(t, cs, "jms_status", map[string]any{})

	if ok, _ := out["auth_ok"].(bool); !ok {
		t.Errorf("auth_ok = %v, want true: a 403 on the users list is a permission gap, "+
			"not a rejected credential", ok)
	}
	errText, _ := out["error"].(string)
	if !strings.Contains(errText, "identity") {
		t.Errorf("error = %q, want it to explain the identity is missing", errText)
	}
}

// A real 401 IS a rejected credential: auth_ok=false and the error says the
// credential was rejected. This is the one case auth is genuinely false —
// the mirror of the 403 permission-gap test.
func TestStatusSaysAuthFailedOnAReal401(t *testing.T) {
	cs := identitySession(t, fakeBastionWithIdentity(t, identityList401))
	out := testsupport.CallToolJSON(t, cs, "jms_status", map[string]any{})

	if ok, _ := out["auth_ok"].(bool); ok {
		t.Errorf("auth_ok = %v, want false: a 401 is a rejected credential", ok)
	}
	errText, _ := out["error"].(string)
	if !strings.Contains(errText, "rejected") {
		t.Errorf("error = %q, want it to say the credential was rejected", errText)
	}
}

// identitySessionDead builds the same tools against a server that is already
// closed, so every request fails in TRANSPORT (connection refused) — the API
// never answers. The token is never tested, so auth_ok must NOT be asserted
// true ("accepted"); it folds to false with an explicit "this is not a
// verdict on the credential".
func identitySessionDead(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := ts.URL
	ts.Close() // now unreachable: connection refused
	c := client.NewBase(url, "test-token", http.DefaultClient)
	return testsupport.NewSession(t, "jms-test", reg.Options{Policy: policy.Table},
		func(r *reg.Registry) {
			RegisterStatusTools(r, c)
			RegisterOnboardingTools(r, c)
		})
}

// When the bastion is unreachable, the answer must not claim the credential
// was accepted — that is a fabrication of a fact no probe established (the
// same sin as inventing an identity). auth_ok folds to false (not proven able
// to call the API) and the error says plainly it is NOT a credential verdict.
func TestStatusDoesNotClaimAcceptedWhenUnreachable(t *testing.T) {
	cs := identitySessionDead(t)
	out := testsupport.CallToolJSON(t, cs, "jms_status", map[string]any{})

	if ok, _ := out["auth_ok"].(bool); ok {
		t.Errorf("auth_ok = %v, want false: an unreachable bastion never proved the token works", ok)
	}
	errText, _ := out["error"].(string)
	if strings.Contains(errText, "accepted") {
		t.Errorf("error claims the credential was accepted (%q) although it was never tested", errText)
	}
	if !strings.Contains(errText, "NOT a verdict") {
		t.Errorf("error = %q, want it to say this is not a verdict on the credential", errText)
	}
}

// When the identity probe fails, the doctor's IdentityRef must have BOTH
// fields NIL — not a fabricated {"is_superuser":false, "username":""}.
// root.go's "print the line only once it is known" guard keys off these being
// non-nil, so a fabricated zero value both lies and defeats the guard. This
// checks the STRUCT (the CLI reads the struct; the JSON wire collapses the
// distinction to {} either way), so a regression here is caught where it does
// real damage.
func TestDoctorIdentityBlockIsAbsentWhenTheProbeFails(t *testing.T) {
	// identityNoUsername: the probe answers 200 but with no username, so the
	// reachability check PASSES (200) while identityOf fails — the one path
	// that reaches the doctor's "identity unknown" branch. (A plain 404 bails
	// out earlier at api_reachable, so it never reaches this branch at all.)
	ts := httptest.NewServer(fakeBastionWithIdentity(t, identityNoUsername))
	t.Cleanup(ts.Close)
	c := client.NewBase(ts.URL, "test-token", ts.Client())

	report, err := Doctor(context.Background(), c, false)
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	if report.Identity.Username != nil {
		t.Errorf("identity.username = %v, want nil when the identity was not established (no fabricated empty name)",
			*report.Identity.Username)
	}
	if report.Identity.IsSuperuser != nil {
		t.Errorf("identity.is_superuser = %v, want nil when the identity was not established (no fabricated false)",
			*report.Identity.IsSuperuser)
	}
}
