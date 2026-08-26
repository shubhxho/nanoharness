package context

var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "to": {}, "of": {},
	"in": {}, "on": {}, "for": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"be": {}, "been": {}, "this": {}, "that": {}, "it": {}, "with": {}, "as": {},
	"by": {}, "from": {}, "at": {}, "into": {}, "about": {}, "how": {}, "what": {},
	"where": {}, "when": {}, "why": {}, "which": {}, "who": {}, "can": {}, "could": {},
	"should": {}, "would": {}, "do": {}, "does": {}, "did": {}, "please": {}, "help": {},
	"me": {}, "my": {}, "our": {}, "your": {}, "you": {}, "we": {}, "i": {}, "if": {},
	"then": {}, "than": {}, "so": {}, "not": {}, "no": {}, "yes": {}, "just": {},
	"also": {}, "any": {}, "all": {}, "some": {}, "more": {}, "most": {}, "find": {},
	"show": {}, "explain": {}, "review": {}, "look": {}, "check": {}, "code": {},
	"file": {}, "files": {}, "here": {}, "there": {}, "using": {}, "use": {},
	"make": {}, "need": {}, "needs": {}, "want": {}, "like": {},
}

// ExtractTerms pulls search tokens from a free-form prompt, dropping stopwords.
func ExtractTerms(prompt string) []string {
	raw := tokenize(prompt)
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, token := range raw {
		if len(token) < 2 {
			continue
		}
		if _, stop := stopwords[token]; stop {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
		if len(out) >= 12 {
			break
		}
	}
	return out
}
