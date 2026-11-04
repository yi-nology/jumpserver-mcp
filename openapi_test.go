package main

import (
	"testing"

	"github.com/yi-nology/jumpserver-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-kit/pkg/testsupport"
	"github.com/yi-nology/jumpserver-mgmt/biz/router"
	"github.com/yi-nology/jumpserver-mgmt/internal/config"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
)

// The OpenAPI document under swagger/ is generated, and a generated thing
// that is not compared against its source is a hand-written thing with
// extra steps.
//
// The source here is the tool registry: the same one the MCP face serves.
// So the contract a REST caller codes against and the tools an agent calls
// cannot drift apart — change a tool, regenerate, or this test names the
// difference.
//
// ⚠️ The version is compared too, and that is the common failure: the
// Version constant is bumped for a release and the document is not
// regenerated ⇒ the shipped contract documents the PREVIOUS release. The
// regeneration is ./script/gen-openapi.sh; its output is checked in.
//
// ⚠️ The document is rendered from the SAME code path the server serves
// (router.New + repository.NewBase), not a parallel assembly — a guard
// that builds its own catalogue proves only that its own catalogue
// renders. The comparison mechanics live in kit's testsupport.CheckOpenAPI
// (two services, one guard body); what is THIS service's own is the
// assembly below.
func TestTheCheckedInOpenAPIDocumentMatchesTheTools(t *testing.T) {
	testsupport.CheckOpenAPI(t, testsupport.OpenAPICheck{
		Root:     repoRoot(t),
		Version:  Version,
		Name:     router.FaceName,
		ToolMark: "/api/v1/tools/jms_",
		New: func() *reg.Registry {
			return router.New(&config.Config{ServerName: router.FaceName},
				repository.NewBase("http://127.0.0.1:1", "t", nil), "guard", router.Options{})
		},
	})
}
