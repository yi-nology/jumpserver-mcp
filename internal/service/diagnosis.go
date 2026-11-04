package service

// The diagnostic contract.
//
// Five tools answer the same kind of question — "is something wrong here,
// and what do I do about it": jms_doctor, jms_asset_audit, jms_command_guard,
// jms_security_review and jms_login_asset_check. They look at different
// things, but a caller should be able to read any of them the same way:
//
//	checks[]   what was examined, one entry per dimension
//	findings[] the entries of checks[] that are not ok
//	status     the overall grade: ok / warn / fail
//	verdict    the conclusion, in one sentence
//	advice     what to do about it, in one sentence
//
// The three statuses mean exactly one thing each:
//
//	ok    nothing to do
//	warn  worth a look, but it does not block
//	fail  this is broken
//
// The shape is not written here: it is the contract in the IDL, generated into
// model/jmsms. These tools produce that contract, so the five of them
// cannot drift apart and the wire format has one owner.
//
// What is shared here is the VOCABULARY, not the verdicts. Each tool keeps
// its own judgement about what counts as broken -- the doctor counts a WARN
// as "needs attention" for its exit code, the guard treats "no filter
// matched" as fine, the asset audit treats a disabled account as softer
// than a missing protocol. Unifying the wording must not flatten those
// differences, because they are the parts that were got right per tool.
//
// It lives in the service layer rather than in the tool layer because it is a
// RULE, not a tool: the grading ("a fail wins over a warn") and the advice
// assembly are the same for every tool that uses them, and a rule that five
// tools share is a rule that belongs to one place.

import (
	"fmt"
	"strings"

	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
)

// The three grades, spelled once.
const (
	OK   = jms.Status_ok
	Warn = jms.Status_warn
	Fail = jms.Status_fail
)

// FixText is the optional "how to fix" of one check.
//
// The contract omits the field when there is nothing to fix, so an empty
// string has to become "absent" rather than an empty string in the JSON.
func FixText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Status grades a set of checks: a fail wins over a warn, a warn over an
// ok. An empty set is ok, because nothing was found wrong.
func Status(checks []*jms.Check) jms.Status {
	status := jms.Status_ok
	for _, ck := range checks {
		switch ck.Status {
		case jms.Status_fail:
			return jms.Status_fail
		case jms.Status_warn:
			status = jms.Status_warn
		}
	}
	return status
}

// Findings returns the checks that are not ok.
//
// It is the same subset for every tool, so a caller can ask "what is wrong"
// without re-implementing the filter.
func Findings(checks []*jms.Check) []*jms.Check {
	findings := []*jms.Check{}
	for _, ck := range checks {
		if ck.Status != jms.Status_ok {
			findings = append(findings, ck)
		}
	}
	return findings
}

// Advice joins the fixes of the checks that are not ok into one sentence.
//
// fallback is used when nothing needs fixing. Each tool words that its own
// way, because "nothing to fix" means something different for a bastion-wide
// self-check and for a single command.
func Advice(checks []*jms.Check, fallback string) string {
	fixes := []string{}
	for _, ck := range checks {
		if ck.Status == jms.Status_ok || ck.Fix == nil || *ck.Fix == "" {
			continue
		}
		fixes = append(fixes, *ck.Fix)
	}
	if len(fixes) == 0 {
		return fallback
	}
	return strings.Join(fixes, "; ")
}

// Diagnosis assembles the shared diagnostic shape.
//
// The verdict is the conclusion in one sentence and the advice is what to do
// about it; both are written by the tool, because only the tool knows what it
// found. Everything else -- the grading, the findings subset and the advice
// assembly -- is derived here so that two tools cannot drift apart.
//
// It returns the shape instead of writing it into a map: a tool whose answer
// is a typed contract carries the diagnosis inside it, and a map here would
// put the field names back into the code by hand.
func Diagnosis(checks []*jms.Check, verdict, okAdvice string) jms.Diagnosis {
	if checks == nil {
		checks = []*jms.Check{}
	}
	return jms.Diagnosis{
		Checks:   checks,
		Findings: Findings(checks),
		Status:   Status(checks),
		Verdict:  jms.Opt(verdict),
		Advice:   jms.Opt(Advice(checks, okAdvice)),
	}
}

// Verdict is the standard conclusion: the ok wording when nothing was
// found, otherwise how many dimensions are not ok and which ones.
func Verdict(checks []*jms.Check, okVerdict string) string {
	findings := Findings(checks)
	if len(findings) == 0 {
		return okVerdict
	}
	names := []string{}
	for _, f := range findings {
		names = append(names, f.Name)
	}
	return fmt.Sprintf("%d of %d checks need attention: %s",
		len(findings), len(checks), strings.Join(names, ", "))
}
