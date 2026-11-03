package main

import (
	"strings"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/model/jmsms"
)

// renderDoctor is the CLI wording. It is a pure function so the text can be
// checked without a bastion, and the grading rule (a WARN still counts as
// "needs attention" here) is pinned explicitly.
//
// The report is the contract type the doctor answers with, so the CLI reads
// the same fields the tool hands out.
func TestRenderDoctor(t *testing.T) {
	report := &jmsms.DoctorResult{
		BaseURL: "http://192.0.2.41",
		Identity: jmsms.IdentityRef{
			Username:    jms.Opt("admin"),
			IsSuperuser: jms.OptBool(true),
		},
		Diagnosis: jms.Diagnosis{
			Checks: []*jms.Check{
				{Name: "api_reachable", Status: jms.Status_ok, Detail: "the bastion answered"},
				{Name: "delegation", Status: jms.Status_warn, Detail: "not configured",
					Fix: jms.Opt("set JMS_DELEGATION_SECRET if you want the jms_delegation_* tools to work")},
			},
			Status: jms.Status_warn,
		},
	}
	out := RenderDoctor(report)
	for _, want := range []string{
		"jumpserver-mcp doctor",
		"base_url: http://192.0.2.41",
		"identity: admin (superuser=true)",
		"[OK  ] api_reachable",
		"[WARN] delegation",
		"fix: set JMS_DELEGATION_SECRET",
		"result: needs attention (see WARN/FAIL above)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderDoctor output missing %q:\n%s", want, out)
		}
	}
}

// A healthy report says so, and says nothing else.
func TestRenderDoctorHealthy(t *testing.T) {
	report := &jmsms.DoctorResult{
		BaseURL: "http://x",
		Diagnosis: jms.Diagnosis{
			Checks: []*jms.Check{
				{Name: "a", Status: jms.Status_ok, Detail: "fine"},
			},
			Status: jms.Status_ok,
		},
	}
	out := RenderDoctor(report)
	if !strings.Contains(out, "result: healthy") {
		t.Errorf("RenderDoctor output missing the healthy verdict:\n%s", out)
	}
	if strings.Contains(out, "needs attention") {
		t.Errorf("renderDoctor reported attention for a healthy report:\n%s", out)
	}
}

// When the identity was NOT established (both fields nil), the identity line
// must be ABSENT — the "print the line only once it is known" guard. A
// fabricated zero value (Username: Opt(""), IsSuperuser: OptBool(false)) would
// make this line appear as "identity:  (superuser=false)", claiming a fact the
// probe never made. This test is the net for that: it must NOT render.
func TestRenderDoctorPrintsNoIdentityWhenUnknown(t *testing.T) {
	report := &jmsms.DoctorResult{
		BaseURL:  "http://x",
		Identity: jmsms.IdentityRef{}, // both nil: identity unknown
		Diagnosis: jms.Diagnosis{
			Checks: []*jms.Check{{Name: "a", Status: jms.Status_ok, Detail: "fine"}},
			Status: jms.Status_ok,
		},
	}
	out := RenderDoctor(report)
	if strings.Contains(out, "identity:") {
		t.Errorf("RenderDoctor printed an identity line although the identity is unknown:\n%s", out)
	}
}
