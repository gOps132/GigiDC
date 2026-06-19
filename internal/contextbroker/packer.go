package contextbroker

import (
	"strings"
	"unicode/utf8"
)

type Snippet struct {
	ID        string
	Source    string
	ChannelID string
	AuthorID  string
	Text      string
	Score     float64
	CreatedAt string
}

type PackRequest struct {
	Snippets []Snippet
	MaxChars int
}

type Pack struct {
	Snippets  []Snippet
	Chars     int
	Truncated bool
}

func PackSnippets(request PackRequest) Pack {
	maxChars := request.MaxChars
	if maxChars <= 0 {
		maxChars = 4000
	}
	pack := Pack{Snippets: make([]Snippet, 0, len(request.Snippets))}
	for _, snippet := range request.Snippets {
		snippet = normalizeSnippet(snippet)
		if snippet.Text == "" {
			continue
		}
		size := utf8.RuneCountInString(snippet.Text)
		if pack.Chars+size > maxChars {
			remaining := maxChars - pack.Chars
			if remaining > 3 {
				snippet.Text = truncateRunes(snippet.Text, remaining)
				pack.Chars += utf8.RuneCountInString(snippet.Text)
				pack.Snippets = append(pack.Snippets, snippet)
			}
			pack.Truncated = true
			break
		}
		pack.Chars += size
		pack.Snippets = append(pack.Snippets, snippet)
	}
	return pack
}

func normalizeSnippet(snippet Snippet) Snippet {
	snippet.ID = strings.TrimSpace(snippet.ID)
	snippet.Source = strings.TrimSpace(snippet.Source)
	snippet.ChannelID = strings.TrimSpace(snippet.ChannelID)
	snippet.AuthorID = strings.TrimSpace(snippet.AuthorID)
	snippet.Text = strings.TrimSpace(snippet.Text)
	snippet.CreatedAt = strings.TrimSpace(snippet.CreatedAt)
	return snippet
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	if limit <= 3 {
		return string(runes[:limit])
	}
	return strings.TrimSpace(string(runes[:limit-3])) + "..."
}
