package opcore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const authNoneSpec = `wrap ward mcp public {
    base-url "%BASE%"
    auth none
    can get thing {
        path "/things"
    }
}`

// A placeholder credential is a wrong Authorization header, which some
// upstreams 403. See docs/specverb-auth-none.md.
func TestAuthNoneSendsNoAuthorizationHeader(t *testing.T) {
	var got string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	_, cfg, err := ParseInline([]byte(strings.Replace(authNoneSpec, "%BASE%", upstream.URL, 1)))
	if err != nil {
		t.Fatalf("ParseInline: %v", err)
	}
	rt := NewRuntime(cfg)
	rt.Client = upstream.Client()
	if _, _, err := rt.send(context.Background(), http.MethodGet, upstream.URL+"/things", nil, ""); err != nil {
		t.Fatalf("send: %v", err)
	}
	if got != "" {
		t.Fatalf("auth none sent an Authorization header: %q", got)
	}
}

// The block stays required. A spec that simply omits it is a spec that forgot,
// and `none` is how an author says the omission is deliberate.
func TestMissingAuthStillFailsClosed(t *testing.T) {
	_, _, err := ParseInline([]byte(`wrap ward mcp public {
    base-url "example.invalid"
    can get thing {
        path "/things"
    }
}`))
	if err == nil {
		t.Fatalf("ParseInline accepted a spec with no auth block")
	}
	if !strings.Contains(err.Error(), "`auth` block is required") {
		t.Fatalf("error = %q, want it to name the required auth block", err)
	}
}

func TestAuthNoneRejectsABlock(t *testing.T) {
	_, _, err := ParseInline([]byte(`wrap ward mcp public {
    base-url "example.invalid"
    auth none {
        value literal "contradiction"
    }
    can get thing {
        path "/things"
    }
}`))
	if err == nil {
		t.Fatalf("ParseInline accepted `auth none` carrying a value")
	}
	if !strings.Contains(err.Error(), "takes no block") {
		t.Fatalf("error = %q, want it to reject the block", err)
	}
}
