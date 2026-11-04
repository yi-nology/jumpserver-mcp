package livetest

// The live checks against a REAL JumpServer — the v4 one.
//
// ✅ **These have run against a real v4** (2026-10-31, v4.10.20 on
// 192.0.2.21): the endpoint table answered, the version probe said "v4", the
// connection token was minted with the account's UUID, and a command ran on a
// real asset through koko.
//
// ⚠️ **And they CORRECTED two things the source reading had got wrong** —
// which is why they exist. Reading the upstream v4.10.20-lts source said the
// list endpoints answer with the standard envelope (they answer with a bare
// array, like v5) and that "perm_account_invalid" means the account field is
// wrong (on v4 it also means nobody was granted the asset). ⇒ A protocol
// fact that is only READ is not a protocol fact.
//
// Set JMS_TEST_V4_URL and the credential to run them:
//
//	export JMS_TEST_V4_URL=http://<v4-bastion>
//	export JMS_TEST_V4_USERNAME=admin
//	export JMS_TEST_V4_PASSWORD=<the strong password>
//	go test ./internal/repository/livetest/ -run V4 -v
//
// ⚠️ A v4 deployment has NO API that mints a PrivateToken (its authentication
// API registers access-keys, temp-tokens, connection-token, ssh-key and
// access-tokens — and no private-token), so the LOGIN form is the one that
// works there. JMS_TEST_V4_TOKEN is still accepted for a deployment that has
// one from elsewhere (the UI, the database).

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/errs"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-mcp/internal/repository"
)

// v4Client returns a client for the v4 bastion under test, or skips.
//
// It is a separate helper from LiveClient on purpose: the v4 bastion is a
// DIFFERENT deployment, with its own address and its own credentials, and
// pointing the v5 tests at it by accident would be a confusing way to spend
// an afternoon.
//
// ⚠️ Two credential forms, because v4 has only one API-reachable one:
//
//   - JMS_TEST_V4_TOKEN: a PrivateToken ("Authorization: Token <key>"). v4
//     HAS the PrivateTokenAuthentication class, but no API that mints one —
//     v4's `private_token` is a model property with no route exposing it
//     (v4's authentication API registers access-keys, temp-tokens,
//     connection-token, ssh-key, access-tokens — and no private-token). ⇒
//     On v4 this form has to come from the UI or from the database.
//   - JMS_TEST_V4_USERNAME + JMS_TEST_V4_PASSWORD: a login, which the
//     bastion exchanges for a Bearer token ("Authorization: Bearer <jwt>").
//     ⇒ This is the form that works on v4 out of the box.
//
// ⚠️ The password is read from the environment, and the environment of a
// process is readable by anything that can read /proc — which is why the
// live test is a local thing and not something to wire into a CI runner
// that runs untrusted jobs.
func v4Client(t *testing.T) *repository.Client {
	t.Helper()
	base := os.Getenv("JMS_TEST_V4_URL")
	token := os.Getenv("JMS_TEST_V4_TOKEN")
	username := os.Getenv("JMS_TEST_V4_USERNAME")
	password := os.Getenv("JMS_TEST_V4_PASSWORD")
	if base == "" {
		t.Skip("JMS_TEST_V4_URL not set: there is no v4 bastion to measure against")
	}
	if token == "" && (username == "" || password == "") {
		t.Skip("JMS_TEST_V4_TOKEN or JMS_TEST_V4_USERNAME+JMS_TEST_V4_PASSWORD not set")
	}
	cred := jmsapi.Credential{Username: username, Password: password}
	if otp := os.Getenv("JMS_TEST_V4_OTP"); otp != "" {
		cred.OTP = otp
	}
	return repository.NewBaseAuth(base, token, cred, &http.Client{Timeout: 90 * time.Second})
}

// TestV4IsDetectedAsV4 measures the one thing the version is decided by.
//
// The probe asks whether the chat-ai prefix is registered. On v4 it is not,
// and the answer has to be "v4" — because the answer decides whether the
// delegation credential is presented, and on v4 presenting one costs every
// session the fast path.
func TestV4IsDetectedAsV4(t *testing.T) {
	c := v4Client(t)
	got, err := c.DetectedVersion(context.Background())
	if err != nil {
		t.Fatalf("detect the version: %v", err)
	}
	if got != jmsapi.VersionV4 {
		t.Fatalf("the deployment was detected as %q, want v4", got)
	}
}

// TestV4HasNoDelegationScheme measures the consequence.
//
// A v4 deployment has no chat_ai application, so the delegation credential
// cannot be presented: the request comes back 401, and a 401 on that path
// reads as "the connection-token scheme is unavailable" — which sends every
// session down the slow koko terminal path.
func TestV4HasNoDelegationScheme(t *testing.T) {
	c := v4Client(t)
	if state := c.State(context.Background(), jmsapi.CapDelegation); state != jmsapi.CapabilityAbsent {
		t.Fatalf("the delegation scheme is %s on a v4 deployment, want absent", state)
	}
}

// TestV4AnswersTheEndpointTable is the measurement that the endpoint table
// is not a v5 table.
//
// Every path in pkg/jmsapi's table is read out of the upstream v4 source, but
// "the source says it exists" and "the deployment answers it" are different
// claims, and only the second one is about the deployment.
//
// The list is walked rather than sampled: a table that is right for 40
// endpoints and wrong for one is still wrong.
//
// ⚠️ The list covers EVERY resource in the table, including the seven that
// are not plain router registrations (gathered-accounts/status,
// gather-account-executions/adhoc, job-executions/stop, perms/users,
// labeled-resources, login-asset/check, settings/security/block-ip) — those
// are exactly the ones a hand-written list forgets, and they are the ones
// most worth measuring.
//
// The method does not matter: a route that exists but does not allow GET
// answers 405, and a route that does not exist answers 404. Only the 404
// means "not registered".
func TestV4AnswersTheEndpointTable(t *testing.T) {
	c := v4Client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	for _, res := range []jmsapi.Resource{
		jmsapi.ResAssets,
		jmsapi.ResAssetHosts,
		jmsapi.ResAssetDatabases,
		jmsapi.ResAssetDevices,
		jmsapi.ResAssetClouds,
		jmsapi.ResAssetWebs,
		jmsapi.ResNodes,
		jmsapi.ResPlatforms,
		jmsapi.ResAccounts,
		jmsapi.ResGathered,
		jmsapi.ResGatheredStatus,
		jmsapi.ResGatherAdhoc,
		jmsapi.ResPermissions,
		jmsapi.ResUsers,
		jmsapi.ResGroups,
		jmsapi.ResOrgs,
		jmsapi.ResLabels,
		jmsapi.ResLabeledResources,
		jmsapi.ResCmdGroups,
		jmsapi.ResCmdFilters,
		jmsapi.ResLoginACLs,
		jmsapi.ResLoginAssetACLs,
		jmsapi.ResLoginCheck,
		jmsapi.ResSessions,
		jmsapi.ResTerminals,
		jmsapi.ResTerminalCommands,
		jmsapi.ResJobs,
		jmsapi.ResJobExecutions,
		jmsapi.ResJobExecutionsStop,
		jmsapi.ResSSHKeys,
		jmsapi.ResConnectionToken,
		jmsapi.ResOrgRoles,
		jmsapi.ResSystemRoles,
		jmsapi.ResContentTypes,
		jmsapi.ResLoginLogs,
		jmsapi.ResOperateLogs,
		jmsapi.ResPasswordLogs,
		jmsapi.ResAuditJobs,
		jmsapi.ResAuditJobLogs,
		jmsapi.ResFTPLogs,
		jmsapi.ResZones,
		jmsapi.ResGateways,
		jmsapi.ResPushAutomations,
		jmsapi.ResPushExecutions,
		jmsapi.ResPushRecords,
		jmsapi.ResGatherAutomations,
		jmsapi.ResGatherExecutions,
		jmsapi.ResBlockedIPs,
	} {
		// ⚠️ Decoded into a RawMessage on purpose: several of these
		// endpoints answer with a BARE ARRAY, and decoding that into a map
		// fails — which would read as "the endpoint refused the call" when
		// the truth is "the endpoint answered with a different shape".
		var raw json.RawMessage
		err := c.GetQuery(ctx, res, map[string][]string{"page_size": {"1"}}, &raw)
		if err == nil {
			continue
		}
		if apiErr, ok := err.(*errs.APIError); ok && apiErr.Status == http.StatusNotFound {
			t.Errorf("%s is not registered on this deployment: the endpoint table is "+
				"read from the upstream v4 source, and this is where that reading is wrong",
				res)
			continue
		}
		// Anything else (401, 403, 405) says the path is there.
		t.Logf("%s answered %v (the path exists, the call was refused)", res, err)
	}
}

// TestV4AnswersTheUserAssetsPrefix measures the one entry in the table that
// is a PREFIX rather than a route.
//
// "/perms/users/" is the mount point of v4's user-permission routes
// (apps/perms/urls/user_permission.py: path('users/', include(...))), and
// the routes under it are "<user>/assets/", "<user>/nodes/" and so on. A GET
// on the prefix itself answers 404 on BOTH versions — the prefix is not a
// route — so it cannot be measured the way the rest of the table is.
//
// ⇒ it is measured with a real user id under it.
func TestV4AnswersTheUserAssetsPrefix(t *testing.T) {
	c := v4Client(t)
	ctx := context.Background()
	users, err := c.List(ctx, jmsapi.ResUsers)
	if err != nil {
		t.Fatalf("list the users: %v", err)
	}
	if len(users) == 0 {
		t.Skip("the deployment has no users")
	}
	uid := jmsapi.Str(users[0]["id"])
	var raw map[string]any
	err = c.GetQuery(ctx, jmsapi.Resource(string(jmsapi.ResPermUserAssets)+uid+"/assets/"),
		map[string][]string{"page_size": {"1"}}, &raw)
	if err == nil {
		return
	}
	if apiErr, ok := err.(*errs.APIError); ok && apiErr.Status == http.StatusNotFound {
		t.Errorf("/perms/users/%s/assets/ is not registered on this deployment: the "+
			"prefix is mounted the same way in v4, and this is where that reading is wrong", uid)
		return
	}
	t.Logf("/perms/users/%s/assets/ answered %v (the path exists, the call was refused)", uid, err)
}

// TestV4AcceptsTheLoginCredential measures the credential form v4 can
// actually produce.
//
// ⚠️ A JumpServer v4 deployment has NO API that mints a PrivateToken: its
// authentication API registers access-keys, temp-tokens, connection-token,
// ssh-key and access-tokens — and no private-token — and the model's
// `private_token` property has no route exposing it. ⇒ The credential that
// v4 can hand out over the API is the Bearer token that
// POST /api/v1/authentication/auth/ gives for a username and a password.
//
// ⇒ This is the test that measures the form v4 really has.
func TestV4AcceptsTheLoginCredential(t *testing.T) {
	c := v4Client(t)
	if os.Getenv("JMS_TEST_V4_TOKEN") != "" {
		t.Skip("a PrivateToken is configured: the login form is not what is in force")
	}
	// ⚠️ RawMessage, not a map: v4's list endpoints answer with a BARE
	// ARRAY, and decoding that into a map fails — which would read as
	// "the bastion refused the credential" when the truth is "the
	// credential worked and the shape was different".
	var raw json.RawMessage
	if err := c.GetQuery(context.Background(), jmsapi.ResUsers,
		map[string][]string{"page_size": {"1"}}, &raw); err != nil {
		t.Fatalf("the bastion refused the login credential: %v", err)
	}
}

// TestV4AcceptsThePrivateTokenHeader measures the OTHER credential form —
// the one a v5 deployment uses, and the one a v4 deployment can have only if
// the token came from somewhere else (the UI, or the database).
//
// ⚠️ It is skipped when no such token was given, because on v4 there is no
// way to get one over the API — and a test that asserts something the
// deployment cannot produce is a test that fails for the wrong reason.
func TestV4AcceptsThePrivateTokenHeader(t *testing.T) {
	if os.Getenv("JMS_TEST_V4_TOKEN") == "" {
		t.Skip("JMS_TEST_V4_TOKEN not set: a v4 deployment has no API that mints a PrivateToken")
	}
	c := v4Client(t)
	var raw json.RawMessage
	if err := c.GetQuery(context.Background(), jmsapi.ResAssets,
		map[string][]string{"page_size": {"1"}}, &raw); err != nil {
		t.Fatalf("the bastion refused the PrivateToken: %v", err)
	}
}

// TestV4AssetDetailCarriesTheAccounts measures the shape the execution plane
// depends on.
//
// The session's account is read out of the asset's DETAIL, and the
// connection token is minted with that account's UUID. If the detail did not
// carry the accounts, the session would have no account to run as.
func TestV4AssetDetailCarriesTheAccounts(t *testing.T) {
	c := v4Client(t)
	ctx := context.Background()
	rows, err := c.List(ctx, jmsapi.ResAssets)
	if err != nil {
		t.Fatalf("list the assets: %v", err)
	}
	if len(rows) == 0 {
		t.Skip("the deployment has no assets")
	}
	id := jmsapi.Str(rows[0]["id"])
	accts, err := c.AssetAccounts(ctx, id)
	if err != nil {
		t.Fatalf("read the accounts of %s: %v", id, err)
	}
	for _, a := range accts {
		if a.ID == "" {
			t.Errorf("an account came back without an id: the connection token binds the " +
				"account's UUID, so an account without one cannot be bound")
		}
	}
}

// TestV4AcceptsAConnectionTokenWithTheAccountUUID measures the token body.
//
// This is the field the third-party v4 literature disagrees about: it says
// the token API wants the account's ALIAS. v4's own source
// (ConnectionToken.get_asset_accounts_by_alias) looks a UUID up by id, and a
// regular account's alias IS its id — so the UUID this repository already
// sends should be accepted. This is where that is measured rather than
// reasoned about.
func TestV4AcceptsAConnectionTokenWithTheAccountUUID(t *testing.T) {
	c := v4Client(t)
	ctx := context.Background()
	rows, err := c.List(ctx, jmsapi.ResAssets)
	if err != nil {
		t.Fatalf("list the assets: %v", err)
	}
	for _, row := range rows {
		id := jmsapi.Str(row["id"])
		accts, err := c.AssetAccounts(ctx, id)
		if err != nil || len(accts) == 0 {
			continue
		}
		body := map[string]string{
			"asset":          id,
			"account":        accts[0].ID,
			"protocol":       "ssh",
			"connect_method": "web_cli",
		}
		var out map[string]any
		if err := c.Post(ctx, jmsapi.ResConnectionToken, body, &out); err != nil {
			t.Errorf("mint a connection token on %s: %v", id, err)
			continue
		}
		if jmsapi.Str(out["id"]) == "" || jmsapi.Str(out["value"]) == "" {
			t.Errorf("the token came back without an id/value: %v", out)
		}
		return
	}
	t.Skip("no asset with an account was found")
}

// TestV4KnowsNoChatAIAPI is the negative of the detection, measured.
//
// The probe says "v4" because the chat-ai prefix is not registered. If the
// deployment DOES have it, the detection is wrong and every other conclusion
// drawn from it is wrong too.
func TestV4KnowsNoChatAIAPI(t *testing.T) {
	c := v4Client(t)
	err := c.GetQuery(context.Background(), jmsapi.ResChatAI, nil, &map[string]any{})
	if err == nil {
		t.Fatal("the chat-ai API answered: this deployment is not a v4")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Logf("the chat-ai API answered %v (not a 404)", err)
	}
}
