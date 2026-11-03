// tools_transfer.go answers "what files went onto (or off) these machines?".
//
// Why this exists when the bastion already records it.
//
// The execution plane moves files through the bastion: jmsx_put_file uploads,
// jmsx_get_file downloads, jmsx_relay copies from one machine to another.
// The bastion records every one of those transfers in its own audit trail
// (/audits/ftp-logs/) -- and until now this repository had no way to read
// it.
//
// That is not a missing convenience. It is a hole in the evidence: the
// question "who put that file on that machine" is answered by the file
// itself, by the shell history of whoever was logged in, or not at all. The
// transfer trail is the one place the bastion answers it directly, and it
// was unreachable.
//
// ⚠️ What this tool is NOT: a second opinion on what a transfer did. The
// execution plane computes a checksum at transfer time; that is a SELF-
// REPORT from the transfer itself. This trail is the bastion's own record.
// They are different claims and neither replaces the other -- the checksum
// says "these bytes arrived", this trail says "this transfer was attempted,
// by whom, onto which machine, and whether the bastion considered it a
// success".
//
// The field names are this endpoint's own, and they are NOT the login log's:
// /audits/ftp-logs/ says date_start and carries `operate` as a labelled
// object, where /audits/login-logs/ says datetime and carries a plain
// string. Reading this trail under the other endpoint's names silently
// yields nulls -- the same trap jms_login_logs documents for the operate
// log.
package handler

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"

	"github.com/yi-nology/jumpserver-mcp/biz/jumpserver/model/jmsms"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oekit/pkg/convert"
	"github.com/yi-nology/jumpserver-mcp-kit/model/jms"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-mcp-kit/pkg/render"
	"github.com/yi-nology/jumpserver-mcp/internal/jumpserver/client"
)

func RegisterTransferTools(r *reg.Registry, c *client.Client) {
	type fileTransfersArgs struct {
		Asset     string `json:"asset,omitempty" jsonschema:"only transfers on this asset (address/name/uuid)"`
		UserName  string `json:"user,omitempty" jsonschema:"only transfers by this JumpServer user"`
		Direction string `json:"direction,omitempty" jsonschema:"upload / download / mkdir / rmdir / delete / rename / rename_dir / symlink"`
		Since     string `json:"since,omitempty" jsonschema:"only transfers at or after this date, e.g. 2026/10/20"`
		Limit     int    `json:"limit,omitempty" jsonschema:"max rows (default 30)"`
	}
	reg.Add(r, &mcp.Tool{
		Name:         "jms_file_transfers",
		Description:  "Query the file transfer trail: who uploaded or downloaded which file onto or off which machine, when, and whether it succeeded. Use it when the question is 'what files went onto this box' -- jms_asset_evidence covers what was TYPED, this covers what was MOVED. The bastion records every transfer the execution plane's jmsx_put_file / jmsx_get_file / jmsx_relay made, so this is the record of those too.",
		OutputSchema: render.Schema[render.ListOf[jmsms.FileTransferRecord]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args fileTransfersArgs) (*mcp.CallToolResult, any, error) {
		limit := client.NormLimit(args.Limit, 30, 200)

		direction := strings.ToLower(strings.TrimSpace(args.Direction))
		if direction != "" && !transferDirections[direction] {
			return render.Failf("direction must be one of %s, got %q",
				strings.Join(transferDirectionNames, "/"), args.Direction)
		}

		// ⚠️ EVERY filter is applied HERE, not on the server.
		//
		// The endpoint's own filters cannot be used, and using them would
		// produce answers that LOOK authoritative and are wrong:
		//
		//   - its user filter is `user` (iexact against the DISPLAY string
		//     "name(username)"), so `user=opsuser` matches nothing while
		//     `username=...` is not a declared parameter at all and is
		//     SILENTLY IGNORED (django-filter drops unknown parameters);
		//   - its date filter reads `date_from`/`date_to` and parses them
		//     with a strict DateTimeField -- a date that does not match
		//     the accepted formats is swallowed by the backend and the
		//     filter simply does not apply.
		//
		// Either way the caller gets "here are the transfers" with the
		// filter silently not applied -- the worst kind of wrong answer
		// for a forensic tool. ⇒ The trail is read whole and narrowed here,
		// the same way jms_commands_search does it.
		//
		// The ordering parameter is `order` -- NOT `ordering`. The bastion's
		// ORDERING_PARAM is "order", so an `ordering=` parameter is not
		// declared, is ignored without a word, and the rows come back in
		// whatever order the view defaults to.
		//
		// ⚠️ The local sort below does not depend on this: it sorts the
		// rows that were read. But the server-side ordering matters for
		// the WINDOW: when only the newest N rows are read, the server has
		// to hand over the NEWEST ones.
		q := url.Values{"order": {"-date_start"}}
		rows, serverTotal, err := c.ListAllCounted(ctx, string(jmsapi.ResFTPLogs), q)
		if err != nil {
			return render.Err(err)
		}

		// The asset filter accepts an address, a name or a uuid. A uuid can
		// only be resolved through the asset list, so that is tried first.
		var matcher *client.AssetMatcher
		assetNote := ""
		if args.Asset != "" {
			ids, meta, err := c.ResolveAssets(ctx, []string{args.Asset})
			switch {
			case err == nil:
				matcher = client.NewAssetMatcher(ids, meta)
			case errors.Is(err, jmsapi.ErrAssetNotFound):
				// The asset may have been DELETED from the bastion while its
				// transfer trail still sits in the record. That is exactly
				// when the question ("what was put on this machine") still
				// matters, so the trail is searched by the label its own
				// rows carry -- the same reason jms_asset_evidence falls
				// back to the label the sessions carry.
				//
				// ⚠️ But a reference that names NOTHING is a different
				// thing: a typo must not read as "this machine never had
				// a file transfer". So the reason is kept and said out
				// loud in the note.
				assetNote = "the asset is not in the bastion's list (" + err.Error() +
					"); the trail was searched by the label its own records carry"
			default:
				// Anything else (the listing could not be refreshed, the
				// bastion unreachable) is a MECHANISM problem, not a
				// "no such asset" answer -- reporting it as "no transfers"
				// would turn an outage into a forensic false negative.
				return render.Err(err)
			}
		}
		ref := strings.ToLower(strings.TrimSpace(args.Asset))
		// The date is normalised to the SAME shape the trail's stamps have
		// ("2026/10/20"), because the comparison below is a string
		// comparison: an ISO date left as "2026-10-20" would compare
		// against "2026/10/01" the wrong way round ('/' sorts after '-')
		// and EVERY row would pass -- including the ones from before the
		// requested date.
		//
		// A date that cannot be read is refused rather than turned into an
		// empty result: "no transfers" and "your date is nonsense" are
		// different claims, and answering the first one would be a
		// forensic false negative.
		since := ""
		if args.Since != "" {
			since = normDate(args.Since)
			if since == "" {
				return render.Failf("cannot read since=%q; use 2026/10/20 or 2026-10-20", args.Since)
			}
		}

		kept := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			// The asset is a flat "name(address)" label here, so it is
			// matched on both halves of it rather than on a parsed id --
			// this endpoint has no asset id field.
			name := convert.AnyToStr(jmsapi.AssetField(row, "name"))
			addr := convert.AnyToStr(jmsapi.AssetField(row, "address"))
			if args.Asset != "" {
				if !transferAssetMatches(matcher, name, addr, ref) {
					continue
				}
			}
			// The user filter matches EITHER half of the display string
			// ("name(username)"), because both halves are things a person
			// types -- the same rule jms_commands_search and jms_sessions_list
			// use. The endpoint's own filter is iexact against the whole
			// string, so `user=opsuser` would match nothing.
			if args.UserName != "" && !client.UserMatches(convert.AnyToStr(row["user"]), args.UserName) {
				continue
			}
			if direction != "" && !strings.EqualFold(jmsapi.Label(row, "operate"), direction) {
				continue
			}
			if since != "" && !transferAtOrAfter(row["date_start"], since) {
				continue
			}
			kept = append(kept, row)
		}

		out := make([]jmsms.FileTransferRecord, 0, len(kept))
		for _, row := range kept {
			out = append(out, jmsms.FileTransferRecord{
				ID:         jmsapi.FieldStr(row["id"]),
				User:       jmsapi.FieldStr(row["user"]),
				RemoteAddr: jmsapi.FieldStr(row["remote_addr"]),
				Asset:      jmsapi.FieldStr(jmsapi.AssetField(row, "name")),
				Address:    jmsapi.FieldStr(jmsapi.AssetField(row, "address")),
				Account:    jmsapi.FieldStr(row["account"]),
				Operate:    jmsapi.FieldStr(jmsapi.Label(row, "operate")),
				Filename:   jmsapi.FieldStr(row["filename"]),
				DateStart:  jmsapi.FieldStr(row["date_start"]),
				IsSuccess:  jmsapi.FieldBool(row["is_success"]),
				HasFile:    jmsapi.FieldBool(row["has_file"]),
				Session:    jmsapi.FieldStr(row["session"]),
			})
		}
		// Sort before capping, so the cap keeps the newest rows.
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].GetDateStart() > out[j].GetDateStart()
		})
		total := len(out)
		if len(out) > limit {
			out = out[:limit]
		}
		res := render.ListOf[jmsms.FileTransferRecord]{
			ListMeta: jms.ListMeta{
				Count:     int32(len(out)),
				Total:     int32(total),
				Truncated: total > len(out),
			},
			Items: out,
		}
		notes := []string{}
		if total > len(out) {
			notes = append(notes, "showing the newest "+convert.AnyToStr(len(out))+" of "+
				convert.AnyToStr(total)+" matching transfers")
		}
		// ⚠️ The server's own count is what the collection HOLDS. If more
		// rows were read than that, the endpoint does not page (it answered
		// with a bare array) and the count is simply not applicable.
		//
		// If FEWER rows were read than the server says it holds, then the
		// trail was NOT read whole -- the endpoint clamped the page size and
		// the walk stopped early. That has to be said out loud: the answer
		// looks like a complete list, and a caller that reports "nothing
		// was transferred" from it would be reporting a gap it never knew
		// about.
		if serverTotal > len(rows) {
			notes = append(notes, "only the newest "+convert.AnyToStr(len(rows))+
				" of the "+convert.AnyToStr(serverTotal)+" transfers the bastion holds were read; "+
				"older transfers may exist and are NOT in this answer")
		}
		if assetNote != "" {
			notes = append(notes, assetNote)
		}
		res.Note = strings.Join(notes, "; ")
		return render.Text(res)
	})
}

// transferAssetMatches reports whether a trail row belongs to the asset the
// caller named.
//
// ⚠️ The ADDRESS is the identity and the NAME is a snapshot, so the address
// is checked first and the name is only consulted when the row carries no
// address. Matching on the name alone would attribute a machine's history
// to whatever machine happens to carry that name today: a box that is
// renamed, or a name that is reused for a different box, would show the
// wrong transfers.
//
// The matcher is nil when the asset is not in the bastion's list (it may
// have been deleted), and then the label the record carries is all there
// is. There the SAME address-first rule applies, and the caller's reference
// decides which half is compared: a reference that IS an address is
// compared against the address, a reference that IS a name against the
// name.
//
// ⚠️ That last part matters: a machine that has been DELETED can only be
// asked for BY NAME (its address is not in the bastion's list any more),
// so a fallback that compared the address only would answer "no transfers"
// for the one question that machine is still asked about -- a forensic
// false negative about a machine that is GONE.
func transferAssetMatches(matcher *client.AssetMatcher, name, addr, ref string) bool {
	if matcher != nil {
		// The asset IS in the list: its address is the identity, so a row
		// whose address is not it is not that machine, whatever its name
		// says today.
		if addr != "" {
			return matcher.MatchAddress(addr)
		}
		// The row carries no address at all: the name is all there is.
		return matcher.MatchNameAddress(name, addr)
	}
	// Not in the list: the label the record carries is all there is.
	// The caller's reference says which half it means.
	//
	// ⚠️ A NAME reference matches the name, and a name can be shared: the
	// answer then covers every machine that ever carried that name. That
	// is said in the note, because the reader cannot see it from the rows
	// (they carry the same name). An ADDRESS reference matches the address
	// only -- that is the identity, and it is never ambiguous.
	if looksLikeAddress(ref) {
		return strings.EqualFold(addr, ref)
	}
	return strings.EqualFold(name, ref)
}

// looksLikeAddress reports whether a reference is an address rather than a
// name.
//
// It is decided by SHAPE, not by asking the bastion: the fallback path
// exists precisely because the bastion cannot be asked (the asset is gone
// from its list). An address is digits and dots; a name is a word.
//
// ⚠️ A reference that is neither (a hostname, say) reads as a NAME, and
// then the name half is compared. A wrong match shows a machine's
// transfers that do not belong to it -- and the reader can see the address
// on the row -- while a MISSED match hides a machine's transfers
// altogether, and the reader cannot see what is missing. ⇒ When in doubt,
// the direction that can hide things is the one to avoid.
func looksLikeAddress(ref string) bool {
	if ref == "" {
		return false
	}
	digits, dots := 0, 0
	for _, r := range ref {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '.':
			dots++
		default:
			return false
		}
	}
	return digits > 0 && dots >= 2
}

// transferDirections is the set of directions the trail can carry.
//
// ⚠️ It is NOT "upload and download": the bastion's OperateChoices has eight
// values (mkdir, rmdir, delete, upload, rename, symlink, download,
// rename_dir), and a file operation that creates or removes a directory is
// recorded too. Refusing those would refuse a real direction the trail
// holds.
var transferDirections = map[string]bool{
	"mkdir":      true,
	"rmdir":      true,
	"delete":     true,
	"upload":     true,
	"rename":     true,
	"symlink":    true,
	"download":   true,
	"rename_dir": true,
}

// transferDirectionNames lists the accepted directions, for the refusal.
var transferDirectionNames = []string{
	"upload", "download", "mkdir", "rmdir", "delete", "rename", "rename_dir", "symlink",
}

// transferAtOrAfter reports whether a trail row's timestamp is at or after
// the caller's date.
//
// ⚠️ The comparison is done on the STRINGS, and that is deliberate: the
// endpoint's timestamp is a fixed-width, zero-padded
// "2026/10/28 00:05:33 +0800", so its lexicographic order IS its
// chronological order. Parsing it would mean pulling in a timezone-aware
// date parser for a format the endpoint never promised.
//
// The caller's date may be shorter ("2026/10/20"), so it is compared as a
// prefix: a row is at or after it when the row's timestamp starts with a
// date that is not earlier. That is done by comparing the first ten
// characters (the date part) against the caller's first ten characters, and
// a caller who gave a full timestamp has its extra part compared too.
//
// ⚠️ A caller date that is not in that shape matches nothing, and the
// filter that drops everything says so in the note rather than silently
// answering "no transfers".
func transferAtOrAfter(stamp any, since string) bool {
	row := convert.AnyToStr(stamp)
	if row == "" {
		return false
	}
	// Compare the date part first, then the time part when both have one.
	rowDate, rowRest := splitStamp(row)
	sinceDate, sinceRest := splitStamp(since)
	if rowDate != sinceDate {
		return rowDate > sinceDate
	}
	// The same date: the time decides, and a caller who gave no time means
	// "the whole day".
	if sinceRest == "" {
		return true
	}
	return rowRest >= sinceRest
}

// splitStamp splits a timestamp into its date part and the rest.
//
// "2026/10/28 00:05:33 +0800" -> "2026/10/28", "00:05:33 +0800"
// "2026/10/20"                -> "2026/10/20", ""
func splitStamp(s string) (date, rest string) {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " T"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}
