package router

// 这个文件是这一面的**装配**：把身份、风险表与工具目录接成一个能跑的
// server。
//
// ⚠️ 它住在 biz/router 而不是仓根，是因为**仓根那个 package 是 main** ——
// Go 不许 import 一个 main 包 ⇒ 仓根里的东西谁都用不了，而子命令
// （cmd/serve、cmd/tools）要用它。
//
// ⚠️ 身份里的**版本**由调用方递进来：版本是**二进制的属性**，而版本常量
// 住在仓根（package main）⇒ 子命令拿不到它 ⇒ 它只能从外面递进来。

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yi-nology/jumpserver-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mgmt/internal/config"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
	"github.com/yi-nology/jumpserver-mgmt/internal/service/policy"
)

// Options is what the caller may turn when it assembles the server.
type Options struct {
	// OnRecord receives one audit record per tool call.
	//
	// ⚠️ 只有**跑服务**那条路要留痕：「列工具目录」那种不碰堡垒机的命令
	// 不写留痕（留痕记的是「谁做了什么」，不是「有哪些工具」）。
	OnRecord func(reg.Record)
}

// New assembles this server: its identity, the risk side of the catalogue,
// and the tools themselves.
//
// ⚠️ 「哪些工具要确认、能不能撤销」是**一份**表（policy.Table），
// 「工具怎么实现」是**另一份**（handler）⇒ 两边必须一起装配：分开放，
// 就会有一份说「这个工具是只读的」、另一份说「它要改堡垒机」。
func New(cfg *config.Config, c *repository.Client, version string, opts Options) *reg.Registry {
	r := reg.New(implementation(cfg, version), reg.Options{
		Policy:     policy.Table,
		ReadOnly:   cfg.ReadOnly,
		ServerName: cfg.ServerName,
		OnRecord:   opts.OnRecord,
	})
	Register(r, c)
	return r
}

// implementation says who this server is.
func implementation(cfg *config.Config, version string) *mcp.Implementation {
	name := cfg.ServerName
	if name == "" {
		name = "jumpserver-mgmt"
	}
	return &mcp.Implementation{Name: name, Version: version}
}
