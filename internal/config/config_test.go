package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func keyFile(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDomainForms(t *testing.T) {
	for in, want := range map[string]string{
		"acme":                                 "https://acme.freshservice.com",
		"acme.freshservice.com":                "https://acme.freshservice.com",
		"https://acme.freshservice.com/":       "https://acme.freshservice.com",
		"https://acme.freshservice.com/api/v2": "https://acme.freshservice.com",
		"http://127.0.0.1:8080":                "http://127.0.0.1:8080",
	} {
		c, _, err := Parse(nil, env(map[string]string{"FRESHSERVICE_DOMAIN": in, "FRESHSERVICE_API_KEY_FILE": keyFile(t, "k")}))
		if err != nil || c.BaseURL.String() != want {
			t.Errorf("%q: got %v, %v; want %s", in, c, err, want)
		}
	}
	for _, bad := range []string{"", "http://acme.freshservice.com", "ftp://x.y"} {
		if _, _, err := Parse([]string{"--domain", bad}, env(map[string]string{"FRESHSERVICE_API_KEY": "k"})); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func TestKeyPrecedence(t *testing.T) {
	cred := t.TempDir()
	os.WriteFile(filepath.Join(cred, "api-key"), []byte("from-cred\n"), 0o600)
	base := map[string]string{"FRESHSERVICE_DOMAIN": "acme", "CREDENTIALS_DIRECTORY": cred}

	c, w, err := Parse(nil, env(base))
	if err != nil || c.APIKey != "from-cred" || len(w) != 0 {
		t.Fatalf("credential: %v %v %v", c, w, err)
	}

	base["FRESHSERVICE_API_KEY"] = "from-env"
	c, w, _ = Parse(nil, env(base))
	if c.APIKey != "from-env" || len(w) != 1 {
		t.Fatalf("env: %q %v", c.APIKey, w)
	}

	c, _, _ = Parse([]string{"--api-key-file", keyFile(t, "  from-file\n")}, env(base))
	if c.APIKey != "from-file" {
		t.Fatalf("file: %q", c.APIKey)
	}

	if _, _, err := Parse(nil, env(map[string]string{"FRESHSERVICE_DOMAIN": "acme"})); err == nil {
		t.Fatal("no key: want error")
	}
	if _, _, err := Parse([]string{"--api-key-file", keyFile(t, "\n")}, env(base)); err == nil {
		t.Fatal("empty key file: want error")
	}
}

func TestLogValueHidesKey(t *testing.T) {
	c, _, _ := Parse(nil, env(map[string]string{"FRESHSERVICE_DOMAIN": "acme", "FRESHSERVICE_API_KEY": "s3cretkey"}))
	if strings.Contains(c.LogValue().String(), "s3cretkey") {
		t.Fatal("key leaked into LogValue")
	}
}

func TestHTTPNeedsTokenOffLoopback(t *testing.T) {
	base := map[string]string{"FRESHSERVICE_DOMAIN": "acme", "FRESHSERVICE_API_KEY_FILE": keyFile(t, "k")}
	if _, _, err := Parse([]string{"--http", "--addr", "0.0.0.0:8234"}, env(base)); err == nil {
		t.Error("non-loopback without token accepted")
	}
	c, _, err := Parse([]string{"--http", "--addr", "0.0.0.0:8234", "--http-auth-token-file", keyFile(t, "tok\n")}, env(base))
	if err != nil || c.HTTPAuthToken != "tok" {
		t.Errorf("with token: %v %v", c, err)
	}
	if _, _, err := Parse([]string{"--http"}, env(base)); err != nil {
		t.Errorf("loopback default: %v", err)
	}
	if _, _, err := Parse([]string{"--http", "--stdio"}, env(base)); err == nil {
		t.Error("--http --stdio accepted")
	}
}
