package otelsink

import "fmt"

// stringify is the last-resort rendering for an attribute type the projection
// does not currently produce, so a new field degrades rather than disappears.
func stringify(v any) string { return fmt.Sprintf("%v", v) }
