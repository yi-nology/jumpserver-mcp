package handler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/audit"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/client"
)

// offlineClient is a client with no token: the doctor stops at the token
// check, and never reaches the bastion.
func offlineClient() *client.Client {
	return &client.Client{Client: jmsapi.New(jmsapi.Options{BaseURL: "http://127.0.0.1:1"})}
}

// The doctor has to report the trail this machine keeps.
//
// ⚠️ The trail's directory is CONFIGURABLE, and each server carries its own
// configuration — so the management plane's trail can stop growing while the
// execution plane's doctor says everything is fine. The check therefore
// lives in the shared base and each server runs it against its OWN settings.
//
// ⚠️ It is about THIS machine and has nothing to do with the bastion, so it
// is reported even when the bastion cannot be reached at all — which is
// what the empty client below stands for.
func TestTheDoctorReportsTheTrail(t *testing.T) {
	dir := t.TempDir()

	report, err := DoctorWithTrail(context.Background(), offlineClient(), false,
		&audit.Settings{Mode: "file", Dir: dir})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	ck := findCheck(t, report, "audit_trail")
	if ck.Status != jms.Status_ok {
		t.Errorf("a healthy trail reported %v (%s), want ok", ck.Status, ck.GetDetail())
	}
}

// A trail that is switched off is not a healthy trail: the gate's record
// goes to the log only, and a call that did NOT run leaves nothing behind.
func TestTheDoctorSaysWhenTheTrailIsOff(t *testing.T) {
	report, err := DoctorWithTrail(context.Background(), offlineClient(), false,
		&audit.Settings{Mode: "off"})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	ck := findCheck(t, report, "audit_trail")
	if ck.Status != jms.Status_warn {
		t.Errorf("a trail that is off reported %v (%s), want warn", ck.Status, ck.GetDetail())
	}
}

// And the tool path says NOTHING about it: it carries no configuration on
// purpose, so it cannot know — and "I do not know" is not "it is broken".
//
// ⚠️ The two silences are different: "this server has no trail" and "this
// caller cannot know" must not look the same.
func TestTheDoctorSaysNothingAboutTheTrailWhenItCannotKnow(t *testing.T) {
	report, err := Doctor(context.Background(), offlineClient(), false)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	for _, ck := range report.Checks {
		if ck.Name == "audit_trail" {
			t.Errorf("the doctor reported a trail check it cannot know about: %+v", ck)
		}
	}
}

// A trail file over the cap means the rotation failed and the file trail
// stopped there. The doctor has to say so out loud.
func TestTheDoctorSaysWhenTheTrailStopped(t *testing.T) {
	dir := t.TempDir()
	over := filepath.Join(dir, audit.FileName())
	f, err := os.Create(over)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := f.Truncate(audit.MaxTrailBytes + 1); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	f.Close()

	report, err := DoctorWithTrail(context.Background(), offlineClient(), false,
		&audit.Settings{Mode: "file", Dir: dir})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	ck := findCheck(t, report, "audit_trail")
	if ck.Status != jms.Status_warn {
		t.Errorf("a trail file over the cap reported %v (%s), want warn", ck.Status, ck.GetDetail())
	}
}

// findCheck picks one check out of the report by name.
func findCheck(t *testing.T, report interface{ GetChecks() []*jms.Check }, name string) *jms.Check {
	t.Helper()
	for _, ck := range report.GetChecks() {
		if ck.Name == name {
			return ck
		}
	}
	t.Fatalf("the doctor reported no %q check", name)
	return nil
}
