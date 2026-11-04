// Package conf holds the server's configuration.
//
// The configuration is environment-driven on purpose: an MCP server is
// launched by the agent runtime, not by a supervisor with a config file on
// disk. The struct is still a struct (not a bag of os.Getenv calls) so the
// settings can be validated once, at start-up, instead of at every call
// site — and so `--doctor` can report the effective configuration.
//
// The tag machinery comes from oekit/core/config: the `env` tag says where a
// value comes from, the `default` tag says what it falls back to, and the
// three passes (env → defaults → validation) run in that order. The
// precedence is the one the rest of the oekit family uses.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/oekit/core/config"

	"github.com/yi-nology/jumpserver-kit/pkg/audit"
	"github.com/yi-nology/jumpserver-kit/pkg/jmsapi"
)

// Config is the effective configuration of the management-plane server.
type Config struct {
	// BaseURL is the root of the JumpServer deployment, without a trailing
	// slash and without /api/v1 (the client appends that itself).
	//
	// ⚠️ It has NO default: a default would point one deployment's clients
	// at somebody else's bastion, and the failure would look like "the
	// bastion is down" rather than "this was never configured".
	BaseURL string `yaml:"base_url" env:"JUMPSERVER_URL"`

	// Token is the JumpServer PrivateToken. It is the only credential the
	// management plane needs; v5 dropped AccessKey/Bearer auth and its OAuth
	// tokens expire after 24h, which makes them useless for a long-running
	// process.
	//
	// ⚠️ It is NOT a v5-only credential: v4 registers
	// PrivateTokenAuthentication too (DRF's TokenAuthentication with
	// model=PrivateToken), so the same header means the same thing on both
	// versions.
	Token string `yaml:"token" env:"JUMPSERVER_TOKEN"`

	// Username and Password are the alternative credential: a LOGIN that
	// the bastion exchanges for a Bearer token
	// (POST /api/v1/authentication/auth/). It is what a deployment that
	// cannot mint a PrivateToken has, and it exists on v4 and on v5 alike.
	//
	// ⚠️ The token WINS when both are set: it does not expire, while the
	// Bearer token does, and a long-running process would have to keep the
	// login alive. The doctor reports that both are configured.
	Username string `yaml:"username" env:"JUMPSERVER_USERNAME"`
	Password string `yaml:"password" env:"JUMPSERVER_PASSWORD"`

	// OTP is the base32 TOTP secret, used when the deployment demands a
	// second factor. It is optional: a deployment without MFA never asks
	// for it.
	OTP string `yaml:"otp" env:"JUMPSERVER_OTP"`

	// Version says which JumpServer generation the deployment runs: "v4",
	// "v5", or "auto" to detect it at the first call that needs the answer.
	//
	// ⚠️ It decides ONE thing: whether the Chat AI delegation scheme
	// exists. v4 has no such scheme, so on v4 the calls run as the API
	// credential's owner rather than as a delegated user. Everything else
	// — the endpoint table, the header, the connection-token body, the
	// response shapes — is the same on both versions.
	Version string `yaml:"version" env:"JUMPSERVER_VERSION" default:"auto" options:"v4,v5,auto"`

	// Timeout bounds a single API call. The bastion is a monolith and a
	// connectivity check on 50 assets is slow, so this is generous on
	// purpose.
	Timeout time.Duration `yaml:"timeout" env:"JUMPSERVER_TIMEOUT" default:"90s"`

	// ReadOnly turns the server into a read-only one: every write tool is
	// refused before it runs. It is the switch to flip when an agent should
	// look but not touch — an access review, a capacity sweep, an incident
	// timeline. The gate is structural (see pkg/reg), not a convention.
	ReadOnly bool `yaml:"read_only" env:"JMS_READ_ONLY" default:"false"`

	// DelegationUserID is the JumpServer user that delegation calls act as.
	// Delegation requests are attributed to this user on the bastion side,
	// NOT to the owner of the long-lived token — which is the only way to
	// act as a specific user (registering an SSH public key, for example).
	DelegationUserID string `yaml:"delegation_user_id" env:"JMS_DELEGATION_USER_ID"`

	// DelegationOrgID is the organization the delegated calls run in.
	DelegationOrgID string `yaml:"delegation_org_id" env:"JMS_DELEGATION_ORG_ID"`

	// RequestsPerSecond caps how fast the client calls the bastion. The
	// bastion is a monolith; a runaway agent does not need to find that out
	// the hard way. The burst is the allowance for a call that legitimately
	// needs several requests at once (a list plus its details).
	RequestsPerSecond int `yaml:"requests_per_second" env:"JMS_REQUESTS_PER_SECOND" default:"20"`
	Burst             int `yaml:"burst" env:"JMS_BURST" default:"40"`

	// ServerName identifies this server in the audit trail of the gate.
	ServerName string `yaml:"server_name" env:"JMS_SERVER_NAME" default:"jumpserver-mgmt"`

	// AuditMode says whether the gate's trail is written to a FILE: "file"
	// or "off" (the log only).
	//
	// The file is the half that answers "who tried to run rm -rf on that
	// box and was stopped": such a call never reaches the bastion, so the
	// bastion has no session and no log line for it.
	AuditMode string `yaml:"audit_mode" env:"JMS_AUDIT" default:"file" options:"file,off"`

	// AuditDir is where the trail files are written.
	AuditDir string `yaml:"audit_dir" env:"JMS_AUDIT_DIR" default:"~/.local/state/jumpserver-mgmt/audit"`
	// AuditRetentionDays is how many days of trail files are kept.
	//
	// ⚠️ The trail is one file per PROCESS, so the directory grows with
	// the number of restarts — and every query that scans it pays for the
	// whole history. Zero keeps them all.
	AuditRetentionDays int `yaml:"audit_retention_days" env:"JMS_AUDIT_RETENTION_DAYS" default:"7"`

	// HTTPAddr is where the HTTP surface listens: the REST face
	// (/api/v1/…) plus MCP over streamable HTTP (/mcp). Empty means no
	// HTTP surface — the server speaks stdio only, which is the mode an
	// MCP runtime launches it in.
	//
	// ⚠️ The default is empty ON PURPOSE: turning a stdio server into a
	// network listener is an operator's decision, not a default. And
	// 127.0.0.1, not 0.0.0.0 — these routes call the bastion with the
	// deployment's own credential; exposing that to a network needs a
	// proxy and a TLS story this process does not have.
	HTTPAddr string `yaml:"http_addr" env:"JMS_HTTP_ADDR"`
	// HTTPToken is the bearer token the HTTP surface demands on every
	// route except /api/v1/health.
	//
	// ⚠️ There is no "auth off": JMS_HTTP_ADDR without JMS_HTTP_TOKEN is
	// refused at start-up. A surface that answers tool calls to whoever
	// can reach the port is not a surface this server will serve.
	HTTPToken string `yaml:"http_token" env:"JMS_HTTP_TOKEN"`

	// The kitex RPC face: where it listens and the per-call token it
	// demands. Both empty means no RPC face; one without the other is
	// refused at start-up (same discipline as the HTTP face).
	RPCAddr  string `yaml:"rpc_addr" env:"JMS_RPC_ADDR"`
	RPCToken string `yaml:"rpc_token" env:"JMS_RPC_TOKEN"`
}

// AuditSettings is where this server's trail goes.
func (c *Config) AuditSettings() audit.Settings {
	return audit.Settings{Mode: c.AuditMode, Dir: c.AuditDir, RetentionDays: c.AuditRetentionDays}
}

// Load reads the configuration from the environment.
//
// A credential is required, and its absence is reported with the same wording
// as before the refactor, because that text is what an operator sees when the
// wiring is wrong and greps for it.
//
// ⚠️ Two credential forms are accepted, and they are not versions:
//
//   - JUMPSERVER_TOKEN — the PrivateToken. It does not expire, and it works
//     on v4 and on v5 alike.
//   - JUMPSERVER_USERNAME + JUMPSERVER_PASSWORD — a login that the bastion
//     exchanges for a Bearer token. It expires, so it is refreshed; it is
//     what a deployment that cannot mint a PrivateToken has.
//
// The token wins when both are set: it does not expire, and a long-running
// process should not have to keep a login alive. The doctor says so when it
// finds both.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := config.ApplyEnv(cfg); err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	if err := config.FillDefaults(cfg); err != nil {
		return nil, fmt.Errorf("apply configuration defaults: %w", err)
	}
	if err := config.ValidateTags(cfg); err != nil {
		return nil, fmt.Errorf("validate configuration: %w", err)
	}

	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.Version = strings.ToLower(strings.TrimSpace(cfg.Version))
	// ⚠️ The deployment's address is required, and it is required for the
	// same reason the credential is: without it there is nothing to talk
	// to. The message says what to set and what it looks like, because the
	// person reading it is an operator wiring a server up, not a
	// developer reading a stack trace.
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("JUMPSERVER_URL is required: the root of your JumpServer deployment, " +
			"e.g. https://jumpserver.example.com (no trailing /api/v1)")
	}
	if cfg.HTTPAddr != "" && cfg.HTTPToken == "" {
		return nil, fmt.Errorf("JMS_HTTP_ADDR is set but JMS_HTTP_TOKEN is not: the HTTP surface answers " +
			"tool calls, so it needs a bearer token. Generate one (openssl rand -hex 32) and set " +
			"JMS_HTTP_TOKEN, or unset JMS_HTTP_ADDR to stay on stdio only")
	}
	if cfg.RPCAddr != "" && cfg.RPCToken == "" {
		return nil, fmt.Errorf("JMS_RPC_ADDR is set but JMS_RPC_TOKEN is not: the RPC face answers " +
			"tool calls to whoever can reach the port, and there is no auth-off mode")
	}
	if cfg.Token == "" && cfg.Username == "" {
		return nil, fmt.Errorf("a credential is required: JUMPSERVER_TOKEN (a JumpServer PrivateToken) " +
			"or JUMPSERVER_USERNAME together with JUMPSERVER_PASSWORD")
	}
	if cfg.Token == "" {
		// The password may live in a file instead of the environment: the
		// environment of a long-running process is readable by anything
		// that can read /proc on Linux, and an MCP server is started by an
		// agent runtime whose environment is not a secret store.
		pw, err := jmsapi.LoadPassword()
		if err != nil {
			return nil, fmt.Errorf("JUMPSERVER_USERNAME is set but the password is missing: %w", err)
		}
		cfg.Password = pw
	}
	if cfg.OTP == "" {
		// The TOTP secret is optional: a deployment without MFA never
		// asks for one. It is only read when the deployment DOES ask, so
		// its absence is not an error here.
		if otp, err := jmsapi.LoadOTPSecret(); err == nil {
			cfg.OTP = otp
		}
	}
	return cfg, nil
}
