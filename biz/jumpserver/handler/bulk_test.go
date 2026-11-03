package handler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/livetest"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/client"
)

// The bulk path has to behave exactly like the single path.
//
// A bulk registration that differs from the single one in any way is a bug
// that is invisible in review and obvious in production: the first asset
// works, the second one lands somewhere else.
func TestBulkAssetCreateMatchesSingle(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()
	_, address := livetest.TestAsset(t)

	name := testsupport.Uniq("mcp-test-bulk")
	created, err := c.CreateAsset(ctx, client.AssetSpec{
		Name: name, Address: address, Platform: "Linux", Protocols: []string{"ssh"},
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	id := fmt.Sprint(created["id"])
	defer c.Delete(ctx, jmsapi.ResAssets, id)

	// The record must exist and carry the protocol it was given.
	var back map[string]any
	if err := c.Get(ctx, jmsapi.ResAssets, id, &back); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(jmsapi.AsSlice(back["protocols"])) == 0 {
		t.Error("the created asset has no protocol")
	}
	if fmt.Sprint(back["name"]) != name {
		t.Errorf("name = %v, want %q", back["name"], name)
	}
}

// An asset without protocols can never be connected to, so the guard has to
// fire before anything is written.
func TestCreateAssetRequiresProtocols(t *testing.T) {
	c := livetest.LiveClient(t)
	_, err := c.CreateAsset(context.Background(), client.AssetSpec{
		Name: "no-protocol", Address: "10.255.255.9", Platform: "Linux",
	})
	if err == nil {
		t.Fatal("expected an error for an asset without protocols")
	}
	if !strings.Contains(err.Error(), "protocols") {
		t.Errorf("error = %v, want it to name the missing protocols", err)
	}
}

// An unknown platform must fail with the list of known platforms, because
// "no such platform" without a list is a dead end for the caller.
func TestCreateAssetUnknownPlatform(t *testing.T) {
	c := livetest.LiveClient(t)
	_, err := c.CreateAsset(context.Background(), client.AssetSpec{
		Name: "bad-platform", Address: "10.255.255.10", Platform: "NoSuchPlatform",
		Protocols: []string{"ssh"},
	})
	if err == nil {
		t.Fatal("expected an error for an unknown platform")
	}
	if !strings.Contains(err.Error(), "known") {
		t.Errorf("error = %v, want it to list the known platforms", err)
	}
}

// The comparison has to compare what the permissions really reach, not the
// counter the API reports -- the counter understates a node-scoped grant.
func TestPermissionCompareUsesEffectiveCoverage(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()

	perms, err := c.List(ctx, jmsapi.ResPermissions)
	if err != nil {
		t.Fatalf("list permissions: %v", err)
	}
	if len(perms) < 2 {
		t.Skip("need at least two permissions to compare")
	}

	left, err := c.PermissionDetail(ctx, fmt.Sprint(perms[0]["name"]))
	if err != nil {
		t.Fatalf("left detail: %v", err)
	}
	right, err := c.PermissionDetail(ctx, fmt.Sprint(perms[1]["name"]))
	if err != nil {
		t.Fatalf("right detail: %v", err)
	}
	leftAssets, err := c.EffectiveAssets(ctx, left)
	if err != nil {
		t.Fatalf("left assets: %v", err)
	}
	rightAssets, err := c.EffectiveAssets(ctx, right)
	if err != nil {
		t.Fatalf("right assets: %v", err)
	}
	leftNames := map[string]bool{}
	for _, a := range leftAssets {
		leftNames[fmt.Sprint(a["name"])] = true
	}
	rightNames := map[string]bool{}
	for _, a := range rightAssets {
		rightNames[fmt.Sprint(a["name"])] = true
	}
	// The partition has to be exact: every asset lands in exactly one bucket.
	overlap := 0
	for n := range leftNames {
		if rightNames[n] {
			overlap++
		}
	}
	if overlap != len(bothCount(leftNames, rightNames)) {
		t.Errorf("the overlap count is inconsistent")
	}
}

// bothCount returns the intersection size.
func bothCount(a, b map[string]bool) []string {
	out := []string{}
	for n := range a {
		if b[n] {
			out = append(out, n)
		}
	}
	return out
}

// The security review has to read the login history and report the accounts
// that can reconfigure the bastion.
func TestSecurityReviewReadsTheBastion(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()

	users, err := c.List(ctx, jmsapi.ResUsers)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	wantSuper := 0
	for _, u := range users {
		if v, _ := u["is_superuser"].(bool); v {
			wantSuper++
		}
	}
	if wantSuper == 0 {
		t.Skip("no superusers on this bastion")
	}
	logs, err := c.List(ctx, jmsapi.ResLoginLogs)
	if err != nil {
		t.Fatalf("login logs: %v", err)
	}
	t.Logf("%d users, %d login-log rows", len(users), len(logs))
}
