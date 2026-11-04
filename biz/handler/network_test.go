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

// fakeBastionWithNetwork serves the network topology.
//
// ⚠️ The rows are the BYTES the real bastion answered with (read off
// http://192.0.2.41 on 2026/11/01), not a shape invented here. What makes
// them worth copying byte for byte:
//
//   - a gateway's `zone` is a NESTED object {"id","name"} -- and it is
//     null on most of them (a machine that belongs to no zone);
//   - the zone's own record carries `gateways` as an array of
//     {"id","name"} objects, NOT names;
//   - `connectivity` is a labelled object;
//   - `nodes_display` is an array of tree paths.
//
// ⚠️ The fixture holds a gateway whose connectivity is "err" on purpose:
// the "some hops are down" answer has to be distinguishable from the
// "all hops are down" one.
func fakeBastionWithNetwork(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/assets/assets/":
			// The asset list: one asset WITH a zone, one without.
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":   "12a327ee-7a0f-49d8-8e2b-f21641c3a149",
					"name": "app-server-01", "address": "198.51.100.3",
					"zone": map[string]any{
						"id":   "04b5ef03-4e76-4f5d-a387-35f3a1116f0a",
						"name": "home",
					},
					"platform":      map[string]any{"id": 1, "name": "Linux", "type": "linux"},
					"nodes_display": []string{"/DEFAULT/home"},
					"connectivity":  map[string]any{"value": "ok", "label": "OK"},
				},
				{
					"id":   "4fd69c40-e87a-4dcd-bc6e-43f5986d26a4",
					"name": "dnsbox1", "address": "192.0.2.2",
					"zone":          nil,
					"platform":      map[string]any{"id": 1, "name": "Linux", "type": "linux"},
					"nodes_display": []string{"/DEFAULT/studio/x86"},
					"connectivity":  map[string]any{"value": "ok", "label": "OK"},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/api/v1/assets/assets/"):
			// The asset DETAIL (jms_net_path reads it for the zone).
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/assets/"), "/")
			switch id {
			case "12a327ee-7a0f-49d8-8e2b-f21641c3a149":
				json.NewEncoder(w).Encode(map[string]any{
					"id":   "12a327ee-7a0f-49d8-8e2b-f21641c3a149",
					"name": "app-server-01", "address": "198.51.100.3",
					"zone": map[string]any{
						"id":   "04b5ef03-4e76-4f5d-a387-35f3a1116f0a",
						"name": "home",
					},
					"connectivity": map[string]any{"value": "ok", "label": "OK"},
				})
			case "4fd69c40-e87a-4dcd-bc6e-43f5986d26a4":
				json.NewEncoder(w).Encode(map[string]any{
					"id":   "4fd69c40-e87a-4dcd-bc6e-43f5986d26a4",
					"name": "dnsbox1", "address": "192.0.2.2",
					"zone":         nil,
					"connectivity": map[string]any{"value": "ok", "label": "OK"},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		case r.URL.Path == "/api/v1/assets/zones/":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"gateways": []map[string]any{{
						"id":   "32cdac22-024a-42e5-a136-ad220ed8e3d8",
						"name": "edge-gw-01",
					}},
					"assets_amount": 1,
					"id":            "04b5ef03-4e76-4f5d-a387-35f3a1116f0a",
					"labels":        []any{}, "cidrs": []string{},
					"name":         "home",
					"org_id":       "00000000-0000-0000-0000-000000000002",
					"org_name":     "DEFAULT",
					"auto_assign":  false,
					"date_created": "2025/05/18 01:55:00 +0800",
					"comment":      "",
				},
				{
					"gateways": []map[string]any{{
						"id":   "ba9f6d0d-3688-4a01-8355-1436fa154789",
						"name": "jumpserver",
					}},
					"assets_amount": 4,
					"id":            "edaeca59-906e-4018-9c6f-ee891a3b759e",
					"labels":        []any{}, "cidrs": []string{},
					"name":         "studio",
					"org_id":       "00000000-0000-0000-0000-000000000002",
					"org_name":     "DEFAULT",
					"auto_assign":  false,
					"date_created": "2025/05/18 01:54:29 +0800",
					"comment":      "",
				},
			})
		case r.URL.Path == "/api/v1/assets/gateways/":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":   "32cdac22-024a-42e5-a136-ad220ed8e3d8",
					"name": "edge-gw-01", "address": "198.51.100.10",
					"zone": map[string]any{
						"id":   "04b5ef03-4e76-4f5d-a387-35f3a1116f0a",
						"name": "home",
					},
					"platform":        map[string]any{"id": 11, "name": "Gateway", "type": "linux"},
					"nodes_display":   []string{"/DEFAULT/home"},
					"accounts_amount": 1,
					"connectivity":    map[string]any{"value": "ok", "label": "OK"},
					"date_verified":   "2026/10/27 07:54:35 +0800",
					"date_last_login": "2026/10/26 08:43:38 +0800",
					"comment":         "",
				},
				{
					// A gateway in the same zone that is DOWN.
					"id":   "dddd1111-2222-3333-4444-555566667777",
					"name": "edge-gw-01-fallback", "address": "198.51.100.11",
					"zone": map[string]any{
						"id":   "04b5ef03-4e76-4f5d-a387-35f3a1116f0a",
						"name": "home",
					},
					"platform":        map[string]any{"id": 11, "name": "Gateway", "type": "linux"},
					"nodes_display":   []string{"/DEFAULT/home"},
					"accounts_amount": 1,
					"connectivity":    map[string]any{"value": "err", "label": "Error"},
					"date_verified":   "2026/10/27 07:54:35 +0800",
					"date_last_login": nil,
					"comment":         "备跳",
				},
				{
					// A gateway in NO zone: an ordinary router that merely
					// carries the Gateway platform.
					"id":   "fc0717fd-21f8-4fd8-8a63-f6edac157a2f",
					"name": "edge-rt-01", "address": "198.51.100.250",
					"zone":            nil,
					"platform":        map[string]any{"id": 11, "name": "Gateway", "type": "linux"},
					"nodes_display":   []string{"/DEFAULT/home"},
					"accounts_amount": 1,
					"connectivity":    map[string]any{"value": "err", "label": "Error"},
					"date_verified":   "2026/10/27 07:54:42 +0800",
					"date_last_login": nil,
					"comment":         "home 段自建路由",
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// The zone list must name the zone and its gateways.
//
// ⚠️ The zone's `gateways` arrive as OBJECTS {"id","name"} -- reading
// them as plain names yields a list of empty maps.
func TestNetZonesListNamesTheZoneAndItsGateways(t *testing.T) {
	cs := testSession(t, fakeBastionWithNetwork(t))
	out := testsupport.CallToolJSON(t, cs, "jms_net_zones_list", map[string]any{})

	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %v, want two rows", items)
	}
	z := items[0].(map[string]any)
	if z["name"] != "home" {
		t.Errorf("name = %v, want home", z["name"])
	}
	if z["id"] != "04b5ef03-4e76-4f5d-a387-35f3a1116f0a" {
		t.Errorf("id = %v, want the zone id", z["id"])
	}
	gws := z["gateways"].([]any)
	if len(gws) != 1 {
		t.Fatalf("gateways = %v, want one", gws)
	}
	if gws[0] != "edge-gw-01" {
		t.Errorf("gateway = %v, want edge-gw-01 (the name, not the object)", gws[0])
	}
	if z["assets_amount"] != float64(1) {
		t.Errorf("assets_amount = %v, want 1", z["assets_amount"])
	}
	if z["auto_assign"] != false {
		t.Errorf("auto_assign = %v, want false", z["auto_assign"])
	}
}

// The gateway list must say which zone each gateway is in, and whether it
// is up.
//
// ⚠️ A gateway's `zone` is a NESTED object and is NULL on the ones that
// belong to no zone -- reading it as a string yields nothing for them.
func TestNetGatewaysListNamesTheZoneAndTheState(t *testing.T) {
	cs := testSession(t, fakeBastionWithNetwork(t))
	out := testsupport.CallToolJSON(t, cs, "jms_net_gateways_list", map[string]any{})

	items := out["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("items = %v, want three rows", items)
	}
	byName := map[string]map[string]any{}
	for _, it := range items {
		row := it.(map[string]any)
		byName[row["name"].(string)] = row
	}
	home := byName["edge-gw-01"]
	if home == nil {
		t.Fatalf("edge-gw-01 missing: %v", items)
	}
	if home["zone"] != "home" {
		t.Errorf("zone = %v, want home", home["zone"])
	}
	if home["connectivity"] != "ok" {
		t.Errorf("connectivity = %v, want ok", home["connectivity"])
	}
	if home["accounts_amount"] != float64(1) {
		t.Errorf("accounts_amount = %v, want 1", home["accounts_amount"])
	}
	// A gateway in no zone must say so -- NOT carry an empty zone that
	// reads like "in a zone called nothing".
	router := byName["edge-rt-01"]
	if router == nil {
		t.Fatalf("edge-rt-01 missing: %v", items)
	}
	if router["zone"] != nil {
		t.Errorf("zone = %v, want nil for a gateway in no zone", router["zone"])
	}
	// A gateway that is down must be visible as such.
	backup := byName["edge-gw-01-fallback"]
	if backup == nil {
		t.Fatalf("edge-gw-01-fallback missing: %v", items)
	}
	if backup["connectivity"] != "err" {
		t.Errorf("connectivity = %v, want err", backup["connectivity"])
	}
	// And the answer must say that some hops are down.
	note, _ := out["note"].(string)
	if !strings.Contains(note, "connectivity=err") {
		t.Errorf("the note does not mention the down gateway: %v", out["note"])
	}
}

// The zone filter must actually filter.
//
// ⚠️ The gateway's zone is an OBJECT, so the filter has to compare the
// zone's name AND id -- a filter that only knew the name would miss a
// caller who passed the id.
func TestNetGatewaysListFiltersByZone(t *testing.T) {
	cs := testSession(t, fakeBastionWithNetwork(t))

	out := testsupport.CallToolJSON(t, cs, "jms_net_gateways_list",
		map[string]any{"zone": "home"})
	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("gateways in the zone = %v, want two", items)
	}
	for _, it := range items {
		if it.(map[string]any)["zone"] != "home" {
			t.Errorf("a gateway of another zone leaked in: %v", it)
		}
	}

	// By the zone's ID.
	out = testsupport.CallToolJSON(t, cs, "jms_net_gateways_list",
		map[string]any{"zone": "04b5ef03-4e76-4f5d-a387-35f3a1116f0a"})
	if n := len(out["items"].([]any)); n != 2 {
		t.Errorf("gateways in the zone by id = %d, want 2", n)
	}
}

// A machine with no zone must be answered honestly.
//
// ⚠️ The zone field is null on nearly every machine, so this is the
// NORMAL case, not an edge case. Saying "no path" would read as "there is
// no way in"; inventing a gateway would be worse.
func TestNetPathSaysDirectWhenTheMachineIsInNoZone(t *testing.T) {
	cs := testSession(t, fakeBastionWithNetwork(t))
	out := testsupport.CallToolJSON(t, cs, "jms_net_path",
		map[string]any{"asset": "192.0.2.2"})

	if out["zone"] != nil {
		t.Errorf("zone = %v, want nil", out["zone"])
	}
	verdict, _ := out["verdict"].(string)
	if !strings.Contains(verdict, "directly") {
		t.Errorf("the verdict does not say the machine is reached directly: %v", out["verdict"])
	}
	steps := out["steps"].([]any)
	if len(steps) != 1 {
		t.Fatalf("steps = %v, want the machine itself and no hop", steps)
	}
	if steps[0].(map[string]any)["kind"] != "target" {
		t.Errorf("the only step = %v, want the target", steps[0])
	}
}

// A machine in a zone must be answered with the hops of that zone.
//
// ⚠️ The zone's gateways are the GATEWAY ASSETS whose zone is that one
// -- the zone record and the gateway list are two views of the same
// records, and the tool has to put them together.
func TestNetPathListsTheHopsOfTheZone(t *testing.T) {
	cs := testSession(t, fakeBastionWithNetwork(t))
	out := testsupport.CallToolJSON(t, cs, "jms_net_path",
		map[string]any{"asset": "198.51.100.3"})

	if out["zone"] != "home" {
		t.Errorf("zone = %v, want home", out["zone"])
	}
	gws := out["zone_gateways"].([]any)
	if len(gws) != 2 {
		t.Fatalf("zone_gateways = %v, want two", gws)
	}
	steps := out["steps"].([]any)
	// Two gateways plus the target.
	if len(steps) != 3 {
		t.Fatalf("steps = %v, want two hops and the target", steps)
	}
	if steps[0].(map[string]any)["kind"] != "gateway" {
		t.Errorf("the first step = %v, want a gateway", steps[0])
	}
	last := steps[len(steps)-1].(map[string]any)
	if last["kind"] != "target" {
		t.Errorf("the last step = %v, want the target", last)
	}
	// The down gateway must be visible AS the down one.
	downFound := false
	for _, it := range steps {
		s := it.(map[string]any)
		if s["name"] == "edge-gw-01-fallback" {
			downFound = true
			if s["connectivity"] != "err" {
				t.Errorf("the down hop's connectivity = %v, want err", s["connectivity"])
			}
			if note, _ := s["note"].(string); !strings.Contains(note, "DOWN") {
				t.Errorf("the down hop is not marked as down: %v", s["note"])
			}
		}
	}
	if !downFound {
		t.Errorf("the down gateway is missing from the path: %v", steps)
	}
	// And the verdict has to say that one hop is up and one is down --
	// that is a different situation from "all hops are down".
	verdict, _ := out["verdict"].(string)
	if !strings.Contains(verdict, "1 of its 2 gateways are up") {
		t.Errorf("the verdict does not distinguish one-down from all-down: %v", out["verdict"])
	}
	if out["zone_assets_amount"] != float64(1) {
		t.Errorf("zone_assets_amount = %v, want 1", out["zone_assets_amount"])
	}
}

// A gateway whose state is UNKNOWN must not be counted as up.
//
// ⚠️ The bastion reports ok / err, and a gateway it has never tested
// carries neither. Counting that as "up" would make the verdict say
// "reached through X" -- a claim the bastion has not made.
func TestNetPathDoesNotCountAnUntestedHopAsUp(t *testing.T) {
	cs := testSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/assets/assets/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "12a327ee", "name": "app-server-01", "address": "198.51.100.3",
				"zone": map[string]any{
					"id":   "04b5ef03-4e76-4f5d-a387-35f3a1116f0a",
					"name": "home",
				},
				"connectivity": map[string]any{"value": "ok", "label": "OK"},
			}})
		case strings.HasPrefix(r.URL.Path, "/api/v1/assets/assets/"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": "12a327ee", "name": "app-server-01", "address": "198.51.100.3",
				"zone": map[string]any{
					"id":   "04b5ef03-4e76-4f5d-a387-35f3a1116f0a",
					"name": "home",
				},
				"connectivity": map[string]any{"value": "ok", "label": "OK"},
			})
		case r.URL.Path == "/api/v1/assets/zones/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "04b5ef03-4e76-4f5d-a387-35f3a1116f0a", "name": "home",
				"gateways":      []map[string]any{},
				"assets_amount": 1, "cidrs": []string{}, "auto_assign": false,
			}})
		case r.URL.Path == "/api/v1/assets/gateways/":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": "32cdac22", "name": "edge-gw-01", "address": "198.51.100.10",
				"zone": map[string]any{
					"id":   "04b5ef03-4e76-4f5d-a387-35f3a1116f0a",
					"name": "home",
				},
				// ⚠️ No connectivity verdict at all.
				"accounts_amount": 1,
			}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	out := testsupport.CallToolJSON(t, cs, "jms_net_path",
		map[string]any{"asset": "198.51.100.3"})
	verdict, _ := out["verdict"].(string)
	if strings.Contains(verdict, "are up") {
		t.Errorf("an untested hop was counted as up: %v", verdict)
	}
	if !strings.Contains(verdict, "known to be up") {
		t.Errorf("the verdict does not say the hop's state is unknown: %v", verdict)
	}
}

// A machine that is not on the bastion must be refused.
//
// ⚠️ The asset resolver is what decides this, and a near miss is
// reported as a near miss (with the candidates) rather than attached to
// the wrong machine.
func TestNetPathRefusesAnUnknownAsset(t *testing.T) {
	cs := testSession(t, fakeBastionWithNetwork(t))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "jms_net_path",
		Arguments: map[string]any{"asset": "192.0.2.99"},
	})
	if err != nil {
		t.Fatalf("call jms_net_path: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an unknown asset was accepted: %s", testsupport.ToolText(t, res))
	}
}
