package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/testsupport"
)

// fakeBastionWithJobs serves the two sides of a batch run: the audit log that
// records who ran what, and the operational record that says which target
// failed and why.
func fakeBastionWithJobs(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/audits/job-logs/":
			json.NewEncoder(w).Encode(map[string]any{"count": 3, "results": []map[string]any{
				{"id": "L1", "material": "shell:echo DOCKER29-OK host=$(hostname)",
					"job_type": "Command", "time_cost": "5.16",
					"is_finished": true, "is_success": true, "task_id": "T1",
					"creator_name":  "Administrator",
					"date_start":    "2026/10/17 15:40:12 +0800",
					"date_finished": "2026/10/17 15:40:17 +0800",
					"date_created":  "2026/10/17 15:40:11 +0800"},
				{"id": "L2",
					"material": "shell:docker ps --format '{{.Names}}' # token=abcdef123456",
					"job_type": "Command", "time_cost": "1.76",
					"is_finished": true, "is_success": false, "task_id": "T2",
					"creator_name":  "Administrator",
					"date_start":    "2026/10/14 09:37:58 +0800",
					"date_finished": "2026/10/14 09:38:00 +0800",
					"date_created":  "2026/10/14 09:37:56 +0800"},
				{"id": "L3", "material": "shell:",
					"job_type": "Command", "time_cost": "1.56",
					"is_finished": true, "is_success": false, "task_id": "T3",
					"creator_name": "Administrator",
					"date_created": "2026/10/14 23:13:50 +0800"},
			}})
		case "/api/v1/ops/job-executions/":
			json.NewEncoder(w).Encode(map[string]any{"count": 2, "results": []map[string]any{
				{"id": "T1", "task_id": "T1", "date_start": "2026/10/17 15:40:12 +0800",
					"is_success": true,
					"summary":    map[string]any{"ok": []any{"rpi4-011"}, "failures": map[string]any{}, "skipped": []any{}}},
				{"id": "T2", "task_id": "T2", "date_start": "2026/10/14 09:37:58 +0800",
					"is_success": false,
					"summary": map[string]any{"ok": []any{}, "skipped": []any{},
						"failures": map[string]any{"node-071": "template error: unexpected '.'"}}},
			}})
		case "/api/v1/audits/jobs/":
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "results": []map[string]any{
				{"id": "J1", "name": "docker-smoke", "created_by": "System",
					"date_last_run": "2026/10/17 15:40:17 +0800", "is_periodic_display": "False",
					"interval": "24", "summary": map[string]any{"total": 1, "success": 1},
					"args": "echo DOCKER29-OK host=$(hostname)"},
			}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// The audit side carries what the operational side does not: who ran it, the
// command as issued, and the time it took.
func TestJobAuditReportsWhoRanWhat(t *testing.T) {
	cs := testSession(t, fakeBastionWithJobs(t))
	out := testsupport.CallToolJSON(t, cs, "jms_job_audit", map[string]any{})

	if out["count"] != float64(3) {
		t.Errorf("count = %v, want 3", out["count"])
	}
	execs := out["executions"].([]any)
	first := execs[0].(map[string]any)
	if first["creator"] != "Administrator" {
		t.Errorf("creator = %v, want Administrator", first["creator"])
	}
	if !strings.Contains(first["command"].(string), "DOCKER29-OK") {
		t.Errorf("command = %v, want the issued command", first["command"])
	}
	// The "shell:" prefix is the transport, not part of the command.
	if strings.HasPrefix(first["command"].(string), "shell:") {
		t.Errorf("command still carries the shell: prefix: %v", first["command"])
	}
	if out["succeeded"] != float64(1) || out["failed"] != float64(2) {
		t.Errorf("succeeded=%v failed=%v, want 1 and 2", out["succeeded"], out["failed"])
	}
}

// The per-target outcome lives on the operational side and is joined on
// task_id; without the join the audit cannot say WHICH machine failed.
func TestJobAuditJoinsTheHostOutcome(t *testing.T) {
	cs := testSession(t, fakeBastionWithJobs(t))
	out := testsupport.CallToolJSON(t, cs, "jms_job_audit", map[string]any{"only_failed": true})

	failed := out["failed_executions"].([]any)
	if len(failed) != 2 {
		t.Fatalf("failed_executions = %d, want 2", len(failed))
	}
	var sawReason bool
	for _, e := range failed {
		hosts, ok := e.(map[string]any)["hosts"].(map[string]any)
		if !ok {
			continue
		}
		reasons := hosts["failed"].(map[string]any)
		if r, ok := reasons["node-071"].(string); ok && strings.Contains(r, "template error") {
			sawReason = true
		}
	}
	if !sawReason {
		t.Errorf("the per-host failure reason was not joined in: %v", failed)
	}
}

// A run the bastion recorded but did not issue a command for must say so,
// rather than printing an empty command.
func TestJobAuditFlagsARunWithNoCommand(t *testing.T) {
	cs := testSession(t, fakeBastionWithJobs(t))
	out := testsupport.CallToolJSON(t, cs, "jms_job_audit", map[string]any{"only_failed": true})
	if out["without_command"] != float64(1) {
		t.Errorf("without_command = %v, want 1", out["without_command"])
	}
	found := false
	for _, e := range out["failed_executions"].([]any) {
		if strings.Contains(e.(map[string]any)["command"].(string), "recorded no command") {
			found = true
		}
	}
	if !found {
		t.Error("the run with no command was not marked")
	}
}

// The command is echoed into an AI context window and a terminal scrollback,
// so a credential in it must not survive.
func TestJobAuditMasksCredentialsInCommands(t *testing.T) {
	cs := testSession(t, fakeBastionWithJobs(t))
	out := testsupport.CallToolJSON(t, cs, "jms_job_audit", map[string]any{"only_failed": true})
	for _, e := range out["failed_executions"].([]any) {
		cmd := e.(map[string]any)["command"].(string)
		if strings.Contains(cmd, "abcdef123456") {
			t.Errorf("the token survived in the command: %q", cmd)
		}
	}
}

// The counts describe the whole matching set, not the rows that survive the
// limit: reporting "failed: 1" when two runs failed because the caller asked
// for one row is a lie about the fleet.
func TestJobAuditCountsTheWholeMatchNotTheCappedList(t *testing.T) {
	cs := testSession(t, fakeBastionWithJobs(t))
	out := testsupport.CallToolJSON(t, cs, "jms_job_audit",
		map[string]any{"limit": 1, "include_definitions": false})

	if out["count"] != float64(1) {
		t.Errorf("count = %v, want 1 (the list is capped)", out["count"])
	}
	if out["failed"] != float64(2) {
		t.Errorf("failed = %v, want 2 (the real number, not the capped one)", out["failed"])
	}
	if out["succeeded"] != float64(1) {
		t.Errorf("succeeded = %v, want 1", out["succeeded"])
	}
	// The failed list is what the caller came for, so it stays complete.
	if n := len(out["failed_executions"].([]any)); n != 2 {
		t.Errorf("failed_executions = %d entries, want 2 (not capped by limit)", n)
	}
}
