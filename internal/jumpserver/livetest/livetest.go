// Package livetest holds the helpers that reach a REAL bastion.
//
// They are separate from the shared test helpers because they are the
// management plane's alone: the execution plane does not have a REST
// client, and it certainly has no business creating accounts on the
// bastion under test.
//
// The tests that use these reach a real bastion, so they need the same
// three things everywhere: a client wired to the bastion under test, an
// asset to hang accounts on, and a name that will not collide with another
// run's.
//
// They live in their own package because the tests span two packages (the
// tools and the API client), and a helper that had to be copied into both
// would drift.
package livetest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/client"
)

// LiveClient returns a client for the bastion under test.
//
// The test is skipped when no token is configured: a unit test run without
// credentials is a normal thing, and skipping is the honest answer.
func LiveClient(t *testing.T) *client.Client {
	t.Helper()
	token := os.Getenv("JUMPSERVER_TOKEN")
	if token == "" {
		t.Skip("JUMPSERVER_TOKEN not set")
	}
	base := os.Getenv("JUMPSERVER_URL")
	if base == "" {
		// ⚠️ No fallback address: it would point the live check at a
		// documentation range, and the failure would read as "the bastion
		// is unreachable" rather than "this was never pointed anywhere".
		// A live test that is not pointed at a deployment is skipped.
		t.Skip("JUMPSERVER_URL not set")
	}
	return client.NewBase(base, token, &http.Client{Timeout: 90 * time.Second})
}

// TestAsset is an asset with an ssh protocol, so a test can attach an
// account to it and log in. Overridable so the test can run against another
// host.
func TestAsset(t *testing.T) (id, address string) {
	t.Helper()
	addr := os.Getenv("JMS_TEST_ASSET")
	if addr == "" {
		// ⚠️ Same as above: no fallback asset — a live test needs to be
		// told which machine it may touch.
		t.Skip("JMS_TEST_ASSET not set")
	}
	c := LiveClient(t)
	ids, meta, err := c.ResolveAssets(context.Background(), []string{addr})
	if err != nil {
		t.Fatalf("resolve test asset %s: %v", addr, err)
	}
	return ids[0], fmt.Sprint(meta[0]["address"])
}
