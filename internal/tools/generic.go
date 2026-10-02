package tools

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// APIInput is freshservice_api's input.
type APIInput struct {
	Method    string         `json:"method" jsonschema:"GET, POST, PUT or PATCH"`
	Path      string         `json:"path" jsonschema:"an /api/v2/ path, e.g. /api/v2/tickets/42/activities; query params go in params"`
	Params    map[string]any `json:"params,omitempty" jsonschema:"query params"`
	Body      any            `json:"body,omitempty" jsonschema:"JSON body for writes"`
	Workspace any            `json:"workspace,omitempty" jsonschema:"creates of workspace-scoped records: the workspace id or name, unless body has workspace_id"`
	Reason    string         `json:"reason,omitempty" jsonschema:"required for writes: why, recorded in the audit log"`
	Confirm   string         `json:"confirm,omitempty" jsonschema:"writes that need it: the record's subject or name, exactly"`
}

// registerGeneric adds the escape hatch for anything the curated tools miss.
// It cannot get round the capability gates: a write must match a write view
// (method + path template) whose capability is on, and then runs through the
// same write path, so reason, confirm and audit apply. Unknown write routes
// and deletes are refused.
func registerGeneric(s *mcp.Server, d Deps) error {
	schema, err := jsonschema.For[APIInput](nil)
	if err != nil {
		return err
	}
	schema.Properties["method"].Enum = []any{"GET", "POST", "PUT", "PATCH"}
	schema.Properties["workspace"].Types = []string{"integer", "string"}
	desc := "Call any documented Freshservice API v2 endpoint the curated freshservice_* tools do not cover. Prefer the curated tools: they page, trim and resolve workspaces for you.\n\n" +
		"GET works on any documented read route. "
	on := d.Config.Enabled()
	if len(on) == 0 {
		desc += "Writes are disabled by the operator."
		delete(schema.Properties, "body")
		delete(schema.Properties, "reason")
		delete(schema.Properties, "confirm")
		delete(schema.Properties, "workspace")
		schema.Properties["method"].Enum = []any{"GET"}
	} else {
		desc += "Writes are allowed only on documented write routes whose capability is enabled (" + strings.Join(on, ", ") + "), need reason, and are never retried. Nothing can be deleted."
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "freshservice_api",
		Title:       "Freshservice API",
		Description: desc,
		InputSchema: schema,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: len(on) == 0, DestructiveHint: new(false), OpenWorldHint: new(true)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in APIInput) (*mcp.CallToolResult, any, error) {
		out, err := d.generic(ctx, in)
		return nil, out, err
	})
	return nil
}

func (d Deps) generic(ctx context.Context, in APIInput) (any, error) {
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	path := strings.TrimRight(strings.TrimSpace(in.Path), "/")
	if !strings.HasPrefix(path, "/api/v2/") || strings.Contains(path, "..") || strings.ContainsAny(path, "?#") {
		return nil, errors.New("path must be an /api/v2/ path without a query string (put query params in params)")
	}
	q, err := toValues(in.Params)
	if err != nil {
		return nil, err
	}
	if method == http.MethodGet {
		if !slices.ContainsFunc(d.routes(), func(v View) bool { return v.method() == "GET" && matches(v.Path, path) }) {
			return nil, fmt.Errorf("GET %s is not a documented read route", path)
		}
		resp, err := d.Client.Do(ctx, method, path, q, nil)
		if err != nil {
			return nil, err
		}
		out, err := resp.Decode()
		if err != nil {
			return nil, err
		}
		return capBytes(unwrap(out)), nil
	}
	if method == http.MethodDelete {
		return nil, errors.New("nothing can be deleted through this server")
	}
	for _, v := range d.routes() {
		if !v.write() || v.method() != method || !matches(v.Path, path) || !fixedParams(v, q) {
			continue
		}
		if !d.Config.Allow[v.Capability] {
			return nil, fmt.Errorf("%s %s needs the %s capability, which is disabled by the operator", method, path, v.Capability)
		}
		in2 := Input{Body: in.Body, Reason: in.Reason, Confirm: in.Confirm, Workspace: in.Workspace}
		return d.writeAt(ctx, "freshservice_api", v, in2, path, q, matchVars(v.Path, path))
	}
	return nil, fmt.Errorf("%s %s is not a documented write route this server allows; it may be admin-only or a delete", method, path)
}

// routes is every view with its asset path expanded both ways, so the raw
// tool and the curated tools share one table and cannot drift apart.
func (d Deps) routes() []View {
	var vs []View
	for _, t := range Tools() {
		for _, v := range t.Views {
			if strings.HasPrefix(v.Path, "{assets}") {
				for _, base := range []string{"/api/v2/assets", "/api/v2/itam/assets"} {
					w := v
					w.Path = base + strings.TrimPrefix(v.Path, "{assets}")
					vs = append(vs, w)
				}
				continue
			}
			vs = append(vs, v)
		}
	}
	// Contract approve/reject share PUT /contracts/{id} with update: most
	// specific (fixed params) first.
	slices.SortStableFunc(vs, func(a, b View) int { return len(b.Defaults) - len(a.Defaults) })
	return vs
}

// matches compares a template (/api/v2/tickets/{id}/notes) with a concrete
// path segment by segment; {x} matches any one segment.
func matches(pattern, path string) bool {
	ps, xs := strings.Split(pattern, "/"), strings.Split(path, "/")
	if len(ps) != len(xs) {
		return false
	}
	for i := range ps {
		if !(strings.HasPrefix(ps[i], "{") || ps[i] == xs[i]) {
			return false
		}
	}
	return true
}

// fixedParams: a view with fixed params (contract approve and reject share
// PUT /contracts/{id} with update) matches only when the call sends them.
func fixedParams(v View, q url.Values) bool {
	for k, x := range v.Defaults {
		if q.Get(k) != x {
			return false
		}
	}
	return true
}

func matchVars(pattern, path string) map[string]string {
	vars := map[string]string{}
	ps, xs := strings.Split(pattern, "/"), strings.Split(path, "/")
	for i := range ps {
		if strings.HasPrefix(ps[i], "{") {
			vars[strings.Trim(ps[i], "{}")] = xs[i]
		}
	}
	return vars
}
