// Package provenance records where a piece of content came from, as a small
// transport-neutral envelope a higher layer can act on.
//
// It is policy-free on purpose. An [Envelope] answers "what do we know about
// this content's origin, and how sure are we", and it never answers "is this
// content safe to act on". That second question belongs to the consumer, which
// knows its own trusted actors and its own risk, so nothing here names an
// organization, a forge, a bot account, or an email domain.
//
// The design rule is that ignorance is never mistaken for trust: the zero
// [Verification] is [Unknown] rather than a pass, an envelope missing fields is
// incomplete rather than assumed, and [Envelope.Trusted] requires both a
// complete envelope and an affirmative check. See docs/provenance.md.
package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Verification is how far an origin claim was checked. It describes the claim,
// never the payload. See docs/provenance.md.
type Verification string

const (
	// Unknown is the zero value: no check attempted or recorded, deliberately
	// distinct from Unverified.
	Unknown Verification = ""
	// Unverified records that a check ran and did not establish the claim.
	Unverified Verification = "unverified"
	// Verified records that a check ran and established the claim.
	Verified Verification = "verified"
	// Refuted records that a check ran and contradicted the claim, a stronger
	// signal than Unverified.
	Refuted Verification = "refuted"
)

// Envelope is one origin claim about a piece of content.
type Envelope struct {
	// Actor identifies who produced the content, in whatever namespace the
	// source uses. Opaque here.
	Actor string `json:"actor"`
	// Source names the system the content was observed in, e.g. a forge or a
	// message bus. Opaque here.
	Source string `json:"source"`
	// SourceID identifies the object within Source, e.g. an issue or comment
	// id. Opaque here.
	SourceID string `json:"source_id"`
	// ContentHash pins the exact bytes the claim covers, as returned by
	// [HashContent].
	ContentHash string `json:"content_hash"`
	// ObservedAt is when the content was read, not when it was authored: only
	// the reader's clock is the reader's to trust.
	ObservedAt time.Time `json:"observed_at"`
	// Verification is how far the claim was checked.
	Verification Verification `json:"verification"`
}

// HashContent pins b as "sha256:<hex>", the form [Envelope.ContentHash] takes.
func HashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Complete reports whether every field carries a value, naming all the missing
// ones at once rather than the first.
func (e Envelope) Complete() error {
	var missing []string
	if e.Actor == "" {
		missing = append(missing, "actor")
	}
	if e.Source == "" {
		missing = append(missing, "source")
	}
	if e.SourceID == "" {
		missing = append(missing, "source_id")
	}
	if e.ContentHash == "" {
		missing = append(missing, "content_hash")
	}
	if e.ObservedAt.IsZero() {
		missing = append(missing, "observed_at")
	}
	if e.Verification == Unknown {
		missing = append(missing, "verification")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("provenance: incomplete envelope, missing %s", strings.Join(missing, ", "))
}

// CoversContent reports whether e's hash matches b, so the claim covers the
// bytes in hand rather than an earlier revision of the same object.
func (e Envelope) CoversContent(b []byte) error {
	if e.ContentHash == "" {
		return errors.New("provenance: envelope carries no content hash")
	}
	if got := HashContent(b); got != e.ContentHash {
		return fmt.Errorf("provenance: content hash mismatch (envelope %s, content %s)", e.ContentHash, got)
	}
	return nil
}

// Trusted reports whether the envelope is complete and affirmatively verified.
// It is input to a consumer's trust decision, not the decision.
func (e Envelope) Trusted() bool {
	return e.Complete() == nil && e.Verification == Verified
}

// String renders one audit-log line, verification first so a scan down the
// column shows the weak rows.
func (e Envelope) String() string {
	v := e.Verification
	if v == Unknown {
		v = "unknown"
	}
	return fmt.Sprintf("provenance=%s actor=%q source=%q source_id=%q hash=%s observed=%s",
		v, e.Actor, e.Source, e.SourceID, e.ContentHash, e.ObservedAt.UTC().Format(time.RFC3339))
}
