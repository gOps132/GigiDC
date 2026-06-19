package contextbroker

import (
	"strings"
	"testing"
)

func TestPackSnippetsSkipsEmptyAndKeepsBudget(t *testing.T) {
	pack := PackSnippets(PackRequest{
		MaxChars: 10,
		Snippets: []Snippet{
			{ID: " empty ", Text: "   "},
			{ID: "one", Text: "12345"},
			{ID: "two", Text: "abcdef"},
		},
	})

	if len(pack.Snippets) != 2 {
		t.Fatalf("snippets = %+v, want 2", pack.Snippets)
	}
	if pack.Chars > 10 || !pack.Truncated {
		t.Fatalf("pack = %+v, want truncated within budget", pack)
	}
	if pack.Snippets[0].ID != "one" || pack.Snippets[1].Text != "ab..." {
		t.Fatalf("snippets = %+v, want normalized truncated snippets", pack.Snippets)
	}
}

func TestPackSnippetsUsesDefaultBudget(t *testing.T) {
	pack := PackSnippets(PackRequest{Snippets: []Snippet{{Text: "hello"}}})
	if len(pack.Snippets) != 1 || pack.Chars != 5 || pack.Truncated {
		t.Fatalf("pack = %+v, want default budget pack", pack)
	}
}

func TestBuildPackMarksPinsChangedOmittedAndRestoreHandles(t *testing.T) {
	unchanged := Snippet{ID: "m1", Source: "discord:channel-1", Text: "already seen"}
	changed := Snippet{ID: "m2", Source: "discord:channel-1", Text: "changed text"}
	pinned := Snippet{ID: "rules", Source: "guild", Text: "always include", Pinned: true}

	pack := BuildPack(BuildRequest{
		MaxChars: 100,
		Previous: SessionState{Seen: map[string]string{
			SourceID(unchanged): Fingerprint(unchanged),
			SourceID(changed):   "old-hash",
			SourceID(pinned):    Fingerprint(pinned),
		}},
		Snippets: []Snippet{
			unchanged,
			changed,
			pinned,
			{ID: "m3", Source: "discord:channel-1", Text: "brand new"},
		},
	})

	if len(pack.Items) != 3 || len(pack.Snippets) != 3 {
		t.Fatalf("items = %+v snippets=%+v, want changed, pinned, and new included", pack.Items, pack.Snippets)
	}
	if pack.Items[0].Status != StatusPinned || pack.Items[0].SourceID != "guild:rules" {
		t.Fatalf("first item = %+v, want pinned item prioritized", pack.Items[0])
	}
	if pack.Items[1].Status != StatusChanged || !pack.Items[1].StalePrevious || pack.Items[1].Citation.Label != "S2" {
		t.Fatalf("second item = %+v, want changed item with stale marker and citation", pack.Items[1])
	}
	if pack.Items[2].Status != StatusNew {
		t.Fatalf("third item = %+v, want new item", pack.Items[2])
	}
	if len(pack.Omitted) != 1 || pack.Omitted[0].Status != StatusOmitUnchanged || pack.Omitted[0].RestoreHandle == "" {
		t.Fatalf("omitted = %+v, want unchanged restore handle", pack.Omitted)
	}
	if len(pack.Invalidations) != 1 || pack.Invalidations[0].Status != StatusInvalidatePrevious || pack.Invalidations[0].SourceID != SourceID(changed) {
		t.Fatalf("invalidations = %+v, want changed source invalidation", pack.Invalidations)
	}
	if pack.NextState.Seen[SourceID(changed)] == "old-hash" || pack.NextState.Seen[SourceID(unchanged)] == "" {
		t.Fatalf("next state = %+v, want refreshed fingerprints", pack.NextState)
	}
}

func TestBuildPackKeepsPinnedSnippetsBeforeBudgetedContext(t *testing.T) {
	pack := BuildPack(BuildRequest{
		MaxChars: 12,
		Snippets: []Snippet{
			{ID: "normal", Source: "discord:channel-1", Text: "normal text"},
			{ID: "pinned", Source: "guild", Text: "must stay", Pinned: true},
		},
	})

	if len(pack.Items) != 1 || pack.Items[0].SourceID != "guild:pinned" || pack.Items[0].Status != StatusPinned {
		t.Fatalf("items = %+v, want pinned item first within budget", pack.Items)
	}
	if pack.Chars > 12 || !pack.Truncated {
		t.Fatalf("pack = %+v, want truncated pack within budget", pack)
	}
}

func TestBuildPackMarksBudgetOmissionsWithRestoreHandles(t *testing.T) {
	pack := BuildPack(BuildRequest{
		MaxChars: 8,
		Snippets: []Snippet{
			{ID: "first", Source: "discord:channel-1", Text: "12345"},
			{ID: "second", Source: "discord:channel-1", Text: "abcdef"},
		},
	})

	if len(pack.Items) != 1 || pack.Items[0].SourceID != "discord:channel-1:first" {
		t.Fatalf("items = %+v, want only first item within budget", pack.Items)
	}
	if len(pack.Omitted) != 1 || pack.Omitted[0].Status != StatusOmitBudget || pack.Omitted[0].SourceID != "discord:channel-1:second" || pack.Omitted[0].RestoreHandle == "" {
		t.Fatalf("omitted = %+v, want budget omission with restore handle", pack.Omitted)
	}
	if !pack.Truncated {
		t.Fatalf("pack = %+v, want truncated marker", pack)
	}
}

func TestBuildPackDoesNotStoreOmittedSnippetText(t *testing.T) {
	pack := BuildPack(BuildRequest{
		MaxChars: 3,
		Snippets: []Snippet{
			{ID: "secret", Source: "discord:channel-1", Text: "secret text"},
		},
	})

	if len(pack.Omitted) != 1 {
		t.Fatalf("omitted = %+v, want one omitted snippet", pack.Omitted)
	}
	rendered := pack.Omitted[0].SourceID + pack.Omitted[0].RestoreHandle + pack.Omitted[0].Reason
	if strings.Contains(rendered, "secret text") {
		t.Fatalf("omitted context leaked text: %+v", pack.Omitted[0])
	}
}
