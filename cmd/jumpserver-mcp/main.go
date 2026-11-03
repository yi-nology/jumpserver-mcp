// Command jumpserver-mcp exposes a curated JumpServer REST API surface as
// MCP tools.
//
// Design note: the upstream official server (github.com/jumpserver/mcp) derives
// its tools from JumpServer's OpenAPI schema, which yields 544 tools for v5.0.
//
// The surface is the same on JumpServer v4: the endpoint paths, the field
// shapes and the constraints below were checked against the upstream v4.10.20-lts
// source (see skills/jumpserver-ops/references/version-compat.md).
// That is too many for reliable tool selection, and the repository carries no
// LICENSE file. This server deliberately exposes a small, explicit set instead,
// and puts a risk gate in front of every one of them.
//
// Transport: stdio. Credential: a JumpServer PrivateToken, because v5 dropped
// AccessKey/Bearer auth and its OAuth tokens expire after 24h.
//
// This is the command line of the management plane. The normal mode is
// serving MCP over stdio: the agent runtime starts the binary with no
// arguments and speaks JSON-RPC over stdin/stdout. The subcommands exist for
// the moments when there is no session yet — the wiring is suspect and the
// tools are not answering.
//
// The exit codes are oekit's: 0 ok, 1 error, 2 usage. The doctor command
// follows the same rule the tool does: a WARN means "needs attention", so it
// exits non-zero.
//
// The repository is one module with two binaries; this is the management
// plane's entry point. The layout follows the CloudWeGo hz convention:
//
//	cmd/jumpserver-mcp/         the command line (this file)
//	conf/jumpserver/             the configuration
//	idl/jmsms/                   the contract, in thrift
//	biz/jumpserver/model/jmsms/  the contract, generated from the IDL
//	biz/jumpserver/handler/      the tools
//	biz/jumpserver/service/      the domain rules
//	biz/jumpserver/router/       the tool catalogue and the wiring
//	internal/jumpserver/client/  the repository over the bastion's resources
//
// The shared base (the protocol layer, the gate, the result shaping) is at the
// repository root under pkg/.

package main

func main() {
	dispatch()
}
