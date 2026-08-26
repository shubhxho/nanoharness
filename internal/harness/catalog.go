package harness

import (
	local "github.com/shubhxho/nanoharness/internal/context"
	"github.com/shubhxho/nanoharness/internal/providers"
)

// Profiles is the provider catalog.
var Profiles = providers.Profiles

func Find(id string) (Profile, bool) { return providers.Find(id) }

func AuthStatus(provider string) string { return providers.AuthStatus(provider) }

func Login(kind string, apiKey bool) error { return providers.Login(kind, apiKey) }

func Top(citations []Citation, n int) []Citation { return local.Top(citations, n) }
