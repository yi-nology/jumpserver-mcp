// tools_account_automation.go reaches the bastion's ACCOUNT AUTOMATION
// facilities: the plans that push a secret onto a machine, and the plans
// that discover the accounts on one.
//
// Why this exists.
//
// The bastion can do two things an agent cannot do for it:
//
//   - PUSH a secret: the bastion generates the password itself, logs into
//     the target machine and installs it, and keeps it in its own vault.
//     The secret never passes through the agent, never through this
//     process, and never through the agent's context window.
//   - GATHER the accounts on a machine: the bastion logs in and discovers
//     what is there.
//
// Both are what "rotate the credentials of this fleet" actually means.
// Without them the only way to change a password is jms_account_update,
// which makes the SECRET PASS THROUGH THE AGENT -- the opposite of the
// point.
//
// What is NOT here, and why.
//
// ⚠️ The bastion has two more automation families -- change-secret and
// check-account -- and on a deployment without a licence they answer
// **403 permission_denied**. That reads like "you lack the permission" and
// is NOT: upstream they carry `permission_classes = [RBACPermission,
// IsValidLicense]`, so the feature itself is unlicensed. The tool says so
// when it meets that answer, instead of repeating "permission denied" and
// sending the caller looking for a role to grant.
//
// ⚠️ The failed-execution REASON is not available: the upstream execution
// serializer exposes neither `result` nor `summary`, so an execution that
// failed says only "failed". The tool says that too, rather than letting
// "failed" be read as "nothing happened".
package handler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/yi-nology/jumpserver-mgmt/biz/model/jmsms"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oekit/pkg/convert"
	"github.com/yi-nology/jumpserver-kit/model/jms"
	"github.com/yi-nology/jumpserver-kit/pkg/errs"
	"github.com/yi-nology/jumpserver-kit/pkg/jmsapi"
	"github.com/yi-nology/jumpserver-kit/pkg/reg"
	"github.com/yi-nology/jumpserver-kit/pkg/render"
	"github.com/yi-nology/jumpserver-mgmt/internal/repository"
)

// accountAutomationFamilies are the automation kinds this tool reaches.
//
// ⚠️ The collection each kind lives in is DIFFERENT -- the bastion has one
// collection per family, and asking the wrong one answers about the wrong
// thing (or 403s).
var accountAutomationFamilies = map[string]jmsapi.Resource{
	"push":   jmsapi.ResPushAutomations,
	"gather": jmsapi.ResGatherAutomations,
}

// accountAutomationExecutionFamilies are the execution collections.
var accountAutomationExecutionFamilies = map[string]jmsapi.Resource{
	"push":   jmsapi.ResPushExecutions,
	"gather": jmsapi.ResGatherExecutions,
}

// accountAutomationRecordFamilies are the per-account result collections.
//
// ⚠️ Only the push family has one. Upstream registers no
// "gather-account-records" collection (asking for it answers 404), and
// that is not an oversight: a push writes a PushSecretRecord per account
// it touched, while a gather produces DISCOVERED ACCOUNTS -- which live in
// /accounts/gathered-accounts/ (jms_gathered_accounts already reads them).
//
// ⇒ The map has one entry on purpose. Adding a gather one would mean
// pointing at a collection that does not exist.
var accountAutomationRecordFamilies = map[string]jmsapi.Resource{
	"push": jmsapi.ResPushRecords,
}

func RegisterAccountAutomationTools(r *reg.Registry, c *repository.Client) {
	registerAutomationListTool(r, c)
	registerAutomationCreateTool(r, c)
	registerAutomationRunTool(r, c)
	registerAutomationExecutionsTool(r, c)
	registerAutomationRecordsTool(r, c)
}

// automationFamilyArg reads the family argument and checks it.
//
// ⚠️ The family picks the COLLECTION, so an unknown one has to be refused
// rather than defaulted: defaulting to "push" would answer a question about
// gather with push's data, and the caller would never know.
func automationFamilyArg(family string) (string, error) {
	f := strings.ToLower(strings.TrimSpace(family))
	if f == "" {
		return "", errors.New("family is required: push or gather")
	}
	if _, ok := accountAutomationFamilies[f]; !ok {
		return "", fmt.Errorf("family must be push or gather, got %q", family)
	}
	return f, nil
}

// registerAutomationListTool lists the plans of one family.
func registerAutomationListTool(r *reg.Registry, c *repository.Client) {
	type listArgs struct {
		Family string `json:"family" jsonschema:"push (the bastion pushes a secret onto the machine) or gather (the bastion discovers the accounts on it)"`
		Search string `json:"search,omitempty" jsonschema:"substring match on the plan name"`
		Limit  int    `json:"limit,omitempty" jsonschema:"max rows (default 20)"`
	}
	reg.Add(r, &mcp.Tool{
		Name: "jms_account_automations_list",
		Description: "List the bastion's account automation plans: the named recipes that push a secret onto machines or discover the accounts on them. " +
			"Use it to find a plan id before triggering one with jms_account_automation_run, or to see whether a fleet already has a plan. " +
			"family=push is the one that keeps secrets out of the agent's hands: the bastion generates the password and installs it itself " +
			"(on machines where that account does not exist yet). Read-only.",
		OutputSchema: render.Schema[render.ListOf[jmsms.AccountAutomation]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listArgs) (*mcp.CallToolResult, any, error) {
		family, err := automationFamilyArg(args.Family)
		if err != nil {
			return render.Failf("%v", err)
		}
		limit := repository.NormLimit(args.Limit, 20, 100)

		rows, err := c.List(ctx, accountAutomationFamilies[family])
		if err != nil {
			return render.Failf("%v", automationError(ctx, err, family))
		}
		kept := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if args.Search != "" &&
				!strings.Contains(strings.ToLower(convert.AnyToStr(row["name"])), strings.ToLower(args.Search)) {
				continue
			}
			kept = append(kept, row)
		}
		out := make([]jmsms.AccountAutomation, 0, len(kept))
		for _, row := range kept {
			out = append(out, automationRow(row))
		}
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].GetName() < out[j].GetName()
		})
		total := len(out)
		if len(out) > limit {
			out = out[:limit]
		}
		res := render.ListOf[jmsms.AccountAutomation]{
			ListMeta: jms.ListMeta{
				Count:     int32(len(out)),
				Total:     int32(total),
				Truncated: total > len(out),
			},
			Items: out,
		}
		if total == 0 {
			res.Note = "no " + family + " automation plan exists on this bastion"
		}
		return render.Text(res)
	})
}

// registerAutomationCreateTool creates a plan.
func registerAutomationCreateTool(r *reg.Registry, c *repository.Client) {
	type createArgs struct {
		Family       string   `json:"family" jsonschema:"push or gather"`
		Name         string   `json:"name" jsonschema:"plan name (must be unique)"`
		Assets       []string `json:"assets,omitempty" jsonschema:"assets to act on (address/name/uuid)"`
		Accounts     []string `json:"accounts,omitempty" jsonschema:"push only: the account names to create on the machines (required for push)"`
		SecretType   string   `json:"secret_type,omitempty" jsonschema:"push only: password or ssh_key (default password)"`
		SecretLength int      `json:"secret_length,omitempty" jsonschema:"push only: length of the generated secret, 8-36 (default 16)"`
		SyncAccount  bool     `json:"sync_account,omitempty" jsonschema:"gather only: also create the discovered accounts in the bastion (default false = only report them)"`
		Comment      string   `json:"comment,omitempty" jsonschema:"a note about the plan"`
		Confirm      bool     `json:"confirm" jsonschema:"must be true; this creates a plan on the bastion"`
	}
	reg.Add(r, &mcp.Tool{
		Name: "jms_account_automation_create",
		Description: "Create an account automation plan on the bastion: a named recipe that acts on given assets. " +
			"family=push makes the bastion generate a secret and install it on the target machines itself -- the secret never reaches the agent; " +
			"it CREATES the accounts named in `accounts` on the machines where they do not exist yet (it does NOT rotate an existing account's password). " +
			"family=gather makes it log into those machines and report the accounts it finds; it needs no `accounts` (it logs in with the accounts the assets already have) " +
			"and `sync_account=true` also creates the discovered accounts in the bastion. " +
			"Use it when a fleet needs one of those recipes; trigger the plan afterwards with jms_account_automation_run. " +
			"Creating a plan changes the bastion's configuration.",
		OutputSchema: render.Schema[jmsms.AccountAutomation](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createArgs) (*mcp.CallToolResult, any, error) {
		family, err := automationFamilyArg(args.Family)
		if err != nil {
			return render.Failf("%v", err)
		}
		name := strings.TrimSpace(args.Name)
		if name == "" {
			return render.Failf("name is required")
		}
		// ⚠️ `accounts` is required for push and NOT for gather, and the
		// two mean different things:
		//
		//   - push: the accounts to CREATE on the machines (upstream marks
		//     the field required for that family);
		//   - gather: NOT the accounts to look for. The gather run logs in
		//     with the accounts the ASSETS already have and reports what it
		//     finds, so the field is not even required there (upstream
		//     requires it only for push).
		//
		// ⇒ Requiring it for gather would make the caller name accounts
		// that have nothing to do with what a gather does.
		if family == "push" && len(args.Accounts) == 0 {
			return render.Failf("accounts is required for a push plan: name the accounts to create on the machines")
		}
		secretType := strings.TrimSpace(args.SecretType)
		if secretType == "" {
			secretType = "password"
		}
		if secretType != "password" && secretType != "ssh_key" {
			return render.Failf("secret_type must be password or ssh_key, got %q", args.SecretType)
		}
		length := args.SecretLength
		if length == 0 {
			length = 16
		}
		if length < 8 || length > 36 {
			return render.Failf("secret_length must be between 8 and 36, got %d", length)
		}

		// The assets are named by address, name or uuid, and the bastion
		// wants their ids.
		assetIDs := []string{}
		if len(args.Assets) > 0 {
			ids, _, err := c.ResolveAssets(ctx, args.Assets)
			if err != nil {
				return render.Failf("%v", err)
			}
			assetIDs = ids
		}

		body := map[string]any{
			"name":    name,
			"comment": args.Comment,
			// The plan is NOT periodic: the agent triggers it explicitly.
			// A periodic plan would run on the bastion's own schedule,
			// which is a different thing from what was asked for.
			"is_periodic": false,
		}
		if family == "push" {
			// The accounts to CREATE on the machines. A gather does not
			// take them: it logs in with the accounts the assets already
			// have, so sending names there would say nothing.
			body["accounts"] = args.Accounts
			body["secret_type"] = secretType
			// random = the bastion generates the secret. It is the whole
			// point of the family: a specific secret would have to pass
			// through the caller, which is what this avoids.
			body["secret_strategy"] = "random"
			body["password_rules"] = map[string]any{
				"length": length, "lowercase": true, "uppercase": true,
				"digit": true, "symbol": true, "exclude_symbols": "",
			}
		} else {
			// gather: whether the DISCOVERED accounts are also CREATED in
			// the bastion. The default upstream is false (report only),
			// and that is the safe one to keep: creating accounts in the
			// bastion is a configuration change nobody asked for.
			//
			// ⚠️ It is sent only when asked for, so that the plan reads
			// as "report only" by default rather than "whatever the
			// caller happened not to say".
			if args.SyncAccount {
				body["is_sync_account"] = true
			}
		}
		if len(assetIDs) > 0 {
			body["assets"] = assetIDs
		}

		var created map[string]any
		if err := c.Post(ctx, accountAutomationFamilies[family], body, &created); err != nil {
			return render.Failf("%v", automationError(ctx, err, family))
		}
		return render.Text(automationRow(created))
	})
}

// registerAutomationRunTool triggers a plan once.
func registerAutomationRunTool(r *reg.Registry, c *repository.Client) {
	type runArgs struct {
		Family  string `json:"family" jsonschema:"push or gather"`
		Plan    string `json:"plan" jsonschema:"the plan id (or its exact name) to trigger"`
		Confirm bool   `json:"confirm" jsonschema:"must be true; this runs the plan on the bastion against real machines"`
	}
	reg.Add(r, &mcp.Tool{
		Name: "jms_account_automation_run",
		Description: "Trigger an account automation plan ONCE on the bastion. " +
			"family=push makes the bastion generate a secret and install it on the target machines itself -- the secret never reaches the agent; " +
			"it creates the named accounts where they do not exist yet (it does not rotate an existing account's password). " +
			"The execution is ASYNCHRONOUS: this answers that the bastion accepted the work, not that it finished; " +
			"read the outcome afterwards with jms_account_automation_executions and jms_account_automation_records. " +
			"Running a plan changes the target machines, so it needs a confirmation.",
		OutputSchema: render.Schema[jmsms.AccountAutomationRun](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args runArgs) (*mcp.CallToolResult, any, error) {
		family, err := automationFamilyArg(args.Family)
		if err != nil {
			return render.Failf("%v", err)
		}
		plan := strings.TrimSpace(args.Plan)
		if plan == "" {
			return render.Failf("plan is required: the plan id or its exact name")
		}
		planID, planName, err := resolveAutomationPlan(ctx, c, family, plan)
		if err != nil {
			return render.Failf("%v", err)
		}

		// The execution collection is a different collection from the plans
		// one: POSTing there is what starts the run.
		var out map[string]any
		err = c.Post(ctx, accountAutomationExecutionFamilies[family],
			map[string]any{"automation": planID}, &out)
		if err != nil {
			return render.Failf("%v", automationError(ctx, err, family))
		}
		res := &jmsms.AccountAutomationRun{
			Task:         jmsapi.FieldStr(out["task"]),
			Automation:   jmsapi.FieldStr(planName),
			AutomationID: jmsapi.FieldStr(planID),
			ExecutionHint: jmsapi.FieldStr("the bastion accepted the run; it executes in the background -- " +
				"read the outcome with jms_account_automation_executions (family=" + family + ")"),
		}
		return render.Text(res)
	})
}

// registerAutomationExecutionsTool lists the runs of one family.
func registerAutomationExecutionsTool(r *reg.Registry, c *repository.Client) {
	type execArgs struct {
		Family string `json:"family" jsonschema:"push or gather"`
		Plan   string `json:"plan,omitempty" jsonschema:"only runs of this plan (id or exact name)"`
		Limit  int    `json:"limit,omitempty" jsonschema:"max rows (default 20)"`
	}
	reg.Add(r, &mcp.Tool{
		Name: "jms_account_automation_executions",
		Description: "List the executions of an account automation family: when they ran, how they were triggered, and whether they succeeded. " +
			"Use it after jms_account_automation_run to find out whether the work finished. " +
			"⚠️ A failed execution says only THAT it failed: the bastion's API does not expose the reason, so read jms_account_automation_records for per-account detail. Read-only.",
		OutputSchema: render.Schema[render.ListOf[jmsms.AccountAutomationExecution]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args execArgs) (*mcp.CallToolResult, any, error) {
		family, err := automationFamilyArg(args.Family)
		if err != nil {
			return render.Failf("%v", err)
		}
		limit := repository.NormLimit(args.Limit, 20, 100)

		rows, err := c.List(ctx, accountAutomationExecutionFamilies[family])
		if err != nil {
			return render.Failf("%v", automationError(ctx, err, family))
		}
		planID, planName := "", ""
		if args.Plan != "" {
			planID, planName, err = resolveAutomationPlan(ctx, c, family, args.Plan)
			if err != nil {
				return render.Failf("%v", err)
			}
		}
		out := make([]jmsms.AccountAutomationExecution, 0, len(rows))
		for _, row := range rows {
			exec := automationExecutionRow(row)
			if planID != "" {
				// The row names the plan; match on the id when it is
				// there, on the name otherwise.
				if exec.GetAutomationID() != planID && exec.GetAutomation() != planName {
					continue
				}
			}
			out = append(out, exec)
		}
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].GetDateStart() > out[j].GetDateStart()
		})
		total := len(out)
		if len(out) > limit {
			out = out[:limit]
		}
		res := render.ListOf[jmsms.AccountAutomationExecution]{
			ListMeta: jms.ListMeta{
				Count:     int32(len(out)),
				Total:     int32(total),
				Truncated: total > len(out),
			},
			Items: out,
		}
		failed := 0
		for _, e := range out {
			if e.GetStatus() == "failed" {
				failed++
			}
		}
		notes := []string{}
		if total > len(out) {
			notes = append(notes, "showing the newest "+convert.AnyToStr(len(out))+" of "+
				convert.AnyToStr(total)+" executions")
		}
		if failed > 0 {
			// ⚠️ The reason is NOT available: the upstream execution
			// serializer exposes neither `result` nor `summary`. Saying
			// "failed" without saying that is what makes a caller go
			// looking for the reason in the wrong place.
			notes = append(notes, convert.AnyToStr(failed)+" execution(s) failed; "+
				"the bastion's API does not expose the reason")
			// And the per-account records say WHERE it stopped: the
			// bastion creates one record per account as it starts on it,
			// so NO record at all means it never got as far as the first
			// machine -- which is a very different thing from "every
			// machine refused". That difference is worth one extra call.
			//
			// ⚠️ Only the push family HAS such records; a gather produces
			// discovered accounts instead. Asking for a gather's records
			// would hit a collection that does not exist (404), so the
			// question is not asked at all.
			if res, ok := accountAutomationRecordFamilies[family]; ok {
				switch n, err := c.List(ctx, res); {
				case err == nil && len(n) == 0:
					notes = append(notes, "and there is no per-account record: "+
						"the run failed before it reached any machine (the bastion creates a record "+
						"per account as it starts on it) -- check the bastion's ansible/celery components")
				case err == nil:
					notes = append(notes, "the per-account detail is in jms_account_automation_records")
				}
			}
		}
		res.Note = strings.Join(notes, "; ")
		return render.Text(res)
	})
}

// registerAutomationRecordsTool lists the per-account results.
func registerAutomationRecordsTool(r *reg.Registry, c *repository.Client) {
	type recordArgs struct {
		Family string `json:"family" jsonschema:"push"`
		Asset  string `json:"asset,omitempty" jsonschema:"only records of this asset (address/name/uuid)"`
		Limit  int    `json:"limit,omitempty" jsonschema:"max rows (default 30)"`
	}
	reg.Add(r, &mcp.Tool{
		Name: "jms_account_automation_records",
		Description: "List the per-account results of a PUSH automation: which account on which machine got a secret, and whether it worked. " +
			"Use it for the detail behind an execution that jms_account_automation_executions reports as failed. " +
			"⚠️ family=push only: the gather family has no per-account result record (upstream registers no such collection); " +
			"what a gather produces is the DISCOVERED ACCOUNTS -- read those with jms_gathered_accounts. Read-only.",
		OutputSchema: render.Schema[render.ListOf[jmsms.AccountAutomationRecord]](),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args recordArgs) (*mcp.CallToolResult, any, error) {
		family, err := automationFamilyArg(args.Family)
		if err != nil {
			return render.Failf("%v", err)
		}
		// ⚠️ Only push has a per-account result record. Asking gather for
		// one would hit a collection that does not exist (404), so the
		// answer says where the gather results ARE instead.
		//
		// A gather produces DISCOVERED ACCOUNTS, not a result record per
		// account -- those live in /accounts/gathered-accounts/, which
		// jms_gathered_accounts reads.
		recordRes, ok := accountAutomationRecordFamilies[family]
		if !ok {
			return render.Failf("the %s family has no per-account result record: "+
				"what a gather produces is the DISCOVERED ACCOUNTS -- read them with jms_gathered_accounts", family)
		}
		limit := repository.NormLimit(args.Limit, 30, 200)

		rows, err := c.List(ctx, recordRes)
		if err != nil {
			return render.Failf("%v", automationError(ctx, err, family))
		}
		var matcher *repository.AssetMatcher
		if args.Asset != "" {
			ids, meta, err := c.ResolveAssets(ctx, []string{args.Asset})
			if err != nil {
				return render.Failf("%v", err)
			}
			matcher = repository.NewAssetMatcher(ids, meta)
		}
		out := make([]jmsms.AccountAutomationRecord, 0, len(rows))
		for _, row := range rows {
			asset := convert.AnyToStr(row["asset"])
			name, addr := jmsapi.SplitAssetLabel(asset)
			if matcher != nil && !matcher.MatchNameAddress(name, addr) {
				continue
			}
			out = append(out, jmsms.AccountAutomationRecord{
				ID:           jmsapi.FieldStr(row["id"]),
				Asset:        jmsapi.FieldStr(name),
				Address:      jmsapi.FieldStr(addr),
				Account:      jmsapi.FieldStr(row["account"]),
				IsSuccess:    jmsapi.FieldBool(row["is_success"]),
				Status:       jmsapi.FieldStr(row["status"]),
				DateFinished: jmsapi.FieldStr(row["date_finished"]),
				Error:        jmsapi.FieldStr(row["error"]),
			})
		}
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].GetDateFinished() > out[j].GetDateFinished()
		})
		total := len(out)
		if len(out) > limit {
			out = out[:limit]
		}
		res := render.ListOf[jmsms.AccountAutomationRecord]{
			ListMeta: jms.ListMeta{
				Count:     int32(len(out)),
				Total:     int32(total),
				Truncated: total > len(out),
			},
			Items: out,
		}
		if total == 0 {
			res.Note = "no per-account record exists for this family"
		}
		return render.Text(res)
	})
}

// resolveAutomationPlan finds a plan by id or by its exact name.
//
// A plan is named by its id or by its name, and the name is what a person
// knows while the id is what a caller usually has. Both work here.
//
// ⚠️ The name match is EXACT: two plans may share a name in the bastion's
// eyes only if it allows it, and a prefix match would trigger the wrong
// one -- which, for a push, installs a password on the wrong machines.
func resolveAutomationPlan(ctx context.Context, c *repository.Client, family, ref string) (id, name string, err error) {
	rows, err := c.List(ctx, accountAutomationFamilies[family])
	if err != nil {
		return "", "", err
	}
	for _, row := range rows {
		rid := convert.AnyToStr(row["id"])
		if rid == ref {
			return rid, convert.AnyToStr(row["name"]), nil
		}
	}
	var matches []map[string]any
	for _, row := range rows {
		if convert.AnyToStr(row["name"]) == ref {
			matches = append(matches, row)
		}
	}
	switch len(matches) {
	case 1:
		return convert.AnyToStr(matches[0]["id"]), convert.AnyToStr(matches[0]["name"]), nil
	case 0:
		return "", "", fmt.Errorf("no %s automation plan named %q on this bastion", family, ref)
	default:
		return "", "", fmt.Errorf("%q matches %d %s automation plans; use the plan id", ref, len(matches), family)
	}
}

// automationRow reads a plan out of its record.
//
// The LIST shape and the DETAIL shape differ: the list carries
// assets_amount / nodes_amount instead of the arrays, so both are read
// from wherever they are.
func automationRow(row map[string]any) jmsms.AccountAutomation {
	out := jmsms.AccountAutomation{
		ID:                   jmsapi.FieldStr(row["id"]),
		Name:                 jmsapi.FieldStr(row["name"]),
		Type:                 jmsapi.FieldStr(row["type"]),
		Accounts:             asStringSlice(row["accounts"]),
		AssetsAmount:         jmsapi.FieldInt32(row["assets_amount"]),
		NodesAmount:          jmsapi.FieldInt32(row["nodes_amount"]),
		ExecutedAmount:       jmsapi.FieldInt32(row["executed_amount"]),
		SecretType:           jmsapi.FieldStr(jmsapi.Label(row, "secret_type")),
		SecretStrategy:       jmsapi.FieldStr(jmsapi.Label(row, "secret_strategy")),
		IsPeriodic:           jmsapi.FieldBool(row["is_periodic"]),
		Interval:             jmsapi.FieldInt32(row["interval"]),
		Crontab:              jmsapi.FieldStr(row["crontab"]),
		IsActive:             jmsapi.FieldBool(row["is_active"]),
		CheckConnAfterChange: jmsapi.FieldBool(row["check_conn_after_change"]),
		LastExecutionDate:    jmsapi.FieldStr(row["last_execution_date"]),
		DateCreated:          jmsapi.FieldStr(row["date_created"]),
		DateUpdated:          jmsapi.FieldStr(row["date_updated"]),
		Comment:              jmsapi.FieldStr(row["comment"]),
		CreatedBy:            jmsapi.FieldStr(row["created_by"]),
	}
	// The password rules are a nested object; only the length is worth
	// reporting (the flags are the bastion's defaults).
	if rules, ok := row["password_rules"].(map[string]any); ok {
		out.PasswordLength = jmsapi.FieldInt32(rules["length"])
	}
	return out
}

// automationExecutionRow reads one execution.
func automationExecutionRow(row map[string]any) jmsms.AccountAutomationExecution {
	out := jmsms.AccountAutomationExecution{
		ID:           jmsapi.FieldStr(row["id"]),
		Automation:   jmsapi.FieldStr(jmsapi.FieldName(row, "automation")),
		AutomationID: jmsapi.FieldStr(jmsapi.Nested(row, "automation", "id")),
		Trigger:      jmsapi.FieldStr(jmsapi.Label(row, "trigger")),
		Status:       jmsapi.FieldStr(jmsapi.Label(row, "status")),
		DateStart:    jmsapi.FieldStr(row["date_start"]),
		DateFinished: jmsapi.FieldStr(row["date_finished"]),
		Duration:     jmsapi.FieldStr(row["duration"]),
	}
	// The snapshot is a nested object with the plan's own shape.
	//
	// ⚠️ The upstream snapshot gives `accounts` as an ARRAY (of account
	// ids) and NO `account_amount` key at all -- reading one would yield
	// nothing. So the count is the length of that array.
	if snap, ok := row["snapshot"].(map[string]any); ok {
		out.SnapshotName = jmsapi.FieldStr(snap["name"])
		out.SnapshotAccountAmount = jmsapi.FieldInt32(len(jmsapi.AsSlice(snap["accounts"])))
		out.SnapshotAssetAmount = jmsapi.FieldInt32(snap["asset_amount"])
		out.SnapshotNodeAmount = jmsapi.FieldInt32(snap["node_amount"])
	}
	return out
}

// automationError explains a failed call in the terms of the deployment.
//
// ⚠️ The 403 from an unlicensed feature reads exactly like the 403 from a
// missing role, and sending the caller to fix a role that is already
// granted wastes their time. The upstream permission classes say which
// families carry a licence check, so the answer can say which it is.
//
// A caller that sees "permission denied" with no further word will assume
// the second.
func automationError(ctx context.Context, err error, family string) error {
	var apiErr *errs.APIError
	if errors.As(err, &apiErr) && apiErr.Status == 403 {
		if automationNeedsLicence(family) {
			return fmt.Errorf("the bastion refused this call: the %s automation family needs a "+
				"JumpServer licence on this deployment (the bastion answers 403 permission_denied for "+
				"an unlicensed feature, which reads like a missing role but is not one)", family)
		}
		return fmt.Errorf("the bastion refused this call (403): the credential lacks the permission "+
			"for the %s automation family", family)
	}
	return err
}

// automationNeedsLicence reports whether a family is licence-gated.
//
// ⚠️ It is a TABLE, and the default is "NOT licence-gated" on purpose.
//
// The families this tool reaches -- push and gather -- are core features
// that work on a community deployment (measured: 200 on a bastion with no
// licence). The licence-gated ones upstream are change-secret and
// check-account, which carry
// `permission_classes = [RBACPermission, IsValidLicense]` -- and they are
// not in this tool, because on a community deployment they cannot be
// reached at all.
//
// ⇒ The default says "no licence involved" because a wrong "yes" would
// send the caller to buy a licence for a feature that is simply missing
// from the tool's reach. A family that IS licence-gated has to be added
// here EXPLICITLY, with the reason.
//
// ⚠️ And a family that is not in the table at all is not licence-gated
// either: the family argument is validated before this is asked, so an
// unknown family never reaches here.
func automationNeedsLicence(family string) bool {
	switch family {
	case "change_secret", "check_account":
		// Upstream: permission_classes = [RBACPermission, IsValidLicense].
		return true
	default:
		return false
	}
}
