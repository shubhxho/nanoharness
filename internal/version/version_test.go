package version_test

import (
	"strings"
	"testing"

	"github.com/shubhxho/nanoharness/internal/version"
)

func TestResolveDefaults(t *testing.T) {
	if version.Resolve() == "" {
		t.Fatal("empty resolve")
	}
}

func TestFullIncludesVersion(t *testing.T) {
	full := version.Full()
	if !strings.Contains(full, version.Resolve()) {
		t.Fatalf("full %q missing resolve", full)
	}
}
