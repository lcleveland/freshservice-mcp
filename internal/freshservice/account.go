package freshservice

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Workspace is one entry of GET /api/v2/workspaces.
type Workspace struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Primary    bool   `json:"primary"`
	Restricted bool   `json:"restricted"`
	State      string `json:"state,omitempty"`
}

// Asset API flavors: accounts created before 2026-03-31 have the classic
// CMDB API, newer ones only ITAM.
const (
	AssetsClassic = "/api/v2/assets"
	AssetsITAM    = "/api/v2/itam/assets"
)

// Account is what the server learns about the account on first use.
type Account struct {
	Workspaces []Workspace
	Default    *Workspace // nil when the account lists no workspaces
	AssetPath  string     // AssetsClassic, AssetsITAM, or "" if neither answered
}

type accountCache struct {
	mu   sync.Mutex
	acct *Account
}

// Account discovers the workspaces, resolves the default workspace (want is
// an id or name; empty means the primary) and detects the asset API. A
// success is cached; a failure is retried on the next call, so the server can
// start while Freshservice is unreachable.
func (c *Client) Account(ctx context.Context, want string) (*Account, error) {
	c.acct.mu.Lock()
	defer c.acct.mu.Unlock()
	if c.acct.acct != nil {
		return c.acct.acct, nil
	}
	var body struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	resp, err := c.Do(ctx, http.MethodGet, "/api/v2/workspaces", url.Values{"per_page": {"100"}}, nil)
	var ae *APIError
	switch {
	case errors.As(err, &ae) && (ae.Status == http.StatusNotFound || ae.Status == http.StatusForbidden) && want == "":
		// No workspaces feature, or the agent may not list them: send no
		// workspace_id and let Freshservice use the primary.
	case err != nil:
		return nil, err
	default:
		if err := decodeInto(resp, &body); err != nil {
			return nil, err
		}
	}
	a := &Account{Workspaces: body.Workspaces}
	if a.Default, err = pick(a.Workspaces, want); err != nil {
		return nil, err
	}
	if a.AssetPath, err = c.assetPath(ctx); err != nil {
		return nil, err
	}
	c.acct.acct = a
	return a, nil
}

func pick(ws []Workspace, want string) (*Workspace, error) {
	for i := range ws {
		w := &ws[i]
		if want == "" && w.Primary || want != "" && (strconv.FormatInt(w.ID, 10) == want || strings.EqualFold(w.Name, want)) {
			return w, nil
		}
	}
	if want == "" {
		return nil, nil
	}
	names := make([]string, len(ws))
	for i, w := range ws {
		names[i] = fmt.Sprintf("%d %s", w.ID, w.Name)
	}
	return nil, fmt.Errorf("default workspace %q not found; the API key's agent sees: %s", want, strings.Join(names, ", "))
}

// assetPath probes classic, then ITAM, with one record each. A 404 or
// require_feature means "not this one"; anything else is a real error.
func (c *Client) assetPath(ctx context.Context) (string, error) {
	for _, p := range []string{AssetsClassic, AssetsITAM} {
		_, err := c.Do(ctx, http.MethodGet, p, url.Values{"per_page": {"1"}}, nil)
		var ae *APIError
		switch {
		case err == nil:
			return p, nil
		case errors.As(err, &ae) && (ae.Status == http.StatusNotFound || ae.Has("require_feature") || ae.Status == http.StatusForbidden):
			continue
		default:
			return "", err
		}
	}
	return "", nil
}
