package opcore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Some APIs refuse an unnamed client outright rather than rate-limit it.
// Measurements in docs/specverb-user-agent.md (umbra#303).
func TestSendNamesItself(t *testing.T) {
	var got string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	rt := &Runtime{Client: upstream.Client()}
	if _, _, err := rt.send(context.Background(), http.MethodGet, upstream.URL, nil, ""); err != nil {
		t.Fatalf("send: %v", err)
	}
	if got != DefaultUserAgent {
		t.Fatalf("User-Agent = %q, want %q", got, DefaultUserAgent)
	}
	if strings.HasPrefix(got, "Go-http-client") {
		t.Fatalf("request went out as the Go default, which reddit blocks: %q", got)
	}
}
