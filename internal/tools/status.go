package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lcleveland/freshservice-mcp/internal/freshservice"
	"github.com/lcleveland/freshservice-mcp/internal/version"
)

type StatusOutput struct {
	BaseURL          string                   `json:"base_url"`
	Version          string                   `json:"version"`
	Connected        bool                     `json:"connected"`
	Workspaces       []freshservice.Workspace `json:"workspaces,omitempty"`
	DefaultWorkspace *freshservice.Workspace  `json:"default_workspace,omitempty"`
	AssetAPI         string                   `json:"asset_api,omitempty"`
	RateLimit        *freshservice.RateLimit  `json:"rate_limit,omitempty"`
	Detail           string                   `json:"detail,omitempty"`
}

func registerStatus(s *mcp.Server, d Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:  "freshservice_status",
		Title: "Freshservice connectivity check",
		Description: "Check that the API key works against this Freshservice domain, and report the workspaces the key's agent can see, " +
			"the default workspace this server uses when a call names none, which asset API the account has (classic or ITAM), " +
			"and the account's current per-minute rate-limit budget.\n\n" +
			"Call this first when another tool fails: it tells a wrong domain apart from a rejected key apart from a missing permission.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: new(true)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, StatusOutput, error) {
		out := StatusOutput{BaseURL: d.Client.BaseURL(), Version: version.Version}
		a, err := d.Client.Account(ctx, d.Config.DefaultWorkspace)
		if err != nil {
			out.Detail = err.Error()
			return nil, out, nil
		}
		out.Connected = true
		out.Workspaces, out.DefaultWorkspace = a.Workspaces, a.Default
		switch a.AssetPath {
		case freshservice.AssetsClassic:
			out.AssetAPI = "classic"
		case freshservice.AssetsITAM:
			out.AssetAPI = "itam"
		default:
			out.AssetAPI = "none (no asset API answered; plan or permissions)"
		}
		if r := d.Client.Rate(); r.Total > 0 {
			out.RateLimit = &r
		}
		return nil, out, nil
	})
}
