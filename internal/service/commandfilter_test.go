package service

// The command-filter rules decide whether a destructive command is blocked.
// A false negative is not a wrong sentence — it is a command that runs.

import (
	"testing"
)

func group(typ string, content string, ignoreCase bool) map[string]any {
	return map[string]any{
		"type":        map[string]any{"value": typ, "label": typ},
		"content":     content,
		"ignore_case": ignoreCase,
	}
}

func TestACommandMatchesAsAStandaloneWord(t *testing.T) {
	g := group("command", "rm", false)
	cases := []struct {
		input string
		want  bool
	}{
		{"rm -rf /", true},
		{"sudo rm /tmp/x", true},
		{"rm", true},
		{"/bin/rm -rf /", true},
		// The trap: "rmdir" is a different command.
		{"rmdir /tmp/x", false},
		// Same rule: the word appears, so it matches.
		{"chmod 755 rm", true},
		// The rule is "the word appears anywhere in the input", so a word
		// that is only an ARGUMENT still matches. That is the bastion's
		// rule, and the guard's job is to report it, not to second-guess it.
		{"echo rm", true},
	}
	for _, c := range cases {
		line, matched := MatchCommandGroup(c.input, g)
		if matched != c.want {
			t.Errorf("MatchCommandGroup(%q, rm) matched=%v (%q), want %v", c.input, matched, line, c.want)
		}
	}
}

func TestIgnoreCaseDecidesTheComparison(t *testing.T) {
	sensitive := group("command", "RM", false)
	if _, matched := MatchCommandGroup("rm -rf /", sensitive); matched {
		t.Error("without ignore_case, RM must not match rm")
	}
	folded := group("command", "RM", true)
	if _, matched := MatchCommandGroup("rm -rf /", folded); !matched {
		t.Error("with ignore_case, RM must match rm")
	}
}

func TestARegexGroupMatchesWhenThePatternIsFound(t *testing.T) {
	g := group("regex", `^git\s+push`, false)
	if _, matched := MatchCommandGroup("git push origin main", g); !matched {
		t.Error("the regex group should have matched")
	}
	if _, matched := MatchCommandGroup("git status", g); matched {
		t.Error("the regex group should not have matched")
	}
}

func TestABrokenPatternMatchesNothing(t *testing.T) {
	// Guessing would be worse than not matching: a pattern the bastion
	// cannot compile cannot block anything.
	g := group("regex", "([unclosed", false)
	if _, matched := MatchCommandGroup("anything", g); matched {
		t.Error("a pattern that does not compile must match nothing")
	}
}

func TestAnEmptyGroupMatchesNothing(t *testing.T) {
	if _, matched := MatchCommandGroup("rm -rf /", group("command", "  \n ", false)); matched {
		t.Error("an empty group must match nothing")
	}
}

func TestTheMatchedLineIsNamed(t *testing.T) {
	// The answer has to say WHICH rule matched, so the operator can go
	// look at that group.
	g := group("command", "ls\nrm\nchmod", false)
	line, matched := MatchCommandGroup("rm -rf /", g)
	if !matched {
		t.Fatal("the group should have matched")
	}
	if line != "rm" {
		t.Errorf("matched line = %q, want %q", line, "rm")
	}
}

// A rule is matched as a STANDALONE WORD, so a line that carries a flag of
// its own ("rm -rf") is never the word that matches -- the word is "rm".
// That is the bastion's rule, and the guard must not invent a looser one:
// a looser matcher would warn about commands the bastion would let through.
func TestARuleWithAFlagInItIsNotAWord(t *testing.T) {
	g := group("command", "rm -rf", false)
	if _, matched := MatchCommandGroup("rm -rf /", g); matched {
		t.Error("a rule line that is not a single word must not match")
	}
}

func TestAGroupWithNoTypeIsACommandGroup(t *testing.T) {
	g := map[string]any{"content": "rm", "ignore_case": false}
	if _, matched := MatchCommandGroup("rm -rf /", g); !matched {
		t.Error("a group without a type should match like a command group")
	}
}

func TestAnEmptyScopeMeansEveryone(t *testing.T) {
	detail := map[string]any{
		"users":    []any{},
		"assets":   []any{},
		"accounts": []any{},
	}
	scope, applies := FilterApplies(detail, "", "", "")
	if !applies {
		t.Error("a filter with empty scope lists applies to everyone")
	}
	if scope == "" {
		t.Error("the scope description should say what was checked")
	}
}

func TestANonEmptyScopeMeansExactlyThose(t *testing.T) {
	detail := map[string]any{
		"users":    []any{map[string]any{"id": "u1"}},
		"assets":   []any{map[string]any{"id": "a1"}},
		"accounts": []any{"root"},
	}
	// The caller is covered.
	if _, applies := FilterApplies(detail, "u1", "a1", "root"); !applies {
		t.Error("the filter should apply to a caller inside its scope")
	}
	// One field outside is enough to be outside.
	if _, applies := FilterApplies(detail, "u2", "a1", "root"); applies {
		t.Error("the filter must not apply to a user outside its scope")
	}
	if _, applies := FilterApplies(detail, "u1", "a2", "root"); applies {
		t.Error("the filter must not apply to an asset outside its scope")
	}
	if _, applies := FilterApplies(detail, "u1", "a1", "other"); applies {
		t.Error("the filter must not apply to an account outside its scope")
	}
	// An unknown caller cannot be inside a named scope.
	if _, applies := FilterApplies(detail, "", "a1", "root"); applies {
		t.Error("an unknown user cannot be inside a named user scope")
	}
}

func TestScopeReadsTheEnvelopeShape(t *testing.T) {
	// The API returns the same list either bare or wrapped.
	bare := map[string]any{"users": []any{"u1", "u2"}}
	wrapped := map[string]any{"users": map[string]any{"ids": []any{"u1", "u2"}}}
	for name, detail := range map[string]map[string]any{"bare": bare, "wrapped": wrapped} {
		if _, applies := FilterApplies(detail, "u1", "", ""); !applies {
			t.Errorf("%s: the filter should apply to u1", name)
		}
		if _, applies := FilterApplies(detail, "u3", "", ""); applies {
			t.Errorf("%s: the filter must not apply to u3", name)
		}
	}
}

func TestSplitCommandWords(t *testing.T) {
	got := SplitCommandWords("sudo rm -rf /tmp/x; whoami")
	want := []string{"sudo", "rm", "-rf", "/tmp/x", "whoami"}
	if len(got) != len(want) {
		t.Fatalf("SplitCommandWords = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("word %d = %q, want %q", i, got[i], want[i])
		}
	}
}
