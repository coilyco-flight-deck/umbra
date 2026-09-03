package execverb

import (
	"strings"
	"testing"
)

// TestEmptyAnswerCoversTheDeclaredSet pins the whole declared set rather than
// sampling it.
func TestEmptyAnswerCoversTheDeclaredSet(t *testing.T) {
	for _, tc := range []struct {
		name  string
		raw   string
		empty bool
	}{
		{"no bytes", "", true},
		{"whitespace only", "  \n\t ", true},
		{"json null", "null", true},
		{"empty string", `""`, true},
		{"whitespace string", `"   "`, true},
		{"empty array", "[]", true},
		{"empty object", "{}", true},
		{"false is an answer", "false", false},
		{"zero is an answer", "0", false},
		{"prose is an answer", "no matching records", false},
		{"populated array", `[1]`, false},
		{"populated object", `{"a":1}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := emptyAnswer([]byte(tc.raw)); got != tc.empty {
				t.Fatalf("emptyAnswer(%q) = %v, want %v", tc.raw, got, tc.empty)
			}
		})
	}
}

// TestRejectEmptyDivergesFromFailWhenOnFalse: if this ever agrees with JMESPath
// truthiness, the control is sugar and should be deleted rather than kept.
func TestRejectEmptyDivergesFromFailWhenOnFalse(t *testing.T) {
	if emptyAnswer([]byte("false")) {
		t.Fatal("`false` was treated as empty, which is JMESPath truthiness rather than this control's rule")
	}
}

// TestRejectEmptyIsOffByDefault: without it, a green suite cannot tell
// enforcement from a control that fires on everything.
func TestRejectEmptyIsOffByDefault(t *testing.T) {
	if err := applyRejectEmpty(execAction{Name: "list"}, nil); err != nil {
		t.Fatalf("undeclared action refused an empty answer: %v", err)
	}
}

func TestRejectEmptyRefusesAndNamesTheAction(t *testing.T) {
	err := applyRejectEmpty(execAction{Name: "list", RejectEmpty: true}, []byte("[]"))
	if err == nil {
		t.Fatal("an empty answer was returned rather than refused")
	}
	if !strings.Contains(err.Error(), "list") {
		t.Fatalf("refusal does not name the action: %v", err)
	}
}

func TestRejectEmptyPassesAnAnswerThrough(t *testing.T) {
	if err := applyRejectEmpty(execAction{Name: "list", RejectEmpty: true}, []byte(`[{"id":1}]`)); err != nil {
		t.Fatalf("a real answer was refused: %v", err)
	}
}
