package harness

import (
	"strings"
	"testing"
)

func TestContinualSummary(t *testing.T) {
	idle := ContinualSummary(Continual{})
	if idle != "continual idle" {
		t.Fatalf("idle = %q", idle)
	}
	c := Continual{Goal: "ship", Autonomous: true, Gates: []string{"go test ./..."}, Memories: []string{"note"}}
	got := ContinualSummary(c)
	for _, want := range []string{"goal", "auto×", "1 gates", "1 memories"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary %q missing %q", got, want)
		}
	}
}

func TestContinualDetailListsGates(t *testing.T) {
	d := ContinualDetail(Continual{Gates: []string{"make test"}})
	if !strings.Contains(d, "make test") {
		t.Fatalf("detail missing gate: %q", d)
	}
}
