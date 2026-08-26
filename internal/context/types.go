package context

const (
	MaxFileBytes    int64 = 1 << 20
	MaxScannedBytes int64 = 8 << 20
	MaxResults            = 25
	MaxSnippetLines       = 80
	MaxContextBytes       = 32 << 10
	AttachLimit           = 8
)

// Mode selects how strictly tokens must match.
type Mode string

const (
	ModeQuery    Mode = "query"    // every token must hit path or content
	ModeResearch Mode = "research" // soft OR: keep files matching enough tokens
	ModeImpact   Mode = "impact"   // prefer exact symbol / identifier hits
)

type Citation struct {
	Path      string
	StartLine int
	EndLine   int
	Snippet   string
	Score     int
}

type Report struct {
	Citations    []Citation
	ScannedBytes int64
	Skipped      int
	Truncated    bool
	Mode         Mode
	Query        string
}
