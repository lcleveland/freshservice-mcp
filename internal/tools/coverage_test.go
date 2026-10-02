package tools

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/lcleveland/freshservice-mcp/internal/config"
)

// excludedReads are documented reads deliberately left out, with why.
var excludedReads = map[string]string{
	"GET /api/v2/attachments/{}":                             "binary file download, not JSON",
	"GET /api/v2/itil_attachments/{}":                        "binary file download, not JSON",
	"GET /api/v2/oncall/shift-events/export":                 "returns an .ical file",
	"POST /api/v2/audit_log/export":                          "starts an asynchronous export delivered out of band",
	"POST /api/v2/post-incident-reports/templates/{}/export": "renders a file export",
	"POST /api/v2/journeys/requests/view":                    "POST-bodied filter; journey requests list covers reads",
}

var placeholder = regexp.MustCompile(`\{[^}]*\}`)

// normalize turns a view path into the inventory's form.
func normalize(method, path string) []string {
	p := placeholder.ReplaceAllString(path, "{}")
	if strings.HasPrefix(path, "{assets}") {
		rest := placeholder.ReplaceAllString(strings.TrimPrefix(path, "{assets}"), "{}")
		return []string{method + " /api/v2/assets" + rest, method + " /api/v2/itam/assets" + rest}
	}
	return []string{method + " " + p}
}

func inventory(t *testing.T, file string) map[string]string {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ops := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		op, note, _ := strings.Cut(line, "  # ")
		ops[op] = note
	}
	return ops
}

func TestEveryDocumentedReadIsCovered(t *testing.T) {
	covered := map[string]bool{}
	for _, tl := range Tools() {
		for _, v := range tl.Views {
			for _, op := range normalize(v.method(), v.Path) {
				covered[op] = true
			}
		}
	}
	covered["GET /api/v2/workspaces"] = true // also freshservice_status
	for op, note := range inventory(t, "testdata/reads.txt") {
		if !covered[op] && excludedReads[op] == "" {
			t.Errorf("documented read not covered or excluded: %s (%s)", op, note)
		}
	}
	for op := range excludedReads {
		if covered[op] {
			t.Errorf("%s is both covered and excluded", op)
		}
	}
}

func TestToolGroups(t *testing.T) {
	groups := map[string]bool{}
	for _, tl := range Tools() {
		groups[tl.Group] = true
		if !strings.HasPrefix(tl.Name, "freshservice_") {
			t.Errorf("%s: bad name", tl.Name)
		}
	}
	if len(groups) != 8 {
		t.Errorf("groups = %v", groups)
	}
}

func TestToolGroupsFilterRegistration(t *testing.T) {
	cs := session(t, &config.Config{Groups: []string{"itil"}}, nil)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tl := range res.Tools {
		got[tl.Name] = true
	}
	if !got["freshservice_ticket"] || !got["freshservice_change"] || got["freshservice_asset"] || got["freshservice_statuspage"] {
		t.Errorf("tools = %v", got)
	}
}

func TestAssetToolFollowsDetectedAPI(t *testing.T) {
	for _, itam := range []bool{false, true} {
		var hit string
		cs := session(t, nil, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/workspaces":
				jsonOK(w, `{"workspaces":[{"id":2,"name":"IT","primary":true}]}`)
			case "/api/v2/assets":
				if itam {
					w.WriteHeader(404)
					jsonOK(w, `{"description":"not found"}`)
					return
				}
				hit = r.URL.Path
				jsonOK(w, `{"assets":[{"display_id":7}]}`)
			case "/api/v2/itam/assets", "/api/v2/itam/assets/7", "/api/v2/assets/7":
				hit = r.URL.Path
				jsonOK(w, `{"assets":[{"id":7}],"asset":{"id":7}}`)
			}
		})
		call(t, cs, "freshservice_asset", map[string]any{"action": "list"})
		want := map[bool]string{false: "/api/v2/assets", true: "/api/v2/itam/assets"}[itam]
		if hit != want {
			t.Errorf("itam=%v: list hit %s", itam, hit)
		}
		call(t, cs, "freshservice_asset", map[string]any{"action": "get", "id": 7})
		if hit != want+"/7" {
			t.Errorf("itam=%v: get hit %s", itam, hit)
		}
	}
}

func TestEveryDocumentedWriteIsMappedOrRefused(t *testing.T) {
	mapped := map[string][]string{}
	for _, tl := range Tools() {
		for _, v := range tl.Views {
			if !v.write() {
				continue
			}
			for _, op := range normalize(v.method(), v.Path) {
				mapped[op] = append(mapped[op], tl.Name+"."+v.Action+" ("+v.Capability+")")
			}
		}
	}
	for op, note := range inventory(t, "testdata/writes.txt") {
		_, never := neverExposed(op, note)
		switch {
		case len(mapped[op]) == 0 && !never:
			t.Errorf("documented write neither mapped nor refused: %s (%s)", op, note)
		case len(mapped[op]) > 0 && never:
			t.Errorf("%s is refused but mapped to %v", op, mapped[op])
		}
	}
}

// neverExposed says whether a documented write is deliberately not offered,
// and why. Every delete is refused (docs/adr/0002-capability-map.md).
func neverExposed(op, note string) (string, bool) {
	if strings.Contains(note, ": delete") {
		return "deletes are never exposed", true
	}
	why, ok := refusedWrites[op]
	return why, ok
}

var refusedWrites = map[string]string{
	"POST /api/v2/agents":                                                           "agent writes are admin",
	"PUT /api/v2/agents/{}":                                                         "agent writes are admin",
	"PUT /api/v2/agents/{}/convert_to_requester":                                    "agent writes are admin",
	"PUT /api/v2/agents/{}/reactivate":                                              "agent writes are admin",
	"POST /api/v2/groups":                                                           "agent-group writes are admin",
	"PUT /api/v2/groups/{}":                                                         "agent-group writes are admin",
	"POST /api/v2/workspace":                                                        "workspace writes are admin (and MSP-only)",
	"PUT /api/v2/workspaces/{}":                                                     "workspace writes are admin (and MSP-only)",
	"PUT /api/v2/requesters/{}/convert_to_agent":                                    "creates an agent: agent writes are admin",
	"POST /api/v2/ticket_fields/sources":                                            "ticket form configuration is admin",
	"POST /api/v2/service-catalog/items":                                            "catalog item administration",
	"PUT /api/v2/service-catalog/items/{}":                                          "catalog item administration",
	"POST /api/v2/service-catalog/shared-fields":                                    "catalog item administration",
	"PUT /api/v2/service-catalog/shared-fields/{}":                                  "catalog item administration",
	"POST /api/v2/service-catalog/shared-fields/{}/archive":                         "catalog item administration",
	"POST /api/v2/service-catalog/shared-fields/{}/unarchive":                       "catalog item administration",
	"PUT /api/v2/itam/custom_fields/assets":                                         "custom field definitions are admin",
	"PUT /api/v2/itam/custom_fields/cloudinfrastructures":                           "custom field definitions are admin",
	"PUT /api/v2/itam/custom_fields/devices":                                        "custom field definitions are admin",
	"PUT /api/v2/itam/custom_fields/resources":                                      "custom field definitions are admin",
	"PUT /api/v2/oncall/ws/{}/schedules/{}/shifts/{}/rosters/override":              "also deletes overrides, and nothing is deleted through this server",
	"POST /api/v2/maintenance-windows/{}/status/pages/{}/maintenances":              "status page publishing needs confirm, and a maintenance window has no readable record to confirm against; publish from the change",
	"POST /api/v2/maintenance-windows/{}/status/pages/{}/maintenances/{}/updates":   "status page publishing needs confirm, and a maintenance window has no readable record to confirm against; publish from the change",
	"PUT /api/v2/maintenance-windows/{}/status/pages/{}/maintenances/{}":            "status page publishing needs confirm, and a maintenance window has no readable record to confirm against; publish from the change",
	"PUT /api/v2/maintenance-windows/{}/status/pages/{}/maintenances/{}/updates/{}": "status page publishing needs confirm, and a maintenance window has no readable record to confirm against; publish from the change",
}
