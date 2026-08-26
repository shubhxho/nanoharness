package context

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

func Search(root, query string) (Report, error) {
	return SearchMode(root, query, ModeQuery)
}

func SearchMode(root, query string, mode Mode) (Report, error) {
	if mode == "" {
		mode = ModeQuery
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return Report{}, err
	}
	tokens := ExtractTerms(query)
	if len(tokens) == 0 {
		tokens = tokenize(query)
	}
	report := Report{Mode: mode, Query: query}
	if len(tokens) == 0 {
		return report, nil
	}
	err = walk(absolute, absolute, tokens, mode, &report)
	sort.Slice(report.Citations, func(i, j int) bool {
		a, b := report.Citations[i], report.Citations[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.StartLine < b.StartLine
	})
	if len(report.Citations) > MaxResults {
		report.Citations = report.Citations[:MaxResults]
		report.Truncated = true
	}
	return report, err
}

func walk(root, dir string, tokens []string, mode Mode, report *Report) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if report.ScannedBytes >= MaxScannedBytes {
			report.Truncated = true
			return nil
		}
		name, path := entry.Name(), filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			report.Skipped++
			continue
		}
		if info.IsDir() {
			if skipDir(name) {
				report.Skipped++
				continue
			}
			if err := walk(root, path, tokens, mode, report); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() || info.Size() > MaxFileBytes || binaryExtension(path) {
			report.Skipped++
			continue
		}
		if info.Size() > MaxScannedBytes-report.ScannedBytes {
			report.Truncated = true
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			report.Skipped++
			continue
		}
		report.ScannedBytes += int64(len(data))
		if strings.IndexByte(string(data), 0) >= 0 || !validUTF8(data) {
			report.Skipped++
			continue
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(relative, "..") {
			report.Skipped++
			continue
		}
		if citation, ok := scoreFile(filepath.ToSlash(relative), string(data), tokens, mode); ok {
			report.Citations = append(report.Citations, citation)
		}
	}
	return nil
}

func skipDir(name string) bool {
	switch name {
	case ".git", "target", "node_modules", ".venv", "vendor", "dist", "build", ".next", "coverage", "bin":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func validUTF8(data []byte) bool { return utf8.Valid(data) }

func binaryExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".pdf", ".zip", ".gz", ".tar", ".lock", ".ico", ".woff", ".woff2", ".ttf", ".otf", ".wasm", ".dylib", ".so", ".a", ".o", ".exe", ".bin":
		return true
	}
	return false
}

func tokenize(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	return fields
}
