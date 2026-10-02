package tools

import (
	"net/http"
	"testing"

	"github.com/lcleveland/freshservice-mcp/internal/config"
)

func TestStatus(t *testing.T) {
	cs := session(t, &config.Config{DefaultWorkspace: "HR"}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Total", "400")
		w.Header().Set("X-Ratelimit-Remaining", "390")
		if r.URL.Path == "/api/v2/workspaces" {
			jsonOK(w, `{"workspaces":[{"id":2,"name":"IT","primary":true},{"id":3,"name":"HR"}]}`)
			return
		}
		jsonOK(w, `{"assets":[]}`)
	})
	out, isErr, _ := call(t, cs, "freshservice_status", nil)
	dw, _ := out["default_workspace"].(map[string]any)
	if isErr || out["connected"] != true || dw["name"] != "HR" || out["asset_api"] != "classic" {
		t.Fatalf("status = %v", out)
	}
	if rl, _ := out["rate_limit"].(map[string]any); rl["total"] != float64(400) {
		t.Errorf("rate_limit = %v", out["rate_limit"])
	}

	cs = session(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		jsonOK(w, `{"description":"Authentication failure"}`)
	})
	out, isErr, _ = call(t, cs, "freshservice_status", nil)
	if isErr || out["connected"] != false || out["detail"] == "" {
		t.Errorf("unauthorized status = %v", out)
	}
}
