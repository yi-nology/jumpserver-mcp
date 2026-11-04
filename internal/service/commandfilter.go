package service

// The command-filter rules.
//
// jms_command_guard answers one question: "would the bastion refuse this
// command?" A false negative here is not a wrong sentence — it is a
// destructive command that runs.
//
// The matching rules are the bastion's own:
//
//   - a command group of type "command" matches a line when that line
//     appears as a standalone word anywhere in the input: "rm" matches
//     "rm -rf" and "sudo rm", but not "rmdir";
//   - a group of type "regex" matches when the pattern is found in the
//     input;
//   - the group's ignore_case flag decides whether the comparison folds
//     case.
//
// A filter applies when its scope covers the caller: an empty users or
// assets list means "everyone" / "every asset", a non-empty one means
// exactly those.
//
// These are pure functions over the API's shapes, so they can be exercised
// without a bastion — which matters, because the interesting cases (a group
// that must NOT match "rmdir", a filter scoped to one user) are exactly the
// ones a live bastion rarely contains.

import (
	"fmt"
	"github.com/oekit/pkg/slicex"
	"regexp"
	"strings"

	"github.com/oekit/pkg/convert"

	"github.com/yi-nology/jumpserver-kit/pkg/jmsapi"
)

// MatchCommandGroup reports whether a command group matches an input line.
//
// It returns the line that matched, so the answer can name it ("rm -rf was
// matched by the rule 'rm'") instead of only saying yes.
func MatchCommandGroup(input string, group map[string]any) (string, bool) {
	content := convert.AnyToStr(group["content"])
	if strings.TrimSpace(content) == "" {
		return "", false
	}
	ignoreCase, _ := group["ignore_case"].(bool)
	typ := jmsapi.Label(group, "type")
	if typ == "" {
		typ = "command"
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if typ == "regex" {
			re, err := regexp.Compile(line)
			if err != nil {
				// A group whose pattern does not compile matches nothing.
				// Guessing would be worse than not matching.
				continue
			}
			if re.MatchString(input) {
				return line, true
			}
			continue
		}
		if CommandWordMatches(input, line, ignoreCase) {
			return line, true
		}
	}
	return "", false
}

// CommandWordMatches reports whether a command appears in the input as a
// standalone word.
//
// A path's last segment counts: "/bin/rm" is the word "rm". Without that, a
// group listing "/bin/rm" would never match a command line that calls it by
// its full path — and the filter would silently not apply.
func CommandWordMatches(input, command string, ignoreCase bool) bool {
	norm := func(s string) string {
		if ignoreCase {
			return strings.ToLower(s)
		}
		return s
	}
	want := norm(command)
	for _, tok := range SplitCommandWords(input) {
		t := norm(tok)
		if t == want {
			return true
		}
		if i := strings.LastIndex(t, "/"); i >= 0 && i+1 < len(t) && t[i+1:] == want {
			return true
		}
	}
	return false
}

// FilterApplies reports whether a command filter covers the given caller.
//
// The scope lists work like the bastion's: an empty list means "everyone",
// a non-empty one means exactly those. A scope that does not cover the
// caller means the filter does not apply to them — and the caller needs to
// know that, because the filter's action (reject) only fires for the scope
// it names.
//
// The returned string describes the scope, for the answer to say what was
// checked.
func FilterApplies(detail map[string]any, userID, assetID, account string) (string, bool) {
	users := EnvelopeIDs(detail["users"])
	if len(users) > 0 {
		if userID == "" || !contains(users, userID) {
			return "", false
		}
	}
	assets := EnvelopeIDs(detail["assets"])
	if len(assets) > 0 {
		if assetID == "" || !contains(assets, assetID) {
			return "", false
		}
	}
	accounts := StringIDs(detail["accounts"])
	if len(accounts) > 0 {
		if account == "" || !contains(accounts, account) {
			return "", false
		}
	}
	return fmt.Sprintf("users=%d assets=%d accounts=%d", len(users), len(assets), len(accounts)), true
}

// SplitCommandWords splits a command line into the words a filter can match
// against.
//
// Shell metacharacters are separators: a filter that lists "rm" must match
// "sudo rm" and "rm; whoami", and must not match "rmdir".
func SplitCommandWords(input string) []string {
	return strings.FieldsFunc(input, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ';', '&', '|', '<', '>', '(', ')', '{', '}', '[', ']', '\'', '"', '`', '=', ',', '!', '?', '*', '#', '~':
			return true
		}
		return false
	})
}

// EnvelopeIDs reads an id list that the API may return either as a bare list
// or wrapped in an {ids: [...]} envelope.
func EnvelopeIDs(v any) []string {
	switch t := v.(type) {
	case map[string]any:
		return StringIDs(t["ids"])
	case []any:
		return StringIDs(t)
	}
	return nil
}

// StringIDs reads a list of ids that may hold plain strings or objects.
func StringIDs(v any) []string {
	out := []string{}
	add := func(x any) {
		switch t := x.(type) {
		case string:
			if t != "" {
				out = append(out, t)
			}
		case map[string]any:
			if id, ok := t["id"].(string); ok && id != "" {
				out = append(out, id)
			}
		}
	}
	switch t := v.(type) {
	case []any:
		for _, x := range t {
			add(x)
		}
	case []string:
		out = append(out, t...)
	case string:
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// contains reports whether the list holds the value.
//
// It is slicex.Contains: the oekit family has one "is this in the list"
// primitive, and a second one here would be a second one to keep in step.
func contains(list []string, want string) bool {
	return slicex.Contains(list, want)
}
