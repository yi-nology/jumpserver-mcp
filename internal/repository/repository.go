// Package client is the management plane's repository over the JumpServer
// API.
//
// The PROTOCOL lives in jms-kit/pkg/jmsapi: the transport, the authentication
// shape, the two list shapes v5 mixes, the retry policy. The execution plane
// speaks the same protocol, and while each server carried its own client the
// two drifted — the byte-slicing Truncate that survived on one side and not
// the other was the visible symptom.
//
// What lives here is MEANING: the resources the bastion holds and the rules
// about them. "Does this permission actually let anybody in" is a fact about
// the bastion's data model, not about its transport, so it stays on this side
// where the tools that need it live.
//
// The split is what makes the layering visible in the imports: a handler
// reaches for client when it asks the bastion a question, and for jmsapi when
// it reads a response.
package repository

import (
	"net/http"

	"github.com/yi-nology/jumpserver-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-mgmt/internal/config"
)

// Client is the management plane's view of the bastion.
//
// The protocol client is embedded, so Do, ListAll and the rest are the shared
// ones — there is exactly one implementation of "how to call the bastion" in
// this repository.
type Client struct {
	*jmsapi.Client

	// Chat AI delegation (X-JMS-AI-Delegation). Optional: when Secret is empty
	// every delegation tool explains that it is not configured instead of
	// failing, and the ordinary Token path is unaffected.
	DelegationSecret   []byte
	DelegationKeyID    string
	DelegationIssuer   string
	DelegationAudience string
	DelegationUserID   string
	DelegationOrgID    string
}

// New builds a client from the configuration.
//
// The credential is not required here: the client is also built for the
// commands that only need the catalogue (jumpserver-mgmt tools) and for the
// tests. What a missing credential breaks is the API calls, not the
// construction.
func New(cfg *config.Config) (*Client, error) {
	c := &Client{
		Client: jmsapi.New(jmsapi.Options{
			BaseURL: cfg.BaseURL,
			Token:   cfg.Token,
			Credential: jmsapi.Credential{
				Username: cfg.Username,
				Password: cfg.Password,
				OTP:      cfg.OTP,
			},
			Version: cfg.Version,
			Timeout: cfg.Timeout,
			// A runaway agent can call a hundred tools in a minute; the
			// bastion is a monolith and does not need that. The bucket is
			// generous enough to be invisible in normal use.
			RequestsPerSecond: float64(cfg.RequestsPerSecond),
			Burst:             cfg.Burst,
		}),
		DelegationKeyID:    delegationKeyID,
		DelegationIssuer:   defaultDelegationIssuer,
		DelegationAudience: defaultDelegationAudience,
		DelegationUserID:   cfg.DelegationUserID,
		DelegationOrgID:    cfg.DelegationOrgID,
	}
	// A missing secret is not fatal: the Token path still works, and the
	// delegation tools report their own unconfigured state.
	if s, err := LoadDelegationSecret(); err == nil {
		c.DelegationSecret = s
	}
	return c, nil
}

// NewBase builds a client from primitives.
//
// The configuration struct is one way in; this is the other, for the tests and
// for a caller that has a URL and a token and nothing else. A nil HTTP client
// means the default one.
//
// There is no rate limiter here: this constructor is for the tests and for
// callers that make a handful of calls, and a bucket that a test has to wait
// on is a test that fails for the wrong reason.
func NewBase(baseURL, token string, hc *http.Client) *Client {
	return NewBaseAuth(baseURL, token, jmsapi.Credential{}, hc)
}

// NewBaseAuth builds a client from primitives, WITH a login credential.
//
// It exists because a JumpServer v4 deployment has no API that mints a
// PrivateToken (v4's authentication API registers access-keys, temp-tokens,
// connection-token, ssh-key and access-tokens — but no private-token route,
// and v4's `private_token` is a MODEL property with no API exposure). ⇒ On
// v4 the API-reachable credential is the Bearer token that
// POST /api/v1/authentication/auth/ hands out for a username and a password.
//
// ⇒ A test against v4 needs the login form, and the client has to be able to
// carry it.
func NewBaseAuth(baseURL, token string, cred jmsapi.Credential, hc *http.Client) *Client {
	return &Client{
		Client: jmsapi.New(jmsapi.Options{
			BaseURL:    baseURL,
			Token:      token,
			Credential: cred,
			HTTPClient: hc,
		}),
		DelegationKeyID:    delegationKeyID,
		DelegationIssuer:   defaultDelegationIssuer,
		DelegationAudience: defaultDelegationAudience,
	}
}
