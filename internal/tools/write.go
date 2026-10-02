package tools

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/lcleveland/freshservice-mcp/internal/freshservice"
)

// write runs a write view. Every write, from any tool, goes through here so
// the capability check, reason, workspace, confirmation and audit log cannot
// be skipped. Nothing here retries: Freshservice writes take no idempotency key.
func (d Deps) write(ctx context.Context, tool string, v View, in Input) (any, error) {
	q, err := toValues(in.Params)
	if err != nil {
		return nil, err
	}
	for k, x := range v.Defaults {
		q.Set(k, x) // fixed operation params, e.g. contract approve
	}
	if v.WS != WSNone && !v.Create {
		// On-call writes carry the workspace in the path.
		if _, err := d.workspace(ctx, v, in, q); err != nil {
			return nil, err
		}
	}
	p := v.Path
	if strings.Contains(p, "{assets}") {
		if p, err = d.assetPath(ctx, p); err != nil {
			return nil, err
		}
	}
	vars := pathVars(p, in.ID, q)
	path, err := fill(p, in.ID, q)
	if err != nil {
		return nil, err
	}
	return d.writeAt(ctx, tool, v, in, path, q, vars)
}

// writeAt sends a write to a concrete path. vars are the path's placeholder
// values, for the confirm lookup.
func (d Deps) writeAt(ctx context.Context, tool string, v View, in Input, path string, q url.Values, vars map[string]string) (any, error) {
	if !d.Config.Allow[v.Capability] {
		return nil, fmt.Errorf("the %s capability is disabled by the operator", v.Capability)
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, errors.New("reason is required for writes; say why, it is recorded in the audit log")
	}
	body, err := writeBody(v, in.Body)
	if err != nil {
		return nil, err
	}
	if v.Create && v.WS != WSNone && !(tool == "freshservice_api" && has(body, "workspace_id")) {
		if body, err = d.createWorkspace(ctx, v, in, body); err != nil {
			return nil, err
		}
	} else if in.Workspace != nil && v.WS == WSNone {
		return nil, fmt.Errorf("action %s takes no workspace (only creates of workspace-scoped records do)", v.Action)
	}
	if v.Confirm != "" && (v.ConfirmIf == "" || has(body, v.ConfirmIf)) {
		if err := d.confirm(ctx, v, in, vars); err != nil {
			return nil, err
		}
	}

	var send any
	if body != nil {
		send = body
	}
	resp, err := d.Client.Do(ctx, v.method(), path, q, send)
	audit := []any{"tool", tool, "action", v.Action, "capability", v.Capability, "method", v.method(), "path", path, "reason", reason}
	if err != nil {
		var ae *freshservice.APIError
		if errors.As(err, &ae) {
			audit = append(audit, "status", ae.Status)
		}
		d.Log.Warn("freshservice write failed", append(audit, "error", err.Error())...)
		return nil, err
	}
	d.Log.Info("freshservice write", append(audit, "status", resp.Status)...)

	out := map[string]any{"status": resp.Status}
	if r, err := resp.Decode(); err == nil && r != nil {
		out["result"] = capBytes(unwrap(r))
	}
	return out, nil
}

func writeBody(v View, body any) (map[string]any, error) {
	if !v.Body {
		if body != nil {
			return nil, fmt.Errorf("action %s takes no body", v.Action)
		}
		return nil, nil
	}
	m, ok := body.(map[string]any)
	if body != nil && !ok {
		return nil, errors.New("body must be a JSON object")
	}
	for _, k := range v.Require {
		if _, ok := m[k]; !ok {
			return nil, fmt.Errorf("body needs %s", strings.Join(v.Require, ", "))
		}
	}
	return maps.Clone(m), nil
}

// createWorkspace puts an explicit workspace_id in a create's body. The
// default workspace is never applied silently: a ticket landing in IT when HR
// was meant is easy to miss.
func (d Deps) createWorkspace(ctx context.Context, v View, in Input, body map[string]any) (map[string]any, error) {
	ws := scalarString(in.Workspace)
	if ws == "" || strings.EqualFold(ws, "all") {
		return nil, errors.New("creating this record needs workspace: the id or name of the one workspace it belongs in (freshservice_workspace lists them)")
	}
	q := url.Values{}
	if _, err := d.workspace(ctx, View{Action: v.Action, WS: WSOne}, in, q); err != nil {
		return nil, err
	}
	if body == nil {
		body = map[string]any{}
	}
	id, err := toNumber(q.Get("workspace_id"))
	if err != nil {
		return nil, err
	}
	body["workspace_id"] = id
	return body, nil
}

// confirm makes the model name the record it means to act on, so a wrong id
// fails before anything is sent.
func (d Deps) confirm(ctx context.Context, v View, in Input, vars map[string]string) error {
	p := v.ConfirmPath
	if p == "" {
		p = v.Path
	}
	q := url.Values{}
	for k, x := range vars {
		q.Set(k, x)
	}
	path, err := fill(p, vars["id"], q)
	if err != nil {
		return err
	}
	resp, err := d.Client.Do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("looking up %s to confirm: %w", path, err)
	}
	r, _ := resp.Decode()
	m, _ := unwrap(r).(map[string]any)
	want, _ := m[v.Confirm].(string)
	if want == "" || strings.TrimSpace(in.Confirm) != want {
		return fmt.Errorf("action %s needs confirm set to the record's %s exactly; %s has %s %q. Check this is the record you mean, then retry",
			v.Action, v.Confirm, path, v.Confirm, want)
	}
	return nil
}

func has(m map[string]any, k string) bool {
	_, ok := m[k]
	return ok
}

// pathVars records the values a path template's placeholders will take.
func pathVars(p string, id any, q url.Values) map[string]string {
	vars := map[string]string{"id": scalarString(id)}
	for _, m := range placeholderRE.FindAllStringSubmatch(p, -1) {
		if m[1] != "id" {
			vars[m[1]] = q.Get(m[1])
		}
	}
	return vars
}

var placeholderRE = regexp.MustCompile(`\{([a-z_]+)\}`)

func toNumber(s string) (any, error) {
	var n int64
	if _, err := fmt.Sscan(s, &n); err != nil {
		return nil, fmt.Errorf("workspace id %q is not a number", s)
	}
	return n, nil
}
