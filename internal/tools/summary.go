package tools

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Summary views answer "how many" without pulling rows. Where the endpoint
// returns a total (the ticket filter), each bucket costs one call; elsewhere
// one capped scan tallies the field.
//
// ponytail: only the ticket filter is known to return "total"; flip Total on
// other views once the live-tenant check confirms theirs.

const (
	reserveFraction = 5 // stop fan-out when under 1/5 (20%) of the rate budget remains
	pool            = 4
)

// bucket is one count to make: a filter clause and how to label it.
type bucket struct {
	Value any    `json:"value"`
	Label string `json:"label,omitempty"`
	Count *int   `json:"count"` // nil when the run stopped before reaching it
	q     string
}

func (d Deps) summary(ctx context.Context, tool string, v View, in Input) (any, error) {
	switch v.Summary {
	case "count":
		return d.count(ctx, v, in)
	case "group_by":
		return d.groupBy(ctx, v, in)
	case "backlog":
		return d.backlog(ctx, v, in)
	case "trend":
		return d.trend(ctx, v, in)
	}
	return nil, fmt.Errorf("unknown summary %q", v.Summary)
}

func (d Deps) count(ctx context.Context, v View, in Input) (any, error) {
	if v.Total {
		n, err := d.total(ctx, v, in, in.Query)
		if err != nil {
			return nil, err
		}
		return map[string]any{"count": n, "query": in.Query}, nil
	}
	t, err := d.scan(ctx, v, in, "")
	if err != nil {
		return nil, err
	}
	out := map[string]any{"count": t.scanned, "query": in.Query}
	if t.partial != "" {
		out["partial"] = t.partial
		out["count_is_at_least"] = true
	}
	return out, nil
}

func (d Deps) groupBy(ctx context.Context, v View, in Input) (any, error) {
	by := strings.TrimSpace(in.By)
	if by == "" {
		return nil, errors.New("group_by needs by: the field to group on, e.g. status, priority, group_id")
	}
	if v.Total {
		bs, err := d.buckets(ctx, v, in, by)
		if err != nil {
			return nil, err
		}
		if bs != nil {
			for i := range bs {
				bs[i].q = and(in.Query, clause(by, bs[i].Value))
			}
			partial, calls, err := d.fanOut(ctx, v, in, bs)
			if err != nil {
				return nil, err
			}
			return withPartial(map[string]any{"by": by, "query": in.Query, "counts": bs, "calls": calls}, partial), nil
		}
	}
	t, err := d.scan(ctx, v, in, by)
	if err != nil {
		return nil, err
	}
	type kv struct {
		Value any `json:"value"`
		Count int `json:"count"`
	}
	var counts []kv
	for k, n := range t.tally {
		counts = append(counts, kv{t.values[k], n})
	}
	slices.SortFunc(counts, func(a, b kv) int { return b.Count - a.Count })
	return withPartial(map[string]any{"by": by, "query": in.Query, "counts": counts, "scanned": t.scanned}, t.partial), nil
}

// backlog: open tickets by status x (group_id or priority), plus overdue.
func (d Deps) backlog(ctx context.Context, v View, in Input) (any, error) {
	by := in.By
	if by == "" {
		by = "group_id"
	}
	if by != "group_id" && by != "priority" {
		return nil, errors.New("backlog groups by group_id (default) or priority")
	}
	statuses, err := d.buckets(ctx, v, in, "status")
	if err != nil {
		return nil, err
	}
	statuses = slices.DeleteFunc(statuses, func(b bucket) bool { return b.Value == 4 || b.Value == 5 || b.Value == nil })
	dims, err := d.buckets(ctx, v, in, by)
	if err != nil {
		return nil, err
	}
	var open []string
	for _, s := range statuses {
		open = append(open, clause("status", s.Value))
	}
	openQ := "(" + strings.Join(open, " OR ") + ")"
	cells := []bucket{{Value: "overdue", Label: "open and due before today", q: and(in.Query, and(openQ, "due_by:<'"+time.Now().UTC().AddDate(0, 0, -1).Format(time.DateOnly)+"'"))}}
	for _, s := range statuses {
		for _, dm := range dims {
			cells = append(cells, bucket{Value: []any{s.Value, dm.Value}, Label: s.Label + " / " + dm.Label,
				q: and(in.Query, and(clause("status", s.Value), clause(by, dm.Value)))})
		}
	}
	partial, calls, err := d.fanOut(ctx, v, in, cells)
	if err != nil {
		return nil, err
	}
	var rows []bucket
	for _, c := range cells[1:] {
		if c.Count == nil || *c.Count > 0 {
			rows = append(rows, c)
		}
	}
	return withPartial(map[string]any{"by": []string{"status", by}, "overdue": cells[0].Count, "cells": rows,
		"note": "empty cells omitted; value is [status, " + by + "]", "calls": calls}, partial), nil
}

// trend: tickets created and resolved per day or week. "resolved" counts
// resolved/closed tickets last updated in the bucket: an approximation.
//
// ponytail: switch resolved to resolved_at once the live check shows the
// ticket filter accepts it.
func (d Deps) trend(ctx context.Context, v View, in Input) (any, error) {
	from, err1 := time.Parse(time.DateOnly, in.From)
	to, err2 := time.Parse(time.DateOnly, in.To)
	if err1 != nil || err2 != nil || to.Before(from) {
		return nil, errors.New("trend needs from and to as YYYY-MM-DD, from <= to")
	}
	step := 1
	switch in.Interval {
	case "", "day":
	case "week":
		step = 7
	default:
		return nil, errors.New("interval is day (default) or week")
	}
	var bs []bucket
	for s := from; !s.After(to); s = s.AddDate(0, 0, step) {
		e := s.AddDate(0, 0, step-1)
		if e.After(to) {
			e = to
		}
		r := "'" + s.Format(time.DateOnly) + "' AND %s:<'" + e.Format(time.DateOnly) + "'"
		bs = append(bs,
			bucket{Value: []string{s.Format(time.DateOnly), "created"}, q: and(in.Query, "created_at:>"+fmt.Sprintf(r, "created_at"))},
			bucket{Value: []string{s.Format(time.DateOnly), "resolved"}, q: and(in.Query, "(status:4 OR status:5) AND updated_at:>"+fmt.Sprintf(r, "updated_at"))})
	}
	partial, calls, err := d.fanOut(ctx, v, in, bs)
	if err != nil {
		return nil, err
	}
	type row struct {
		Start    string `json:"start"`
		Created  *int   `json:"created"`
		Resolved *int   `json:"resolved"`
	}
	var rows []row
	for i := 0; i < len(bs); i += 2 {
		rows = append(rows, row{bs[i].Value.([]string)[0], bs[i].Count, bs[i+1].Count})
	}
	interval := "day"
	if step == 7 {
		interval = "week"
	}
	return withPartial(map[string]any{"interval": interval, "rows": rows, "calls": calls,
		"note": "resolved approximates: resolved or closed tickets last updated in the period. Dates are in the account's time zone."}, partial), nil
}

// fanOut fills each bucket's Count with a total, at most pool at a time,
// refusing plans over --max-buckets and stopping early when the account's
// rate budget runs low. It returns why it stopped early, if it did.
func (d Deps) fanOut(ctx context.Context, v View, in Input, bs []bucket) (string, int, error) {
	if len(bs) > d.Config.MaxBuckets {
		return "", 0, fmt.Errorf("this needs %d API calls (one per bucket), over the limit of %d (--max-buckets); narrow it: fewer values, a shorter range, weekly interval, or a tighter query",
			len(bs), d.Config.MaxBuckets)
	}
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, pool)
		first   error
		partial string
		calls   int
		ok      int
	)
	for i := range bs {
		if low := d.lowBudget(); low != "" {
			mu.Lock()
			partial = low
			mu.Unlock()
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(b *bucket) {
			defer func() { <-sem; wg.Done() }()
			n, err := d.total(ctx, v, in, b.q)
			mu.Lock()
			defer mu.Unlock()
			calls++
			if err != nil {
				if first == nil {
					first = err
				}
				return
			}
			b.Count = &n
			ok++
		}(&bs[i])
	}
	wg.Wait()
	if first != nil && ok == 0 {
		return "", calls, first
	}
	if first != nil && partial == "" {
		partial = "some buckets failed: " + first.Error()
	}
	return partial, calls, nil
}

func (d Deps) lowBudget() string {
	r := d.Client.Rate()
	if r.Total > 0 && r.Remaining*reserveFraction < r.Total {
		return fmt.Sprintf("stopped early to keep the account's rate budget: %d of %d calls left this minute", r.Remaining, r.Total)
	}
	return ""
}

// total asks the filter endpoint for one page and reads its total.
func (d Deps) total(ctx context.Context, v View, in Input, query string) (int, error) {
	q := url.Values{}
	if _, err := d.workspace(ctx, v, in, q); err != nil {
		return 0, err
	}
	if query == "" {
		query = v.AllQuery
	}
	fq, err := filterQuery(query)
	if err != nil {
		return 0, err
	}
	q.Set(v.Filter, fq)
	q.Set("page", "1")
	resp, err := d.Client.Do(ctx, http.MethodGet, v.Path, q, nil)
	if err != nil {
		return 0, err
	}
	raw, err := resp.Decode()
	if err != nil {
		return 0, err
	}
	m, _ := raw.(map[string]any)
	n, ok := m["total"].(float64)
	if !ok {
		return 0, fmt.Errorf("%s returned no total", v.Path)
	}
	return int(n), nil
}

type tally struct {
	tally   map[string]int
	values  map[string]any
	scanned int
	partial string
}

// scan pages through the records (up to --max-records), tallying field by
// when given.
func (d Deps) scan(ctx context.Context, v View, in Input, by string) (*tally, error) {
	t := &tally{tally: map[string]int{}, values: map[string]any{}}
	q := url.Values{}
	if _, err := d.workspace(ctx, v, in, q); err != nil {
		return nil, err
	}
	query, path, size := in.Query, v.ScanPath, perPage
	if query == "" {
		query = v.AllQuery // the ticket filter needs some query
	}
	if query != "" {
		if v.Filter == "" {
			return nil, errors.New("this resource has no filter; drop query to summarise all of it")
		}
		fq, err := filterQuery(query)
		if err != nil {
			return nil, err
		}
		q.Set(v.Filter, fq)
		path, size = v.Path, filterPage
	} else {
		q.Set("per_page", strconv.Itoa(perPage))
	}
	p, err := d.assetPath(ctx, path)
	if err != nil {
		return nil, err
	}
	for page := 1; ; page++ {
		if low := d.lowBudget(); low != "" {
			t.partial = low
			return t, nil
		}
		q.Set("page", strconv.Itoa(page))
		resp, err := d.Client.Do(ctx, http.MethodGet, p, q, nil)
		if err != nil {
			if page == 1 {
				return nil, err
			}
			t.partial = "stopped at page " + strconv.Itoa(page) + ": " + err.Error()
			return t, nil
		}
		raw, err := resp.Decode()
		if err != nil {
			return nil, err
		}
		got := firstList(raw)
		for _, x := range got {
			if t.scanned == d.Config.MaxRecords {
				t.partial = fmt.Sprintf("scanned the first %d records (--max-records); narrow the query for exact counts", t.scanned)
				return t, nil
			}
			t.scanned++
			if by != "" {
				m, _ := x.(map[string]any)
				val := m[by]
				k := fmt.Sprint(val)
				t.tally[k]++
				t.values[k] = val
			}
		}
		if len(got) == 0 || !hasNext(resp.Header, len(got), size) {
			return t, nil
		}
	}
}

func (d Deps) assetPath(ctx context.Context, p string) (string, error) {
	if !strings.Contains(p, "{assets}") {
		return p, nil
	}
	a, err := d.Client.Account(ctx, d.Config.DefaultWorkspace)
	if err != nil {
		return "", err
	}
	if a.AssetPath == "" {
		return "", errors.New("this account answers neither asset API")
	}
	return strings.Replace(p, "{assets}", a.AssetPath, 1), nil
}

// buckets lists the values of a fixed-value field, from the form fields'
// choices or the list behind an id field. nil means "no fixed values: scan".
func (d Deps) buckets(ctx context.Context, v View, in Input, by string) ([]bucket, error) {
	if len(in.Values) > 0 {
		bs := make([]bucket, len(in.Values))
		for i, x := range in.Values {
			bs[i] = bucket{Value: x}
		}
		return bs, nil
	}
	if src, ok := idLists[by]; ok {
		bs, err := d.idBuckets(ctx, in, src)
		return append(bs, bucket{Value: nil, Label: "(none)"}), err
	}
	if v.FieldsPath == "" {
		return nil, nil
	}
	name := by
	if by == "type" {
		name = "ticket_type"
	}
	q := url.Values{}
	if _, err := d.workspace(ctx, View{WS: WSOne}, Input{Workspace: in.Workspace}, q); err != nil && !strings.EqualFold(scalarString(in.Workspace), "all") {
		return nil, err
	}
	resp, err := d.Client.Do(ctx, http.MethodGet, v.FieldsPath, q, nil)
	if err != nil {
		return nil, err
	}
	raw, err := resp.Decode()
	if err != nil {
		return nil, err
	}
	for _, f := range firstList(raw) {
		m, _ := f.(map[string]any)
		if m["name"] != name {
			continue
		}
		if bs := asChoices(m["choices"], numericFields[by]); len(bs) > 0 {
			return bs, nil
		}
	}
	return nil, nil
}

// asChoices reads Freshservice's choice shapes: [{id, value}], ["a", "b"],
// or {"label": id}.
func asChoices(c any, numeric bool) []bucket {
	var out []bucket
	switch t := c.(type) {
	case []any:
		for _, x := range t {
			switch y := x.(type) {
			case string:
				out = append(out, bucket{Value: y})
			case map[string]any:
				label := fmt.Sprint(firstOf(y, "value", "label", "name"))
				if id, ok := y["id"].(float64); ok && numeric {
					out = append(out, bucket{Value: int(id), Label: label})
				} else {
					out = append(out, bucket{Value: label})
				}
			}
		}
	case map[string]any:
		for label, id := range t {
			if f, ok := id.(float64); ok && numeric {
				out = append(out, bucket{Value: int(f), Label: label})
			} else {
				out = append(out, bucket{Value: label})
			}
		}
		slices.SortFunc(out, func(a, b bucket) int { return strings.Compare(a.Label, b.Label) })
	}
	return out
}

// numericFields filter by a choice's id; other choice fields (type,
// category) filter by its text.
var numericFields = map[string]bool{"status": true, "priority": true, "source": true, "impact": true, "urgency": true,
	"risk": true, "change_type": true, "release_type": true}

func firstOf(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return ""
}

// idLists are id fields whose values come from a list endpoint.
var idLists = map[string]struct {
	path string
	ws   WSMode
}{
	"group_id":      {"/api/v2/groups", WSOne},
	"agent_id":      {"/api/v2/agents", WSNone},
	"responder_id":  {"/api/v2/agents", WSNone},
	"department_id": {"/api/v2/departments", WSNone},
	"location_id":   {"/api/v2/locations", WSNone},
	"asset_type_id": {"/api/v2/asset_types", WSNone},
}

func (d Deps) idBuckets(ctx context.Context, in Input, src struct {
	path string
	ws   WSMode
}) ([]bucket, error) {
	ws := in.Workspace
	if strings.EqualFold(scalarString(ws), "all") {
		ws = nil // agent groups cannot span workspaces: use the default one
	}
	out, err := d.list(ctx, "buckets", View{Path: src.path, List: true}, src.path, d.mustWS(ctx, src.ws, ws), Input{Limit: d.Config.MaxBuckets + 1})
	if err != nil {
		return nil, err
	}
	var bs []bucket
	for _, x := range out["results"].([]any) {
		m, _ := x.(map[string]any)
		id, _ := m["id"].(float64)
		label := fmt.Sprint(firstOf(m, "name", "email"))
		if fn, ok := m["first_name"].(string); ok {
			label = strings.TrimSpace(fn + " " + fmt.Sprint(firstOf(m, "last_name")))
		}
		bs = append(bs, bucket{Value: int64(id), Label: label})
	}
	return bs, nil
}

func (d Deps) mustWS(ctx context.Context, mode WSMode, ws any) url.Values {
	q := url.Values{}
	d.workspace(ctx, View{WS: mode}, Input{Workspace: ws}, q)
	return q
}

// clause is one filter term: field:value, with strings quoted.
func clause(field string, v any) string {
	switch t := v.(type) {
	case nil:
		return field + ":null"
	case string:
		return field + ":'" + strings.ReplaceAll(t, "'", `\'`) + "'"
	case float64:
		return field + ":" + strconv.FormatFloat(t, 'f', -1, 64)
	}
	return fmt.Sprintf("%s:%v", field, v)
}

func and(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return "(" + a + ") AND " + b
}

func withPartial(m map[string]any, partial string) map[string]any {
	if partial != "" {
		m["partial"] = partial
	}
	return m
}
