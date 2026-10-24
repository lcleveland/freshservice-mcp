package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lcleveland/freshservice-mcp/internal/config"
)

func allow(caps ...string) *config.Config {
	c := &config.Config{Allow: map[string]bool{}}
	for _, n := range caps {
		c.Allow[n] = true
	}
	return c
}

func actionsOf(t *testing.T, cs *mcp.ClientSession, tool string) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range res.Tools {
		if tl.Name != tool {
			continue
		}
		b, _ := json.Marshal(tl.InputSchema)
		var s struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		}
		json.Unmarshal(b, &s)
		return s.Properties["action"].Enum
	}
	t.Fatalf("%s not registered", tool)
	return nil
}

// fakeWrites records the last write and serves a ticket and a requester for
// confirm lookups.
type fakeWrites struct {
	method, path string
	body         map[string]any
}

func (f *fakeWrites) handler(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/v2/workspaces":
		jsonOK(w, `{"workspaces":[{"id":2,"name":"IT","primary":true},{"id":3,"name":"HR"}]}`)
	case r.Method == "GET" && r.URL.Path == "/api/v2/tickets/42":
		jsonOK(w, `{"ticket":{"id":42,"subject":"Printer on fire"}}`)
	case r.Method == "GET" && r.URL.Path == "/api/v2/tickets/42/approvals":
		jsonOK(w, `{"approvals":[{"id":1,"approval_group":{"id":9},"approval_status":{"id":0}},
			{"id":2,"approval_group":{"id":9},"approval_status":{"id":1}},{"id":3,"approval_group":{"id":8},"approval_status":{"id":0}}]}`)
	case r.Method == "GET":
		jsonOK(w, `{}`)
	default:
		f.method, f.path, f.body = r.Method, r.URL.Path, nil
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &f.body)
		w.WriteHeader(201)
		jsonOK(w, `{"ticket":{"id":43}}`)
	}
}

func TestCapabilityGatesSchema(t *testing.T) {
	off := session(t, nil, nil)
	if a := actionsOf(t, off, "freshservice_ticket"); slices.Contains(a, "create") || slices.Contains(a, "reply") {
		t.Errorf("writes off but schema has %v", a)
	}
	on := session(t, allow("tickets"), nil)
	a := actionsOf(t, on, "freshservice_ticket")
	if !slices.Contains(a, "create") || !slices.Contains(a, "note") || slices.Contains(a, "reply") || slices.Contains(a, "promote_major") {
		t.Errorf("tickets on: schema has %v", a)
	}
	_, isErr, text := call(t, off, "freshservice_ticket", map[string]any{"action": "create", "body": map[string]any{}, "reason": "x"})
	if !isErr || !(strings.Contains(text, "not available") || strings.Contains(text, "enum")) {
		t.Errorf("hidden action: %v %s", isErr, text)
	}
}

func TestWriteGuards(t *testing.T) {
	f := &fakeWrites{}
	var logs bytes.Buffer
	cs := sessionLog(t, allow("tickets", "people"), slog.New(slog.NewTextHandler(&logs, nil)), f.handler)

	for _, c := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"no reason", map[string]any{"action": "note", "id": 42, "body": map[string]any{"body": "x"}}, "reason is required"},
		{"create without workspace", map[string]any{"action": "create", "reason": "r", "body": map[string]any{"status": 2, "priority": 1}}, "needs workspace"},
		{"create in all", map[string]any{"action": "create", "reason": "r", "workspace": "all", "body": map[string]any{"status": 2, "priority": 1}}, "needs workspace"},
		{"missing required", map[string]any{"action": "create", "reason": "r", "workspace": "HR", "body": map[string]any{"status": 2}}, "body needs"},
		{"confirm mismatch", map[string]any{"action": "move_workspace", "id": 42, "reason": "r", "confirm": "Printer", "body": map[string]any{"workspace_id": 3}}, `"Printer on fire"`},
		{"assets without confirm", map[string]any{"action": "update", "id": 42, "reason": "r", "body": map[string]any{"assets": []any{}}}, "confirm"},
		{"workspace on a non-create", map[string]any{"action": "note", "id": 42, "reason": "r", "workspace": "HR", "body": map[string]any{"body": "x"}}, "takes no workspace"},
	} {
		f.method = ""
		_, isErr, text := call(t, cs, "freshservice_ticket", c.args)
		if !isErr || !strings.Contains(text, c.want) || f.method != "" {
			t.Errorf("%s: err=%v sent=%s %s", c.name, isErr, f.method, text)
		}
	}

	out, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "create", "reason": "new hire laptop", "workspace": "HR",
		"body": map[string]any{"subject": "s", "status": 2, "priority": 1}})
	if isErr || f.method != "POST" || f.path != "/api/v2/tickets" || f.body["workspace_id"] != float64(3) || out["status"] != float64(201) {
		t.Fatalf("create: %v %s %s %v %s", out, f.method, f.path, f.body, text)
	}
	if !strings.Contains(logs.String(), "freshservice write") || !strings.Contains(logs.String(), `reason="new hire laptop"`) {
		t.Errorf("audit log = %s", logs.String())
	}

	_, isErr, text = call(t, cs, "freshservice_ticket", map[string]any{"action": "update", "id": 42, "reason": "r", "confirm": "Printer on fire", "body": map[string]any{"assets": []any{}}})
	if isErr || f.method != "PUT" {
		t.Errorf("confirmed asset update: %s", text)
	}
	_, isErr, _ = call(t, cs, "freshservice_ticket", map[string]any{"action": "update", "id": 42, "reason": "r", "body": map[string]any{"priority": 3}})
	if isErr {
		t.Error("plain update needed confirm")
	}

	// Freshservice reads a bodiless cancel as an empty update (400).
	_, isErr, text = call(t, cs, "freshservice_ticket", map[string]any{"action": "cancel_approval", "id": 42, "reason": "r", "params": map[string]any{"approval_id": 7}})
	if s, _ := f.body["approval_status"].(map[string]any); isErr || f.path != "/api/v2/tickets/42/approvals/7" || s["id"] != float64(3) {
		t.Errorf("cancel_approval: %s %s %v", text, f.path, f.body)
	}
	// The group itself rejects approval_status, so its pending approvals are cancelled instead.
	out, isErr, text = call(t, cs, "freshservice_ticket", map[string]any{"action": "cancel_approval_group", "id": 42, "reason": "r", "params": map[string]any{"group_id": 9}})
	if isErr || f.path != "/api/v2/tickets/42/approvals/1" || fmt.Sprint(out["cancelled_approvals"]) != "[1]" {
		t.Errorf("cancel_approval_group: %s %s %v", text, f.path, out)
	}
}

func TestNoDeleteRoutes(t *testing.T) {
	unlink := map[string]bool{"remove_group_member": true, "remove_installations": true, "remove_users": true}
	for _, tl := range Tools() {
		for _, v := range tl.Views {
			if v.method() == "DELETE" && !unlink[v.Action] && (v.Capability != "soft-delete" || v.Confirm == "") {
				t.Errorf("%s %s is a DELETE outside soft-delete or without confirm", tl.Name, v.Action)
			}
			if strings.Contains(v.Path, "delete_forever") || strings.HasSuffix(v.Path, "/forget") {
				t.Errorf("%s %s reaches %s", tl.Name, v.Action, v.Path)
			}
		}
	}
}

func TestApprovalsAndStatusPublishNeedConfirm(t *testing.T) {
	var sent string
	cs := session(t, allow("approvals", "ops"), func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v2/contracts/5":
			jsonOK(w, `{"contract":{"id":5,"name":"Dell support"}}`)
		case r.Method == "GET" && r.URL.Path == "/api/v2/tickets/42":
			jsonOK(w, `{"ticket":{"id":42,"subject":"Email down"}}`)
		case r.Method == "GET":
			jsonOK(w, `{}`)
		default:
			sent = r.Method + " " + r.URL.Path + "?" + r.URL.RawQuery
			jsonOK(w, `{}`)
		}
	})
	if _, isErr, _ := call(t, cs, "freshservice_contract", map[string]any{"action": "approve", "id": 5, "reason": "r"}); !isErr || sent != "" {
		t.Errorf("approve without confirm went through: %s", sent)
	}
	if _, isErr, text := call(t, cs, "freshservice_contract", map[string]any{"action": "approve", "id": 5, "reason": "r", "confirm": "Dell support"}); isErr || sent != "PUT /api/v2/contracts/5?operation=approve" {
		t.Errorf("approve: %s %s", sent, text)
	}
	sent = ""
	args := map[string]any{"action": "publish_incident", "id": 42, "reason": "r", "params": map[string]any{"page_id": 9}, "body": map[string]any{"title": "Email outage"}}
	if _, isErr, _ := call(t, cs, "freshservice_statuspage", args); !isErr || sent != "" {
		t.Errorf("publish without confirm went through: %s", sent)
	}
	args["confirm"] = "Email down"
	if _, isErr, text := call(t, cs, "freshservice_statuspage", args); isErr || sent != "POST /api/v2/tickets/42/status/pages/9/incidents?" {
		t.Errorf("publish: %s %s", sent, text)
	}
}

func TestSoftDeleteTrashesWithConfirm(t *testing.T) {
	if a := actionsOf(t, session(t, allow("tickets", "people"), nil), "freshservice_ticket"); slices.Contains(a, "trash") {
		t.Errorf("soft-delete off but schema has trash")
	}
	f := &fakeWrites{}
	cs := session(t, allow("soft-delete"), f.handler)
	if _, isErr, _ := call(t, cs, "freshservice_ticket", map[string]any{"action": "trash", "id": 42, "reason": "spam", "confirm": "wrong"}); !isErr || f.method != "" {
		t.Fatalf("wrong confirm sent %s %s", f.method, f.path)
	}
	if _, isErr, text := call(t, cs, "freshservice_ticket", map[string]any{"action": "trash", "id": 42, "reason": "spam", "confirm": "Printer on fire"}); isErr || f.method != "DELETE" || f.path != "/api/v2/tickets/42" {
		t.Fatalf("trash: %s %s %s", text, f.method, f.path)
	}
	if _, isErr, _ := call(t, cs, "freshservice_api", map[string]any{"method": "DELETE", "path": "/api/v2/tickets/42", "reason": "r"}); !isErr {
		t.Error("freshservice_api sent a DELETE")
	}
}
