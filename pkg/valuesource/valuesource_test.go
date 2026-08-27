package valuesource

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// providers builds a registry from a map of provider name to a func over the
// address, for terse table tests.
func providers(m map[string]Provider) map[string]Provider { return m }

// TestResolveFirstPrefersFirst proves the first source that yields a non-empty
// value with no error wins, and later sources are never consulted.
func TestResolveFirstPrefersFirst(t *testing.T) {
	var envHit, ssmHit int
	provs := providers(map[string]Provider{
		"env": func(context.Context, string) (string, error) { envHit++; return "from-env", nil },
		"ssm": func(context.Context, string) (string, error) { ssmHit++; return "from-ssm", nil },
	})
	got, err := ResolveFirst(context.Background(), provs, []Source{
		{Provider: "env", Address: "TOKEN"},
		{Provider: "ssm", Address: "/token"},
	})
	if err != nil {
		t.Fatalf("ResolveFirst: %v", err)
	}
	if got != "from-env" {
		t.Errorf("value = %q, want from-env", got)
	}
	if envHit != 1 || ssmHit != 0 {
		t.Errorf("hits env=%d ssm=%d, want the second source untouched", envHit, ssmHit)
	}
}

// TestResolveFirstFallsThroughError proves a source that errors is skipped for the
// next one - the fast-local-then-durable-backup pattern.
func TestResolveFirstFallsThroughError(t *testing.T) {
	provs := providers(map[string]Provider{
		"env": func(context.Context, string) (string, error) { return "", errors.New("not set") },
		"ssm": func(context.Context, string) (string, error) { return "from-ssm", nil },
	})
	got, err := ResolveFirst(context.Background(), provs, []Source{
		{Provider: "env", Address: "TOKEN"},
		{Provider: "ssm", Address: "/token"},
	})
	if err != nil {
		t.Fatalf("ResolveFirst: %v", err)
	}
	if got != "from-ssm" {
		t.Errorf("value = %q, want from-ssm (env errored, ssm backs it)", got)
	}
}

// TestResolveFirstFallsThroughEmpty proves an empty (but error-free) value is a
// failure to resolve: success requires BOTH no error AND a non-empty value.
func TestResolveFirstFallsThroughEmpty(t *testing.T) {
	provs := providers(map[string]Provider{
		"env": func(context.Context, string) (string, error) { return "", nil },
		"ssm": func(context.Context, string) (string, error) { return "from-ssm", nil },
	})
	got, err := ResolveFirst(context.Background(), provs, []Source{
		{Provider: "env", Address: "TOKEN"},
		{Provider: "ssm", Address: "/token"},
	})
	if err != nil {
		t.Fatalf("ResolveFirst: %v", err)
	}
	if got != "from-ssm" {
		t.Errorf("value = %q, want from-ssm (empty env is not a resolution)", got)
	}
}

// TestResolveFirstAllFailCombined proves an all-failed chain returns a combined
// error naming every provider/address tried, and never a resolved value.
func TestResolveFirstAllFailCombined(t *testing.T) {
	provs := providers(map[string]Provider{
		"env": func(context.Context, string) (string, error) { return "SECRET-ENV", errors.New("boom") },
		"ssm": func(context.Context, string) (string, error) { return "", nil },
	})
	got, err := ResolveFirst(context.Background(), provs, []Source{
		{Provider: "env", Address: "TOKEN"},
		{Provider: "ssm", Address: "/token"},
	})
	if err == nil {
		t.Fatal("expected an error when every source fails")
	}
	if got != "" {
		t.Errorf("value = %q, want empty on failure", got)
	}
	msg := err.Error()
	for _, want := range []string{"env TOKEN", "ssm /token", "boom", "resolved empty"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
	// A value returned alongside an error must never leak into the combined message.
	if strings.Contains(msg, "SECRET-ENV") {
		t.Errorf("combined error leaked a resolved value:\n%s", msg)
	}
}

// TestResolveFirstEmptyChain proves an empty chain fails closed rather than
// resolving to a silent empty string.
func TestResolveFirstEmptyChain(t *testing.T) {
	if _, err := ResolveFirst(context.Background(), nil, nil); err == nil {
		t.Fatal("expected an error for an empty chain")
	}
}

// TestResolveFirstMissingProvider proves an unregistered provider is a failure,
// falling through to a registered backup.
func TestResolveFirstMissingProvider(t *testing.T) {
	provs := providers(map[string]Provider{
		"ssm": func(context.Context, string) (string, error) { return "from-ssm", nil },
	})
	got, err := ResolveFirst(context.Background(), provs, []Source{
		{Provider: "vault", Address: "secret/token"},
		{Provider: "ssm", Address: "/token"},
	})
	if err != nil {
		t.Fatalf("ResolveFirst: %v", err)
	}
	if got != "from-ssm" {
		t.Errorf("value = %q, want from-ssm (unregistered vault skipped)", got)
	}
}

// The newline an editor left in an uploaded credential survives every layer
// down to the env var, and Go then refuses to write the header (#304).
func TestBuiltinEnvTrimsSoACredentialCanReachAnAuthHeader(t *testing.T) {
	t.Setenv("UMBRA_TEST_TOKEN", "  abc\n")
	got, err := Builtins()["env"](context.Background(), "UMBRA_TEST_TOKEN")
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	if got != "abc" {
		t.Fatalf("env resolved %q, want the value trimmed like file", got)
	}

	seen := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("Authorization")
	}))
	defer srv.Close()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Authorization", "Bot "+got)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("the resolved value must be writable as a header: %v", err)
	}
	_ = resp.Body.Close()
	if header := <-seen; header != "Bot abc" {
		t.Fatalf("upstream saw %q", header)
	}
}

// Whitespace-only resolves empty rather than to a blank that reads as a
// value, so ResolveFirst falls through to the next source in the chain.
func TestBuiltinEnvResolvesWhitespaceOnlyToEmpty(t *testing.T) {
	t.Setenv("UMBRA_TEST_TOKEN", " \n\t ")
	got, err := Builtins()["env"](context.Background(), "UMBRA_TEST_TOKEN")
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	if got != "" {
		t.Fatalf("env resolved %q, want empty", got)
	}
}

// literal keeps every byte: it is author-supplied in the guardfile rather than
// arriving through a store, so trailing space there is visible in review.
func TestBuiltinLiteralKeepsWhatTheAuthorWrote(t *testing.T) {
	got, err := Builtins()["literal"](context.Background(), " spaced ")
	if err != nil {
		t.Fatalf("literal: %v", err)
	}
	if got != " spaced " {
		t.Fatalf("literal resolved %q, want it untouched", got)
	}
}
