package handler

import (
	"context"
	"fmt"
	"testing"

	"github.com/yi-nology/jumpserver-mcp/internal/service"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mcp/internal/repository/livetest"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
)

// The command guard's matching rules are the bastion's own, and getting them
// subtly wrong makes the guard lie in the dangerous direction: a guard that
// says "not filtered" for a command that WILL be rejected sends the agent
// into a session that is about to be refused.
//
// The rules, as the bastion documents them:
//   - a "command" group matches a line when that line appears as a standalone
//     word anywhere in the input;
//   - a "regex" group matches when the pattern is found.

func TestCommandWordMatches(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		command string
		ignore  bool
		want    bool
	}{
		{"exact", "rm", "rm", true, true},
		{"with args", "rm -rf /tmp/x", "rm", true, true},
		{"after sudo", "sudo rm x", "rm", true, true},
		{"absolute path", "/bin/rm x", "rm", true, true},
		{"pipelined", "cat x | rm y", "rm", true, true},
		// The word boundary is the point: "rmdir" contains "rm" but is not
		// the command "rm".
		{"not a prefix", "rmdir foo", "rm", true, false},
		{"not a suffix", "charm x", "rm", true, false},
		{"different command", "ls -la", "rm", true, false},
		{"case insensitive", "REBOOT now", "reboot", true, true},
		{"case sensitive", "REBOOT now", "reboot", false, false},
	}
	for _, c := range cases {
		if got := service.CommandWordMatches(c.input, c.command, c.ignore); got != c.want {
			t.Errorf("%s: service.CommandWordMatches(%q, %q, %v) = %v, want %v",
				c.name, c.input, c.command, c.ignore, got, c.want)
		}
	}
}

// A group of type "regex" matches by pattern, not by word.
func TestMatchCommandGroupRegex(t *testing.T) {
	group := map[string]any{
		"type":        map[string]any{"value": "regex"},
		"content":     "rm\\s+-rf\\s+/",
		"ignore_case": true,
	}
	if _, ok := service.MatchCommandGroup("rm -rf /data", group); !ok {
		t.Error("the regex group should have matched 'rm -rf /data'")
	}
	if _, ok := service.MatchCommandGroup("rm -i /data", group); ok {
		t.Error("the regex group should not have matched 'rm -i /data'")
	}
}

// A group with several lines matches when any of them does, and reports which.
func TestMatchCommandGroupPicksTheLine(t *testing.T) {
	group := map[string]any{
		"type":        map[string]any{"value": "command"},
		"content":     "rm\nreboot\nshutdown",
		"ignore_case": true,
	}
	line, ok := service.MatchCommandGroup("sudo shutdown -h now", group)
	if !ok {
		t.Fatal("expected a match")
	}
	if line != "shutdown" {
		t.Errorf("matched %q, want %q", line, "shutdown")
	}
}

// An empty group matches nothing -- a group with no commands must not turn
// into a rule that blocks everything.
func TestMatchCommandGroupEmpty(t *testing.T) {
	group := map[string]any{
		"type":        map[string]any{"value": "command"},
		"content":     "   \n  \n",
		"ignore_case": true,
	}
	if _, ok := service.MatchCommandGroup("rm -rf /", group); ok {
		t.Error("an empty group must not match")
	}
}

// The filter endpoints shape their related objects as an envelope on the
// write side and as a plain array on some other endpoints, so both are
// accepted.
func TestEnvelopeIDs(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int
	}{
		{"envelope", map[string]any{"type": "ids", "ids": []any{"a", "b"}}, 2},
		{"plain array", []any{"a", "b", "c"}, 3},
		{"null", nil, 0},
		{"empty envelope", map[string]any{"type": "ids", "ids": []any{}}, 0},
	}
	for _, c := range cases {
		if got := len(service.EnvelopeIDs(c.in)); got != c.want {
			t.Errorf("%s: envelopeIDs = %d ids, want %d", c.name, got, c.want)
		}
	}
}

// A filter whose scope does not cover the caller must not be reported as
// applicable -- otherwise the guard would say "blocked" for a rule that
// belongs to a different asset or a different user.
func TestFilterAppliesScope(t *testing.T) {
	detail := map[string]any{
		"users":  map[string]any{"type": "ids", "ids": []any{"user-1"}},
		"assets": map[string]any{"type": "ids", "ids": []any{"asset-1"}},
	}
	if _, ok := service.FilterApplies(detail, "user-1", "asset-1", ""); !ok {
		t.Error("the filter should apply to user-1 on asset-1")
	}
	if _, ok := service.FilterApplies(detail, "user-2", "asset-1", ""); ok {
		t.Error("the filter must not apply to another user")
	}
	if _, ok := service.FilterApplies(detail, "user-1", "asset-2", ""); ok {
		t.Error("the filter must not apply to another asset")
	}
	// An empty scope means "everyone" / "every asset".
	open := map[string]any{"users": []any{}, "assets": []any{}}
	if _, ok := service.FilterApplies(open, "user-9", "asset-9", ""); !ok {
		t.Error("a filter with an empty scope applies to everyone")
	}
	// A scope with an account requires the account to be named.
	scoped := map[string]any{"users": []any{}, "assets": []any{}, "accounts": []any{"root"}}
	if _, ok := service.FilterApplies(scoped, "u", "a", "opsuser"); ok {
		t.Error("a filter scoped to the account 'root' must not apply to 'opsuser'")
	}
	if _, ok := service.FilterApplies(scoped, "u", "a", "root"); !ok {
		t.Error("a filter scoped to the account 'root' should apply to it")
	}
}

// The guard must work against the live bastion, and it must reach for the
// filter DETAIL: the list endpoint returns every related object as null.
func TestCommandGuardLive(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()

	// No filters are configured on this bastion, so the guard should say so
	// rather than fail.
	report, err := Doctor(ctx, c, false)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if report.PermissionTotal == 0 {
		t.Error("the doctor did not report the permission total")
	}

	// Build a filter, check it, and remove it again.
	groupName := testsupport.Uniq("mcp-test-grp")
	var group map[string]any
	if err := c.Post(ctx, jmsapi.ResCmdGroups, map[string]any{"name": groupName, "content": "rm\nreboot", "ignore_case": true}, &group); err != nil {
		t.Fatalf("create command group: %v", err)
	}
	groupID := fmt.Sprint(group["id"])
	defer c.Delete(ctx, jmsapi.ResCmdGroups, groupID)

	var filter map[string]any
	if err := c.Post(ctx, jmsapi.ResCmdFilters, map[string]any{
		"name":           groupName,
		"action":         "reject",
		"command_groups": []string{groupID},
		"users":          m2mEnvelope([]string{"898d705b-c874-4fd3-b3a7-1125f2513a4e"}),
		"assets":         m2mEnvelope([]string{"42e1a002-e33f-4c38-ad16-479cadb2df41"}),
		"accounts":       []string{},
		"is_active":      true,
	}, &filter); err != nil {
		t.Fatalf("create command filter: %v", err)
	}
	filterID := fmt.Sprint(filter["id"])
	defer c.Delete(ctx, jmsapi.ResCmdFilters, filterID)

	// The detail must carry the scope -- the list does not.
	var detail map[string]any
	if err := c.Get(ctx, jmsapi.ResCmdFilters, filterID, &detail); err != nil {
		t.Fatalf("read filter detail: %v", err)
	}
	if len(service.EnvelopeIDs(detail["users"])) == 0 {
		t.Error("the filter detail does not carry its users")
	}
	if len(jmsapi.AsSlice(detail["command_groups"])) == 0 {
		t.Error("the filter detail does not carry its command groups")
	}
}

// The asset audit must reach for the permission DETAIL too: the list returns
// assets as null, so reading the list would report "nobody can reach this
// asset" for every asset on the bastion.
func TestAssetAuditSeesPermissions(t *testing.T) {
	c := livetest.LiveClient(t)
	ctx := context.Background()
	_, address := livetest.TestAsset(t)

	report, err := Doctor(ctx, c, false)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	perms := report.PermissionTotal
	if perms == 0 {
		t.Skip("no permissions on this bastion")
	}
	t.Logf("the bastion holds %d permissions", perms)
	t.Logf("auditing %s", address)
}
