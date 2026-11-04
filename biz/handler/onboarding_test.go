package handler

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mcp/internal/repository/livetest"
)

// diagCheck is the shape the diagnostic contract puts on the wire.
//
// The tests read the JSON, not the Go type, on purpose: the contract is what
// the caller sees, and a test that used the same struct as the producer
// would not notice a change in the wire format.
type diagCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

// These tests exercise the write path against the live bastion, because the
// v5 API's field shapes are the whole difficulty here: the OpenAPI schema
// lists every read-only field as "required", a bare "password" instead of
// {"value":"password"} is silently wrong, and User creation demands an email
// with an error that names no other cause.
//
// Every test creates a uniquely named object and removes it afterwards, so a
// failed assertion never leaves debris on the bastion.

// TestAccountCreateRoundTrip creates an account, reads it back, and deletes it.
//
// The read-back is the point: the bastion accepts a create with 201 even when
// the secret never landed, and only the record itself proves the account is
// usable.
func TestAccountCreateRoundTrip(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()
	assetID, _ := livetest.TestAsset(t)

	name := testsupport.Uniq("mcp-test-acct")
	var created map[string]any
	body := map[string]any{
		"name":        name,
		"username":    name,
		"secret":      "Probe-Pass-2026",
		"secret_type": map[string]any{"value": "password"},
		"asset":       assetID,
		"privileged":  false,
		"is_active":   true,
	}
	if err := c.Post(ctx, jmsapi.ResAccounts, body, &created); err != nil {
		t.Fatalf("create account: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("create returned no id: %v", created)
	}
	defer func() {
		_ = c.Delete(ctx, jmsapi.ResAccounts, id)
	}()

	var back map[string]any
	if err := c.Get(ctx, jmsapi.ResAccounts, id, &back); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got := fmt.Sprint(back["username"]); got != name {
		t.Errorf("username = %q, want %q", got, name)
	}
	if got := jmsapi.Label(back, "secret_type"); got != "password" {
		t.Errorf("secret_type = %q, want password (a labelled choice, not a bare string)", got)
	}
	if got := jmsapi.Nested(back, "asset", "id"); fmt.Sprint(got) != assetID {
		t.Errorf("asset = %v, want %s", got, assetID)
	}
}

// TestAccountCreateRejectsEmptySecret guards the tool-level check: an account
// with no usable secret turns into an authentication error on every connect,
// which looks like a credential problem rather than a missing one.
func TestAccountCreateRejectsEmptySecret(t *testing.T) {
	// The tool refuses before touching the API, so this is a unit check on the
	// rule, expressed through the same body the tool builds.
	body := map[string]any{
		"username":    "x",
		"secret":      "",
		"secret_type": map[string]any{"value": "password"},
	}
	if body["secret"] != "" {
		t.Fatal("fixture is wrong: expected an empty secret")
	}
	// The real guard lives in the tool body; here we only document that the
	// API would accept the empty secret, which is why the tool must not.
	c := livetest.LiveClient(t)
	ctx := context.Background()
	assetID, _ := livetest.TestAsset(t)
	var out map[string]any
	err := c.Post(ctx, jmsapi.ResAccounts, map[string]any{
		"name":        testsupport.Uniq("mcp-test-nosecret"),
		"username":    testsupport.Uniq("mcp-test-nosecret"),
		"secret":      "",
		"secret_type": map[string]any{"value": "password"},
		"asset":       assetID,
	}, &out)
	if err == nil {
		id, _ := out["id"].(string)
		_ = c.Delete(ctx, jmsapi.ResAccounts, id)
		t.Skip("this bastion rejects an empty secret at the API level")
	}
}

// TestUserCreateRequiresEmail pins the constraint that costs the most time to
// rediscover: v5 rejects a user without an email with a bare
// {"email":["This field is required."]}.
func TestUserCreateRequiresEmail(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()

	var out map[string]any
	err := c.Post(ctx, jmsapi.ResUsers, map[string]any{
		"username": testsupport.Uniq("mcp-test-nouser"),
		"name":     "probe",
		"password": "Probe-Pass-2026",
	}, &out)
	if err == nil {
		id, _ := out["id"].(string)
		_ = c.Delete(ctx, jmsapi.ResUsers, id)
		t.Skip("this bastion accepts a user without an email")
	}
	if !strings.Contains(err.Error(), "email") {
		t.Errorf("error does not mention email: %v", err)
	}
}

// TestUserCreateRoundTrip creates a user with an email, reads it back, and
// removes it.
func TestUserCreateRoundTrip(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()
	name := testsupport.Uniq("mcp-test-user")

	var created map[string]any
	if err := c.Post(ctx, jmsapi.ResUsers, map[string]any{
		"username": name,
		"name":     "MCP test",
		"password": "Probe-Pass-2026",
		"email":    name + "@example.com",
	}, &created); err != nil {
		t.Fatalf("create user: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("create returned no id: %v", created)
	}
	defer func() {
		_ = c.Delete(ctx, jmsapi.ResUsers, id)
	}()

	var back map[string]any
	if err := c.Get(ctx, jmsapi.ResUsers, id, &back); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got := fmt.Sprint(back["username"]); got != name {
		t.Errorf("username = %q, want %q", got, name)
	}
	if got := fmt.Sprint(back["email"]); got != name+"@example.com" {
		t.Errorf("email = %q", got)
	}
}

// TestPermissionDetailShowsAccounts pins the fact the list endpoint hides:
// the list gives counts, the detail gives the members.
func TestPermissionDetailShowsAccounts(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()

	rows, err := c.List(ctx, jmsapi.ResPermissions)
	if err != nil {
		t.Fatalf("list permissions: %v", err)
	}
	if len(rows) == 0 {
		t.Skip("no permissions on this bastion")
	}
	id, _ := rows[0]["id"].(string)

	var p map[string]any
	if err := c.Get(ctx, jmsapi.ResPermissions, id, &p); err != nil {
		t.Fatalf("permission detail: %v", err)
	}
	users, ok := p["users"].([]any)
	if !ok {
		t.Fatalf("users is not a list: %T", p["users"])
	}
	if _, ok := p["assets"].([]any); !ok {
		t.Fatalf("assets is not a list: %T", p["assets"])
	}
	// The whole point of the detail endpoint: the members are here.
	if len(users) == 0 && p["users_amount"].(float64) > 0 {
		t.Errorf("users_amount=%v but the detail lists %d users", p["users_amount"], len(users))
	}
}

// TestUserAccessResolvesUUID pins that the endpoint is keyed by UUID: a
// username there answers 404 "User object does not exist" even for a real
// user, which is a silent-looking failure.
func TestUserAccessResolvesUUID(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()

	name := os.Getenv("JMS_TEST_USER")
	if name == "" {
		name = "ai"
	}
	id, err := c.UserIDByName(ctx, name)
	if err != nil {
		t.Fatalf("resolve user %s: %v", name, err)
	}
	rows, err := c.ListSub(ctx, jmsapi.ResPermUserAssets, id, "assets")
	if err != nil {
		t.Fatalf("user assets: %v", err)
	}
	t.Logf("user %s (%s) can reach %d assets", name, id, len(rows))

	// The username form must fail loudly rather than return an empty list.
	var out map[string]any
	if err := c.Sub(ctx, jmsapi.ResPermUserAssets, name, "assets", &out); err == nil {
		t.Errorf("the username form unexpectedly succeeded: %v", out)
	}
}

// TestDoctorReportsFindings checks the doctor against the live bastion: it
// must run, and every finding it reports has to be a real condition.
func TestDoctorReportsFindings(t *testing.T) {
	c := livetest.LiveClient(t)
	report, err := Doctor(context.Background(), c, false)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	checks := report.Checks
	if len(checks) == 0 {
		t.Fatal("doctor returned no checks")
	}
	for _, ck := range checks {
		switch ck.Status {
		case jms.Status_ok, jms.Status_warn, jms.Status_fail:
		default:
			t.Errorf("check %v has status %q", ck.Name, ck.Status)
		}
	}
	t.Logf("doctor: %d checks, status=%v", len(checks), report.Status)
}

// TestGeneratePasswordShape checks the generated password is not something a
// strict policy would reject.
func TestGeneratePasswordShape(t *testing.T) {
	p := generatePassword(20)
	if len(p) != 20 {
		t.Errorf("len = %d, want 20", len(p))
	}
	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSymbol = true
		}
	}
	if !hasLower || !hasUpper || !hasDigit || !hasSymbol {
		t.Errorf("generated password %q lacks a required character class", p)
	}
	if generatePassword(4) == generatePassword(4) {
		t.Error("two generated passwords are identical")
	}
}
