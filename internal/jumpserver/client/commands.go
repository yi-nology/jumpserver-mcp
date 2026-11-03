// Part of the management plane's repository over the JumpServer API.
//
// The transport and the parsing of the two response shapes v5 mixes live in
// jms-kit/pkg/jmsapi; what is here is the questions the tools ask and the
// resources they get back.
package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/oekit/pkg/convert"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
)

type CommandRecord struct {
	ID         string
	Session    string
	User       string
	Asset      string
	Address    string
	Account    string
	RemoteAddr string
	Input      string
	Output     string
	RiskLevel  string
	Timestamp  float64
	When       string
}

func (c *Client) FetchCommands(ctx context.Context, q url.Values) ([]CommandRecord, error) {
	// listAll, not a single page_size request: the endpoint ignores
	// page_size above its own cap of 200 rows, so a one-shot read returns
	// only the newest 200 commands and silently truncates the audit trail.
	path := string(jmsapi.ResTerminalCommands)
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	rows, err := c.ListAll(ctx, path)
	if err != nil {
		return nil, err
	}
	out := make([]CommandRecord, 0, len(rows))
	for _, m := range rows {
		ts := convert.AnyToFloat64(m["timestamp"])
		out = append(out, CommandRecord{
			ID:         convert.AnyToStr(m["id"]),
			Session:    convert.AnyToStr(m["session"]),
			User:       convert.AnyToStr(m["user"]),
			Asset:      fmt.Sprint(jmsapi.AssetField(m, "name")),
			Address:    fmt.Sprint(jmsapi.AssetField(m, "address")),
			Account:    convert.AnyToStr(m["account"]),
			RemoteAddr: convert.AnyToStr(m["remote_addr"]),
			Input:      convert.AnyToStr(m["input"]),
			Output:     convert.AnyToStr(m["output"]),
			RiskLevel:  jmsapi.Label(m, "risk_level"),
			Timestamp:  ts,
			When:       convert.AnyToStr(m["timestamp_display"]),
		})
	}
	return out, nil
}
