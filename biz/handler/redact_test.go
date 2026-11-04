package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

// An audit tool that echoes the command log verbatim will print the private
// key somebody typed. The VALUE has to go; the command must stay, because
// "cat /etc/wireguard/wg0.conf" is exactly what an audit wants to show.
func TestRedactSecretsMasksValues(t *testing.T) {
	cases := []struct{ in, secret, readable string }{
		{"PrivateKey = EMrb0obFo0sJqh6KjNz9WThsO9gh4P1Lrk0/1SgcXno=",
			"EMrb0obFo0sJqh6KjNz9WThsO9gh4P1Lrk0/1SgcXno=", "PrivateKey"},
		{"password = hunter2", "hunter2", "password"},
		{"token: abc123XYZ", "abc123XYZ", "token"},
		{"mysql -uroot --password=hunter2 db", "hunter2", "--password"},
		{"Authorization: Bearer eyJhbGciOi.JIUzI1NiJ9", "eyJhbGciOi.JIUzI1NiJ9", "Bearer"},
		{"-----BEGIN RSA PRIVATE KEY-----\nMIIEow\n-----END RSA PRIVATE KEY-----", "MIIEow", "REDACTED"},
	}
	for _, c := range cases {
		got := redactSecrets(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("redactSecrets(%q) = %q, still contains the secret", c.in, got)
		}
		if !strings.Contains(got, c.readable) {
			t.Errorf("redactSecrets(%q) = %q, lost the readable part %q", c.in, got, c.readable)
		}
	}
}

// Ordinary audit output must come through untouched, or the tool stops being
// worth reading.
func TestRedactSecretsLeavesOrdinaryOutputAlone(t *testing.T) {
	cases := []string{
		"cat /etc/wireguard/wg0.conf",
		"ls -la /home",
		"[Interface]\nAddress = 203.0.113.8/24\nListenPort = 56789",
		"root@VM-0-8-ubuntu:~# uptime",
		"total 4\ndrwxr-xr-x 2 root root 4096 Jan 1 00:00 .",
	}
	for _, in := range cases {
		if got := redactSecrets(in); got != in {
			t.Errorf("redactSecrets(%q) = %q, want it unchanged", in, got)
		}
	}
}

// The JSON form of a credential must not reach the agent.
//
// ⚠️ The answer path used to know the FREE-TEXT shape only ("key=value",
// "key: value"), so a recorded command that carried its own JSON
// arguments — the most ordinary thing in the world — passed through
// untouched: `{"password":"x"}` reached the agent's context window in
// the clear.
//
// ⚠️ And the words were the answer path's own list, which had drifted
// from the trail's: fourteen of the words the trail knows were unknown
// here ("passphrase", "credential", "cookie", "kubeconfig", "id_token",
// "refresh_token", …) ⇒ a word masked in one path and not in the other.
// The word list is the trail's now.
func TestRedactSecretsMasksTheJSONForm(t *testing.T) {
	cases := []struct{ in, secret string }{
		{`{"password":"hunter2","user":"bob"}`, "hunter2"},
		{`{"api_key":"AKIAX","user":"bob"}`, "AKIAX"},
		{`{"my_password":"hunter2"}`, "hunter2"},
		{`{"client_secret":"s3cr3t"}`, "s3cr3t"},
		{`{"params":"{\"password\":\"hunter2\"}"}`, "hunter2"},
		// ⚠️ The words the answer path did not know.
		{`{"passphrase":"hunter2"}`, "hunter2"},
		{`{"credential":"hunter2"}`, "hunter2"},
		{`{"cookie":"s=1"}`, "s=1"},
		{`{"kubeconfig":"aaa"}`, "aaa"},
		{`{"id_token":"aaa"}`, "aaa"},
		{`{"refresh_token":"aaa"}`, "aaa"},
		{`{"authorization":"Basic eA=="}`, "eA=="},
	}
	for _, c := range cases {
		got := redactSecrets(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("redactSecrets(%q) = %q, still contains the secret", c.in, got)
		}
	}
}

// A non-scalar value must be masked WHOLE, and the document must survive.
//
// ⚠️ The string rule matches only the first FRAGMENT of an object or an
// array, and what stands behind that fragment is a field whose name may
// not say "credential" at all (`{"secret":{"value":"s3cr3t"}}` ⇒ the
// string rule matches `"secret":{` and leaves the inner credential in
// the clear).
//
// ⚠️ And masking only the fragment would leave broken JSON —
// `{"secret":"[REDACTED]"{"value":…}}` — with the credential still
// inside it.
func TestRedactSecretsMasksNonScalarValues(t *testing.T) {
	for _, c := range []struct{ in, secret string }{
		{`{"secret":{"value":"s3cr3t"}}`, "s3cr3t"},
		{`{"password":{"value":"s3cr3t"}}`, "s3cr3t"},
		{`{"password":["a","b"]}`, `"a"`},
		{`{"secret":{"nested":{"deep":"s3cr3t"}}}`, "s3cr3t"},
	} {
		got := redactSecrets(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("redactSecrets(%q) = %q, still contains the secret", c.in, got)
		}
		// ⚠️ And the document must still be a document.
		var v any
		if err := json.Unmarshal([]byte(got), &v); err != nil {
			t.Errorf("redactSecrets(%q) = %q, the document is no longer JSON (%v)",
				c.in, got, err)
		}
	}
}

// An unquoted value must not eat the NEXT field.
//
// ⚠️ The shape rules eat the key's closing quote when they mask a field,
// so the text right after such a field starts with a quote ⇒ an unquoted
// value that ran into it would swallow the next field with it, and the
// answer would lose a fact ("user" disappears).
func TestRedactSecretsDoesNotEatTheNextField(t *testing.T) {
	in := `{"password":x"user":"bob"}`
	got := redactSecrets(in)
	if !strings.Contains(got, "bob") {
		t.Errorf("redactSecrets(%q) = %q, the next field was eaten", in, got)
	}
}

// The free-text form must use the SAME word list as the JSON form.
//
// ⚠️ The answer path used to carry its own word list, and the two
// drifted: fourteen of the words the trail knows were unknown here ⇒
// `passphrase=hunter2` reached the agent in the clear, while the JSON
// form of the SAME word was masked ⇒ one word masked in one shape and
// not in the other.
//
// ⚠️ The word list is the trail's (redact.IsCredentialKey) — the whole
// point of the shared base is that this path does not carry a second
// one.
func TestRedactSecretsUsesTheSharedWordList(t *testing.T) {
	for _, c := range []struct{ in, secret string }{
		{`passphrase=hunter2`, "hunter2"},
		{`credential=hunter2`, "hunter2"},
		{`pwd=hunter2`, "hunter2"},
		{`cookie=hunter2`, "hunter2"},
		{`kubeconfig=hunter2`, "hunter2"},
		{`authorization=hunter2`, "hunter2"},
		{`bearer=hunter2`, "hunter2"},
		{`token_value=hunter2`, "hunter2"},
		{`secret_value=hunter2`, "hunter2"},
		{`password=hunter2`, "hunter2"},
		{`password: hunter2`, "hunter2"},
	} {
		got := redactSecrets(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("redactSecrets(%q) = %q, still contains the secret", c.in, got)
		}
	}
}

// The authentication scheme must survive — it says HOW, not WHO.
//
// ⚠️ The generic key=value rule would match "Authorization: Bearer …"
// and take "Bearer" for the VALUE ⇒ the scheme would be replaced and
// the transcript would stop saying how the caller authenticated.
func TestRedactSecretsKeepsTheAuthScheme(t *testing.T) {
	for _, c := range []struct{ in, keep string }{
		{`Authorization: Bearer eyJhbGciOi.JIUzI1NiJ9`, "Bearer"},
		{`Authorization: Basic dXNlcjpwYXNz`, "Basic"},
	} {
		got := redactSecrets(c.in)
		if !strings.Contains(got, c.keep) {
			t.Errorf("redactSecrets(%q) = %q, lost the scheme %q", c.in, got, c.keep)
		}
		if strings.Contains(got, "eyJhbGciOi") || strings.Contains(got, "dXNlcjpwYXNz") {
			t.Errorf("redactSecrets(%q) = %q, the credential survived", c.in, got)
		}
	}
}

// The credential of an auth header is its ARGUMENT, not the rest of the
// line — and a challenge scheme's PARAMETERS are all credential.
//
// ⚠️ A Digest header carries parameters ("username=…, response=…") ⇒
// taking only the first word leaves the response hash in the clear.
//
// ⚠️ And the run must NOT eat to the end of the line: the upstream rules
// do, and that cost the transcript the URL in
// `curl -H "Authorization: Bearer x" https://h/api` ⇒ the reader lost
// WHAT was run.
func TestRedactSecretsMasksTheAuthCredentialOnly(t *testing.T) {
	got := redactSecrets(`Authorization: Digest username="x", response="y"`)
	if strings.Contains(got, "response") || strings.Contains(got, `"y"`) {
		t.Errorf("redactSecrets kept a challenge parameter: %q", got)
	}
	got = redactSecrets(`curl -H "Authorization: Bearer abc" https://x/api`)
	if strings.Contains(got, "abc") {
		t.Errorf("redactSecrets kept the token: %q", got)
	}
	if !strings.Contains(got, "https://x/api") {
		t.Errorf("redactSecrets ate the URL after the header: %q", got)
	}
}

// A credential-shaped parameter inside a URL query string.
//
// ⚠️ The free-text rule cannot see it: in a URL the "scheme:value" shape
// ("https://x/api?token=abc") is matched FIRST ⇒ the rule decides
// "https" is not a credential and leaves the WHOLE URL alone ⇒ the token
// inside it survives. ⇒ the query-string parameters need their own rule.
//
// ⚠️ And the parameter name is the credential: "token=…" in a query
// string is a token exactly as much as "token=…" in prose.
func TestRedactSecretsMasksQueryParameters(t *testing.T) {
	for _, c := range []struct{ in, secret string }{
		{`curl https://x/api?token=abc&user=bob`, "abc"},
		{`{"url":"https://x/api?token=abc&user=bob"}`, "abc"},
		{`https://x/api?password=abc&user=bob`, "abc"},
		{`?api_key=abc&user=bob`, "abc"},
	} {
		got := redactSecrets(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("redactSecrets(%q) = %q, still contains the secret", c.in, got)
		}
	}
	// ⚠️ And the OTHER parameters stay: the transcript has to say WHAT
	// was run.
	got := redactSecrets(`curl https://x/api?token=abc&user=bob`)
	if !strings.Contains(got, "user=bob") {
		t.Errorf("redactSecrets ate the other parameters: %q", got)
	}
}

// A URL with a credential IN it has no key name to key on.//
// ⚠️ "https://user:pass@host" ⇒ the credential is the part before the "@"
// and there is no field name ⇒ no key-based rule can ever catch it ⇒ it
// needs its own rule (the upstream rules have one; the answer path does
// not RUN the upstream rules ⇒ it needed its own).
func TestRedactSecretsMasksTheURLCredential(t *testing.T) {
	for _, c := range []struct{ in, secret string }{
		{`https://user:pass@host/path`, "pass"},
		{`postgres://u:p@db/x`, "p@"},
		{`mysql://root:secret@10.0.0.1:3306/db`, "secret"},
	} {
		got := redactSecrets(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("redactSecrets(%q) = %q, still contains the secret", c.in, got)
		}
	}
}

// A fact must not become a credential.
//
// ⚠️ The answer path masks the VALUE and keeps the key — but a value that
// says something about the account is a fact, and masking it turns the
// fact into a string: "this account HAS a password" and "this account
// has NO password" are different statements, and only one of them may
// survive.
//
// ⚠️ And an innocent field whose name merely ENDS with a credential word
// ("key_id", "token_count", "secret_type") is not a credential at all —
// the trail's word list has always been careful about that, and the
// answer path now uses the same one.
func TestRedactSecretsKeepsTheFacts(t *testing.T) {
	for _, in := range []string{
		`{"has_password":true}`,
		`{"need_password":false}`,
		`{"password":null}`,
		`{"session":"abc"}`,
		`{"key_id":"k"}`,
		`{"secret_type":"password"}`,
		`{"token_count":5}`,
		`{"keyword":"x"}`,
		`{"monkey":"x"}`,
		`{"public_key":"ssh-rsa AAA"}`,
		`{"tokens":"t"}`,
		`{"passwords":"p"}`,
		`{"session_id":"s"}`,
		`{"secretary":"bob"}`,
		`{"keywords":["a","b"]}`,
	} {
		if got := redactSecrets(in); got != in {
			t.Errorf("redactSecrets(%q) = %q, want it unchanged — that is a fact, not a credential",
				in, got)
		}
	}
}

// A non-credential key says nothing about the VALUE.
//
// ⚠️ The rule matches "key=value" for ANY key and then asks whether the
// key names a credential. Returning the whole match when it does not
// would hand the VALUE to the regex engine and never look at it again
// ⇒ a credential inside a value ("description=token=abc") would reach
// the agent in the clear.
//
// ⚠️ The trail's path never had this problem: its rule looks for the
// credential words themselves, so it finds the token wherever it is.
// The answer path's rule looks for a key ⇒ it has to look INSIDE the
// value too.
func TestRedactSecretsLooksInsideANonCredentialValue(t *testing.T) {
	for _, c := range []struct{ in, secret string }{
		{`description=token=abc`, "abc"},
		{`note=password=hunter2`, "hunter2"},
		{`x=secret=s3cr3t`, "s3cr3t"},
	} {
		got := redactSecrets(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("redactSecrets(%q) = %q, still contains the secret", c.in, got)
		}
	}
}

// A "}" inside a string value is not the end of the object.
//
// ⚠️ The mask of a non-scalar value has to find where the value ENDS,
// and a "}" inside a string ("{"note":"a}b"}") is not it ⇒ counting it
// ends the value in the middle ⇒ the rest of the object stands in the
// clear AND the document is left broken.
//
// ⚠️ The trail's path knows this (its matcher tracks strings); the
// answer path's did not.
func TestRedactSecretsFindsTheEndOfANonScalarValue(t *testing.T) {
	in := `{"secret":{"note":"a}b","value":"s3cr3t"}}`
	got := redactSecrets(in)
	if strings.Contains(got, "s3cr3t") {
		t.Errorf("redactSecrets(%q) = %q, the inner credential survived", in, got)
	}
	var v any
	if err := json.Unmarshal([]byte(got), &v); err != nil {
		t.Errorf("redactSecrets(%q) = %q, the document is no longer JSON (%v)",
			in, got, err)
	}
}

// The ESCAPED prose form is the shape a command's own JSON arguments
// carry.
//
// ⚠️ Without it the value stops at the backslash and the credential
// stands in the clear behind it.
func TestRedactSecretsMasksTheEscapedProseForm(t *testing.T) {
	in := `{"cmd":"export PASSWORD=\"SECRET\""}`
	got := redactSecrets(in)
	if strings.Contains(got, "SECRET") {
		t.Errorf("redactSecrets(%q) = %q, the credential survived", in, got)
	}
}

// The answer path has to be IDEMPOTENT.
//
// ⚠️ The trail's path is idempotent (that is what lets the registry
// scrub the arguments and the trail scrub them again). The answer path
// is applied to text that may already have been through it ⇒ it has to
// give the same answer twice.
//
// ⚠️ The failure is subtle: the marker gets CHEWED UP ("x-api-key =
// [REDACTED]" ⇒ "x-api-key [REDACTED] [REDACTED]"), and nothing looks
// wrong until the text is read twice.
func TestRedactSecretsIsIdempotent(t *testing.T) {
	for _, in := range []string{
		`{"password":"x"}`,
		`{"secret":{"value":"s3cr3t"}}`,
		`{"params":"{\"password\":\"x\"}"}`,
		`passphrase=hunter2`,
		`https://user:pass@host/path`,
		`curl https://x/api?token=abc&user=bob`,
		`Authorization: Digest username="x", response="y"`,
		`Authorization: Bearer eyJhbGciOi.JIUzI1NiJ9`,
		`x-api-key: value`,
		`session-token: v`,
		`description=token=abc`,
		`?token=abc&password=def&q=1`,
		`{"has_password":true}`,
		`{"password":null}`,
	} {
		once := redactSecrets(in)
		if twice := redactSecrets(once); twice != once {
			t.Errorf("redacting twice gives two different strings:\n once:  %s\n twice: %s",
				once, twice)
		}
	}
}

// A credential whose value carries an "&" is masked WHOLE — unless that
// "&" separates two query-string parameters.
//
// ⚠️ "export PASSWORD=p&ssw0rd" ⇒ "ssw0rd" is the rest of the
// credential ⇒ the whole value goes. "?token=abc&user=bob" ⇒ "user=bob"
// is another parameter ⇒ it stays.
//
// ⚠️ And the "&" of "PASSWORD=AAA&bbb=1" is part of the credential too
// ⇒ it has to be the SAME query string ⇒ a "?" has to open it.
func TestRedactSecretsHandlesTheAmpersand(t *testing.T) {
	for _, in := range []string{
		`export PASSWORD=p&ssw0rd`,
		`PASSWORD=AAA&bbb=1`,
		`{"cmd":"export PASSWORD=p&ssw0rd"}`,
	} {
		out := redactSecrets(in)
		if strings.Contains(out, "ssw0rd") || strings.Contains(out, "bbb=1") {
			t.Errorf("the credential tail stayed in the clear %s -> %s", in, out)
		}
	}
	out := redactSecrets(`?token=abc&user=bob`)
	if !strings.Contains(out, "user=bob") {
		t.Errorf("a query string lost its other parameters: %s", out)
	}
}

// A credential in a URL and a credential in a query string have shapes
// their own.
//
// ⚠️ The URL's credential has no key name ("user" is not a credential
// word) ⇒ no key-based rule can ever catch it ⇒ it needs its own.
//
// ⚠️ And the password runs to the LAST "@": a password may contain one
// itself ("p@ss") ⇒ stopping at the first would leave the tail in the
// clear.
//
// ⚠️ The query string's value may be QUOTED ("?token=\"a b\""): the
// value shape has to take it whole, or the quotes stay standing next to
// the mask and the value is half in the clear.
func TestRedactSecretsMasksTheURLAndQueryShapes(t *testing.T) {
	for _, c := range []struct{ in, secret string }{
		{`https://user:pass@host/path`, "pass"},
		// ⚠️ The password runs to the LAST "@": a password may contain one
		// itself ⇒ stopping at the first would leave "ss@host" in the
		// clear ⇒ and the assertion has to name THAT, not the whole
		// password ("p@ss" would not be found in "[REDACTED]@ss" ⇒ the
		// test would pass on a leak).
		{`https://user:p@ss@host/path`, "ss@host"},
		{`https://user:pa/ss@host/path`, "ss@host"},
		{`postgres://u:p@h/db`, "p@"},
		{`?token="a b"&user=bob`, "a b"},
		{`?token=abc&user=bob`, "abc"},
	} {
		got := redactSecrets(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("redactSecrets(%q) = %q, still contains the secret", c.in, got)
		}
	}
}

// A credential must be found WHEREVER it is — not only where the
// free-text rule happens to start.
//
// ⚠️ The rule used to match ANY key and ask afterwards whether the key
// names a credential. That has a blind spot: the match eats the word in
// FRONT of the credential ("login failed: password: x" ⇒ the key comes
// out as "login" and the value as "failed:") ⇒ the credential never gets
// a match of its own ⇒ it reached the agent in the clear.
//
// ⚠️ The rule's key is now built from the SHARED word list, so the match
// STARTS at the credential ⇒ it is found wherever it is.
func TestRedactSecretsFindsTheCredentialWhereverItIs(t *testing.T) {
	for _, c := range []struct{ in, secret string }{
		{`login failed: password: S3CRETVALUE`, "S3CRETVALUE"},
		{`error: token: S3CRETVALUE`, "S3CRETVALUE"},
		{`x: y: z: password: S3CRETVALUE`, "S3CRETVALUE"},
		{`a=b password=c`, "c"},
	} {
		got := redactSecrets(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("redactSecrets(%q) = %q, still contains the secret", c.in, got)
		}
	}
}

// The command-line argument must not be masked TWICE.
//
// ⚠️ The flag rule and the free-text rule both match "--password=x" ⇒
// two markers ("--password = [REDACTED] [REDACTED]") ⇒ and on the next
// pass the text grows ANOTHER one ⇒ **not idempotent**.
func TestRedactSecretsMasksACommandLineArgumentOnce(t *testing.T) {
	// ⚠️ BOTH separators are a shape: "--password=x" and "--password x".
	// ⚠️ ALL THREE separators are a shape: "=", a space and ":".
	for _, in := range []string{
		`mysql -uroot --password=S3CRETVALUE`,
		`mysql -uroot --password S3CRETVALUE`,
		`mysql -uroot --password: S3CRETVALUE`,
	} {
		once := redactSecrets(in)
		if strings.Contains(once, "S3CRETVALUE") {
			t.Errorf("redactSecrets(%q) = %q, the secret survived", in, once)
		}
		if strings.Count(once, "[REDACTED]") != 1 {
			t.Errorf("redactSecrets(%q) = %q, the argument was masked more than once", in, once)
		}
		if twice := redactSecrets(once); twice != once {
			t.Errorf("redacting twice gives two different strings:\n once:  %s\n twice: %s",
				once, twice)
		}
	}
}

// The DOUBLE-escaped JSON must be masked on the answer path too.
//
// ⚠️ The SDK serialises the arguments as JSON, and a command that
// carries its own JSON arguments ends up escaped twice ⇒ the one-level
// rule does not match it.
func TestRedactSecretsMasksTheDoubleEscapedJSON(t *testing.T) {
	in := `{"params":"{\\"password\\":\\"S3CRETVALUE\\"}"}`
	got := redactSecrets(in)
	if strings.Contains(got, "S3CRETVALUE") {
		t.Errorf("redactSecrets(%q) = %q, still contains the secret", in, got)
	}
}

// A scheme without a credential is a FACT — it must not grow a marker.
//
// ⚠️ "authorization: bearer" says HOW the caller authenticated and
// carries nothing secret ⇒ masking it would put a credential in the
// transcript that was never there (and the reader would take the text
// for a masked one).
func TestRedactSecretsKeepsASchemeWithoutACredential(t *testing.T) {
	for _, in := range []string{
		`authorization: bearer`,
		`Authorization: Basic`,
	} {
		if got := redactSecrets(in); got != in {
			t.Errorf("redactSecrets(%q) = %q, want it unchanged — a scheme is a fact", in, got)
		}
	}
}

// The prose form must be masked in BOTH shapes — with a separator and
// with a space.
//
// ⚠️ The trail's rule masks "the password was v" (the key names a
// credential and the space separates them) ⇒ the answer path has to
// agree ⇒ otherwise the same word is a credential on one path and not
// on the other.
func TestRedactSecretsMasksTheProseShapeWithASpace(t *testing.T) {
	for _, in := range []string{
		`the password was v`,
		`login failed password S3CRETVALUE`,
	} {
		got := redactSecrets(in)
		if strings.Contains(got, "S3CRETVALUE") || strings.Contains(got, " was v") {
			t.Errorf("redactSecrets(%q) = %q, the credential survived", in, got)
		}
	}
}
