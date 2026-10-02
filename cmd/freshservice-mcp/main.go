// Command freshservice-mcp is an MCP server for the Freshservice API v2.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/lcleveland/freshservice-mcp/internal/config"
	"github.com/lcleveland/freshservice-mcp/internal/freshservice"
	"github.com/lcleveland/freshservice-mcp/internal/server"
	"github.com/lcleveland/freshservice-mcp/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "freshservice-mcp:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, warnings, err := config.Parse(args, os.Getenv)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if cfg.ShowVersion {
		fmt.Println(version.Version)
		return nil
	}
	// stdout belongs to the stdio transport; logs go to stderr.
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	for _, w := range warnings {
		log.Warn(w)
	}
	log.Info("starting", "version", version.Version, "config", cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c := freshservice.New(cfg.BaseURL, cfg.APIKey, &http.Client{Timeout: cfg.RequestTimeout}, log)
	s, n := server.New(cfg, c, log)
	log.Info("registered tools", "count", n)
	return server.ServeStdio(ctx, s)
}
