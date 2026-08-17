package provenance_test

import (
	"strings"
	"testing"
	"time"

	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/pkg/provenance"
)

// complete returns an envelope with every field populated, so each test can
// blank exactly the one field it is about.
func complete() provenance.Envelope {
	return provenance.Envelope{
		Actor:        "actor",
		Source:       "source",
		SourceID:     "42",
		ContentHash:  provenance.HashContent([]byte("body")),
		ObservedAt:   time.Unix(1, 0),
		Verification: provenance.Verified,
	}
}

func TestZeroEnvelopeIsIncompleteAndUntrusted(t *testing.T) {
	var e provenance.Envelope
	err := e.Complete()
	if err == nil {
		t.Fatal("zero envelope must not report complete")
	}
	for _, field := range []string{"actor", "source", "source_id", "content_hash", "observed_at", "verification"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("missing-field list omits %q: %v", field, err)
		}
	}
	if e.Trusted() {
		t.Fatal("zero envelope must not be trusted")
	}
}

func TestCompleteEnvelope(t *testing.T) {
	if err := complete().Complete(); err != nil {
		t.Fatalf("want complete, got %v", err)
	}
	if !complete().Trusted() {
		t.Fatal("complete + verified envelope should be trusted")
	}
}

func TestEachMissingFieldIsNamed(t *testing.T) {
	cases := map[string]func(*provenance.Envelope){
		"actor":        func(e *provenance.Envelope) { e.Actor = "" },
		"source":       func(e *provenance.Envelope) { e.Source = "" },
		"source_id":    func(e *provenance.Envelope) { e.SourceID = "" },
		"content_hash": func(e *provenance.Envelope) { e.ContentHash = "" },
		"observed_at":  func(e *provenance.Envelope) { e.ObservedAt = time.Time{} },
		"verification": func(e *provenance.Envelope) { e.Verification = provenance.Unknown },
	}
	for field, blank := range cases {
		t.Run(field, func(t *testing.T) {
			e := complete()
			blank(&e)
			err := e.Complete()
			if err == nil {
				t.Fatalf("blanking %s should make the envelope incomplete", field)
			}
			if !strings.Contains(err.Error(), field) {
				t.Errorf("error does not name %q: %v", field, err)
			}
			if e.Trusted() {
				t.Errorf("incomplete envelope (missing %s) must not be trusted", field)
			}
		})
	}
}

// An unverified or refuted claim is complete but must never read as trusted:
// this is the "explicit rather than silently trusted" rule.
func TestOnlyVerifiedIsTrusted(t *testing.T) {
	for _, v := range []provenance.Verification{provenance.Unverified, provenance.Refuted} {
		e := complete()
		e.Verification = v
		if err := e.Complete(); err != nil {
			t.Fatalf("%s envelope should still be complete: %v", v, err)
		}
		if e.Trusted() {
			t.Errorf("%s envelope must not be trusted", v)
		}
	}
}

func TestCoversContent(t *testing.T) {
	body := []byte("body")
	e := complete()
	if err := e.CoversContent(body); err != nil {
		t.Fatalf("want match, got %v", err)
	}
	if err := e.CoversContent([]byte("tampered")); err == nil {
		t.Fatal("want mismatch on different content")
	}
	e.ContentHash = ""
	if err := e.CoversContent(body); err == nil {
		t.Fatal("want error when the envelope carries no hash")
	}
}

func TestHashContentIsStableAndPrefixed(t *testing.T) {
	h := provenance.HashContent([]byte("body"))
	if !strings.HasPrefix(h, "sha256:") {
		t.Errorf("hash is not algorithm-prefixed: %s", h)
	}
	if h != provenance.HashContent([]byte("body")) {
		t.Error("hash is not stable across calls")
	}
	if h == provenance.HashContent([]byte("other")) {
		t.Error("distinct content produced the same hash")
	}
}

func TestStringNamesUnknownVerification(t *testing.T) {
	var e provenance.Envelope
	if got := e.String(); !strings.Contains(got, "provenance=unknown") {
		t.Errorf("zero verification should render as unknown, got %s", got)
	}
}
