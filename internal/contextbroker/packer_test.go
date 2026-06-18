package contextbroker

import "testing"

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
