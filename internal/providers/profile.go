package providers

type Profile struct {
	ID, Label, Default string
	Models             []string
}

var Profiles = []Profile{
	{"codex", "Codex", "", []string{"", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5"}},
	{"openai", "OpenAI", "gpt-5.6-terra", []string{"gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5"}},
	{"anthropic", "Anthropic", "claude-sonnet-5", []string{"claude-sonnet-5", "claude-haiku-4-5-20251001", "claude-opus-5"}},
	{"pi", "pi", "", []string{"", "openai-codex/gpt-5.6-terra", "openai-codex/gpt-5.5", "anthropic/claude-sonnet-5"}},
}

func Find(id string) (Profile, bool) {
	for _, p := range Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}
