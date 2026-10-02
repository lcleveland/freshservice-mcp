// Package config parses flags and environment into a Config.
//
// The API key is only ever read from a file, a systemd credential, or, with a
// warning, the environment. There is deliberately no flag that takes it on
// argv, where any local user can read it from /proc/<pid>/cmdline.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	BaseURL          *url.URL // https://<sub>.freshservice.com, no /api/v2
	APIKey           string
	DefaultWorkspace string // id or name; empty means the primary workspace
	RequestTimeout   time.Duration
	LogLevel         slog.Level

	ShowVersion bool
}

// LogValue keeps the API key out of logs however the config is printed.
func (c *Config) LogValue() slog.Value {
	u := ""
	if c.BaseURL != nil {
		u = c.BaseURL.String()
	}
	return slog.GroupValue(
		slog.String("base_url", u),
		slog.Bool("api_key_set", c.APIKey != ""),
		slog.String("default_workspace", c.DefaultWorkspace),
	)
}

// Parse reads args and the environment. getenv is injected for tests.
// Warnings (non-fatal) are returned for the caller to log once a logger exists.
func Parse(args []string, getenv func(string) string) (*Config, []string, error) {
	var (
		c                         Config
		domain, keyFile, logLevel string
		warnings                  []string
	)
	fs := flag.NewFlagSet("freshservice-mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&domain, "domain", getenv("FRESHSERVICE_DOMAIN"), "Freshservice domain: acme, acme.freshservice.com or a full URL (env FRESHSERVICE_DOMAIN)")
	fs.StringVar(&keyFile, "api-key-file", getenv("FRESHSERVICE_API_KEY_FILE"), "file holding the agent API key (env FRESHSERVICE_API_KEY_FILE)")
	fs.StringVar(&c.DefaultWorkspace, "default-workspace", getenv("FRESHSERVICE_DEFAULT_WORKSPACE"), "workspace id or name used when a call names none (default: the primary workspace) (env FRESHSERVICE_DEFAULT_WORKSPACE)")
	fs.DurationVar(&c.RequestTimeout, "request-timeout", 30*time.Second, "per-request timeout to Freshservice")
	fs.StringVar(&logLevel, "log-level", or(getenv("FRESHSERVICE_MCP_LOG_LEVEL"), "info"), "debug|info|warn|error (env FRESHSERVICE_MCP_LOG_LEVEL)")
	fs.BoolVar(&c.ShowVersion, "version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(os.Stderr)
			fmt.Fprintln(os.Stderr, "Usage: freshservice-mcp --domain <domain> --api-key-file <path> [flags]")
			fs.PrintDefaults()
		}
		return nil, nil, err
	}
	if c.ShowVersion {
		return &c, nil, nil
	}
	if fs.NArg() > 0 {
		return nil, nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if err := c.LogLevel.UnmarshalText([]byte(logLevel)); err != nil {
		return nil, nil, fmt.Errorf("--log-level: %w", err)
	}

	u, err := baseURL(domain)
	if err != nil {
		return nil, nil, err
	}
	c.BaseURL = u

	switch credDir := getenv("CREDENTIALS_DIRECTORY"); {
	case keyFile != "":
		c.APIKey, err = readSecret(keyFile)
	case getenv("FRESHSERVICE_API_KEY") != "":
		c.APIKey = strings.TrimSpace(getenv("FRESHSERVICE_API_KEY"))
		warnings = append(warnings, "API key read from FRESHSERVICE_API_KEY; prefer --api-key-file, the environment is readable via /proc")
	case credDir != "":
		c.APIKey, err = readSecret(filepath.Join(credDir, "api-key"))
	default:
		err = errors.New("no API key: set --api-key-file, FRESHSERVICE_API_KEY_FILE, FRESHSERVICE_API_KEY or the systemd credential api-key")
	}
	if err != nil {
		return nil, nil, err
	}
	return &c, warnings, nil
}

// baseURL accepts "acme", "acme.freshservice.com" or a full URL. Plain http is
// only allowed to a loopback host (test stubs). Freshservice's API does not
// work through custom CNAMEs, so any other host is taken as given but must be
// https.
func baseURL(domain string) (*url.URL, error) {
	domain = strings.TrimRight(strings.TrimSpace(domain), "/")
	switch {
	case domain == "":
		return nil, errors.New("--domain (or FRESHSERVICE_DOMAIN) is required")
	case !strings.Contains(domain, "://") && !strings.Contains(domain, "."):
		domain = "https://" + domain + ".freshservice.com"
	case !strings.Contains(domain, "://"):
		domain = "https://" + domain
	}
	u, err := url.Parse(strings.TrimSuffix(domain, "/api/v2"))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("--domain must be a Freshservice domain or https URL, got %q", domain)
	}
	if u.Scheme == "http" && !loopback(u.Host) {
		return nil, fmt.Errorf("--domain must use https unless the host is loopback, got %q", domain)
	}
	u.Path = ""
	return u, nil
}

func readSecret(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading secret: %w", err)
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", fmt.Errorf("secret file %s is empty", path)
	}
	return s, nil
}

// loopback reports whether addr ("host:port" or a bare host) is loopback.
func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = strings.Trim(addr, "[]")
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
