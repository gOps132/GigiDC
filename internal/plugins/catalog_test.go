package plugins

import "testing"

func TestStaticCatalogFindsByExactDiscordApplicationID(t *testing.T) {
	manifest := validManifest()
	catalog, err := NewStaticCatalog(manifest)
	if err != nil {
		t.Fatalf("NewStaticCatalog returned error: %v", err)
	}

	got, ok := catalog.FindByDiscordIdentity("1511678703963209813", "")
	if !ok {
		t.Fatal("expected exact application ID match")
	}
	if got.ID != manifest.ID {
		t.Fatalf("manifest ID = %q, want %q", got.ID, manifest.ID)
	}

	if _, ok := catalog.FindByDiscordIdentity("Music", ""); ok {
		t.Fatal("catalog must not match fuzzy plugin names")
	}
}

func TestStaticCatalogFindsByExactDiscordBotUserID(t *testing.T) {
	manifest := validManifest()
	catalog, err := NewStaticCatalog(manifest)
	if err != nil {
		t.Fatalf("NewStaticCatalog returned error: %v", err)
	}

	got, ok := catalog.FindByDiscordIdentity("", "1511678703963209814")
	if !ok {
		t.Fatal("expected exact bot user ID match")
	}
	if got.ID != manifest.ID {
		t.Fatalf("manifest ID = %q, want %q", got.ID, manifest.ID)
	}
}

func TestStaticCatalogRejectsDuplicateDiscordIdentity(t *testing.T) {
	first := validManifest()
	second := validManifest()
	second.ID = "example-tool-two"

	_, err := NewStaticCatalog(first, second)
	if err == nil {
		t.Fatal("expected duplicate identity error")
	}
}
