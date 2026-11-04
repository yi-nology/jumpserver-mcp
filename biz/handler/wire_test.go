package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/yi-nology/jumpserver-kit/pkg/jmsapi"

	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
)

// TestFilterBodyWireFormat sends the exact body the tool builds and prints the
// JSON that actually goes on the wire, because the API rejects the Go version
// while an identical Python payload is accepted.
func TestFilterBodyWireFormat(t *testing.T) {
	token := os.Getenv("JUMPSERVER_TOKEN")
	if token == "" {
		t.Skip("JUMPSERVER_TOKEN not set")
	}
	c := repository.NewBase("http://192.0.2.41", token, &http.Client{Timeout: 60 * time.Second})

	ctx := context.Background()

	// Create the group this filter refers to.
	var grp map[string]any
	if err := c.Post(ctx, jmsapi.ResCmdGroups, map[string]any{"name": "go-wire-probe", "content": "rm -rf"}, &grp); err != nil {
		t.Fatalf("group: %v", err)
	}
	grpID := grp["id"].(string)
	defer c.Delete(ctx, jmsapi.ResCmdGroups, grpID)

	body := map[string]any{
		"name":           "go-wire-probe",
		"action":         "reject",
		"command_groups": repository.IDList([]string{grpID}),
		"users":          m2mEnvelope([]string{"898d705b-c874-4fd3-b3a7-1125f2513a4e"}),
		"assets":         m2mEnvelope([]string{"42e1a002-e33f-4c38-ad16-479cadb2df41"}),
		"accounts":       repository.IDList(nil),
		"is_active":      true,
	}
	wire, _ := json.MarshalIndent(body, "", "  ")
	t.Logf("wire body:\n%s", wire)

	var out map[string]any
	if err := c.Post(ctx, jmsapi.ResCmdFilters, body, &out); err != nil {
		t.Fatalf("filter create failed: %v", err)
	}
	t.Logf("created: %v", out["id"])
	c.Delete(ctx, jmsapi.ResCmdFilters, fmt.Sprint(out["id"]))
}
