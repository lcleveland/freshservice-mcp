package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input is shared by every first-class tool.
type Input struct {
	Action    string         `json:"action" jsonschema:"what to do"`
	ID        any            `json:"id,omitempty" jsonschema:"the object id the action needs"`
	Query     string         `json:"query,omitempty" jsonschema:"filter actions: a Freshservice query, e.g. status:2 AND priority:>3 AND created_at:>'2026-01-01'. Strings in single quotes; AND/OR with parentheses; :> and :< mean >= and <="`
	Params    map[string]any `json:"params,omitempty" jsonschema:"extra query params (and path values the action names), e.g. {\"updated_since\": \"2026-01-01T00:00:00Z\"}"`
	Fields    string         `json:"fields,omitempty" jsonschema:"comma-separated top-level fields to return instead of the default brief set"`
	Cursor    string         `json:"cursor,omitempty" jsonschema:"next_cursor from the previous call of the same action, to get the next page"`
	Limit     int            `json:"limit,omitempty" jsonschema:"list actions: max items"`
	Workspace any            `json:"workspace,omitempty" jsonschema:"workspace id, name (e.g. \"HR\") or \"all\"; default: the server's default workspace"`
}

func registerTool(s *mcp.Server, d Deps, t Tool) error {
	views := t.Views
	schema, err := jsonschema.For[Input](nil)
	if err != nil {
		return err
	}
	schema.Properties["id"].Types = []string{"integer", "string"}
	actions := make([]any, len(views))
	for i, v := range views {
		actions[i] = v.Action
	}
	schema.Properties["action"].Enum = actions
	schema.Required = []string{"action"}
	schema.Properties["limit"].Description = fmt.Sprintf("list actions: max items (default %d, max %d)", min(defaultLimit, d.Config.MaxRecords), d.Config.MaxRecords)
	if !slices.ContainsFunc(views, func(v View) bool { return v.Filter != "" }) {
		delete(schema.Properties, "query")
	}
	if !slices.ContainsFunc(views, func(v View) bool { return v.WS != WSNone }) {
		delete(schema.Properties, "workspace")
	} else {
		schema.Properties["workspace"].Types = []string{"integer", "string"}
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description + help(views),
		InputSchema: schema,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(true)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in Input) (*mcp.CallToolResult, any, error) {
		i := slices.IndexFunc(views, func(v View) bool { return v.Action == in.Action })
		if i < 0 {
			return nil, nil, fmt.Errorf("action %q is not available on %s (available: %s)", in.Action, t.Name, strings.Join(names(views), ", "))
		}
		out, err := d.read(ctx, t.Name, views[i], in)
		return nil, out, err
	})
	return nil
}

func help(views []View) string {
	var b strings.Builder
	b.WriteString("\n\nActions:")
	for _, v := range views {
		b.WriteString("\n- " + v.Action + ": " + v.Help)
	}
	b.WriteString("\n\nLists return brief items plus custom_fields (pass fields for others) and next_cursor when there is more; pass it back as cursor.")
	return b.String()
}

func names(vs []View) []string {
	s := make([]string, len(vs))
	for i, v := range vs {
		s[i] = v.Action
	}
	return s
}
