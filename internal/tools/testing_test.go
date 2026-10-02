package tools

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lcleveland/freshservice-mcp/internal/config"
	"github.com/lcleveland/freshservice-mcp/internal/freshservice"
)

// session starts a fake Freshservice (h) and an in-memory MCP client/server
// pair. A nil h answers the workspace and classic-asset probes.
func session(t *testing.T, cfg *config.Config, h http.HandlerFunc) *mcp.ClientSession {
	t.Helper()
	return sessionLog(t, cfg, nil, h)
}

func sessionLog(t *testing.T, cfg *config.Config, log *slog.Logger, h http.HandlerFunc) *mcp.ClientSession {
	t.Helper()
	if h == nil {
		h = func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v2/workspaces" {
				jsonOK(w, `{"workspaces":[{"id":2,"name":"IT","primary":true},{"id":3,"name":"HR"}]}`)
				return
			}
			jsonOK(w, `{}`)
		}
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	if cfg == nil {
		cfg = &config.Config{}
	}
	cfg.BaseURL = u
	if cfg.MaxRecords == 0 {
		cfg.MaxRecords = 2000
	}
	if cfg.MaxBuckets == 0 {
		cfg.MaxBuckets = 60
	}
	if cfg.Allow == nil {
		cfg.Allow = map[string]bool{}
	}
	c := freshservice.New(u, "key", srv.Client(), nil)
	c.Attempts, c.BaseDelay = 1, time.Millisecond

	s := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	Register(s, Deps{Client: c, Config: cfg, Log: log})
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := s.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

// call invokes a tool and decodes its structured result, also returning
// IsError and the first text content.
func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (map[string]any, bool, string) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	text := ""
	if len(res.Content) > 0 {
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			text = tc.Text
		}
	}
	var out map[string]any
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		json.Unmarshal(b, &out)
	}
	return out, res.IsError, text
}

func jsonOK(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, body)
}
