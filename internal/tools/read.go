package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	defaultLimit = 500
	perPage      = 100      // Freshservice's maximum
	filterPage   = 30       // filter endpoints' fixed page size
	maxQuery     = 512      // Freshservice's filter query limit
	maxBytes     = 60 << 10 // keeps a result inside a sensible slice of context
)

// fill substitutes {placeholders} in a view's path: {id} from id, the rest
// from params (consumed). Values are path-escaped.
func fill(path string, id any, params url.Values) (string, error) {
	var b strings.Builder
	for {
		i := strings.IndexByte(path, '{')
		if i < 0 {
			b.WriteString(path)
			return b.String(), nil
		}
		j := strings.IndexByte(path[i:], '}')
		name := path[i+1 : i+j]
		b.WriteString(path[:i])
		path = path[i+j+1:]
		var v string
		if name == "id" {
			v = scalarString(id)
			if v == "" {
				return "", errors.New("this action needs id")
			}
		} else {
			v = params.Get(name)
			if v == "" {
				return "", fmt.Errorf("this action needs params.%s", name)
			}
			params.Del(name)
		}
		b.WriteString(url.PathEscape(v))
	}
}

func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	}
	return ""
}

// read runs a read view and shapes the reply for the model.
func (d Deps) read(ctx context.Context, tool string, v View, in Input) (any, error) {
	q, err := toValues(in.Params)
	if err != nil {
		return nil, err
	}
	for k, x := range v.Defaults {
		if !q.Has(k) {
			q.Set(k, x)
		}
	}
	// Workspace first: on-call carries it in the path ({workspace_id}).
	all, err := d.workspace(ctx, v, in, q)
	if err != nil {
		return nil, err
	}
	p := v.Path
	if strings.Contains(p, "{assets}") {
		a, err := d.Client.Account(ctx, d.Config.DefaultWorkspace)
		if err != nil {
			return nil, err
		}
		if a.AssetPath == "" {
			return nil, errors.New("this account answers neither the classic nor the ITAM asset API (plan or the API key's permissions); freshservice_status shows which is available")
		}
		p = strings.Replace(p, "{assets}", a.AssetPath, 1)
	}
	path, err := fill(p, in.ID, q)
	if err != nil {
		return nil, err
	}
	if v.Filter != "" {
		fq, err := filterQuery(in.Query)
		if err != nil {
			return nil, err
		}
		q.Set(v.Filter, fq)
	}
	if v.List {
		out, err := d.list(ctx, tool+"."+v.Action, v, path, q, in)
		if err == nil && all && v.AllNote {
			out["_note"] = "across all workspaces Freshservice returns only global fields: workspace custom fields are omitted. Repeat with one workspace to get them."
		}
		return out, err
	}

	resp, err := d.Client.Do(ctx, v.method(), path, q, nil)
	if err != nil {
		return nil, err
	}
	out, err := resp.Decode()
	if err != nil {
		return nil, err
	}
	out = unwrap(out)
	if m, ok := out.(map[string]any); ok && v.Link != "" {
		if p, err := fill(v.Link, in.ID, url.Values{}); err == nil {
			m["url"] = strings.TrimRight(d.Client.BaseURL(), "/") + p
		}
	}
	if in.Fields != "" {
		out = project(out, strings.Split(in.Fields, ","))
	}
	return capBytes(out), nil
}

// workspace sets workspace_id for a scoped view: the caller's choice (id,
// name or "all", mapped per the view's mode) or else the default workspace.
// It never leaves it unset, which Freshservice would read as "primary only".
// It reports whether "all" was used.
func (d Deps) workspace(ctx context.Context, v View, in Input, q url.Values) (bool, error) {
	want := scalarString(in.Workspace)
	if v.WS == WSNone {
		if want != "" {
			return false, fmt.Errorf("action %s is account-level; workspace does not apply", v.Action)
		}
		return false, nil
	}
	if q.Has("workspace_id") && want == "" {
		return false, nil
	}
	if strings.EqualFold(want, "all") {
		switch v.WS {
		case WSAll:
			q.Set("workspace_id", "0")
		case WSGlobal:
			q.Set("workspace_id", "1")
		case WSOne:
			return false, fmt.Errorf("action %s cannot span workspaces (Freshservice has no all-workspaces view of it); name one workspace", v.Action)
		}
		return true, nil
	}
	if _, err := strconv.ParseInt(want, 10, 64); err == nil {
		q.Set("workspace_id", want)
		return false, nil
	}
	a, err := d.Client.Account(ctx, d.Config.DefaultWorkspace)
	if err != nil {
		return false, fmt.Errorf("resolving the workspace: %w", err)
	}
	if want == "" {
		if a.Default != nil {
			q.Set("workspace_id", strconv.FormatInt(a.Default.ID, 10))
		}
		return false, nil
	}
	var known []string
	for _, w := range a.Workspaces {
		if strings.EqualFold(w.Name, want) {
			q.Set("workspace_id", strconv.FormatInt(w.ID, 10))
			return false, nil
		}
		known = append(known, fmt.Sprintf("%d %q", w.ID, w.Name))
	}
	return false, fmt.Errorf("no workspace named %q; the API key's agent sees: %s, or \"all\"", want, strings.Join(known, ", "))
}

// filterQuery validates a Freshservice query and wraps it in the double
// quotes the API requires.
func filterQuery(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("this action needs query, e.g. status:2 AND priority:>3")
	}
	if !(len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"') {
		s = `"` + s + `"`
	}
	if len(s)-2 > maxQuery {
		return "", fmt.Errorf("query is %d characters; Freshservice allows %d", len(s)-2, maxQuery)
	}
	return s, nil
}

// pos is a place in a paged list: a page and how many of its items precede.
type pos struct{ page, skip int }

func (d Deps) list(ctx context.Context, key string, v View, path string, q url.Values, in Input) (map[string]any, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, d.Config.MaxRecords)
	at, err := decodeCursor(key, in.Cursor)
	if err != nil {
		return nil, err
	}
	size := perPage
	if v.Filter != "" {
		size = filterPage // and per_page is not sent: filters ignore or reject it
	} else {
		q.Set("per_page", strconv.Itoa(perPage))
	}

	var (
		items []any
		after []pos // after[i] is where the list resumes after items[i]
		total any
		next  *pos
		cur   = at
	)
pages:
	for {
		q.Set("page", strconv.Itoa(cur.page))
		resp, err := d.Client.Do(ctx, v.method(), path, q, nil)
		if err != nil {
			return nil, err
		}
		raw, err := resp.Decode()
		if err != nil {
			return nil, err
		}
		if m, ok := raw.(map[string]any); ok && m["total"] != nil {
			total = m["total"]
		}
		got := firstList(raw)
		for i := cur.skip; i < len(got); i++ {
			if len(items) == limit {
				next = &pos{cur.page, i}
				break pages
			}
			items = append(items, got[i])
			after = append(after, pos{cur.page, i + 1})
		}
		if len(got) == 0 || !hasNext(resp.Header, len(got), size) {
			break
		}
		cur = pos{cur.page + 1, 0}
		if len(items) == limit {
			next = &cur
			break
		}
	}

	fields := v.Brief
	if fields != nil {
		fields = append(slices.Clone(fields), "custom_fields")
	}
	if in.Fields != "" {
		fields = strings.Split(in.Fields, ",")
	}
	for i, x := range items {
		if fields != nil {
			items[i] = project(x, fields)
		}
	}

	out := map[string]any{}
	of := len(items)
	for len(items) > 1 && sizeOf(items) > maxBytes {
		items = items[:len(items)/2]
	}
	if len(items) < of {
		out["_truncation"] = map[string]any{"returned": len(items), "of": of,
			"note": "result too large; next_cursor resumes after the last item returned. Pass fields, narrow the query, or lower limit"}
		next = &after[len(items)-1]
	}
	if next != nil {
		out["next_cursor"] = encodeCursor(key, *next)
	}
	if total != nil {
		out["total"] = total
	}
	if items == nil {
		items = []any{}
	}
	out["results"] = items
	return out, nil
}

// hasNext reads the Link header when Freshservice sends one; otherwise a full
// page suggests another.
func hasNext(h http.Header, got, size int) bool {
	if l := h.Get("Link"); l != "" {
		return strings.Contains(l, `rel="next"`)
	}
	return got >= size
}

// firstList finds the item list in an envelope like {"tickets": [...]}.
func firstList(raw any) []any {
	if l, ok := raw.([]any); ok {
		return l
	}
	m, _ := raw.(map[string]any)
	for _, k := range sortedKeys(m) {
		if l, ok := m[k].([]any); ok {
			return l
		}
	}
	return nil
}

// unwrap turns {"ticket": {...}} into the ticket.
func unwrap(v any) any {
	if m, ok := v.(map[string]any); ok && len(m) == 1 {
		for _, x := range m {
			if inner, ok := x.(map[string]any); ok {
				return inner
			}
		}
	}
	return v
}

// The cursor is opaque to the model and bound to the tool.action that issued it.
func encodeCursor(action string, p pos) string {
	return base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "%s\x00%d,%d", action, p.page, p.skip))
}

func decodeCursor(action, c string) (pos, error) {
	if c == "" {
		return pos{page: 1}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	a, rest, ok := strings.Cut(string(b), "\x00")
	var p pos
	if err == nil && ok && a == action {
		if _, err := fmt.Sscanf(rest, "%d,%d", &p.page, &p.skip); err == nil && p.page >= 1 && p.skip >= 0 {
			return p, nil
		}
	}
	return pos{}, fmt.Errorf("cursor does not belong to action %s; pass next_cursor from the previous %s call unchanged", action, action)
}

// project keeps only the named top-level fields.
func project(v any, fields []string) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if x, ok := m[f]; ok {
			out[f] = x
		}
	}
	return out
}

// capBytes stops one huge object from flooding the context.
func capBytes(v any) any {
	if sizeOf(v) <= maxBytes {
		return v
	}
	if m, ok := v.(map[string]any); ok {
		return map[string]any{"_truncation": map[string]any{"note": "object too large to return (" + strconv.Itoa(sizeOf(v)) + " bytes); pass fields to pick the parts you need"},
			"fields": sortedKeys(m)}
	}
	return map[string]any{"_truncation": map[string]any{"note": "result too large; pass fields or narrow the query"}}
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

func sizeOf(v any) int {
	b, _ := json.Marshal(v)
	return len(b)
}

// toValues turns {"updated_since": "...", "include": ["stats"], "per_page": 10}
// into query params. Arrays repeat the key.
func toValues(m map[string]any) (url.Values, error) {
	q := url.Values{}
	for k, v := range m {
		xs, ok := v.([]any)
		if !ok {
			xs = []any{v}
		}
		for _, x := range xs {
			s, err := scalar(k, x)
			if err != nil {
				return nil, err
			}
			q.Add(k, s)
		}
	}
	return q, nil
}

func scalar(k string, v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(t), nil
	}
	return "", fmt.Errorf("params %q: values must be strings, numbers, booleans or arrays of them", k)
}
