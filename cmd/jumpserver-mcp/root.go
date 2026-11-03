package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/cli"

	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/audit"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/handler"
	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/model/jmsms"
	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/policy"
	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/router"
	"github.com/yi-nology/jumpserver-mcp/conf/jumpserver"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/client"
)

// Version is the version of the management-plane server.
//
// ⚠️ **它必须与本仓的 tag 一致** —— `go install ...@v0.3.1` 装出来的二进制
// 要自己说对版本；说错的话，用户看 `--version` 会以为装错了版本。
const Version = "0.3.2"

// Main runs the command line and exits with the command's exit code.
func dispatch() {
	// The log backend writes to stderr, never to stdout: stdout carries the
	// JSON-RPC stream, and a log line there would corrupt the protocol.
	logx.SetDefault(logx.NewSlogLogger("jumpserver-mcp",
		logx.WithHandler(slog.NewJSONHandler(os.Stderr, nil))))

	app := &cli.App{
		Name:    "jumpserver-mcp",
		Version: Version,
		// No arguments means "run the default way": this server is started by
		// a framework that passes no subcommand name, and "no subcommand"
		// in a program like this one means "serve", not "usage error".
		Default: "serve",
		Commands: []cli.Command{
			{Name: "serve", Summary: "以 stdio 提供 MCP 服务（默认）", Run: runServe},
			// The flag spelling is kept for compatibility: it used to be
			// this server's only spelling before it became a subcommand.
			{Name: "doctor", Summary: "检查接线与堡垒机配置（不需要 MCP 会话）",
				Aliases: []string{"--doctor", "-doctor"}, Run: runDoctor},
			{Name: "tools", Summary: "列出工具目录：名字、风险、确认要求", Run: runTools},
		},
	}
	os.Exit(app.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// mcpImplementation describes this server to the client.
func mcpImplementation(cfg *conf.Config) *mcp.Implementation {
	name := cfg.ServerName
	if name == "" {
		name = "jumpserver-mcp"
	}
	return &mcp.Implementation{Name: name, Version: Version}
}

// serve is the default mode: MCP over stdio.
func serve(ctx context.Context) int {
	cfg, err := conf.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL  %v\n", err)
		return cli.ExitError
	}
	c, err := client.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL  %v\n", err)
		return cli.ExitError
	}
	r := reg.New(mcpImplementation(cfg), reg.Options{
		Policy:     policy.Table,
		ReadOnly:   cfg.ReadOnly,
		ServerName: cfg.ServerName,
		OnRecord:   audit.Trail(cfg.AuditSettings().Options(cfg.ServerName)),
	})
	router.Register(r, c)
	if err := r.Serve(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL  server failed: %v\n", err)
		return cli.ExitError
	}
	return cli.ExitOK
}

// runServe is the "serve" subcommand.
func runServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return serve(ctx)
}

// runDoctor is the "doctor" subcommand: the same self-check the jms_doctor
// tool runs, but without an MCP session.
//
// It takes its streams from the caller and returns the exit code instead of
// terminating the process, so a test can drive it directly.
func runDoctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cfg, err := conf.Load()
	if err != nil {
		fmt.Fprintf(stderr, "FATAL  %v\n", err)
		return cli.ExitError
	}
	c, err := client.New(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "FATAL  %v\n", err)
		return cli.ExitError
	}
	// ⚠️ The CLI knows this server's configuration, so the doctor can also
	// report on the trail this machine keeps — the one check that does not
	// need the bastion at all. The tool path (jms_doctor) cannot: it
	// carries no configuration on purpose.
	trail := cfg.AuditSettings()
	report, err := handler.DoctorWithTrail(ctx, c, true, &trail)
	if err != nil {
		fmt.Fprintf(stderr, "FATAL  %v\n", err)
		return cli.ExitError
	}
	fmt.Fprint(stdout, RenderDoctor(report))
	if report.Status == jms.Status_ok {
		return cli.ExitOK
	}
	return cli.ExitError
}

// runTools is the "tools" subcommand: the catalogue, with the risk of each
// tool and what it asks for.
//
// It is the audit view of the policy table: a tool that changes the bastion
// has to say so here, and the confirm sentence is the one the caller sees.
func runTools(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cfg, err := conf.Load()
	if err != nil {
		// The catalogue does not need a bastion, but the config does need to
		// parse. A missing token is not fatal here.
		cfg = &conf.Config{ServerName: "jumpserver-mcp"}
	}
	c, err := client.New(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "FATAL  %v\n", err)
		return cli.ExitError
	}
	r := reg.New(mcpImplementation(cfg), reg.Options{
		Policy:     policy.Table,
		ServerName: cfg.ServerName,
	})
	router.Register(r, c)
	tbl := cli.NewTable()
	for _, spec := range r.Specs() {
		confirm := ""
		if spec.Confirm != nil {
			confirm = *spec.Confirm
		} else if p, ok := policy.Table[spec.Name]; ok {
			// A conditional confirmation has no sentence to refuse with,
			// but the rule is still worth showing.
			confirm = p.Hint
		}
		tbl.Append(spec.Name, spec.Risk.String(), confirm)
	}
	tbl.Render(stdout)
	return cli.ExitOK
}

// RenderDoctor turns the doctor report into the text the CLI prints.
//
// It is a pure function so the wording can be checked without a bastion. The
// grade comes from the shared status field: a WARN is "needs attention" for
// the exit code, exactly as before.
//
// The report is the contract type the doctor answers with, so the CLI reads
// the same fields the tool hands out.
func RenderDoctor(report *jmsms.DoctorResult) string {
	r := cli.NewReport("jumpserver-mcp doctor")
	r.Header(fmt.Sprintf("base_url: %v", report.BaseURL))
	// The identity is only known once the bastion answered, so the line is
	// printed only then -- the same as before, when it was a map that was
	// either there or not.
	if report.Identity.Username != nil || report.Identity.IsSuperuser != nil {
		r.Header(fmt.Sprintf("identity: %v (superuser=%v)",
			report.Identity.GetUsername(), report.Identity.GetIsSuperuser()))
	}
	for _, ck := range report.Checks {
		status := cli.StatusOK
		switch ck.Status {
		case jms.Status_warn:
			status = cli.StatusWarn
		case jms.Status_fail:
			status = cli.StatusFail
		}
		fix := ""
		if ck.Fix != nil {
			fix = *ck.Fix
		}
		r.AddFix(status, ck.Name, ck.Detail, fix)
	}
	if report.Verdict != nil && *report.Verdict != "" {
		r.AddConclusion("verdict: " + *report.Verdict)
	}
	if report.Advice != nil && *report.Advice != "" {
		r.AddConclusion("advice: " + *report.Advice)
	}
	// This side counts a WARN as "needs attention" -- that is the grading
	// rule it has always had, so the wording stays its own.
	if report.Status == jms.Status_ok {
		r.Result("result: healthy")
	} else {
		r.Result("result: needs attention (see WARN/FAIL above)")
	}
	return r.String()
}
