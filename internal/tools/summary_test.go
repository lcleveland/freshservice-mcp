package tools

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lcleveland/freshservice-mcp/internal/config"
)

// fakeSummary answers the ticket filter with a total derived from the query
// (status:2 -> 5, status:3 -> 2, group_id:10 x2, else 1), the ticket form
// fields, the groups list, and a plain requester list for scans.
func fakeSummary(calls *atomic.Int32, remaining string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if remaining != "" {
			w.Header().Set("X-Ratelimit-Total", "100")
			w.Header().Set("X-Ratelimit-Remaining", remaining)
		}
		switch r.URL.Path {
		case "/api/v2/workspaces":
			jsonOK(w, `{"workspaces":[{"id":2,"name":"IT","primary":true}]}`)
		case "/api/v2/ticket_form_fields":
			jsonOK(w, `{"ticket_fields":[{"name":"status","choices":[{"id":2,"value":"Open"},{"id":3,"value":"Pending"},{"id":4,"value":"Resolved"},{"id":5,"value":"Closed"}]},
				{"name":"priority","choices":[{"id":1,"value":"Low"},{"id":4,"value":"Urgent"}]},
				{"name":"ticket_type","choices":["Incident","Service Request"]}]}`)
		case "/api/v2/groups":
			jsonOK(w, `{"groups":[{"id":10,"name":"Service Desk"},{"id":11,"name":"Network"}]}`)
		case "/api/v2/tickets/filter":
			calls.Add(1)
			q := r.URL.Query().Get("query")
			n := 1
			switch {
			case strings.Contains(q, "status:2"):
				n = 5
			case strings.Contains(q, "status:3"):
				n = 2
			}
			if strings.Contains(q, "group_id:10") {
				n *= 2
			}
			jsonOK(w, fmt.Sprintf(`{"tickets":[],"total":%d}`, n))
		case "/api/v2/requesters":
			calls.Add(1)
			page := r.URL.Query().Get("page")
			if page == "1" {
				w.Header().Set("Link", `<x?page=2>; rel="next"`)
				jsonOK(w, `{"requesters":[{"id":1,"location_id":7},{"id":2,"location_id":7}]}`)
				return
			}
			jsonOK(w, `{"requesters":[{"id":3,"location_id":8}]}`)
		default:
			jsonOK(w, `{}`)
		}
	}
}

func TestCountIsOneCall(t *testing.T) {
	var calls atomic.Int32
	cs := session(t, nil, fakeSummary(&calls, ""))
	out, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "count", "query": "status:2"})
	if isErr || out["count"] != float64(5) || calls.Load() != 1 {
		t.Fatalf("count: %v calls=%d %s", out, calls.Load(), text)
	}
}

func TestGroupByFansOutPerChoice(t *testing.T) {
	var calls atomic.Int32
	cs := session(t, nil, fakeSummary(&calls, ""))
	out, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "group_by", "by": "status"})
	if isErr {
		t.Fatal(text)
	}
	counts := out["counts"].([]any)
	if len(counts) != 4 || calls.Load() != 4 {
		t.Fatalf("counts = %v, calls %d", counts, calls.Load())
	}
	if c := counts[0].(map[string]any); c["value"] != float64(2) || c["label"] != "Open" || c["count"] != float64(5) {
		t.Errorf("first bucket = %v", c)
	}

	calls.Store(0)
	out, _, _ = call(t, cs, "freshservice_ticket", map[string]any{"action": "group_by", "by": "group_id", "query": "status:2"})
	counts = out["counts"].([]any)
	if len(counts) != 3 || counts[0].(map[string]any)["count"] != float64(10) || counts[2].(map[string]any)["label"] != "(none)" {
		t.Errorf("group buckets = %v", counts)
	}
}

func TestMaxBucketsRefusesWithEstimate(t *testing.T) {
	var calls atomic.Int32
	cs := session(t, &config.Config{MaxBuckets: 3}, fakeSummary(&calls, ""))
	_, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "group_by", "by": "status"})
	if !isErr || !strings.Contains(text, "needs 4 API calls") || calls.Load() != 0 {
		t.Fatalf("err=%v %s calls=%d", isErr, text, calls.Load())
	}
}

func TestLowBudgetStopsEarly(t *testing.T) {
	var calls atomic.Int32
	cs := session(t, nil, fakeSummary(&calls, "10"))
	call(t, cs, "freshservice_ticket", map[string]any{"action": "count", "query": "status:2"}) // learn the budget
	calls.Store(0)
	out, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "group_by", "by": "status"})
	if isErr || out["partial"] == nil || calls.Load() != 0 {
		t.Fatalf("out=%v calls=%d %s", out, calls.Load(), text)
	}
}

func TestScanFallbackTallies(t *testing.T) {
	var calls atomic.Int32
	cs := session(t, nil, fakeSummary(&calls, ""))
	out, isErr, text := call(t, cs, "freshservice_requester", map[string]any{"action": "group_by", "by": "location_id"})
	if isErr || out["scanned"] != float64(3) || out["partial"] != nil {
		t.Fatalf("scan: %v %s", out, text)
	}
	if top := out["counts"].([]any)[0].(map[string]any); top["value"] != float64(7) || top["count"] != float64(2) {
		t.Errorf("top = %v", top)
	}

	cs = session(t, &config.Config{MaxRecords: 2}, fakeSummary(&calls, ""))
	out, _, _ = call(t, cs, "freshservice_requester", map[string]any{"action": "count"})
	if out["count"] != float64(2) || out["partial"] == nil {
		t.Errorf("capped count = %v", out)
	}
}

func TestBacklogAndTrend(t *testing.T) {
	var calls atomic.Int32
	cs := session(t, nil, fakeSummary(&calls, ""))
	out, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "backlog"})
	if isErr {
		t.Fatal(text)
	}
	// open statuses 2,3 x groups 10,11,(none) = 6 cells + overdue
	if cells := out["cells"].([]any); len(cells) != 6 || calls.Load() != 7 || out["overdue"] == nil {
		t.Fatalf("backlog: %d cells, %d calls, %v", len(out["cells"].([]any)), calls.Load(), out)
	}

	out, isErr, text = call(t, cs, "freshservice_ticket", map[string]any{"action": "trend", "from": "2026-09-01", "to": "2026-09-14", "interval": "week"})
	rows, _ := out["rows"].([]any)
	if isErr || len(rows) != 2 || rows[1].(map[string]any)["start"] != "2026-09-08" {
		t.Fatalf("trend: %v %s", out, text)
	}
	if _, isErr, _ := call(t, cs, "freshservice_ticket", map[string]any{"action": "trend", "from": "2026-01-01", "to": "2026-12-31"}); !isErr {
		t.Error("a 365-day daily trend (730 calls) was not refused")
	}
}
