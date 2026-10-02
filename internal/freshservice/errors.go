package freshservice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const maxErrBody = 2 << 10

// APIError is a non-2xx reply from Freshservice. Error() is written for the
// model: what failed, Freshservice's own words, and what to try next.
type APIError struct {
	Status int
	Method string
	Path   string // never includes the query string
	Detail string // description plus each field error, capped at 2 KiB
	Codes  []string
	Hint   string
}

func (e *APIError) Error() string {
	s := fmt.Sprintf("Freshservice %s %s: HTTP %d", e.Method, e.Path, e.Status)
	if e.Detail != "" {
		s += ": " + e.Detail
	}
	if e.Hint != "" {
		s += "\nHint: " + e.Hint
	}
	return s
}

// Has reports whether Freshservice sent the given error code.
func (e *APIError) Has(code string) bool {
	for _, c := range e.Codes {
		if c == code {
			return true
		}
	}
	return false
}

func newAPIError(method, path string, resp *http.Response, body []byte, retried bool, scrub func(string) string) *APIError {
	e := &APIError{Status: resp.StatusCode, Method: method, Path: path}
	e.decode(body)
	e.Detail = scrub(e.Detail)
	if len(e.Detail) > maxErrBody {
		e.Detail = e.Detail[:maxErrBody] + "…"
	}
	e.Hint = hint(e, retried)
	return e
}

// decode reads {description, errors: [{field, message, code}]}, falling back
// to the raw body for anything else (gateway pages, plain text).
func (e *APIError) decode(body []byte) {
	var b struct {
		Description string `json:"description"`
		Message     string `json:"message"`
		Code        string `json:"code"`
		Errors      []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &b) != nil {
		if s := strings.TrimSpace(string(body)); strings.HasPrefix(s, "<") {
			e.Detail = "(HTML page instead of JSON)"
		} else {
			e.Detail = s
		}
		return
	}
	parts := []string{}
	if d := strings.TrimSpace(b.Description + " " + b.Message); d != "" {
		parts = append(parts, d)
	}
	if b.Code != "" {
		e.Codes = append(e.Codes, b.Code)
	}
	for _, fe := range b.Errors {
		p := fe.Message
		if fe.Field != "" {
			p = fe.Field + ": " + p
		}
		if fe.Code != "" {
			p += " (" + fe.Code + ")"
			e.Codes = append(e.Codes, fe.Code)
		}
		parts = append(parts, p)
	}
	e.Detail = strings.Join(parts, "; ")
	if e.Detail == "" {
		e.Detail = strings.TrimSpace(string(body))
	}
}

func hint(e *APIError, retried bool) string {
	switch {
	case e.Status == http.StatusTooManyRequests && retried:
		return "Freshservice is rate limiting (the limit is per minute and shared with every app on the account); the request was retried after Retry-After and is still throttled. Wait a minute and make fewer, narrower calls."
	case e.Status == http.StatusTooManyRequests:
		return "Freshservice is rate limiting. Writes are never retried automatically: check whether it took effect before trying again."
	case e.Status == http.StatusUnauthorized:
		return "the API key was rejected: check it belongs to an active agent on this domain. freshservice_status checks the connection."
	case e.Has("require_feature"):
		return "this module is not enabled on the Freshservice plan or portal."
	case e.Status == http.StatusForbidden:
		return "the API key's agent lacks permission for this (its role, or a restricted workspace it is not a member of)."
	case e.Status == http.StatusNotFound:
		return "no such object, or it is in a workspace the API key's agent cannot see. List or filter to find the right id."
	case e.Status == http.StatusConflict:
		return "Freshservice refused due to a conflict (for example a duplicate value). Re-read the object and retry."
	case e.Status == http.StatusBadRequest:
		return "Freshservice rejected the request; the message above names the field. Fix the parameters or body and retry."
	case e.Status >= 500:
		return "Freshservice hit an internal error; retry later if it persists."
	}
	return ""
}
