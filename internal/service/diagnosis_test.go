package service

// The diagnostic vocabulary is shared by five tools, and it used to live in
// the tool layer with no test at all: the grading rule ("a fail wins over a
// warn") and the advice assembly were exercised only by calling a tool
// against a live bastion.
//
// These tests are the reason it moved.

import (
	"strings"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
)

func check(name string, status jms.Status, fix string) *jms.Check {
	return &jms.Check{Name: name, Status: status, Detail: "d", Fix: FixText(fix)}
}

func TestStatusGradesAWholeSet(t *testing.T) {
	cases := []struct {
		name   string
		checks []*jms.Check
		want   jms.Status
	}{
		{"nothing examined is ok", nil, jms.Status_ok},
		{"all ok", []*jms.Check{check("a", jms.Status_ok, ""), check("b", jms.Status_ok, "")}, jms.Status_ok},
		{"a warn warns", []*jms.Check{check("a", jms.Status_ok, ""), check("b", jms.Status_warn, "")}, jms.Status_warn},
		{"a fail wins over a warn", []*jms.Check{check("a", jms.Status_warn, ""), check("b", jms.Status_fail, "")}, jms.Status_fail},
		{"a fail wins whatever the order", []*jms.Check{check("a", jms.Status_fail, ""), check("b", jms.Status_warn, "")}, jms.Status_fail},
	}
	for _, c := range cases {
		if got := Status(c.checks); got != c.want {
			t.Errorf("%s: Status = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFindingsAreTheChecksThatAreNotOk(t *testing.T) {
	checks := []*jms.Check{
		check("a", jms.Status_ok, ""),
		check("b", jms.Status_warn, "look"),
		check("c", jms.Status_ok, ""),
		check("d", jms.Status_fail, "fix"),
	}
	got := Findings(checks)
	if len(got) != 2 {
		t.Fatalf("Findings returned %d checks, want 2: %+v", len(got), got)
	}
	if got[0].Name != "b" || got[1].Name != "d" {
		t.Errorf("Findings = %v, want b and d", []string{got[0].Name, got[1].Name})
	}
	// The empty case must be an empty list, not nil: it goes into the JSON.
	if empty := Findings(nil); empty == nil || len(empty) != 0 {
		t.Errorf("Findings(nil) = %v, want an empty list", empty)
	}
}

func TestAdviceJoinsOnlyTheFixesThatMatter(t *testing.T) {
	checks := []*jms.Check{
		check("a", jms.Status_ok, "not needed"),
		check("b", jms.Status_warn, "look at b"),
		check("c", jms.Status_fail, "fix c"),
	}
	got := Advice(checks, "nothing to do")
	if got != "look at b; fix c" {
		t.Errorf("Advice = %q", got)
	}
	// The fallback is what "nothing to fix" says, and each tool words it its
	// own way.
	if got := Advice([]*jms.Check{check("a", jms.Status_ok, "")}, "all good"); got != "all good" {
		t.Errorf("Advice with nothing to fix = %q, want the fallback", got)
	}
	// A check that is not ok but carries no fix must not add an empty chunk.
	if got := Advice([]*jms.Check{check("a", jms.Status_fail, "")}, "fallback"); got != "fallback" {
		t.Errorf("Advice with a fixless finding = %q, want the fallback", got)
	}
}

func TestVerdictNamesTheDimensionsThatNeedAttention(t *testing.T) {
	checks := []*jms.Check{
		check("a", jms.Status_ok, ""),
		check("b", jms.Status_warn, ""),
		check("c", jms.Status_fail, ""),
	}
	got := Verdict(checks, "all good")
	want := "2 of 3 checks need attention: b, c"
	if got != want {
		t.Errorf("Verdict = %q, want %q", got, want)
	}
	if got := Verdict([]*jms.Check{check("a", jms.Status_ok, "")}, "all good"); got != "all good" {
		t.Errorf("Verdict with nothing to report = %q, want the ok wording", got)
	}
}

func TestDiagnosisFillsTheSharedFields(t *testing.T) {
	out := Diagnosis([]*jms.Check{
		check("a", jms.Status_ok, ""),
		check("b", jms.Status_warn, "look at b"),
	}, "v", "nothing to do")

	if len(out.Checks) != 2 {
		t.Errorf("checks = %d, want 2", len(out.Checks))
	}
	if len(out.Findings) != 1 || out.Findings[0].Name != "b" {
		t.Errorf("findings = %+v, want just b", out.Findings)
	}
	if out.GetVerdict() != "v" {
		t.Errorf("verdict = %v, want the tool's own wording", out.GetVerdict())
	}
	if out.GetAdvice() != "look at b" {
		t.Errorf("advice = %v, want the joined fixes", out.GetAdvice())
	}
	if out.Status != jms.Status_warn {
		t.Errorf("status = %v, want warn", out.Status)
	}
}

func TestDiagnosisWithNoChecksIsStillAnAnswer(t *testing.T) {
	out := Diagnosis(nil, "v", "nothing to do")
	if out.Status != jms.Status_ok {
		t.Errorf("status = %v, want ok", out.Status)
	}
	// A nil set must serialize as an empty list, not as null: the contract
	// says findings[] is a list.
	if out.Findings == nil {
		t.Errorf("findings = %#v, want an empty list", out.Findings)
	}
}

func TestFixTextOmitsTheEmptyCase(t *testing.T) {
	if FixText("") != nil {
		t.Error("an empty fix should be absent, not an empty string")
	}
	got := FixText("do this")
	if got == nil || *got != "do this" {
		t.Errorf("FixText = %v, want the text", got)
	}
	// The wording is what the model reads; it must not be mangled.
	if strings.TrimSpace(*got) != "do this" {
		t.Errorf("FixText mangled the text: %q", *got)
	}
}
