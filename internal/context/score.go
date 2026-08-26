package context

import (
	"fmt"
	"strings"
	"unicode"
)

func scoreFile(path, text string, tokens []string, mode Mode) (Citation, bool) {
	lowerPath := strings.ToLower(path)
	lines := strings.Split(text, "\n")
	lowerLines := make([]string, len(lines))
	for i, line := range lines {
		lowerLines[i] = strings.ToLower(line)
	}

	score, matched, first, bestLine := 0, 0, -1, -1
	bestLineHits := 0
	for _, token := range tokens {
		hit := false
		if strings.Contains(lowerPath, token) {
			score += 10
			hit = true
		}
		for i, line := range lowerLines {
			if !strings.Contains(line, token) {
				continue
			}
			hit = true
			weight := 3
			if mode == ModeImpact && identifierHit(line, token) {
				weight = 12
			}
			score += weight
			if first < 0 {
				first = i
			}
			hits := tokenHits(line, tokens)
			if hits > bestLineHits {
				bestLineHits = hits
				bestLine = i
			}
		}
		if hit {
			matched++
		} else if mode == ModeQuery || mode == ModeImpact {
			return Citation{}, false
		}
	}

	switch mode {
	case ModeResearch:
		need := (len(tokens) + 1) / 2
		if need < 1 {
			need = 1
		}
		if matched < need {
			return Citation{}, false
		}
		score += matched * 2
	case ModeImpact:
		if matched == 0 {
			return Citation{}, false
		}
	default:
		if matched < len(tokens) {
			return Citation{}, false
		}
	}

	// Phrase boost: consecutive tokens co-occurring on one line.
	if len(tokens) >= 2 {
		phrase := strings.Join(tokens, " ")
		joinedPath := strings.ReplaceAll(lowerPath, "/", " ")
		joinedPath = strings.ReplaceAll(joinedPath, "_", " ")
		joinedPath = strings.ReplaceAll(joinedPath, "-", " ")
		if strings.Contains(joinedPath, phrase) {
			score += 15
		}
		for _, line := range lowerLines {
			if strings.Contains(line, phrase) {
				score += 20
				break
			}
		}
	}

	anchor := first
	if bestLine >= 0 {
		anchor = bestLine
	}
	if anchor < 0 {
		anchor = 0
	}
	start, end := anchor-3, anchor-3+MaxSnippetLines
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	numbered := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		numbered = append(numbered, fmt.Sprintf("%4d  %s", i+1, lines[i]))
	}
	return Citation{
		Path:      path,
		StartLine: start + 1,
		EndLine:   end,
		Snippet:   strings.Join(numbered, "\n"),
		Score:     score,
	}, true
}

func tokenHits(line string, tokens []string) int {
	n := 0
	for _, token := range tokens {
		if strings.Contains(line, token) {
			n++
		}
	}
	return n
}

func identifierHit(line, token string) bool {
	for _, field := range strings.FieldsFunc(line, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
	}) {
		if field == token {
			return true
		}
	}
	return false
}
