package handler

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// authorizedKey renders a public key in the authorized_keys form.
func authorizedKey(t *testing.T, pub ssh.PublicKey) string {
	t.Helper()
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
}

// rsaPublicKey makes an ssh-rsa public key.
func rsaPublicKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate an RSA key: %v", err)
	}
	pub, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("make the public key: %v", err)
	}
	return authorizedKey(t, pub)
}

// ed25519PublicKey makes an ssh-ed25519 public key.
func ed25519PublicKey(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate an ed25519 key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("make the public key: %v", err)
	}
	return authorizedKey(t, sshPub)
}

// TestCheckRSAPublicKey is the table for the write-path check.
//
// The case that matters most is the multi-line one: the bastion stores the
// WHOLE string while the public-key parser judges only the first line it can
// parse, so a two-line input can pass the check and then be stored as two
// keys -- with the bastion accepting the ed25519 one silently. That is
// exactly the failure this tool exists to prevent.
func TestCheckRSAPublicKey(t *testing.T) {
	rsaKey := rsaPublicKey(t)
	edKey := ed25519PublicKey(t)

	cases := []struct {
		name     string
		key      string
		wantErr  bool
		contains string
	}{
		{"合法 rsa", rsaKey, false, ""},
		{"带注释的 rsa", rsaKey + " someone@example", false, ""},
		{"前后有空白", "  " + rsaKey + "  ", false, ""},
		{"空", "", true, "empty"},
		{"只有空白", "   \n  ", true, "empty"},
		{"ed25519", edKey, true, "ed25519"},
		{"损坏的 rsa", "ssh-rsa AAAA-not-base64 user@host", true, "not a usable"},
		{"类型不对", "ssh-dss AAAA user@host", true, "must start with 'ssh-rsa '"},
		{"多行：第一行 ed25519", edKey + "\n" + rsaKey, true, "exactly one key"},
		{"多行：垃圾在前", "garbage\n" + rsaKey, true, "exactly one key"},
		{"多行：两把 rsa", rsaKey + "\n" + rsaKey, true, "exactly one key"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkRSAPublicKey(c.key)
			if c.wantErr {
				if err == nil {
					t.Fatalf("输入应当被拒，实际通过了：%q", c.key)
				}
				if c.contains != "" && !strings.Contains(err.Error(), c.contains) {
					t.Errorf("错误信息缺少 %q，实际 = %v", c.contains, err)
				}
				return
			}
			if err != nil {
				t.Errorf("输入应当通过，实际被拒：%v（输入 %q）", err, c.key)
			}
		})
	}
}
