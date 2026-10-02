package tools

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestGenericAPI(t *testing.T) {
	var sent string
	var logs bytes.Buffer
	h := func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/workspaces":
			jsonOK(w, `{"workspaces":[{"id":2,"name":"IT","primary":true}]}`)
		case r.Method == "GET" && r.URL.Path == "/api/v2/contracts/5":
			jsonOK(w, `{"contract":{"id":5,"name":"Dell support"}}`)
		default:
			sent = r.Method + " " + r.URL.Path + "?" + r.URL.RawQuery
			jsonOK(w, `{"ticket":{"id":1}}`)
		}
	}
	cs := sessionLog(t, allow("tickets", "approvals"), slog.New(slog.NewTextHandler(&logs, nil)), h)

	out, isErr, text := call(t, cs, "freshservice_api", map[string]any{"method": "GET", "path": "/api/v2/tickets/1/activities"})
	if isErr || out["id"] != float64(1) || sent != "GET /api/v2/tickets/1/activities?" {
		t.Fatalf("read: %v %s %s", out, sent, text)
	}
	for _, c := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"method": "GET", "path": "/api/v2/secret/thing"}, "not a documented read route"},
		{map[string]any{"method": "POST", "path": "/api/v2/agents", "reason": "r", "body": map[string]any{}}, "not a documented write route"},
		{map[string]any{"method": "POST", "path": "/api/v2/problems", "reason": "r", "body": map[string]any{}}, "itil capability"},
		{map[string]any{"method": "POST", "path": "/api/v2/tickets/1/notes", "body": map[string]any{"body": "x"}}, "reason is required"},
		{map[string]any{"method": "PUT", "path": "/api/v2/contracts/5", "params": map[string]any{"operation": "approve"}, "reason": "r"}, "confirm"},
		{map[string]any{"method": "GET", "path": "/api/v2/../v1/x"}, "/api/v2/ path"},
	} {
		sent = ""
		_, isErr, text := call(t, cs, "freshservice_api", c.args)
		if !isErr || !strings.Contains(text, c.want) || strings.HasPrefix(sent, "POST") || strings.HasPrefix(sent, "PUT") {
			t.Errorf("%v: err=%v sent=%s %s", c.args, isErr, sent, text)
		}
	}
	// DELETE is not even in the method enum; called anyway it is refused.
	if _, isErr, _ := call(t, cs, "freshservice_api", map[string]any{"method": "DELETE", "path": "/api/v2/tickets/1"}); !isErr {
		t.Error("DELETE accepted")
	}

	_, isErr, text = call(t, cs, "freshservice_api", map[string]any{"method": "POST", "path": "/api/v2/tickets/1/notes", "reason": "triage", "body": map[string]any{"body": "x"}})
	if isErr || sent != "POST /api/v2/tickets/1/notes?" || !strings.Contains(logs.String(), "tool=freshservice_api") {
		t.Errorf("allowed write: %s %s %s", sent, text, logs.String())
	}
	_, isErr, text = call(t, cs, "freshservice_api", map[string]any{"method": "PUT", "path": "/api/v2/contracts/5", "params": map[string]any{"operation": "approve"}, "reason": "r", "confirm": "Dell support"})
	if isErr || sent != "PUT /api/v2/contracts/5?operation=approve" {
		t.Errorf("approve: %s %s", sent, text)
	}
}
