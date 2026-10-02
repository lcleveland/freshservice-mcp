package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestWorkspaceInputs(t *testing.T) {
	var got string
	cs := session(t, nil, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/workspaces":
			jsonOK(w, `{"workspaces":[{"id":2,"name":"IT","primary":true},{"id":3,"name":"Human Resources"}]}`)
		case "/api/v2/assets":
			jsonOK(w, `{"assets":[]}`)
		default:
			got = r.URL.Query().Get("workspace_id")
			if !r.URL.Query().Has("workspace_id") {
				got = "<none>"
			}
			jsonOK(w, `{"tickets":[],"groups":[],"sla_policies":[],"requesters":[]}`)
		}
	})
	for _, c := range []struct {
		tool, action string
		ws           any
		want         string
		note         bool
	}{
		{"freshservice_ticket", "list", nil, "2", false},
		{"freshservice_ticket", "list", 3, "3", false},
		{"freshservice_ticket", "list", "human resources", "3", false},
		{"freshservice_ticket", "list", "all", "0", true},
		{"freshservice_lookup", "sla_policies", "all", "1", false},
		{"freshservice_agent", "groups", nil, "2", false},
	} {
		args := map[string]any{"action": c.action}
		if c.ws != nil {
			args["workspace"] = c.ws
		}
		out, isErr, text := call(t, cs, c.tool, args)
		if isErr || got != c.want || (out["_note"] != nil) != c.note {
			t.Errorf("%s %s %v: sent %s, note %v, err %s", c.tool, c.action, c.ws, got, out["_note"], text)
		}
	}

	for _, c := range []struct {
		tool, action string
		ws           any
		want         string
	}{
		{"freshservice_ticket", "list", "Facilities", `2 "IT", 3 "Human Resources"`},
		{"freshservice_agent", "groups", "all", "cannot span workspaces"},
		{"freshservice_ticket", "get", "IT", "account-level"},
	} {
		_, isErr, text := call(t, cs, c.tool, map[string]any{"action": c.action, "id": 1, "workspace": c.ws})
		if !isErr || !strings.Contains(text, c.want) {
			t.Errorf("%s %s %v: err=%v %s", c.tool, c.action, c.ws, isErr, text)
		}
	}
}

func TestAccountLevelToolsHaveNoWorkspaceInput(t *testing.T) {
	cs := session(t, nil, nil)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range res.Tools {
		b, _ := tl.InputSchema.(map[string]any)
		props, _ := b["properties"].(map[string]any)
		_, has := props["workspace"]
		switch tl.Name {
		case "freshservice_requester", "freshservice_workspace", "freshservice_status":
			if has {
				t.Errorf("%s advertises workspace", tl.Name)
			}
		case "freshservice_ticket", "freshservice_agent", "freshservice_lookup":
			if !has {
				t.Errorf("%s lacks workspace", tl.Name)
			}
		}
	}
}
