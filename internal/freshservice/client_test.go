package freshservice

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func stub(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	c := New(u, "s3cretkey", srv.Client(), nil)
	c.BaseDelay, c.MaxDelay = time.Millisecond, time.Millisecond
	return c
}

func TestBasicAuthAndRateHeaders(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "s3cretkey" || p != "X" {
			t.Errorf("auth = %q %q %v", u, p, ok)
		}
		w.Header().Set("X-Ratelimit-Total", "500")
		w.Header().Set("X-Ratelimit-Remaining", "498")
		w.Header().Set("X-Ratelimit-Used-CurrentRequest", "1")
		io.WriteString(w, `{}`)
	})
	if _, err := c.Do(context.Background(), "GET", "/api/v2/tickets", nil, nil); err != nil {
		t.Fatal(err)
	}
	if r := c.Rate(); r.Total != 500 || r.Remaining != 498 || r.UsedByLast != 1 {
		t.Errorf("rate = %+v", r)
	}
}

func TestRetryAfter429OnlyForGET(t *testing.T) {
	n := 0
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 || r.Method == "POST" {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		io.WriteString(w, `{}`)
	})
	if _, err := c.Do(context.Background(), "GET", "/api/v2/tickets", nil, nil); err != nil || n != 2 {
		t.Fatalf("GET: err=%v calls=%d", err, n)
	}
	n = 10
	_, err := c.Do(context.Background(), "POST", "/api/v2/tickets", nil, map[string]any{"subject": "x"})
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 429 || n != 11 || !strings.Contains(ae.Hint, "never retried") {
		t.Fatalf("POST: err=%v calls=%d", err, n)
	}
}

func TestErrorMapping(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		io.WriteString(w, `{"description":"Validation failed","errors":[{"field":"email","message":"It should be a valid email address","code":"invalid_value"}]}`)
	})
	_, err := c.Do(context.Background(), "POST", "/api/v2/requesters", nil, map[string]any{})
	var ae *APIError
	if !errors.As(err, &ae) || !ae.Has("invalid_value") {
		t.Fatalf("err = %v", err)
	}
	if s := err.Error(); !strings.Contains(s, "email: It should be a valid email address") || !strings.Contains(s, "Hint:") {
		t.Errorf("message = %s", s)
	}
}

func TestKeyScrubbed(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, `{"description":"bad key s3cretkey"}`)
	})
	_, err := c.Do(context.Background(), "GET", "/api/v2/tickets", nil, nil)
	if err == nil || strings.Contains(err.Error(), "s3cretkey") {
		t.Fatalf("err = %v", err)
	}
}

func TestAccountDiscovery(t *testing.T) {
	calls := 0
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/api/v2/workspaces":
			io.WriteString(w, `{"workspaces":[{"id":2,"name":"IT","primary":true},{"id":3,"name":"HR"}]}`)
		case AssetsClassic:
			w.WriteHeader(404)
			io.WriteString(w, `{"description":"not found"}`)
		case AssetsITAM:
			io.WriteString(w, `{"assets":[]}`)
		}
	})
	a, err := c.Account(context.Background(), "")
	if err != nil || a.Default.ID != 2 || a.AssetPath != AssetsITAM {
		t.Fatalf("account = %+v, %v", a, err)
	}
	c.Account(context.Background(), "")
	if calls != 3 {
		t.Errorf("not cached: %d calls", calls)
	}

	c.acct.acct = nil
	if a, _ := c.Account(context.Background(), "hr"); a.Default.ID != 3 {
		t.Errorf("by name: %+v", a.Default)
	}
	c.acct.acct = nil
	if _, err := c.Account(context.Background(), "Facilities"); err == nil || !strings.Contains(err.Error(), "3 HR") {
		t.Errorf("unknown: %v", err)
	}
}
