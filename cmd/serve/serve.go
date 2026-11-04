// Package serve is the "serve" subcommand: the server itself.
//
// Two run modes: `serve` is MCP over stdio (the default — the MCP
// runtime launches the binary with no arguments), and `serve http` is
// the HTTP face only (REST + MCP over streamable HTTP, for deployment
// as a service). Either way it is ONE registry behind the faces: one
// risk gate, one confirm rule, one audit trail.
//
// ⚠️ 它是**子命令的实现**，不是二进制：入口层（仓根那个 package main）
// import 它。一个仓一个二进制 ⇒ cmd/ 下的每个包都只提供「一段活」，
// 谁都不许再长出一个自己的 main。
//
// ⚠️ 留痕接在这里，而不是接在入口层：留痕是「**谁在这台机器上做了什么**」
// 的记录，只有**跑服务**那条路会调工具 ⇒ 「列工具目录」那种命令不该
// 留下记录。
//
// ⚠️ 生命周期（socket、信号、优雅关闭）不在这里：那是两个服务共用的活，
// 住在 kit 的 pkg/httpapi.Run。这里曾经有一份 startHTTP / serveHTTP 的
// 拷贝，与执行面那份各自为政，日志措辞都漂了 —— 一份生命周期，两个服务。
package serve

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/oekit/pkg/cli"

	"github.com/yi-nology/jumpserver-kit/pkg/audit"
	"github.com/yi-nology/jumpserver-kit/pkg/faces"
	"github.com/yi-nology/jumpserver-kit/pkg/httpapi"
	"github.com/yi-nology/jumpserver-kit/pkg/rpcapi"
	"github.com/yi-nology/jumpserver-mgmt/biz/router"
	"github.com/yi-nology/jumpserver-mgmt/internal/config"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
)

// Run serves MCP over stdio until the transport closes (`serve`), or runs
// the HTTP face until SIGINT/SIGTERM (`serve http`). What is genuinely
// this service's own is the wiring: the config, the client, the registry,
// and the trail — the lifecycle around them is httpapi.Run's.
//
// ⚠️ 版本从入口层递进来：版本是**二进制的属性**，而版本常量住在仓根
// （package main）⇒ 子命令拿不到它。
func Run(ctx context.Context, version string, args []string, stdout, stderr io.Writer) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL  %v\n", err)
		return cli.ExitError
	}
	c, err := repository.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL  %v\n", err)
		return cli.ExitError
	}
	r := router.New(cfg, c, version, router.Options{
		// ⚠️ The trail is wired HERE, on the path that actually calls
		// tools: a DANGEROUS command refused for want of a confirmation
		// never reaches the bastion, so the bastion has no session to
		// show for it. Without this line, "who tried to run rm -rf on
		// that box and was stopped" has no answer anywhere.
		OnRecord: audit.Trail(cfg.AuditSettings().Options(cfg.ServerName)),
	})

	// The faces: HTTP configured ⇒ the REST/MCP-HTTP face rides; RPC
	// configured ⇒ the kitex face rides. The lifecycle around them is
	// faces.Run's — one owner, three faces.
	opts := faces.Options{Stderr: stderr}
	// Unconditional: the HTTP config (including its ExampleAddr) must
	// reach faces.plan even when no address is set — the refusal for
	// `serve http` is exactly the case where Addr is empty, and the hint
	// needs the example port that only travels on a constructed option.
	opts.HTTP = &httpapi.ServeOptions{
		Options: httpapi.Options{
			Token:    cfg.HTTPToken,
			Name:     cfg.ServerName,
			Version:  version,
			ReadOnly: cfg.ReadOnly,
		},
		Addr:   cfg.HTTPAddr,
		Stderr: stderr,
		// This service's own documented example port: the shared refusal
		// hint must not point at the other plane's port.
		ExampleAddr: "127.0.0.1:8731",
	}
	if cfg.RPCAddr != "" {
		opts.RPC = &rpcapi.ServeOptions{
			Options: rpcapi.Options{Token: cfg.RPCToken},
			Addr:    cfg.RPCAddr,
			Stderr:  stderr,
		}
	}
	if err := faces.Run(ctx, r, opts, args); err != nil {
		fmt.Fprintf(stderr, "FATAL  %v\n", err)
		return cli.ExitError
	}
	return cli.ExitOK
}
