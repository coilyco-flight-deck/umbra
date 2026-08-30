package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// refusingServer answers every request with code and an optional challenge.
func refusingServer(t *testing.T, code int, challenge string) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if challenge != "" {
			w.Header().Set("WWW-Authenticate", challenge)
		}
		w.WriteHeader(code)
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

func TestConnect_RefusalIsActionable(t *testing.T) {
	oauthChallenge := `Bearer realm="mcp", resource_metadata="https://h/.well-known/oauth-protected-resource"`
	cases := []struct {
		name    string
		code    int
		chal    string
		headers map[string]string
		want    []string
		absent  []string
	}{
		{
			// The bare SDK error is the single word "Unauthorized", which does
			// not say whether a credential was even sent. See umbra#340.
			name: "401 with no auth declared",
			code: http.StatusUnauthorized,
			want: []string{"refused the credential (HTTP 401)", "declares no `auth`"},
		},
		{
			name:    "401 with auth declared says the value was rejected",
			code:    http.StatusUnauthorized,
			headers: map[string]string{"Authorization": "Bearer stale"},
			want:    []string{"refused the credential (HTTP 401)", "resolved but was rejected"},
			absent:  []string{"declares no `auth`"},
		},
		{
			// The gap umbra#340 is about: umbra reads a token, it does not go
			// and get one. Saying so beats the operator inferring it.
			name: "an OAuth challenge names the gap and the way round it",
			code: http.StatusUnauthorized,
			chal: oauthChallenge,
			want: []string{"advertises OAuth", "already-minted token", "auth bearer"},
		},
		{
			name: "403 is reported too",
			code: http.StatusForbidden,
			want: []string{"refused the credential (HTTP 403)"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			url := refusingServer(t, c.code, c.chal)
			_, err := Connect(context.Background(), Server{
				Name: "probe",
				HTTP: &HTTPEndpoint{URL: url, Headers: c.headers},
			})
			if err == nil {
				t.Fatal("Connect against a refusing upstream = nil, want an error")
			}
			for _, want := range c.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %v\n  want it to contain %q", err, want)
				}
			}
			for _, absent := range c.absent {
				if strings.Contains(err.Error(), absent) {
					t.Errorf("error = %v\n  should not contain %q", err, absent)
				}
			}
		})
	}
}

func TestConnect_NonAuthFailureIsNotDressedAsOne(t *testing.T) {
	// A 500 is not a credential problem, and saying so would send the operator
	// looking for a token that was never the issue.
	url := refusingServer(t, http.StatusInternalServerError, "")
	_, err := Connect(context.Background(), Server{Name: "probe", HTTP: &HTTPEndpoint{URL: url}})
	if err == nil {
		t.Fatal("Connect = nil, want an error")
	}
	if strings.Contains(err.Error(), "refused the credential") {
		t.Errorf("error = %v, want no credential guidance for a 500", err)
	}
}

func TestWantsOAuth(t *testing.T) {
	for _, c := range []struct {
		challenge string
		want      bool
	}{
		{`Bearer realm="mcp"`, true},
		{`bearer resource_metadata="https://h/.well-known/oauth-protected-resource"`, true},
		{`OAuth realm="x"`, true},
		{`Basic realm="x"`, false},
		{"", false},
	} {
		if got := wantsOAuth(c.challenge); got != c.want {
			t.Errorf("wantsOAuth(%q) = %v, want %v", c.challenge, got, c.want)
		}
	}
}
