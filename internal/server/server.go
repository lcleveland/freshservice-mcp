// Package server builds the MCP server and serves it over stdio.
package server

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lcleveland/freshservice-mcp/internal/config"
	"github.com/lcleveland/freshservice-mcp/internal/freshservice"
	"github.com/lcleveland/freshservice-mcp/internal/tools"
	"github.com/lcleveland/freshservice-mcp/internal/version"
)

// New builds the server with the enabled tools and reports how many.
func New(cfg *config.Config, c *freshservice.Client, log *slog.Logger) (*mcp.Server, int) {
	s := mcp.NewServer(&mcp.Implementation{Name: "freshservice-mcp", Title: "Freshservice", Version: version.Version},
		&mcp.ServerOptions{Logger: log, Instructions: instructions(cfg)})
	return s, tools.Register(s, tools.Deps{Client: c, Config: cfg, Log: log})
}

func instructions(cfg *config.Config) string {
	return "Tools for the Freshservice account at " + cfg.BaseURL.String() + " (API v2), called directly with an agent API key: no monthly action budget, " +
		"but the account has a per-minute rate limit shared with every other app on it.\n\n" +
		"Call freshservice_status first if anything fails: it separates a wrong domain from a rejected key from a missing permission.\n\n" +
		"This server is read-only: every write capability is disabled by the operator. Do not suggest workarounds; ask the user to change the server configuration if a write is needed."
}

// ServeStdio runs until ctx is cancelled. Nothing else may write to stdout.
func ServeStdio(ctx context.Context, s *mcp.Server) error {
	return s.Run(ctx, &mcp.StdioTransport{})
}
