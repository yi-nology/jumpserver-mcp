package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yi-nology/jumpserver-kit/pkg/testsupport"
)

// fakeBastionWithTransfers serves the file transfer trail.
//
// ⚠️ The rows are the BYTES the real bastion answered with (read off
// http://192.0.2.41/api/v1/audits/ftp-logs/ on 2026/11/01), not a shape
// invented here. Three details of that shape are load-bearing and easy to
// get wrong:
//
//   - `asset` is a FLAT "name(address)" string -- this endpoint carries no
//     asset id and no nested asset object;
//   - `operate` is a LABELLED OBJECT {"value":"upload","label":"Upload"},
//     not the plain string the login log uses;
//   - the time field is `date_start`, where the login log says `datetime`.
//
// A test written against a hand-made shape would pass while the tool read
// nulls off the real endpoint.
func fakeBastionWithTransfers(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "a1", "name": "node-071", "address": "192.0.2.71",
					"platform": map[string]any{"id": 1, "name": "Linux"},
				},
				// ⚠️ A SECOND asset, with a transfer of its own. Without it
				// the fixture holds one machine only, and a filter that does
				// not filter answers exactly like one that does — which is
				// how "the asset filter was quietly removed" passes green.
				{
					"id": "a2", "name": "dnsbox1", "address": "192.0.2.2",
					"platform": map[string]any{"id": 1, "name": "Linux"},
				},
				{
					// ⚠️ A SECOND asset with the SAME NAME and a different
					// address. Without it the fixture cannot tell "matched
					// on the identity" from "matched on the label".
					"id": "a3", "name": "node-071", "address": "192.0.2.99",
					"platform": map[string]any{"id": 1, "name": "Linux"},
				},
			})
		case "/api/v1/audits/ftp-logs/":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":          "fd05340c-ef34-468a-80b1-d686be1c7738",
					"user":        "Administrator(admin)",
					"remote_addr": "192.0.2.152",
					"asset":       "node-071(192.0.2.71)",
					"account":     "opsuser(opsuser)",
					"operate":     map[string]any{"value": "upload", "label": "Upload"},
					"filename":    "/tmp/tmp/jms-probe.txt",
					"date_start":  "2026/10/28 00:05:33 +0800",
					"is_success":  false,
					"has_file":    false,
					"session":     "bb1f57a7-f9fe-40df-846b-ca435de2c3d8",
				},
				{
					"id":          "aa11bb22-cc33-dd44-ee55-ff6677889900",
					"user":        "Ops User(opsuser)",
					"remote_addr": "192.0.2.152",
					"asset":       "node-071(192.0.2.71)",
					"account":     "opsuser(opsuser)",
					"operate":     map[string]any{"value": "download", "label": "Download"},
					"filename":    "/etc/shadow",
					"date_start":  "2026/10/29 11:20:00 +0800",
					"is_success":  true,
					"has_file":    true,
					"session":     "cc22dd33-ee44-ff55-6677-889900aabbcc",
				},
				{
					// The OTHER machine's transfer: it must not show up in
					// an answer about node-071.
					"id":          "bb22cc33-dd44-ee55-ff66-778899aabb00",
					"user":        "Ops User(opsuser)",
					"remote_addr": "192.0.2.152",
					"asset":       "dnsbox1(192.0.2.2)",
					"account":     "root(root)",
					"operate":     map[string]any{"value": "upload", "label": "Upload"},
					"filename":    "/etc/dnsbox1.yaml",
					"date_start":  "2026/10/30 08:00:00 +0800",
					"is_success":  true,
					"has_file":    true,
					"session":     "dd44ee55-ff66-7788-99aa-bbccddeeff00",
				},
				{
					// A transfer by ANOTHER user on the SAME machine: the
					// user filter has to be able to tell them apart.
					"id":          "cc33dd44-ee55-ff66-7788-99aabbccddee",
					"user":        "运维组(ops-team)",
					"remote_addr": "192.0.2.201",
					"asset":       "node-071(192.0.2.71)",
					"account":     "opsuser(opsuser)",
					"operate":     map[string]any{"value": "download", "label": "Download"},
					"filename":    "/var/log/backup.tar.gz",
					"date_start":  "2026/10/25 12:00:00 +0800",
					"is_success":  true,
					"has_file":    true,
					"session":     "ee55ff66-7788-99aa-bbcc-ddeeff00112",
				},
				{
					// A directory operation: the trail carries more than
					// upload and download.
					"id":          "dd44ee55-ff66-7788-99aa-bbccddeeff2233",
					"user":        "Administrator(admin)",
					"remote_addr": "192.0.2.152",
					"asset":       "node-071(192.0.2.71)",
					"account":     "opsuser(opsuser)",
					"operate":     map[string]any{"value": "mkdir", "label": "Mkdir"},
					"filename":    "/opt/deploy",
					"date_start":  "2026/10/26 09:30:00 +0800",
					"is_success":  true,
					"has_file":    false,
					"session":     "ff667788-99aa-bbcc-ddee-ff0011223344",
				},
				{
					// ⚠️ A DIFFERENT machine that carries the SAME NAME.
					// The trail's asset is a SNAPSHOT of the name at the
					// time of the transfer, so matching on the name alone
					// would attribute this machine's transfers to the one
					// called node-071 today. The ADDRESS is the identity.
					"id":          "ee66ff77-8899-aabb-ccdd-eeff00112233",
					"user":        "Administrator(admin)",
					"remote_addr": "192.0.2.152",
					"asset":       "node-071(192.0.2.99)",
					"account":     "opsuser(opsuser)",
					"operate":     map[string]any{"value": "upload", "label": "Upload"},
					"filename":    "/tmp/other-box.txt",
					"date_start":  "2026/10/24 08:00:00 +0800",
					"is_success":  true,
					"has_file":    true,
					"session":     "11223344-5566-7788-99aa-bbccddeeff00",
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// The trail must name the file, the direction, the machine and the outcome.
//
// Every field here comes from a place where reading the wrong one yields a
// silent null: the asset is a flat label, `operate` is a labelled object,
// and the time is `date_start`. A tool that got any of them wrong would
// still return rows -- just rows with holes in them.
func TestFileTransfersNamesTheFileAndTheDirection(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))
	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers", map[string]any{})

	items := out["items"].([]any)
	if len(items) != 6 {
		t.Fatalf("items = %v, want six rows", items)
	}
	// Newest first: the limit must keep the newest rows, not whichever ones
	// the endpoint happened to return. The newest transfer here is the
	// 10/30 one on the OTHER machine, so the order is observable.
	first := items[0].(map[string]any)
	if first["filename"] != "/etc/dnsbox1.yaml" {
		t.Errorf("filename = %v, want /etc/dnsbox1.yaml (the newest transfer)", first["filename"])
	}
	if first["date_start"] != "2026/10/30 08:00:00 +0800" {
		t.Errorf("date_start = %v, want 2026/10/30 08:00:00 +0800", first["date_start"])
	}

	second := items[1].(map[string]any)
	if second["filename"] != "/etc/shadow" {
		t.Errorf("filename = %v, want /etc/shadow", second["filename"])
	}
	if second["operate"] != "download" {
		t.Errorf("operate = %v, want download", second["operate"])
	}
	if second["is_success"] != true {
		t.Errorf("is_success = %v, want true", second["is_success"])
	}
	if second["asset"] != "node-071" {
		t.Errorf("asset = %v, want node-071", second["asset"])
	}
	if second["address"] != "192.0.2.71" {
		t.Errorf("address = %v, want 192.0.2.71", second["address"])
	}
	if second["date_start"] != "2026/10/29 11:20:00 +0800" {
		t.Errorf("date_start = %v, want 2026/10/29 11:20:00 +0800", second["date_start"])
	}
	if second["session"] != "cc22dd33-ee44-ff55-6677-889900aabbcc" {
		t.Errorf("session = %v, want the session id", second["session"])
	}

	third := items[2].(map[string]any)
	if third["filename"] != "/tmp/tmp/jms-probe.txt" {
		t.Errorf("filename = %v, want /tmp/tmp/jms-probe.txt", third["filename"])
	}
	if third["operate"] != "upload" {
		t.Errorf("operate = %v, want upload", third["operate"])
	}
	if third["is_success"] != false {
		t.Errorf("is_success = %v, want false", third["is_success"])
	}
	if third["user"] != "Administrator(admin)" {
		t.Errorf("user = %v, want Administrator(admin)", third["user"])
	}

	fourth := items[3].(map[string]any)
	if fourth["filename"] != "/opt/deploy" {
		t.Errorf("filename = %v, want /opt/deploy", fourth["filename"])
	}
	if fourth["operate"] != "mkdir" {
		t.Errorf("operate = %v, want mkdir", fourth["operate"])
	}
}

// Filtering by asset must actually filter. The endpoint's asset is a flat
// "name(address)" string, so the filter has to match on either half --
// otherwise the tool answers "no transfers" for a machine that has them.
//
// ⚠️ The fixture holds a transfer on ANOTHER machine too, and this asserts
// it is gone from the answer. With a single machine in the fixture, a filter
// that does not filter at all answers exactly like one that does -- which is
// how "the asset filter was quietly removed" passes green.
func TestFileTransfersFiltersByAsset(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))

	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"asset": "192.0.2.71"})
	items := out["items"].([]any)
	if len(items) != 4 {
		t.Fatalf("transfers for the asset = %v, want 4", items)
	}
	for _, it := range items {
		row := it.(map[string]any)
		if row["address"] != "192.0.2.71" {
			t.Errorf("a transfer of another machine leaked into the answer: %v", row)
		}
	}

	// The NAME half must work as well as the address half.
	out = testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"asset": "node-071"})
	if n := len(out["items"].([]any)); n != 4 {
		t.Errorf("transfers for the asset by name = %d, want 4", n)
	}

	// The other machine's own transfers must be reachable by ITS address.
	out = testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"asset": "192.0.2.2"})
	items = out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("transfers for the second asset = %v, want 1", items)
	}
	if items[0].(map[string]any)["filename"] != "/etc/dnsbox1.yaml" {
		t.Errorf("filename = %v, want /etc/dnsbox1.yaml", items[0])
	}
}

// Two machines can carry the SAME NAME, and the trail records the name as
// it was AT THE TIME of the transfer.
//
// ⇒ matching on the name alone would attribute one machine's history to
// whatever carries that name today. The ADDRESS is the identity, so an
// answer about one address must not contain the other's transfers.
//
// ⚠️ Without a same-named asset in the fixture, "matched on the address"
// and "matched on the name" answer identically, and this cannot fail.
func TestFileTransfersTellsSameNamedMachinesApart(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))

	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"asset": "192.0.2.71"})
	items := out["items"].([]any)
	for _, it := range items {
		row := it.(map[string]any)
		if row["filename"] == "/tmp/other-box.txt" {
			t.Errorf("the transfer of the same-named machine at 192.0.2.99 "+
				"leaked into the answer for 192.0.2.71: %v", row)
		}
	}

	// And the other one is still reachable under ITS address.
	out = testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"asset": "192.0.2.99"})
	items = out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("transfers for the same-named machine = %v, want 1", items)
	}
	if items[0].(map[string]any)["filename"] != "/tmp/other-box.txt" {
		t.Errorf("filename = %v, want /tmp/other-box.txt", items[0])
	}
}

// An asset that has been DELETED from the bastion still has a transfer trail,
// and "what was put on this machine" is exactly the question that gets asked
// about a machine that is gone.
//
// The tool must answer it instead of failing with "no such asset" -- the
// same reason jms_asset_evidence falls back to the label the sessions carry.
//
// ⚠️ But the fallback has to SAY that it fell back: a reference that names
// nothing (a typo) must not read as "this machine never had a file
// transfer". That is a forensic false negative, and the note is what tells
// the two apart.
func TestFileTransfersSurvivesADeletedAsset(t *testing.T) {
	// The bastion knows no such asset, and no session names it either --
	// so the trail is searched by the label the records carry.
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "a1", "name": "other-box", "address": "192.0.2.2",
				"platform": map[string]any{"id": 1, "name": "Linux"},
			}})
		case "/api/v1/terminal/sessions/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/v1/audits/ftp-logs/":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "fd05340c", "user": "Administrator(admin)",
					"remote_addr": "192.0.2.152", "asset": "node-071(192.0.2.71)",
					"account":    "opsuser(opsuser)",
					"operate":    map[string]any{"value": "upload", "label": "Upload"},
					"filename":   "/tmp/tmp/jms-probe.txt",
					"date_start": "2026/10/28 00:05:33 +0800",
					"is_success": false, "has_file": false,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"asset": "192.0.2.71"})
	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("transfers for the deleted asset = %v, want one row", items)
	}
	if items[0].(map[string]any)["filename"] != "/tmp/tmp/jms-probe.txt" {
		t.Errorf("filename = %v, want the transfer of the deleted asset", items[0])
	}
	// The answer must say it fell back to the label -- otherwise a reader
	// cannot tell "the machine had no transfers" from "the machine is gone".
	if note, _ := out["note"].(string); !strings.Contains(note, "not in the bastion's list") {
		t.Errorf("the answer does not say it fell back to the record's own label: %v", out["note"])
	}
}

// A reference that names NOTHING is not the same as a deleted asset.
//
// A typo must not read as "this machine never had a file transfer" -- that
// is a forensic false negative, and the agent that reads it would report
// "nothing was ever transferred" about a machine that was never even
// looked up. The note has to say the asset is unknown.
func TestFileTransfersSaysSoWhenTheAssetNamesNothing(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))

	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"asset": "no-such-machine"})
	if n := len(out["items"].([]any)); n != 0 {
		t.Errorf("transfers for an unknown asset = %d, want 0", n)
	}
	note, _ := out["note"].(string)
	if !strings.Contains(note, "not in the bastion's list") {
		t.Errorf("the answer does not say the asset is unknown: %v", out["note"])
	}
}

// The fallback path must tell same-named machines apart too.
//
// When the asset is not in the bastion's list, the label the record carries
// is all there is -- and that label carries BOTH a name and an address. The
// name is a snapshot and the address is the identity, so the fallback has
// to compare the ADDRESS, or a machine that was renamed (or a name that was
// reused) would collect another machine's history.
//
// ⚠️ Without a same-named machine in the fixture, "compare the address"
// and "compare the name" answer identically, and this cannot fail.
func TestFileTransfersFallbackTellsSameNamedMachinesApart(t *testing.T) {
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/assets/assets/":
			// The bastion holds neither of the two same-named machines.
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "a1", "name": "other-box", "address": "192.0.2.2",
				"platform": map[string]any{"id": 1, "name": "Linux"},
			}})
		case "/api/v1/terminal/sessions/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/v1/audits/ftp-logs/":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "e1", "user": "Administrator(admin)",
					"remote_addr": "192.0.2.152", "asset": "node-071(192.0.2.71)",
					"account":    "opsuser(opsuser)",
					"operate":    map[string]any{"value": "upload", "label": "Upload"},
					"filename":   "/tmp/mine.txt",
					"date_start": "2026/10/28 00:05:33 +0800",
					"is_success": true, "has_file": true,
				},
				{
					"id": "e2", "user": "Administrator(admin)",
					"remote_addr": "192.0.2.152", "asset": "node-071(192.0.2.99)",
					"account":    "opsuser(opsuser)",
					"operate":    map[string]any{"value": "upload", "label": "Upload"},
					"filename":   "/tmp/not-mine.txt",
					"date_start": "2026/10/27 00:05:33 +0800",
					"is_success": true, "has_file": true,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"asset": "192.0.2.71"})
	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("transfers for the deleted asset = %v, want one row", items)
	}
	if items[0].(map[string]any)["filename"] != "/tmp/mine.txt" {
		t.Errorf("filename = %v, want /tmp/mine.txt", items[0])
	}
}

// The date filter must accept the ISO form too -- and it must NOT be
// compared against the trail's own format by accident.
//
// ⚠️ The trail's stamps are "2026/10/28 00:05:33 +0800" and the caller's
// date may be "2026-10-20". Compared as raw strings, "/" sorts after "-",
// so EVERY row passes and the answer includes transfers from BEFORE the
// requested date -- which reads as "there were transfers after 10/20" when
// the truth is "these are all the transfers, including old ones".
//
// The date is normalised to the trail's own shape before the comparison.
func TestFileTransfersAcceptsTheISODate(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))

	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"since": "2026-10-29"})
	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("transfers since 2026-10-29 = %v, want two rows", items)
	}
	for _, it := range items {
		stamp, _ := it.(map[string]any)["date_start"].(string)
		if len(stamp) < 10 || stamp[:10] < "2026/10/29" {
			t.Errorf("a transfer from %s is before the requested date", stamp)
		}
	}
}

// A date that cannot be read must be REFUSED, not answered with an empty
// list.
//
// "No transfers" and "your date is nonsense" are different claims, and
// answering the first one is a forensic false negative: the caller would
// report "nothing was transferred" about a machine that was never queried.
func TestFileTransfersRefusesADateItCannotRead(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_file_transfers",
		Arguments: map[string]any{"since": "garbage"},
	})
	if err != nil {
		t.Fatalf("call jms_file_transfers: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an unreadable date was accepted: %s", testsupport.ToolText(t, res))
	}
	txt := testsupport.ToolText(t, res)
	if !strings.Contains(txt, "since") {
		t.Errorf("the refusal does not say what is wrong: %s", txt)
	}
}

// A machine that has been DELETED can only be asked for BY NAME.
//
// Its address is not in the bastion's list any more, so the label the
// record carries is the only handle left -- and a fallback that compared
// the address only would answer "no transfers" for the one question a
// deleted machine is still asked about.
//
// ⚠️ That is a forensic false negative about a machine that is GONE: the
// reader has no way to tell "nothing was transferred to it" from "the
// tool could not find it".
func TestFileTransfersFindsADeletedAssetByItsName(t *testing.T) {
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "a1", "name": "other-box", "address": "192.0.2.2",
				"platform": map[string]any{"id": 1, "name": "Linux"},
			}})
		case "/api/v1/terminal/sessions/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/v1/audits/ftp-logs/":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "f1", "user": "Administrator(admin)",
					"remote_addr": "192.0.2.152", "asset": "node-071(192.0.2.71)",
					"account":    "opsuser(opsuser)",
					"operate":    map[string]any{"value": "upload", "label": "Upload"},
					"filename":   "/tmp/mine.txt",
					"date_start": "2026/10/28 00:05:33 +0800",
					"is_success": true, "has_file": true,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"asset": "node-071"})
	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("transfers for the deleted asset by name = %v, want one row", items)
	}
	if items[0].(map[string]any)["filename"] != "/tmp/mine.txt" {
		t.Errorf("filename = %v, want /tmp/mine.txt", items[0])
	}
}

// A trail the bastion holds MORE of than was read must say so.
//
// The bastion caps a page, and a deployment may configure that cap lower
// than the page this client asks for. The walk then reads a window and
// stops, and the answer looks like a complete list.
//
// ⚠️ "There were no transfers" is what a caller reports from a window
// that missed the transfers -- so the gap has to be in the answer, not in
// the reader's head.
func TestFileTransfersSaysSoWhenTheTrailWasNotReadWhole(t *testing.T) {
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/v1/audits/ftp-logs/":
			// The server says it holds 500 rows but hands over two.
			w.Write([]byte(`{"count":500,"next":null,"previous":null,"results":[` +
				`{"id":"t1","user":"a(a)","remote_addr":"1.1.1.1","asset":"box(10.0.0.1)",` +
				`"account":"r(r)","operate":{"value":"upload","label":"Upload"},` +
				`"filename":"/tmp/a","date_start":"2026/10/30 08:00:00 +0800",` +
				`"is_success":true,"has_file":true,"session":"s1"},` +
				`{"id":"t2","user":"a(a)","remote_addr":"1.1.1.1","asset":"box(10.0.0.1)",` +
				`"account":"r(r)","operate":{"value":"upload","label":"Upload"},` +
				`"filename":"/tmp/b","date_start":"2026/10/29 08:00:00 +0800",` +
				`"is_success":true,"has_file":true,"session":"s2"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers", map[string]any{})
	if n := len(out["items"].([]any)); n != 2 {
		t.Fatalf("items = %v, want the two rows that were read", out["items"])
	}
	note, _ := out["note"].(string)
	if !strings.Contains(note, "may exist") {
		t.Errorf("the answer does not say the trail was not read whole: %v", out["note"])
	}
}

// A MECHANISM failure is not "no such asset" and must not be swallowed.
//
// When the asset listing cannot be read (the bastion is unreachable, the
// listing is stale), the tool has to say so -- reporting it as "this
// machine never had a file transfer" would turn an outage into a forensic
// false negative, which is the most expensive kind of wrong answer.
//
// ⚠️ This is the difference the upstream resolver makes explicit: "no such
// asset" is a problem with the REQUEST, while "the listing could not be
// refreshed" is a problem with the MECHANISM.
func TestFileTransfersReportsAMechanismFailureInsteadOfEmpty(t *testing.T) {
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/assets/assets/":
			// The listing cannot be read: the bastion answers with an
			// error instead of the asset list.
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"detail": "boom"})
		case "/api/v1/audits/ftp-logs/":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "fd05340c", "user": "Administrator(admin)",
					"remote_addr": "192.0.2.152", "asset": "node-071(192.0.2.71)",
					"account":    "opsuser(opsuser)",
					"operate":    map[string]any{"value": "upload", "label": "Upload"},
					"filename":   "/tmp/tmp/jms-probe.txt",
					"date_start": "2026/10/28 00:05:33 +0800",
					"is_success": false, "has_file": false,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_file_transfers",
		Arguments: map[string]any{"asset": "192.0.2.71"},
	})
	if err != nil {
		t.Fatalf("call jms_file_transfers: %v", err)
	}
	if !res.IsError {
		t.Fatalf("a mechanism failure was reported as an empty answer: %s",
			testsupport.ToolText(t, res))
	}
}

// The limit must actually cut, and it must keep the NEWEST rows.
//
// Without this case a limit that does nothing passes green: the fixture
// holds three transfers and every assertion above reads them all.
func TestFileTransfersKeepsTheNewestRowsUnderTheLimit(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))
	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"limit": 1})

	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v, want one row", items)
	}
	if items[0].(map[string]any)["filename"] != "/etc/dnsbox1.yaml" {
		t.Errorf("the kept row = %v, want the newest one", items[0])
	}
	// The count fields must say the list was cut: count is what came back,
	// total is how many matched.
	if out["count"] != float64(1) {
		t.Errorf("count = %v, want 1", out["count"])
	}
	if out["total"] != float64(6) {
		t.Errorf("total = %v, want 6", out["total"])
	}
	if out["truncated"] != true {
		t.Errorf("truncated = %v, want true", out["truncated"])
	}
	if out["note"] == nil {
		t.Error("a truncated list says nothing about what it left out")
	}
}

// The direction filter must actually filter.
//
// ⚠️ It is applied HERE, not on the server: the endpoint's own filters are
// not usable (its user filter is iexact against the whole display string,
// its date filter parses a strict format and silently drops what it cannot
// parse, and a parameter it does not declare is ignored without a word) ⇒
// a server-side filter that does not apply produces an answer that LOOKS
// authoritative and is wrong. The trail is read whole and narrowed here.
func TestFileTransfersFiltersByDirection(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))

	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"direction": "download"})
	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("downloads = %v, want two rows", items)
	}
	for _, it := range items {
		if got := it.(map[string]any)["operate"]; got != "download" {
			t.Errorf("operate = %v, want download", got)
		}
	}

	// A direction that is not "upload or download" is a real one: the
	// bastion records directory operations too, and refusing them would
	// refuse a direction the trail holds.
	out = testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"direction": "mkdir"})
	items = out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("mkdirs = %v, want one row", items)
	}
	if items[0].(map[string]any)["filename"] != "/opt/deploy" {
		t.Errorf("filename = %v, want /opt/deploy", items[0])
	}
}

// The user filter must match either half of the display string.
//
// The endpoint writes the user as "name(username)" and its own filter is
// iexact against that WHOLE string -- so `user=opsuser` matches nothing
// there. Here the filter matches either half, because both halves are
// things a person types.
func TestFileTransfersFiltersByUser(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))

	// The USERNAME half.
	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"user": "opsuser"})
	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("transfers by opsuser = %v, want two rows", items)
	}
	for _, it := range items {
		if got := it.(map[string]any)["user"]; got != "Ops User(opsuser)" {
			t.Errorf("user = %v, want Ops User(opsuser)", got)
		}
	}

	// The DISPLAY NAME half.
	out = testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"user": "运维组"})
	items = out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("transfers by the ops team = %v, want one row", items)
	}
	if items[0].(map[string]any)["filename"] != "/var/log/backup.tar.gz" {
		t.Errorf("filename = %v, want /var/log/backup.tar.gz", items[0])
	}

	// A user nobody is: the trail says so, and does not answer "everything".
	out = testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"user": "nobody-here"})
	if n := len(out["items"].([]any)); n != 0 {
		t.Errorf("transfers by an unknown user = %d, want 0", n)
	}
}

// The date filter must cut the trail at the caller's date.
//
// ⚠️ It is applied HERE: the endpoint's own date filter reads `date_from`
// and parses it with a strict DateTimeField, and a value it cannot parse
// is swallowed by the backend -- the filter then simply does not apply,
// and the answer looks complete.
func TestFileTransfersFiltersByDate(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))

	out := testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"since": "2026/10/29"})
	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("transfers since 2026/10/29 = %v, want two rows", items)
	}
	// ⚠️ The date part is compared, NOT the whole stamp: the stamp is
	// "2026/10/29 11:20:00 +0800" and the requested date is "2026/10/29",
	// so comparing whole strings would put the 11:20 row "after" it (a
	// space sorts before a digit) and the assertion would hold for every
	// row -- an assertion that cannot fail proves nothing.
	for _, it := range items {
		stamp, _ := it.(map[string]any)["date_start"].(string)
		if len(stamp) < 10 {
			t.Fatalf("the stamp %q is too short to judge", stamp)
		}
		if stamp[:10] < "2026/10/29" {
			t.Errorf("a transfer from %s is before the requested date", stamp)
		}
	}
	// And the rows that were cut must be gone: the one on 2026/10/25 and
	// the one on 2026/10/26 are both before the requested date.
	for _, it := range items {
		if f, _ := it.(map[string]any)["filename"].(string); f == "/var/log/backup.tar.gz" || f == "/opt/deploy" {
			t.Errorf("a transfer from before the requested date survived: %v", it)
		}
	}

	// A date in the future cuts everything.
	out = testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"since": "2027/01/01"})
	if n := len(out["items"].([]any)); n != 0 {
		t.Errorf("transfers since a future date = %d, want 0", n)
	}

	// A date in the past keeps everything.
	out = testsupport.CallToolJSON(t, cs, "jms_file_transfers",
		map[string]any{"since": "2020/01/01"})
	if n := len(out["items"].([]any)); n != 6 {
		t.Errorf("transfers since a distant past = %d, want 6", n)
	}
}

// The filters must NOT be handed to the server.
//
// ⚠️ This is the load-bearing assertion of the whole design. The endpoint's
// own filters cannot be used: its user filter is iexact against the whole
// display string ("Ops User(opsuser)"), so `user=opsuser` matches nothing
// there; its date filter reads `date_from` and parses it strictly, and a
// value it cannot parse is swallowed by the backend; and a parameter it does
// not declare at all (`username`, `date_start__gte`) is IGNORED WITHOUT A
// WORD by django-filter.
//
// ⇒ A server-side filter that does not apply returns "here are the
// transfers" with the filter silently dropped -- the worst kind of wrong
// answer for a forensic tool: it looks complete.
//
// So the tool sends no filter at all, and narrows the trail itself.
func TestFileTransfersSendsNoFilterToTheServer(t *testing.T) {
	var seenQuery string
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "a1", "name": "node-071", "address": "192.0.2.71",
				"platform": map[string]any{"id": 1, "name": "Linux"},
			}})
		case "/api/v1/audits/ftp-logs/":
			seenQuery = r.URL.RawQuery
			json.NewEncoder(w).Encode([]map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	testsupport.CallToolJSON(t, cs, "jms_file_transfers", map[string]any{
		"user":      "opsuser",
		"direction": "upload",
		"since":     "2026/10/20",
		"asset":     "192.0.2.71",
	})

	for _, forbidden := range []string{"user=", "username=", "operate=", "date_from=", "date_start__gte="} {
		if strings.Contains(seenQuery, forbidden) {
			t.Errorf("the filter %q was sent to the server (query = %q); "+
				"a filter the endpoint does not honour is dropped in silence", forbidden, seenQuery)
		}
	}

	// The ordering parameter is `order` -- the bastion's ORDERING_PARAM.
	// ⚠️ An `ordering=` parameter is not declared by the endpoint, is
	// IGNORED without a word, and the rows come back in the view's default
	// order. That default happens to be the same order today, so the
	// mistake is invisible in the ANSWER -- which is exactly why it has to
	// be pinned here: the day the view's default changes, the window this
	// client reads would no longer be the newest one.
	if !strings.Contains(seenQuery, "order=-date_start") {
		t.Errorf("the ordering was not sent as the bastion's ORDERING_PARAM: query = %q", seenQuery)
	}
	if strings.Contains(seenQuery, "ordering=") {
		t.Errorf("the ordering was sent as `ordering`, which the endpoint does not declare: query = %q", seenQuery)
	}
}

// A direction that is neither upload nor download is a mistake worth
// reporting -- silently returning everything would answer a question the
// caller did not ask.
//
// ⚠️ The failure is read from the raw call, not through CallToolJSON: that
// helper treats an error result as a test failure, and here the error IS the
// thing under test.
func TestFileTransfersRejectsAnUnknownDirection(t *testing.T) {
	cs := testSession(t, fakeBastionWithTransfers(t))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_file_transfers",
		Arguments: map[string]any{"direction": "sideways"},
	})
	if err != nil {
		t.Fatalf("call jms_file_transfers: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an unknown direction was accepted: %s", testsupport.ToolText(t, res))
	}
	if txt := testsupport.ToolText(t, res); !strings.Contains(txt, "direction") {
		t.Errorf("the refusal does not say what is wrong: %s", txt)
	}
}
