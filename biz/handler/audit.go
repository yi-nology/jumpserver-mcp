package handler

import (
	"context"
	"net/url"
	"strings"

	"github.com/yi-nology/jumpserver-mgmt/biz/model/jmsms"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oekit/pkg/convert"
	"github.com/yi-nology/jumpserver-kit/model/jms"
	"github.com/yi-nology/jumpserver-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-kit/pkg/render"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
)

func RegisterAuditTools(r *reg.Registry, c *repository.Client) {
	type sessionsArgs struct {
		Limit     int    `json:"limit,omitempty" jsonschema:"max rows (default 20)"`
		Asset     string `json:"asset,omitempty" jsonschema:"only sessions of this asset (address/name/uuid)"`
		UserName  string `json:"user,omitempty" jsonschema:"only sessions of this JumpServer user"`
		SearchRef string `json:"search,omitempty" jsonschema:"substring match on asset address or username"`
	}
	reg.Add(r, &mcp.Tool{
		Name:         "jms_sessions_list",
		Description:  "List recent terminal sessions with user, asset, protocol, dates and whether a replay exists. Use it to find a session id for jms_session_transcript; jms_sessions_live is the other end of it (who is connected right now).",
		OutputSchema: render.Schema[render.ListOf[jmsms.SessionSummary]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args sessionsArgs) (*mcp.CallToolResult, any, error) {
		rows, err := c.List(ctx, jmsapi.ResSessions)
		if err != nil {
			return render.Err(err)
		}
		var matcher *repository.AssetMatcher
		if args.Asset != "" {
			ids, meta, err := c.ResolveAssets(ctx, []string{args.Asset})
			if err != nil {
				return render.Err(err)
			}
			matcher = repository.NewAssetMatcher(ids, meta)
		}
		// filter → map → sort → truncate → count: render.ListSpec owns the
		// order and the three counting fields. Sorting before the cap is
		// what keeps the NEWEST rows rather than whichever rows the endpoint
		// returned first; total is how many matched the filters, before the
		// cap.
		return render.Text(render.ListSpec[map[string]any, jmsms.SessionSummary]{
			Rows: rows,
			Keep: func(s map[string]any) bool {
				if !matcher.Match(s) {
					return false
				}
				if args.UserName != "" && !repository.UserMatches(convert.AnyToStr(s["user"]), args.UserName) {
					return false
				}
				if args.SearchRef != "" {
					q := strings.ToLower(args.SearchRef)
					// The flat "name(address)" label carries both halves, so match
					// against it whole rather than against a parsed field that this
					// endpoint does not provide.
					blob := strings.ToLower(convert.AnyToStr(s["asset"]) + " " + convert.AnyToStr(s["user"]) +
						" " + convert.AnyToStr(s["account"]))
					if !strings.Contains(blob, q) {
						return false
					}
				}
				return true
			},
			Map: func(s map[string]any) jmsms.SessionSummary {
				// The replay size is a byte count, and the contract says i64.
				var replaySize *int64
				if v := s["replay_size"]; v != nil {
					n := convert.AnyToInt64(v)
					replaySize = &n
				}
				return jmsms.SessionSummary{
					ID:            jmsapi.Str(s["id"]),
					User:          jmsapi.FieldStr(s["user"]),
					Asset:         jmsapi.FieldStr(jmsapi.AssetField(s, "name")),
					Address:       jmsapi.FieldStr(jmsapi.AssetField(s, "address")),
					AssetID:       jmsapi.FieldStr(jmsapi.AssetField(s, "id")),
					Account:       jmsapi.FieldStr(s["account"]),
					Protocol:      jmsapi.FieldStr(jmsapi.FieldName(s, "protocol")),
					Type:          jmsapi.FieldStr(jmsapi.Label(s, "type")),
					RemoteAddr:    jmsapi.FieldStr(s["remote_addr"]),
					DateStart:     jmsapi.FieldStr(s["date_start"]),
					DateEnd:       jmsapi.FieldStr(s["date_end"]),
					Duration:      jmsapi.FieldStr(s["duration"]),
					HasReplay:     jmsapi.FieldBool(s["has_replay"]),
					CanReplay:     jmsapi.FieldBool(s["can_replay"]),
					CommandAmount: jmsapi.FieldInt32(s["command_amount"]),
					ErrorReason:   jmsapi.FieldStr(jmsapi.Label(s, "error_reason")),
					IsSuccess:     jmsapi.FieldBool(s["is_success"]),
					LoginFrom:     jmsapi.FieldStr(jmsapi.Label(s, "login_from")),
					ReplaySize:    replaySize,
				}
			},
			Less:  func(a, b jmsms.SessionSummary) bool { return a.GetDateStart() > b.GetDateStart() },
			Limit: args.Limit,
			Def:   20,
			Max:   100,
			// has_command is false on every row this bastion returns even when
			// command_amount is 5, so it must not be offered as the signal for
			// "is there a transcript"; the count is the honest one.
			Note: "use command_amount for the transcript size -- the bastion's has_command flag reads false on every session here",
		}.Build())
	})

	type commandsArgs struct {
		Search   string `json:"search,omitempty" jsonschema:"substring match on the recorded command line"`
		Asset    string `json:"asset,omitempty" jsonschema:"only commands on this asset"`
		UserName string `json:"user,omitempty" jsonschema:"only commands from this JumpServer user"`
		Limit    int    `json:"limit,omitempty" jsonschema:"max rows (default 30)"`
	}
	reg.Add(r, &mcp.Tool{
		Name:         "jms_commands_search",
		Description:  "Search recorded session commands (the audit trail) and get EVERY match: the whole command log is read and filtered here, so a filter never silently truncates the trail. Filter by command text, asset or user. Rows carry the command but not its output -- use jms_command_audit when you need what came back.",
		OutputSchema: render.Schema[render.ListOf[jmsms.CommandSearchSummary]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args commandsArgs) (*mcp.CallToolResult, any, error) {
		// The whole command log is read and filtered here. The endpoint caps a
		// page at 200 rows and matches a user filter on one half of the
		// "name(username)" display, so a server-side query both truncated the
		// trail and answered "nothing" for a filter like "opsuser".
		recs, err := c.FetchCommands(ctx, nil)
		if err != nil {
			return render.Err(err)
		}
		var matcher *repository.AssetMatcher
		if args.Asset != "" {
			ids, meta, err := c.ResolveAssets(ctx, []string{args.Asset})
			if err != nil {
				return render.Err(err)
			}
			matcher = repository.NewAssetMatcher(ids, meta)
		}
		// filter → map → sort → truncate → count: render.ListSpec owns the
		// order and the counting.
		res := render.ListSpec[repository.CommandRecord, jmsms.CommandSearchSummary]{
			Rows: recs,
			Keep: func(r repository.CommandRecord) bool {
				if !matcher.MatchNameAddress(r.Asset, r.Address) {
					return false
				}
				if args.UserName != "" && !repository.UserMatches(r.User, args.UserName) {
					return false
				}
				if args.Search != "" {
					q := strings.ToLower(args.Search)
					blob := strings.ToLower(r.Input + " " + r.Output + " " + r.Asset + " " + r.User)
					if !strings.Contains(blob, q) {
						return false
					}
				}
				return true
			},
			Map: func(r repository.CommandRecord) jmsms.CommandSearchSummary {
				return jmsms.CommandSearchSummary{
					ID:         jms.Opt(r.ID),
					User:       jms.Opt(r.User),
					Asset:      jms.Opt(r.Asset),
					Address:    jms.Opt(r.Address),
					Account:    jms.Opt(r.Account),
					Session:    jms.Opt(r.Session),
					RemoteAddr: jms.Opt(r.RemoteAddr),
					Input:      jms.Opt(jmsapi.Truncate(redactSecrets(r.Input), 500)),
					RiskLevel:  jms.Opt(r.RiskLevel),
					DateStart:  jms.Opt(r.When),
				}
			},
			Less:  func(a, b jmsms.CommandSearchSummary) bool { return a.GetDateStart() > b.GetDateStart() },
			Limit: args.Limit,
			Def:   30,
			Max:   200,
		}.Build()
		// count / total / truncated are always present, so a caller never has
		// to guess whether a field exists: count is what came back, total is
		// how many matched, truncated says whether the list was cut.
		if res.Truncated {
			res.Note = "showing the newest " + convert.AnyToStr(res.Count) + " of " +
				convert.AnyToStr(res.Total) + " matching commands"
		}
		return render.Text(res)
	})

	type loginArgs struct {
		Limit    int    `json:"limit,omitempty" jsonschema:"max rows (default 30)"`
		UserName string `json:"user,omitempty" jsonschema:"only logins of this user"`
		Success  *bool  `json:"success,omitempty" jsonschema:"true for successful logins only, false for failures"`
	}
	reg.Add(r, &mcp.Tool{
		Name:         "jms_login_logs",
		Description:  "Query authentication logs: who logged in, from where, and failed attempts. Use to spot brute-force or stale sessions.",
		OutputSchema: render.Schema[render.ListOf[jmsms.LoginLogSummary]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args loginArgs) (*mcp.CallToolResult, any, error) {
		q := url.Values{"ordering": {"-date_created"}}
		if args.UserName != "" {
			q.Set("username", args.UserName)
		}
		if args.Success != nil {
			q.Set("success", convert.AnyToStr(*args.Success))
		}
		// listAll, not a one-shot page_size request: the endpoint ignores
		// page_size and answers with its own cap of 200 rows, so the limit has
		// to be applied on this side or the caller gets 200 rows whatever they
		// asked for.
		rows, err := c.ListQuery(ctx, jmsapi.ResLoginLogs, q)
		if err != nil {
			return render.Err(err)
		}
		// filter → map → sort → truncate → count: render.ListSpec owns the
		// counting. Less is nil: the server answers in -date_created order
		// and that order is the answer.
		res := render.ListSpec[map[string]any, jmsms.LoginLogSummary]{
			Rows: rows,
			Map: func(l map[string]any) jmsms.LoginLogSummary {
				// /audits/login-logs/ names its fields ip / datetime and carries
				// backend as a plain string, unlike /audits/operate-logs/ which
				// uses remote_addr / datetime and a labelled object. Reading the
				// login log under the operate-log names silently yields nulls.
				return jmsms.LoginLogSummary{
					Username:    jms.Opt(convert.AnyToStr(l["username"])),
					Success:     jms.OptBool(jmsapi.Label(l, "status") == "success"),
					Backend:     jms.Opt(jmsapi.FieldName(l, "backend")),
					RemoteAddr:  jms.Opt(convert.AnyToStr(l["ip"])),
					UserAgent:   jms.Opt(jmsapi.Truncate(convert.AnyToStr(l["user_agent"]), 120)),
					DateCreated: jms.Opt(convert.AnyToStr(l["datetime"])),
				}
			},
			Limit: args.Limit,
			Def:   30,
			Max:   200,
		}.Build()
		if res.Truncated {
			res.Note = "showing the first " + convert.AnyToStr(res.Count) + " of " +
				convert.AnyToStr(res.Total) + " login rows; narrow with user or success"
		}
		return render.Text(res)
	})
}
