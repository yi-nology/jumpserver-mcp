package handler

import (
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
)

// A bulk registration that partly failed has to say so AT THE CALL LEVEL.
//
// The per-asset rows say what went wrong, but the trail reads the call-level
// account: without it, a bulk registration where a third of the assets were
// never created is written down as a successful call.
func TestABulkThatPartlyFailedSaysSoAtTheCallLevel(t *testing.T) {
	cs := writeTestSession(t, fakeBastionForWrites(t))

	out := testsupport.CallToolJSON(t, cs, "jms_assets_bulk_create", map[string]any{
		"assets": []map[string]any{
			{"name": "new-box", "address": "192.0.2.99", "platform": "Linux",
				"protocols": []string{"ssh"}},
			// No protocols: the guard fires before anything is written,
			// so this one cannot be created.
			{"name": "no-protocol-box", "address": "192.0.2.98", "platform": "Linux"},
		},
		"confirm": true,
	})

	if out["failed"] == float64(0) {
		t.Fatalf("the answer says no asset failed: %v", out)
	}
	reason, _ := out["error"].(string)
	if reason == "" {
		t.Fatal("the answer does not say at the call level that part of the batch failed")
	}
}
