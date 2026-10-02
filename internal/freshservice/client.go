// Package freshservice is a small REST client for the Freshservice API v2:
// Basic auth with an agent API key, GET-only retry with backoff, and errors
// that tell the model what to do next.
package freshservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	base   *url.URL
	apiKey string
	hc     *http.Client
	log    *slog.Logger

	mu   sync.Mutex
	rate RateLimit

	acct accountCache

	// Retry tuning; tests shrink these.
	Attempts  int
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// RateLimit is the last X-Ratelimit-* seen. Freshservice counts per minute,
// account-wide, shared with every other app using the account.
type RateLimit struct {
	Total      int       `json:"total"`
	Remaining  int       `json:"remaining"`
	UsedByLast int       `json:"used_by_last_request"`
	Seen       time.Time `json:"seen"`
}

// New builds a client. hc may be nil; its Timeout is the per-request timeout.
func New(base *url.URL, apiKey string, hc *http.Client, log *slog.Logger) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Client{base: base, apiKey: apiKey, hc: hc, log: log,
		Attempts: 4, BaseDelay: 500 * time.Millisecond, MaxDelay: 8 * time.Second}
}

func (c *Client) BaseURL() string { return c.base.String() }

// Rate returns the most recent rate-limit headers (zero if none seen yet).
func (c *Client) Rate() RateLimit {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rate
}

// Response is a successful (2xx) reply.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Decode unmarshals the body; an empty body (204) decodes to nil.
func (r *Response) Decode() (any, error) {
	if len(bytes.TrimSpace(r.Body)) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(r.Body, &v); err != nil {
		return nil, fmt.Errorf("decoding Freshservice response: %w", err)
	}
	return v, nil
}

func decodeInto(r *Response, v any) error {
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("decoding Freshservice response: %w", err)
	}
	return nil
}

// Do sends one API request. path starts with /api/v2/. body is JSON-encoded
// when non-nil. Non-2xx replies return *APIError.
//
// Only GETs are retried: Freshservice writes take no idempotency key, so a
// retried POST can create a second ticket or send a reply twice.
func (c *Client) Do(ctx context.Context, method, path string, q url.Values, body any) (*Response, error) {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
	}
	u := strings.TrimRight(c.base.String(), "/") + path
	if len(q) > 0 {
		u += "?" + strings.ReplaceAll(q.Encode(), "+", "%20")
	}
	retryable := method == http.MethodGet

	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(c.apiKey, "X")
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.hc.Do(req)
		if err != nil {
			if ctx.Err() == nil && retryable && attempt < c.Attempts {
				if werr := c.wait(ctx, attempt, ""); werr != nil {
					return nil, werr
				}
				continue
			}
			return nil, fmt.Errorf("%s %s: %s", method, path, c.scrub(err.Error()))
		}
		b, rerr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		if rerr != nil {
			return nil, fmt.Errorf("%s %s: reading response: %w", method, path, rerr)
		}
		c.noteRate(resp.Header)
		c.log.Debug("freshservice request", "method", method, "path", path, "status", resp.StatusCode)

		if resp.StatusCode/100 == 2 {
			return &Response{Status: resp.StatusCode, Header: resp.Header, Body: b}, nil
		}
		retry := resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode != 501)
		if retry && retryable && attempt < c.Attempts {
			if werr := c.wait(ctx, attempt, resp.Header.Get("Retry-After")); werr != nil {
				return nil, werr
			}
			continue
		}
		return nil, newAPIError(method, path, resp, b, retryable, c.scrub)
	}
}

func (c *Client) noteRate(h http.Header) {
	total, err := strconv.Atoi(h.Get("X-Ratelimit-Total"))
	if err != nil {
		return
	}
	rem, _ := strconv.Atoi(h.Get("X-Ratelimit-Remaining"))
	used, _ := strconv.Atoi(h.Get("X-Ratelimit-Used-CurrentRequest"))
	c.mu.Lock()
	c.rate = RateLimit{Total: total, Remaining: rem, UsedByLast: used, Seen: time.Now()}
	c.mu.Unlock()
}

// wait sleeps for Retry-After if given (capped at a minute: the limit is per
// minute), else full-jitter exponential backoff.
func (c *Client) wait(ctx context.Context, attempt int, retryAfter string) error {
	d := c.BaseDelay << (attempt - 1)
	if d > c.MaxDelay || d <= 0 {
		d = c.MaxDelay
	}
	d = time.Duration(rand.Int64N(int64(d) + 1))
	if s, err := strconv.Atoi(retryAfter); err == nil && s >= 0 {
		d = min(time.Duration(s)*time.Second, 60*time.Second)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// scrub hides the API key.
func (c *Client) scrub(s string) string {
	if len(c.apiKey) >= 4 {
		s = strings.ReplaceAll(s, c.apiKey, "[redacted]")
	}
	return s
}
