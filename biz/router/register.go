package router

import (
	"github.com/yi-nology/jumpserver-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mgmt/biz/handler"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
)

// Register puts every tool of the management plane on the server.
//
// The order is the order the tools are meant to be reached in: what the
// bastion holds, what may be done to it, and then the audit trail of what
// was done. It has no effect on the tool selection — the model picks by
// description — but it keeps the catalogue readable.
//
// Every registration goes through pkg/reg, which is where the gate lives.
// There is no second way to add a tool to this server, which is what makes
// the confirmation check unavoidable.
func Register(r *reg.Registry, c *repository.Client) {
	// What the bastion holds.
	handler.RegisterStatusTools(r, c)
	handler.RegisterAdminTools(r, c)
	handler.RegisterScopeTools(r, c)
	handler.RegisterGatherTools(r, c)
	handler.RegisterLabelRoleTools(r, c)
	handler.RegisterRBACTools(r, c)
	handler.RegisterExtraTools(r, c)

	// What may be done to it.
	handler.RegisterOnboardingTools(r, c)
	handler.RegisterFilterTools(r, c)
	handler.RegisterLoginACLTools(r, c)
	handler.RegisterDataMaskingTools(r, c)
	handler.RegisterClipboardACLTools(r, c)
	handler.RegisterJobTools(r, c)
	handler.RegisterBulkTools(r, c)
	handler.RegisterCoverageTools(r, c)
	handler.RegisterDelegationTools(r, c)
	handler.RegisterAccountAutomationTools(r, c)
	handler.RegisterNetworkTools(r, c)

	// What was done to it.
	handler.RegisterAuditTools(r, c)
	handler.RegisterAuditViewTools(r, c)
	handler.RegisterEvidenceTools(r, c)
	handler.RegisterTouchTools(r, c)
	handler.RegisterTransferTools(r, c)
	handler.RegisterSecurityTools(r, c)
}
