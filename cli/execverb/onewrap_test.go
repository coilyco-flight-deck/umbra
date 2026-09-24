package execverb

import (
	"strings"
	"testing"
)

// umbra#8163: a second wrap used to be dropped without a word.
func TestParseRefusesASecondWrap(t *testing.T) {
	_, err := Parse([]byte("wrap w git {\n}\nwrap w go {\n}\n"))
	if err == nil || !strings.Contains(err.Error(), "2 top-level `wrap` nodes") {
		t.Fatalf("err = %v, want the second wrap refused", err)
	}
}
