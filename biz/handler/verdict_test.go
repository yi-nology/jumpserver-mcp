package handler

import (
	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/render"
	"github.com/yi-nology/jumpserver-mcp/biz/model/jmsms"
)

// The refusal of an answer has to be FINDABLE.
//
// The trail reads it by name — the contract's `blocked` field — and a name
// that stops matching does not fail loudly: it just makes the trail say "the
// command ran" for every dangerous command that was stopped. So the
// management plane's refusal shapes are pinned here, the way the execution
// plane's are: renaming the field breaks this test at COMPILE time, which is
// the loudest failure there is.
//
// The management plane had no such guard, and it is the plane where the
// consequence is worst: `jms_job_run` is the tool that stops a DANGEROUS
// batch job on the bastion.
func TestEveryRefusalIsFindable(t *testing.T) {
	const reason = "classified DANGEROUS"
	cases := []struct {
		name   string
		answer any
	}{
		{"jms_job_run", &jmsms.JobPlan{Blocked: jms.Opt(reason)}},
		{"jms_delegation_call", &jmsms.DelegationCallResult{Blocked: jms.Opt(reason)}},
	}
	for _, c := range cases {
		got := render.OutcomeOf(c.answer)
		if got == nil {
			t.Errorf("%s: the answer declares no verdict at all", c.name)
			continue
		}
		if got.Refused != reason {
			t.Errorf("%s: the refusal was not found: %q", c.name, got.Refused)
		}
		if got.Acted() {
			t.Errorf("%s: a refusal counts as something that acted", c.name)
		}
	}
}

// A rehearsal is not a refusal, and the management plane has to say which
// one it was — including whether the command WOULD have run.
//
// The two planes answer the same question ("would this rehearsal have run
// the command?"), so they have to answer it the same way: the execution
// plane fills `would_execute` from its rehearsal judgement, and the trail
// reads it from the answer. A contract type with no such field would leave
// the trail unable to say "this one would have run" — and the log line
// would claim the opposite.
func TestARehearsalSaysWhetherItWouldHaveRun(t *testing.T) {
	got := render.OutcomeOf(&jmsms.JobPlan{
		DryRun:       jms.OptBool(true),
		WouldExecute: jms.OptBool(true),
	})
	if got == nil {
		t.Fatal("the answer declares no verdict at all")
	}
	if !got.DryRun {
		t.Error("the rehearsal was not recognised as one")
	}
	if got.Refused != "" {
		t.Errorf("the rehearsal was taken for a refusal: %q", got.Refused)
	}
	if got.WouldExecute == nil {
		t.Fatal("the rehearsal does not say whether it would have run")
	}
	if !*got.WouldExecute {
		t.Error("the rehearsal says it would not have run, but it would")
	}
}
