package tools

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/lcleveland/freshservice-mcp/internal/config"
)

// fakeTickets serves n tickets 100 per page with a Link header, and the
// workspace probe. It records every query it saw.
func fakeTickets(n int, seen *[]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/workspaces":
			jsonOK(w, `{"workspaces":[{"id":2,"name":"IT","primary":true},{"id":3,"name":"HR"}]}`)
			return
		case "/api/v2/assets":
			jsonOK(w, `{"assets":[]}`)
			return
		}
		*seen = append(*seen, r.URL.RawQuery)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		var items []string
		for i := (page-1)*per + 1; i <= min(page*per, n); i++ {
			items = append(items, fmt.Sprintf(`{"id":%d,"subject":"t%d","description":"long","custom_fields":{"cf":1}}`, i, i))
		}
		if page*per < n {
			w.Header().Set("Link", fmt.Sprintf(`<http://x/api/v2/tickets?page=%d>; rel="next"`, page+1))
		}
		jsonOK(w, `{"tickets":[`+strings.Join(items, ",")+`]}`)
	}
}

func TestListPagesAndCursor(t *testing.T) {
	var seen []string
	cs := session(t, nil, fakeTickets(250, &seen))
	out, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "list", "limit": 150})
	if isErr {
		t.Fatal(text)
	}
	res := out["results"].([]any)
	if len(res) != 150 || out["next_cursor"] == nil {
		t.Fatalf("got %d, next %v", len(res), out["next_cursor"])
	}
	first := res[0].(map[string]any)
	if first["description"] != nil || first["custom_fields"] == nil || first["subject"] != "t1" {
		t.Errorf("brief = %v", first)
	}
	if !strings.Contains(seen[0], "updated_since=2000-01-01") || !strings.Contains(seen[0], "workspace_id=2") {
		t.Errorf("query = %s", seen[0])
	}

	out, _, _ = call(t, cs, "freshservice_ticket", map[string]any{"action": "list", "limit": 500, "cursor": out["next_cursor"]})
	res = out["results"].([]any)
	if len(res) != 100 || res[0].(map[string]any)["id"] != float64(151) || out["next_cursor"] != nil {
		t.Fatalf("second call: %d items from %v, next %v", len(res), res[0], out["next_cursor"])
	}

	out, _, _ = call(t, cs, "freshservice_ticket", map[string]any{"action": "list", "limit": 200, "fields": "id,description"})
	if r := out["results"].([]any); len(r) != 200 || r[0].(map[string]any)["description"] != "long" || out["next_cursor"] == nil {
		t.Errorf("fields/boundary: %d %v %v", len(r), r[0], out["next_cursor"])
	}

	if _, isErr, _ := call(t, cs, "freshservice_requester", map[string]any{"action": "list", "cursor": out["next_cursor"]}); !isErr {
		t.Error("a ticket cursor was accepted by another action")
	}
}

func TestMaxRecordsCaps(t *testing.T) {
	var seen []string
	cs := session(t, &config.Config{MaxRecords: 120}, fakeTickets(300, &seen))
	out, _, _ := call(t, cs, "freshservice_ticket", map[string]any{"action": "list", "limit": 1000})
	if r := out["results"].([]any); len(r) != 120 || out["next_cursor"] == nil {
		t.Fatalf("got %d", len(r))
	}
}

func TestByteCapTruncates(t *testing.T) {
	big := strings.Repeat("x", 2000)
	cs := session(t, nil, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/workspaces" {
			jsonOK(w, `{"workspaces":[]}`)
			return
		}
		var items []string
		for i := 1; i <= 100; i++ {
			items = append(items, fmt.Sprintf(`{"id":%d,"subject":"%s"}`, i, big))
		}
		jsonOK(w, `{"tickets":[`+strings.Join(items, ",")+`]}`)
	})
	out, _, _ := call(t, cs, "freshservice_ticket", map[string]any{"action": "list", "limit": 100})
	res := out["results"].([]any)
	if out["_truncation"] == nil || len(res) >= 100 || out["next_cursor"] == nil {
		t.Fatalf("not truncated: %d items, %v", len(res), out["_truncation"])
	}
	if sizeOf(out) > maxBytes+1024 {
		t.Errorf("result is %d bytes", sizeOf(out))
	}
	out2, _, _ := call(t, cs, "freshservice_ticket", map[string]any{"action": "list", "limit": 1, "cursor": out["next_cursor"]})
	want := float64(len(res) + 1)
	if got := out2["results"].([]any)[0].(map[string]any)["id"]; got != want {
		t.Errorf("resumed at %v, want %v", got, want)
	}
}

func TestFilterQuery(t *testing.T) {
	var q, path string
	cs := session(t, nil, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/workspaces" {
			jsonOK(w, `{"workspaces":[{"id":2,"name":"IT","primary":true}]}`)
			return
		}
		if r.URL.Path != "/api/v2/tickets/filter" {
			jsonOK(w, `{}`)
			return
		}
		path, q = r.URL.Path, r.URL.Query().Get("query")
		if r.URL.Query().Has("per_page") {
			t.Error("per_page sent to a filter")
		}
		jsonOK(w, `{"tickets":[{"id":1}],"total":1}`)
	})
	out, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "filter", "query": "status:2 AND tag:'it''s'"})
	if isErr || path != "/api/v2/tickets/filter" || q != `"status:2 AND tag:'it''s'"` || out["total"] != float64(1) {
		t.Fatalf("filter: %v %s %q %v %s", isErr, path, q, out, text)
	}
	if _, isErr, _ := call(t, cs, "freshservice_ticket", map[string]any{"action": "filter"}); !isErr {
		t.Error("empty query accepted")
	}
	if _, isErr, _ := call(t, cs, "freshservice_ticket", map[string]any{"action": "filter", "query": strings.Repeat("a", 513)}); !isErr {
		t.Error("over-long query accepted")
	}
}

func TestGetUnwrapsAndLinks(t *testing.T) {
	var path string
	cs := session(t, nil, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path + "?" + r.URL.RawQuery
		jsonOK(w, `{"ticket":{"id":42,"subject":"printer"}}`)
	})
	out, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "get", "id": 42})
	if isErr || out["subject"] != "printer" || !strings.HasSuffix(out["url"].(string), "/a/tickets/42") {
		t.Fatalf("get: %v %s", out, text)
	}
	if strings.Contains(path, "workspace_id") {
		t.Errorf("get sent a workspace: %s", path)
	}
	call(t, cs, "freshservice_ticket", map[string]any{"action": "task", "id": 42, "params": map[string]any{"task_id": 7}})
	if path != "/api/v2/tickets/42/tasks/7?" {
		t.Errorf("task path = %s", path)
	}
}

func TestFieldsProjectsArrayResponse(t *testing.T) {
	cs := session(t, nil, func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, `{"ticket_fields":[{"name":"status","label":"Status","choices":[1,2]}]}`)
	})
	_, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "fields", "fields": "name,label"})
	if isErr || text != `{"ticket_fields":[{"label":"Status","name":"status"}]}` {
		t.Fatalf("fields projection: %s", text)
	}
}
