package config

import (
	"strings"
	"testing"
)

// The deployment's address is required, and its absence has to be reported
// as a CONFIGURATION problem.
//
// ⚠️ It used to have a default, and that default was a real deployment's
// address: a copy of this server pointed at somebody else's bastion, and
// the failure looked like "the bastion is down" rather than "this was
// never configured". So the default is gone, and the message says what to
// set — the person reading it is an operator wiring a server up, not a
// developer reading a stack trace.
func TestTheDeploymentAddressIsRequired(t *testing.T) {
	t.Setenv("JUMPSERVER_URL", "")
	t.Setenv("JUMPSERVER_TOKEN", "some-token")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted a missing JUMPSERVER_URL")
	}
	if !strings.Contains(err.Error(), "JUMPSERVER_URL") {
		t.Errorf("the error does not name the variable to set: %v", err)
	}
	// The message must be actionable: it says what the value looks like.
	if !strings.Contains(err.Error(), "https://") {
		t.Errorf("the error does not show what the value looks like: %v", err)
	}
}

// A configured deployment loads, and the trailing slash is gone (the client
// appends /api/v1 itself).
func TestTheConfiguredDeploymentLoads(t *testing.T) {
	t.Setenv("JUMPSERVER_URL", "https://jumpserver.example.com/")
	t.Setenv("JUMPSERVER_TOKEN", "some-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.BaseURL != "https://jumpserver.example.com" {
		t.Errorf("BaseURL = %q, want the trailing slash removed", cfg.BaseURL)
	}
}

// The address is checked BEFORE the credential, so an operator who has set
// neither is told about the address first — it is the one that has no
// fallback form at all.
func TestTheAddressIsCheckedBeforeTheCredential(t *testing.T) {
	t.Setenv("JUMPSERVER_URL", "")
	t.Setenv("JUMPSERVER_TOKEN", "")
	t.Setenv("JUMPSERVER_USERNAME", "")
	t.Setenv("JUMPSERVER_PASSWORD", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted a missing JUMPSERVER_URL")
	}
	if !strings.Contains(err.Error(), "JUMPSERVER_URL") {
		t.Errorf("the error does not name the address first: %v", err)
	}
}
