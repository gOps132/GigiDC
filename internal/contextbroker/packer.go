package contextbroker

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
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
	Pinned    bool
	Stale     bool
}

type PackRequest struct {
	Snippets []Snippet
	MaxChars int
}

type BuildRequest struct {
	Snippets []Snippet
	Previous SessionState
	MaxChars int
}

type SessionState struct {
	Seen map[string]string
}

type ContextStatus string

const (
	StatusNew                ContextStatus = "NEW"
	StatusChanged            ContextStatus = "CHANGED"
	StatusPinned             ContextStatus = "PINNED"
	StatusOmitUnchanged      ContextStatus = "OMIT_UNCHANGED"
	StatusOmitBudget         ContextStatus = "OMIT_BUDGET"
	StatusInvalidatePrevious ContextStatus = "INVALIDATE_PREVIOUS"
)

type Citation struct {
	Label     string
	SourceID  string
	SnippetID string
	Source    string
	CreatedAt string
}

type ContextItem struct {
	Status        ContextStatus
	SourceID      string
	Snippet       Snippet
	Citation      Citation
	RestoreHandle string
	Chars         int
	Pinned        bool
	StalePrevious bool
	Stale         bool
}

type OmittedContext struct {
	Status        ContextStatus
	SourceID      string
	RestoreHandle string
	Reason        string
}

type ContextInvalidation struct {
	Status              ContextStatus
	SourceID            string
	PreviousFingerprint string
	CurrentFingerprint  string
	RestoreHandle       string
}

type Pack struct {
	Snippets      []Snippet
	Items         []ContextItem
	Omitted       []OmittedContext
	Invalidations []ContextInvalidation
	Citations     []Citation
	Chars         int
	Truncated     bool
	NextState     SessionState
}

func PackSnippets(request PackRequest) Pack {
	return BuildPack(BuildRequest{
		Snippets: request.Snippets,
		MaxChars: request.MaxChars,
	})
}

func BuildPack(request BuildRequest) Pack {
	maxChars := request.MaxChars
	if maxChars <= 0 {
		maxChars = 4000
	}
	previousSeen := copySeen(request.Previous.Seen)
	pack := Pack{
		Snippets:  make([]Snippet, 0, len(request.Snippets)),
		Items:     make([]ContextItem, 0, len(request.Snippets)),
		NextState: SessionState{Seen: previousSeen},
	}
	for _, snippet := range prioritizeSnippets(request.Snippets) {
		snippet = normalizeSnippet(snippet)
		if snippet.Text == "" {
			continue
		}
		sourceID := SourceID(snippet)
		fingerprint := Fingerprint(snippet)
		previousFingerprint, seenBefore := previousSeen[sourceID]
		pack.NextState.Seen[sourceID] = fingerprint

		if seenBefore && previousFingerprint == fingerprint && !snippet.Pinned {
			pack.Omitted = append(pack.Omitted, OmittedContext{
				Status:        StatusOmitUnchanged,
				SourceID:      sourceID,
				RestoreHandle: RestoreHandle(sourceID, fingerprint),
				Reason:        "unchanged",
			})
			continue
		}

		size := utf8.RuneCountInString(snippet.Text)
		if pack.Chars+size > maxChars {
			remaining := maxChars - pack.Chars
			if remaining > 3 {
				snippet.Text = truncateRunes(snippet.Text, remaining)
				size = utf8.RuneCountInString(snippet.Text)
				pack.Chars += size
				pack.appendItem(snippet, sourceID, fingerprint, previousFingerprint, seenBefore, size)
			} else {
				pack.Omitted = append(pack.Omitted, OmittedContext{
					Status:        StatusOmitBudget,
					SourceID:      sourceID,
					RestoreHandle: RestoreHandle(sourceID, fingerprint),
					Reason:        "budget",
				})
			}
			pack.Truncated = true
			break
		}
		pack.Chars += size
		pack.appendItem(snippet, sourceID, fingerprint, previousFingerprint, seenBefore, size)
	}
	return pack
}

func (p *Pack) appendItem(snippet Snippet, sourceID string, fingerprint string, previousFingerprint string, seenBefore bool, size int) {
	status := StatusNew
	stalePrevious := false
	if snippet.Pinned {
		status = StatusPinned
	} else if snippet.Stale || (seenBefore && previousFingerprint != fingerprint) {
		status = StatusChanged
		stalePrevious = seenBefore && previousFingerprint != fingerprint
	}
	if stalePrevious {
		p.Invalidations = append(p.Invalidations, ContextInvalidation{
			Status:              StatusInvalidatePrevious,
			SourceID:            sourceID,
			PreviousFingerprint: previousFingerprint,
			CurrentFingerprint:  fingerprint,
			RestoreHandle:       RestoreHandle(sourceID, previousFingerprint),
		})
	}

	citation := Citation{
		Label:     "S" + strconv.Itoa(len(p.Citations)+1),
		SourceID:  sourceID,
		SnippetID: snippet.ID,
		Source:    snippet.Source,
		CreatedAt: snippet.CreatedAt,
	}
	item := ContextItem{
		Status:        status,
		SourceID:      sourceID,
		Snippet:       snippet,
		Citation:      citation,
		RestoreHandle: RestoreHandle(sourceID, fingerprint),
		Chars:         size,
		Pinned:        snippet.Pinned,
		StalePrevious: stalePrevious,
		Stale:         snippet.Stale,
	}
	p.Citations = append(p.Citations, citation)
	p.Items = append(p.Items, item)
	p.Snippets = append(p.Snippets, snippet)
}

func SourceID(snippet Snippet) string {
	snippet = normalizeSnippet(snippet)
	switch {
	case snippet.Source != "" && snippet.ID != "":
		return snippet.Source + ":" + snippet.ID
	case snippet.Source != "":
		return snippet.Source
	case snippet.ID != "":
		return snippet.ID
	default:
		return "snippet:" + shortHash(snippet.Text)
	}
}

func Fingerprint(snippet Snippet) string {
	snippet = normalizeSnippet(snippet)
	return shortHash(strings.Join([]string{
		SourceID(snippet),
		snippet.AuthorID,
		snippet.CreatedAt,
		snippet.Text,
	}, "\x00"))
}

func RestoreHandle(sourceID string, fingerprint string) string {
	return "ctx:" + shortHash(strings.TrimSpace(sourceID)+"\x00"+strings.TrimSpace(fingerprint))
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

func prioritizeSnippets(snippets []Snippet) []Snippet {
	ordered := make([]Snippet, 0, len(snippets))
	for _, snippet := range snippets {
		if snippet.Pinned {
			ordered = append(ordered, snippet)
		}
	}
	for _, snippet := range snippets {
		if !snippet.Pinned {
			ordered = append(ordered, snippet)
		}
	}
	return ordered
}

func copySeen(seen map[string]string) map[string]string {
	copied := make(map[string]string, len(seen))
	for key, value := range seen {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			copied[key] = value
		}
	}
	return copied
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
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
