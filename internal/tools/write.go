package tools

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"

	"github.com/lcleveland/freshservice-mcp/internal/freshservice"
)

// write runs a write view. Every write, from any tool, goes through here so
// the capability check, reason, workspace, confirmation and audit log cannot
// be skipped. Nothing here retries: Freshservice writes take no idempotency key.
func (d Deps) write(ctx context.Context, tool string, v View, in Input) (any, error) {
	if !d.Config.Allow[v.Capability] {
		return nil, fmt.Errorf("the %s capability is disabled by the operator", v.Capability)
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, errors.New("reason is required for writes; say why, it is recorded in the audit log")
	}
	q, err := toValues(in.Params)
	if err != nil {
		return nil, err
	}
	path, err := fill(v.Path, in.ID, q)
	if err != nil {
		return nil, err
	}
	body, err := writeBody(v, in.Body)
	if err != nil {
		return nil, err
	}
	if v.Create && v.WS != WSNone {
		if body, err = d.createWorkspace(ctx, v, in, body); err != nil {
			return nil, err
		}
	} else if in.Workspace != nil {
		return nil, fmt.Errorf("action %s takes no workspace (only creates of workspace-scoped records do)", v.Action)
	}
	if v.Confirm != "" && (v.ConfirmIf == "" || has(body, v.ConfirmIf)) {
		if err := d.confirm(ctx, v, in); err != nil {
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
func (d Deps) confirm(ctx context.Context, v View, in Input) error {
	p := v.ConfirmPath
	if p == "" {
		p = v.Path
	}
	path, err := fill(p, in.ID, mustValues(in.Params))
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

func mustValues(m map[string]any) url.Values {
	q, _ := toValues(m)
	return q
}

func toNumber(s string) (any, error) {
	var n int64
	if _, err := fmt.Sscan(s, &n); err != nil {
		return nil, fmt.Errorf("workspace id %q is not a number", s)
	}
	return n, nil
}
