package handler

// tools_redact.go masks credential-looking values in audit output.
//
// Why this exists: an audit tool that echoes the command log verbatim will
// happily print a WireGuard private key, a password or a bearer token that
// somebody typed -- and the output lands in an AI context window, a terminal
// scrollback and a session recording of its own. The first version of
// jms_asset_evidence did exactly that: asking it about a machine returned
// "PrivateKey = EMrb0obFo0sJqh6KjNz9WThsO9gh4P1Lrk0/1SgcXno=" verbatim. // jms:allow-credential
//
// What is masked is the VALUE, never the command. "cat /etc/wireguard/wg0.conf"
// is exactly the kind of thing an audit wants to show; the key next to it is
// not.

import (
	"regexp"
	"strings"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/redact"
)

var (
	// key = value / key: value, where the key names a credential.
	//
	// ⚠️ The pattern matches ANY key; whether that key says "credential"
	// is asked of redact.IsCredentialKey — the trail's single word list.
	// The regex used to carry its OWN word list, and the two drifted:
	// fourteen of the words the trail knows ("passphrase", "credential",
	// "cookie", "kubeconfig", "id_token", "refresh_token", …) were unknown
	// here ⇒ `passphrase=hunter2` reached the agent in the clear, while
	// the JSON form of the SAME word was masked ⇒ the failure mode
	// AGENTS.md names: one word masked in one path and not in the other.
	//
	// ⚠️ And the JSON form ("password":"x") was not recognised at all ⇒
	// `{"password":"x"}` reached the agent in the clear. It is the JSON
	// rules' job below.
	//
	// ⚠️ In a URL ("https://x/api?token=abc") the colon of the scheme is
	// read as a separator and the key comes out as "https" ⇒ this rule
	// matches the WHOLE URL, decides "https" is not a credential, and
	// leaves the URL alone ⇒ the token in it survives.
	//
	// ⇒ The URL's parameters are a rule of their own (queryParamRe
	// below), and this one does NOT try to skip the scheme's colon.
	//
	// ⚠️ And the value has FOUR forms, and they are not interchangeable:
	// a plain quoted value, a single-quoted one, an ESCAPED quoted one
	// (the form a JSON string carries: `{"cmd":"export PASSWORD=\"SECRET\""}`),
	// and an unquoted one.
	//
	// ⚠️ The escaped form is not optional: without it the value stops at
	// the backslash and the credential stands in the clear behind it
	// (`{"cmd":"export PASSWORD = [REDACTED]"SECRET\""}`).
	//
	// ⚠️ And the unquoted value must NOT eat an `&`: that is the query
	// string's separator ⇒ a value that ran into it would take the
	// NEXT parameter with it ("?token=abc&user=bob" ⇒ "user=bob" gone,
	// and the transcript stops saying WHAT was run).
	//
	// ⚠️ And the part behind the `&` is captured SEPARATELY (the fifth
	// group): only it says whether the `&` separates two parameters
	// ("&user=bob") or is the rest of a credential ("&ssw0rd") ⇒ and
	// the two are handled completely differently.
	//
	// ⚠️ The tail runs to the NEXT `&` (not to the next space): the
	// rule rewrites the separator to " = " ⇒ the text after a mask
	// reads "&password = [REDACTED]" ⇒ a tail that stopped at the
	// space would leave the rest of the parameter behind.
	//
	// ⚠️ The SEPARATOR is its own group too: the rule rewrites it to
	// " = " when it masks the value, and keeps it as it was when it
	// does not ⇒ the text has to be rebuilt from the pieces.
	//
	// ⚠️ And the KEY is matched by the SHARED word list (not "any key,
	// judged afterwards"): a rule that matches any key eats the word in
	// FRONT of the credential ("login failed: password: x" ⇒ the key
	// comes out as "login" and the value as "failed:") ⇒ the credential
	// never gets a match of its own ⇒ it reaches the agent in the clear.
	// ⇒ the pattern is built from the shared word list, so the match
	// STARTS at the credential.
	// ⚠️ The separator is "=" or ":" **or whitespace**: a command-line
	// argument can be written "--password secret" ⇒ and the trail's own
	// rule masks that shape ⇒ the two paths have to agree on the same
	// shape (AGENTS.md: "the same word is a credential in both paths or
	// in neither").
	secretValueRe = regexp.MustCompile(
		`(?i)([a-z0-9_.-]*(?:` + redact.CredentialWordPattern() + `))((?:\s*[:=]\s*|[ \t]+))` +
			`("(?:[^"\\]|\\[^"])*"|'[^']*'|[^\s"'&]+)(&[^&\r\n]*)?`)

	// The ESCAPED prose form: a value written with escaped quotes — the
	// shape a command's own JSON arguments carry
	// (`{"cmd":"export PASSWORD=\"SECRET\""}`).
	//
	// ⚠️ It is a rule of its own: without it the value stops at the
	// backslash and the credential stands in the clear behind it.
	//
	// ⚠️ And the value must not CROSS an escaped quote: `\\.` would
	// match the `\"` that starts the next field ⇒ the match swallowed
	// two fields at once.
	secretEscapedValueRe = regexp.MustCompile(
		`(?i)([a-z0-9_.-]+)(\s*[:=]\s*)(\\"(?:[^"\\]|\\[^"])*\\")`)

	// A query-string parameter: "?token=abc" and "&user=bob".
	//
	// ⚠️ The free-text rule cannot see it: in a URL the key is preceded
	// by "?" or "&" (fine) but the rule's VALUE stops at the first
	// whitespace or quote, and — worse — the URL's own "scheme:value"
	// shape swallows the match before the parameter is ever reached.
	//
	// ⚠️ And the parameter name is the credential: "token=…" in a query
	// string is a token exactly as much as "token=…" in prose.
	//
	// ⚠️ The value may be QUOTED ("?token=\"a b\""): the value shape has
	// to take the quoted form whole, or the quotes would be left
	// standing next to the mask ("?token = [REDACTED]\"a b\"") ⇒ the
	// value would be half in the clear.
	queryParamRe = regexp.MustCompile(
		`(?i)([?&])([a-z0-9_.-]+)=("[^"]*"|'[^']*'|[^\s&"']*)`)

	// A PEM private key block, in full.
	pemRe = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)

	// A URL with a credential IN it: "postgres://user:pass@host/db".
	//
	// ⚠️ The upstream rules know this shape (logredact masks it) but the
	// answer path does not RUN the upstream rules ⇒ it needs its own ⇒
	// without it, "https://user:pass@host" reached the agent in the
	// clear, and the shape has no key name to key on ("user" is not a
	// credential word) ⇒ no key-based rule can ever catch it.
	//
	// ⚠️ The PASSWORD runs to the LAST "@" before the host: a password
	// may contain an "@" itself ("p@ss") ⇒ stopping at the first one
	// would leave the tail in the clear ("https://user:[REDACTED]@ss@host").
	//
	// ⚠️ And the password may contain a "/" ⇒ the host part cannot be
	// recognised by a "/" ⇒ the password is taken as everything up to
	// the last "@" of the authority.
	urlCredentialRe = regexp.MustCompile(
		`(?i)\b(nats|postgres|postgresql|mysql|mongodb\+srv|mongodb|http|https|redis|amqps|amqp)://([^\s:@/]+):([^\s]+)@`)

	// An HTTP Authorization header: the scheme stays, the credential goes.
	//
	// ⚠️ It has to be its OWN rule: the generic key=value rule would
	// match "Authorization: Bearer eyJ…" and take "Bearer" for the
	// VALUE ⇒ the scheme would be replaced ("Authorization = [REDACTED]")
	// and the transcript would stop saying how it was authenticated.
	// And the scheme is exactly what an audit reader wants to keep.
	//
	// ⚠️ And the scheme is OPTIONAL: "authorization=…" carries no
	// scheme, and then the whole value goes.
	//
	// ⚠️ The credential is the scheme's ARGUMENT, not the rest of the
	// line: a Digest header carries parameters ("username=…, response=…")
	// and each of them is part of the credential ⇒ taking only the first
	// word would leave the response hash in the clear.
	//
	// ⚠️ But it must NOT eat to the end of the line: the upstream rules
	// do (`authorization: …` ⇒ the rest of the line is gone), and that
	// cost the transcript the URL in `curl -H "Authorization: Bearer x"
	// https://h/api` ⇒ the reader lost WHAT was run.
	//
	// ⇒ The credential is the quoted argument, or — unquoted — the run of
	// characters up to the next whitespace.
	authHeaderRe = regexp.MustCompile(
		`(?i)(authorization\s*[:=]\s*)((?:bearer|basic|digest|negotiate|ntlm)\s+)?("[^"]*"|'[^']*'|[^\s]+)`)

	// A --password= / --token= style command-line argument.
	//
	// ⚠️ The leading dash must NOT be part of a longer name: without
	// that, "x-api-key: value" matches as "-api-key" ⇒ the rule
	// rewrites it to "x-api-key [REDACTED]" ⇒ the NEXT pass sees
	// "x-api-key [REDACTED]" and the free-text rule turns it into
	// "x-api-key = [REDACTED]" ⇒ the pass after that bites it again ⇒
	// the text is **not idempotent** (the marker gets chewed up).
	//
	// ⇒ The dash has to stand alone: preceded by the start of the text
	// or by something that is not a name character.
	// ⚠️ The separator must NOT be "spaces" alone: the rule itself
	// rewrites "--password=x" into "--password = x" ⇒ the NEXT pass sees
	// "--password = x" and the space-separated branch would take the "="
	// for the VALUE ⇒ the old marker stays and a new one is added ⇒ the
	// text grows a marker on every pass ⇒ **not idempotent**.
	// ⇒ the separator is "=" or spaces, but the spaces may not be
	// followed by another "=".
	//
	// ⚠️ And the VALUE may not start with "=" ⇒ same reason.
	//
	// ⚠️ And the word list is the SHARED one (the same list the trail
	// uses) ⇒ "one list" holds literally, not just in effect.
	// ⚠️ The separator is "=" OR SPACES ("--password x" is a shape too) —
	// but the spaces may not be followed by another "=": that shape is
	// the one this rule itself produced on the previous pass
	// ("--password = x") ⇒ taking the "=" for the value would leave the
	// old marker standing and add a new one ⇒ the text grows a marker
	// on every pass ⇒ **not idempotent**. ⇒ the pattern lets the "="
	// be part of the value and the CALLBACK throws that case away.
	//
	// ⚠️ And the VALUE may not start with "=" ⇒ same reason.
	//
	// ⚠️ And the word list is the SHARED one (the same list the trail
	// uses) ⇒ "one list" holds literally, not just in effect.
	flagSecretRe = regexp.MustCompile(
		`(?i)(^|[^a-z0-9_.-])(--?[a-z0-9_.-]*(?:` + redact.CredentialWordPattern() + `))` +
			`((?:[:=][ \t]*|[ \t]+)(("[^"]*")|('[^']*')|[^\s"'=]+))`,
	)

	// The PARAMETERS of a challenge scheme: "Digest username=…, response=…".
	//
	// ⚠️ Each parameter is part of the credential ⇒ taking only the first
	// one would leave the response hash in the clear.
	//
	// ⚠️ And the run has to stop at something that is not a parameter:
	// the URL in `curl -H "Authorization: Digest …" https://h/api` must
	// survive ⇒ the run ends at the first token that is not "name=value".
	authParamsRe = regexp.MustCompile(
		`(?i)(authorization\s*[:=]\s*(?:digest|negotiate)\s+)((?:[a-z0-9_-]+=(?:"[^"]*"|'[^']*'|[^\s,]+)\s*,?\s*)+)`)

	// paramNameRe matches the NAME of a query-string parameter: "user=".
	//
	// ⚠️ It is what tells the `&` of "?token=abc&user=bob" (a separator)
	// from the `&` of "export PASSWORD=p&ssw0rd" (part of the credential).
	//
	// ⚠️ The name may be followed by spaces around the "=": the rule
	// rewrites the separator to " = " ⇒ the text after the mask says
	// "&password = [REDACTED]" ⇒ a pattern that demanded "password="
	// exactly would not recognise it ⇒ the tail would be masked as if
	// it were part of the credential.
	paramNameRe = regexp.MustCompile(`^[a-z0-9_.-]+\s*=`)

	// A credential field in the JSON form: a quoted key and a quoted or
	// unquoted value.
	//
	// ⚠️ It is the JSON form, which this path did not recognise at all:
	// `{"password":"x"}` reached the agent in the clear, because the
	// rules above know the free-text shape only. A command that carries
	// its own JSON arguments is the most ordinary thing in the world —
	// and it is exactly the shape that leaked.
	//
	// ⚠️ The unquoted value must NOT eat a quote: the shape rules eat
	// the key's closing quote when they mask a field, so the text right
	// after such a field starts with a quote ⇒ a value that ran into it
	// would swallow the NEXT field with it.
	answerKeyRe = regexp.MustCompile(
		`(?i)"([a-z0-9_.-]+)"\s*:\s*("(?:[^"\\]|\\.)*"|[^\s",}\]\[{}]*)`)

	// A credential field whose value is NOT a string — an object or an
	// array.
	//
	// ⚠️ It has to be matched WHOLE: the string rule matches only the
	// first fragment of such a value, and what stands behind that
	// fragment is a field whose name may not say "credential" at all
	// (`{"secret":{"value":"s3cr3t"}}` ⇒ the string rule matches
	// `"secret":{` and leaves `"value":"s3cr3t"}` standing in the clear).
	//
	// ⚠️ And the replacement keeps the document a document: masking only
	// the fragment would leave `{"secret":"[REDACTED]"{"value":…}}` —
	// broken JSON with the credential still inside it.
	//
	// ⚠️ The value is matched by its FIRST character only, and the rest
	// is found by matching brackets: a nested object
	// (`{"secret":{"nested":{"deep":"x"}}}`) has more than one `}` ⇒ a
	// pattern that stopped at the first one would leave the tail behind
	// and break the document.
	answerNonScalarRe = regexp.MustCompile(
		`(?i)"([a-z0-9_.-]+)"\s*:\s*([\{\[])`)

	// A credential inside an ESCAPED JSON string:
	// `{"params":"{\"password\":\"x\"}"}`.
	answerEscapedKeyRe = regexp.MustCompile(
		`(?i)` + redact.EscapePrefix(1) + `"([a-z0-9_.-]+)` + redact.EscapePrefix(1) +
			`"\s*[:=]\s*(` + redact.EscapePrefix(1) + `"(?:[^"\\]|\\[^"])*` + redact.EscapePrefix(1) + `"|)`)

	// The DOUBLE-escaped form: a JSON string inside a JSON string.
	// `{"params":"{\\"password\\":\\"x\\"}"}`.
	//
	// ⚠️ The SDK serialises the arguments as JSON, and a command that
	// carries its own JSON arguments ends up escaped TWICE ⇒ the
	// one-level rule does not match it ⇒ the credential survives.
	answerDoubleEscapedKeyRe = regexp.MustCompile(
		`(?i)` + redact.EscapePrefix(2) + `"([a-z0-9_.-]+)` + redact.EscapePrefix(2) +
			`"\s*[:=]\s*(` + redact.EscapePrefix(2) + `"(?:[^"\\]|\\[^"])*` + redact.EscapePrefix(2) + `"|)`)
)

// answerMarker is what this path leaves behind.
//
// ⚠️ It is NOT the trail's `****`: the two have different readers. The
// trail's readers are people reading a log; the answer's readers are an
// agent reading a command transcript, and the transcript has to stay
// READABLE ("what was run" is the point of the audit answer). So the
// answer keeps the key name and the scheme — "Authorization: Bearer
// [REDACTED]", "password = [REDACTED]" — where the trail collapses the
// whole field.
//
// ⚠️ The two shapes are a DELIBERATE difference, not drift: the same word
// is masked in both, in both shapes. What must not happen is the same
// word masked in one and not the other — that was the actual bug, and
// closing it is why the word list is shared (redact.IsCredentialKey)
// while the shapes stay each path's own.
const answerMarker = "[REDACTED]"

// redactSecrets masks credential-looking values, keeping the surrounding text
// readable so the audit still shows WHAT was done.
//
// ⚠️ It is the management plane's half of ONE redaction: the WORD LIST is
// the shared base's (redact.IsCredentialKey), so a word is a credential in
// both paths or in neither; the SHAPES are this path's own — the key name
// and the scheme survive, so the transcript still reads.
//
// ⚠️ The free-text rule asks redact.IsCredentialKey whether the key names a
// credential — the SAME question the trail asks ⇒ a word is a credential in
// both paths or in neither.
func redactSecrets(s string) string {
	if s == "" {
		return s
	}
	// ⚠️ 这一行长得像一份私钥，是因为它**就是**私钥头的形状：它是把真私钥
	// 换成掩码的替换模板 ⇒ 它必须长得一模一样才能替换。⇒ 标记它，让凭据
	// 守卫知道这是「关于凭据的代码」，不是「一份凭据」。
	s = pemRe.ReplaceAllString(s, "-----BEGIN PRIVATE KEY----- [REDACTED] -----END PRIVATE KEY-----") // jms:allow-credential
	s = urlCredentialRe.ReplaceAllString(s, "$1://$2:"+answerMarker+"@")
	s = flagSecretRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := flagSecretRe.FindStringSubmatch(m)
		if len(sub) < 5 {
			return m
		}
		// ⚠️ The groups are: the prefix, the flag's NAME, the separator
		// together with the value, and the value alone.
		sep := strings.TrimSuffix(sub[3], sub[4])
		// ⚠️ "spaces then =" is the shape this rule itself produced on
		// the previous pass ("--password = x") ⇒ taking the "=" for the
		// value would leave the old marker standing and add a new one ⇒
		// the text grows a marker on every pass ⇒ **not idempotent**.
		// ⇒ that case is left to the free-text rule.
		if strings.HasPrefix(sep, " ") || strings.HasPrefix(sep, "\t") {
			if strings.HasPrefix(strings.TrimLeft(sep, " \t"), "=") {
				return m
			}
		}
		// ⚠️ The separator is KEPT as it was (and no extra space is
		// added): the separator already carries the space ⇒ adding one
		// would give "--password  [REDACTED]" ⇒ the next pass sees two
		// spaces and grows another one ⇒ **not idempotent**.
		return sub[1] + sub[2] + sep + answerMarker
	})
	// ⚠️ The query-string parameters run BEFORE the free-text rule: the
	// free-text one reads the URL's "scheme:value" shape first and
	// leaves the whole URL alone ⇒ the token inside it survives.
	s = queryParamRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := queryParamRe.FindStringSubmatch(m)
		if sub == nil || !redact.IsCredentialKey(sub[2]) {
			return m
		}
		return sub[1] + sub[2] + "=" + answerMarker
	})
	// ⚠️ The challenge scheme's PARAMETERS run BEFORE the header rule:
	// the header rule takes the first word after the scheme ⇒ without
	// this the response hash would survive.
	s = authParamsRe.ReplaceAllString(s, "$1"+answerMarker)
	// ⚠️ The Authorization header runs BEFORE the generic key=value rule:
	// the generic one would take "Bearer" for the value and replace the
	// scheme ⇒ the transcript would stop saying HOW it was authenticated.
	s = authHeaderRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := authHeaderRe.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		// ⚠️ The scheme is kept ("Bearer", "Basic", …): it says HOW the
		// caller authenticated, and that is what an audit reader wants.
		// It is empty when the header carries no scheme ⇒ then the whole
		// value goes.
		//
		// ⚠️ And a header that carries the scheme but NO credential
		// ("authorization: bearer") keeps it too: the scheme is a fact
		// about the request, not a secret.
		scheme := strings.TrimSpace(sub[2])
		// ⚠️ A header that carries the scheme but NO credential
		// ("authorization: bearer") ⇒ the scheme is a FACT, not a
		// secret ⇒ it is left exactly as it was ⇒ the transcript does
		// not grow a marker for a credential that was never there.
		if isAuthScheme(sub[3]) {
			return sub[1] + sub[3]
		}
		if scheme == "" {
			return sub[1] + answerMarker
		}
		return sub[1] + scheme + " " + answerMarker
	})
	s = maskFreeText(s)
	return redactJSONCredentials(s)
}

// isAuthScheme says whether a value is an authentication scheme's NAME.
//
// ⚠️ It is a FACT, not a credential: "authorization: bearer" says HOW the
// caller authenticated and carries nothing secret ⇒ the scheme stays.
func isAuthScheme(v string) bool {
	switch strings.ToLower(v) {
	case "bearer", "basic", "digest", "negotiate", "ntlm":
		return true
	}
	return false
}

// maskFreeText masks a credential written as "key=value" / "key: value"
// in free text.
//
// ⚠️ It is separate from redactSecrets because it has to be re-runnable
// on a single VALUE: a value can hold a credential of its own
// ("description=token=abc") ⇒ the key says nothing about it.
//
// ⚠️ And it is the only part that recurses ⇒ re-running the whole
// pipeline on a value would recurse forever.
func maskFreeText(s string) string {
	return maskFreeTextBefore(s, "")
}

// maskEscapedFreeText masks a credential whose value is written with
// escaped quotes — the shape a command's own JSON arguments carry.
//
// ⚠️ It is a rule of its own: the plain rule's value stops at the
// backslash ⇒ the credential would stand in the clear behind it.
func maskEscapedFreeText(s, before string) string {
	return secretEscapedValueRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := secretEscapedValueRe.FindStringSubmatch(m)
		if sub == nil || !redact.IsCredentialKey(sub[1]) {
			return m
		}
		if strings.EqualFold(sub[1], "authorization") ||
			strings.EqualFold(sub[1], "www-authenticate") {
			return m
		}
		return sub[1] + " = " + answerMarker
	})
}

// maskFreeTextBefore is maskFreeText with the text in front of it.
//
// ⚠️ `before` is what tells a query string from a command line: the `&`
// of "PASSWORD=AAA&bbb=1" is part of the credential, while the `&` of
// "?token=abc&user=bob" separates two parameters — and the difference
// is the "?" that OPENS the query string.
//
// ⚠️ It walks the matches BY HAND rather than with ReplaceAllStringFunc:
// the context in front of a match is only known by POSITION, and the
// callback of ReplaceAllStringFunc does not get to see where the match
// was.
func maskFreeTextBefore(s, before string) string {
	// ⚠️ The ESCAPED form runs FIRST: it is a shape of its own, and the
	// plain rule's value stops at the backslash ⇒ the escaped
	// credential would stand in the clear behind it.
	s = maskEscapedFreeText(s, before)
	matches := secretValueRe.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		key := s[m[2]:m[3]]
		sep := s[m[4]:m[5]]
		value := s[m[6]:m[7]]
		tail := ""
		if len(m) > 8 && m[8] >= 0 {
			tail = s[m[8]:m[9]]
		}
		// ⚠️ The context reaches to the end of what came before this
		// match: the "?" of a query string sits there.
		ctx := before + s[last:m[0]]
		// ⚠️ A COMMAND-LINE argument ("--password=x") belongs to the
		// flag rule: the two rules must not both mask it, or the text
		// carries TWO markers ("--password = [REDACTED] [REDACTED]").
		// ⇒ the free-text rule leaves it alone.
		if strings.HasPrefix(key, "-") {
			b.WriteString(s[last:m[1]])
			last = m[1]
			continue
		}
		b.WriteString(s[last:m[0]])
		switch {
		case strings.EqualFold(key, "authorization"),
			strings.EqualFold(key, "www-authenticate"):
			// ⚠️ The auth header's own rule owns it: it keeps the scheme.
			b.WriteString(s[m[0]:m[1]])
		default:
			// ⚠️ A credential whose value is followed by an `&` is one of
			// two things, and only the `&`'s RIGHT SIDE tells them apart:
			//
			//   - a query string's separator ("?token=abc&user=bob") ⇒
			//     the tail is another parameter and has to STAY ⇒ the
			//     transcript must still say WHAT was run;
			//   - the rest of the credential ("export PASSWORD=p&ssw0rd")
			//     ⇒ the tail is SECRET ⇒ the whole value goes.
			//
			// ⚠️ When it cannot tell ⇒ the whole value goes: a tail left
			// in the clear is a credential in the clear.
			//
			// ⚠️ The recursion gets the CONTEXT TOO: the tail is judged
			// against the same "?" ⇒ without it the recursion sees only
			// "&password=def" and cannot tell a query string from a
			// credential ⇒ the SAME "&" gets two different answers in
			// the two passes ⇒ the tail is masked and then unmasked.
			if tail != "" && !ampersandSeparates(tail[1:], ctx) {
				b.WriteString(key + " = " + answerMarker)
			} else {
				b.WriteString(key + " = " + answerMarker +
					maskFreeTextBefore(tail, ctx+key+sep+value))
			}
		}
		last = m[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

// ampersandSeparates says whether the `&` separates two query-string
// parameters rather than being part of a credential.
//
// ⚠️ A parameter name after the `&` ("&user=bob") ⇒ it separates.
//
// ⚠️ And anything else ("&ssw0rd") ⇒ it is part of the credential ⇒
// the safe side is to mask the whole thing.
//
// ⚠️ The name may be preceded by SPACES: the rule rewrites the
// separator to " = " ⇒ the text after a mask reads "&password = …"
// ⇒ a pattern that demanded the name immediately after the "&" would
// not recognise it ⇒ the tail would be masked as if it were part of
// the credential.
//
// ⚠️ And it has to be the SAME query string: "PASSWORD=AAA&bbb=1" has
// no "?" in front ⇒ the `&` is part of the credential ⇒ the whole
// value goes. ⇒ the "?" has to OPEN this query string: the text
// between it and the match must have no whitespace or quote in it.
func ampersandSeparates(rest, before string) bool {
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" {
		return true
	}
	switch rest[0] {
	case '&', ' ', '\t', '\n', '/', '.', ';', '|', '>', '<':
		return true
	}
	// A parameter name: "name=" at the start of the rest.
	if !paramNameRe.MatchString(rest) {
		return false
	}
	return redact.InQueryString(before)
}

// redactJSONCredentials masks the JSON form of a credential.
//
// ⚠️ It is the free-text rules' blind spot: they know "key=value" and
// "key: value" in prose, and a JSON field is neither ⇒ `{"password":"x"}`
// used to reach the agent in the clear.
//
// ⚠️ And the word list is the SHARED one (redact.IsCredentialKey) — the
// whole point of the shared base is that this path does not carry a
// second one: fourteen of its words used to be unknown here, so a word
// was masked in the JSON form and not in the prose form.
func redactJSONCredentials(s string) string {
	s = answerEscapedKeyRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := answerEscapedKeyRe.FindStringSubmatch(m)
		if sub == nil || !redact.IsCredentialKey(sub[1]) {
			return m
		}
		return redact.EscapeLiteral(1) + `"` + sub[1] + redact.EscapeLiteral(1) +
			`":` + redact.EscapeLiteral(1) + `"` + answerMarker + redact.EscapeLiteral(1) + `"`
	})
	// ⚠️ The DOUBLE-escaped form: the SDK serialises the arguments as
	// JSON, and a command that carries its own JSON arguments ends up
	// escaped twice ⇒ the one-level rule above does not match it.
	s = answerDoubleEscapedKeyRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := answerDoubleEscapedKeyRe.FindStringSubmatch(m)
		if sub == nil || !redact.IsCredentialKey(sub[1]) {
			return m
		}
		// ⚠️ The replacement keeps the SAME escape depth as the match: one
		// level more and the document stops being the document it was.
		return redact.EscapeLiteral(2) + `"` + sub[1] + redact.EscapeLiteral(2) +
			`":` + redact.EscapeLiteral(2) + `"` + answerMarker + redact.EscapeLiteral(2) + `"`
	})
	// ⚠️ The non-scalar rule runs FIRST: the string rule would match
	// only the first fragment of `{"secret":{"value":"x"}}` ⇒ the
	// document would end up broken AND the inner credential would still
	// be standing in the clear.
	//
	// ⚠️ It walks the matches by hand rather than with
	// ReplaceAllStringFunc: the END of the value is found by matching
	// brackets (a nested object has more than one `}`), and the callback
	// of ReplaceAllStringFunc does not get to see where the match was.
	s = maskNonScalarCredentials(s)
	return answerKeyRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := answerKeyRe.FindStringSubmatch(m)
		if sub == nil || !redact.IsCredentialKey(sub[1]) {
			return m
		}
		// ⚠️ A boolean is a FACT, not a credential: `{"has_password":true}`
		// says the account HAS a password, and masking it would turn that
		// fact into a string. `null` says "there is none" ⇒ a fact too.
		if isFactLiteral(sub[2]) {
			return m
		}
		return `"` + sub[1] + `":"` + answerMarker + `"`
	})
}

// maskNonScalarCredentials masks a credential field whose value is an
// object or an array — the WHOLE value, brackets included.
//
// ⚠️ The value has to be taken out WHOLE: masking only the fragment
// would leave `{"secret":"[REDACTED]"{"value":…}}` — broken JSON with
// the credential still inside it.
//
// ⚠️ And the end is found by matching brackets: a nested object has more
// than one `}` ⇒ a pattern that stopped at the first one would leave
// the tail behind AND break the document.
func maskNonScalarCredentials(s string) string {
	matches := answerNonScalarRe.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		key := s[m[2]:m[3]]
		if !redact.IsCredentialKey(key) {
			continue
		}
		// ⚠️ The value starts at the bracket the match ended on. The
		// boundary mechanics are the SHARED ones (redact.MatchBracket):
		// where a value ends is not a formatting decision ⇒ not copied.
		start := m[1] - 1
		end, ok := redact.MatchBracket(s, start)
		if !ok {
			// ⚠️ An unbalanced bracket: the value's end cannot be found
			// ⇒ the text is left exactly as it was (masking a guess
			// would break the document).
			continue
		}
		b.WriteString(s[last:m[0]])
		b.WriteString(`"` + key + `":"` + answerMarker + `"`)
		last = end
	}
	b.WriteString(s[last:])
	return b.String()
}

// isFactLiteral says whether a value is a FACT rather than a credential:
// a boolean or a null.
//
// ⚠️ The comparison is case-insensitive, and it has to be: the pattern
// that finds the value is case-insensitive too (`(?i)`), so it matches
// "TRUE" and "Null" as readily as "true" and "null" ⇒ a case-sensitive
// comparison here would read "TRUE" as a CREDENTIAL and mask it.
func isFactLiteral(v string) bool {
	return strings.EqualFold(v, "true") || strings.EqualFold(v, "false") ||
		strings.EqualFold(v, "null")
}

// redactAll applies redactSecrets to a list of strings.
func redactAll(list []string) []string {
	out := make([]string, len(list))
	for i, v := range list {
		out[i] = redactSecrets(v)
	}
	return out
}

// looksSecretive reports whether a string contains anything worth masking, so
// a caller can note it rather than silently altering the audit.
func looksSecretive(s string) bool {
	return redactSecrets(s) != s
}

// trimSecretNote is the standard warning appended to redacted output.
const trimSecretNote = "credential-looking values in this output are masked"
