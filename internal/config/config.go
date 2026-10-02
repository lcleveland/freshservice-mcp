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
	"slices"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL          *url.URL // https://<sub>.freshservice.com, no /api/v2
	APIKey           string
	DefaultWorkspace string          // id or name; empty means the primary workspace
	MaxRecords       int             // cap on records one list call may return
	Groups           []string        // tool groups to register; empty means all
	MaxBuckets       int             // most API calls one summary may fan out to
	Allow            map[string]bool // capability -> enabled

	HTTP           bool
	Addr           string
	Path           string
	HTTPAuthToken  string
	RequestTimeout time.Duration
	LogLevel       slog.Level

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
		slog.Int("max_records", c.MaxRecords),
		slog.Any("groups", c.Groups),
		slog.Any("capabilities", c.Enabled()),
		slog.Bool("http", c.HTTP),
		slog.String("addr", c.Addr),
		slog.Bool("http_auth_set", c.HTTPAuthToken != ""),
	)
}

// Capabilities are the opt-in write classes, each enabled by --allow-<name>.
// See docs/adr/0002-capability-map.md.
var Capabilities = []string{"tickets", "ticket-replies", "itil", "assets", "knowledge", "projects", "people", "approvals", "ops", "custom-objects"}

// Groups are the tool groups --tool-groups may name. core is always on.
var Groups = []string{"core", "itil", "assets", "knowledge", "catalog", "projects", "ops", "custom"}

// Enabled lists the enabled capabilities, in declaration order.
func (c *Config) Enabled() []string {
	var on []string
	for _, n := range Capabilities {
		if c.Allow[n] {
			on = append(on, n)
		}
	}
	return on
}

// GroupOn reports whether a tool group is on.
func (c *Config) GroupOn(group string) bool {
	return group == "core" || len(c.Groups) == 0 || slices.Contains(c.Groups, group)
}

// Parse reads args and the environment. getenv is injected for tests.
// Warnings (non-fatal) are returned for the caller to log once a logger exists.
func Parse(args []string, getenv func(string) string) (*Config, []string, error) {
	var (
		c                         Config
		domain, keyFile, logLevel string
		groups, hauth             string
		stdio                     bool
		warnings                  []string
	)
	fs := flag.NewFlagSet("freshservice-mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	allow := map[string]*bool{}
	for _, name := range Capabilities {
		allow[name] = fs.Bool("allow-"+name, false, "enable the "+name+" write capability")
	}
	fs.StringVar(&domain, "domain", getenv("FRESHSERVICE_DOMAIN"), "Freshservice domain: acme, acme.freshservice.com or a full URL (env FRESHSERVICE_DOMAIN)")
	fs.StringVar(&keyFile, "api-key-file", getenv("FRESHSERVICE_API_KEY_FILE"), "file holding the agent API key (env FRESHSERVICE_API_KEY_FILE)")
	fs.StringVar(&c.DefaultWorkspace, "default-workspace", getenv("FRESHSERVICE_DEFAULT_WORKSPACE"), "workspace id or name used when a call names none (default: the primary workspace) (env FRESHSERVICE_DEFAULT_WORKSPACE)")
	fs.IntVar(&c.MaxRecords, "max-records", atoi(getenv("FRESHSERVICE_MAX_RECORDS"), 2000), "most records one list call may return (env FRESHSERVICE_MAX_RECORDS)")
	fs.IntVar(&c.MaxBuckets, "max-buckets", atoi(getenv("FRESHSERVICE_MAX_BUCKETS"), 60), "most API calls one summary (group_by, backlog, trend) may make (env FRESHSERVICE_MAX_BUCKETS)")
	fs.StringVar(&groups, "tool-groups", getenv("FRESHSERVICE_TOOL_GROUPS"), "comma-separated tool groups to enable (default all): "+strings.Join(Groups, ",")+" (env FRESHSERVICE_TOOL_GROUPS)")
	fs.DurationVar(&c.RequestTimeout, "request-timeout", 30*time.Second, "per-request timeout to Freshservice")
	fs.StringVar(&logLevel, "log-level", or(getenv("FRESHSERVICE_MCP_LOG_LEVEL"), "info"), "debug|info|warn|error (env FRESHSERVICE_MCP_LOG_LEVEL)")
	fs.BoolVar(&stdio, "stdio", false, "serve over stdio (default)")
	fs.BoolVar(&c.HTTP, "http", false, "serve over streamable HTTP")
	fs.StringVar(&c.Addr, "addr", "127.0.0.1:8234", "HTTP listen address")
	fs.StringVar(&c.Path, "path", "/mcp", "HTTP MCP endpoint path")
	fs.StringVar(&hauth, "http-auth-token-file", getenv("FRESHSERVICE_MCP_HTTP_AUTH_TOKEN_FILE"), "file holding the bearer token HTTP clients must send (env FRESHSERVICE_MCP_HTTP_AUTH_TOKEN_FILE)")
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
	if stdio && c.HTTP {
		return nil, nil, errors.New("--stdio and --http are mutually exclusive")
	}
	c.Allow = map[string]bool{}
	for name, on := range allow {
		c.Allow[name] = *on
	}
	if err := c.LogLevel.UnmarshalText([]byte(logLevel)); err != nil {
		return nil, nil, fmt.Errorf("--log-level: %w", err)
	}

	if c.MaxRecords < 1 || c.MaxBuckets < 1 {
		return nil, nil, errors.New("--max-records and --max-buckets must be at least 1")
	}

	if groups != "" {
		for g := range strings.SplitSeq(groups, ",") {
			g = strings.TrimSpace(g)
			if !slices.Contains(Groups, g) {
				return nil, nil, fmt.Errorf("--tool-groups: unknown group %q (want %s)", g, strings.Join(Groups, ","))
			}
			c.Groups = append(c.Groups, g)
		}
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

	if c.HTTP {
		if !strings.HasPrefix(c.Path, "/") {
			return nil, nil, errors.New("--path must start with /")
		}
		credDir := getenv("CREDENTIALS_DIRECTORY")
		switch {
		case hauth != "":
			c.HTTPAuthToken, err = readSecret(hauth)
		case credDir != "":
			if s, e := readSecret(filepath.Join(credDir, "http-auth-token")); e == nil {
				c.HTTPAuthToken = s
			}
		}
		if err != nil {
			return nil, nil, err
		}
		if c.HTTPAuthToken == "" && !loopback(c.Addr) {
			return nil, nil, fmt.Errorf("refusing to listen on non-loopback %s without --http-auth-token-file", c.Addr)
		}
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

func atoi(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
