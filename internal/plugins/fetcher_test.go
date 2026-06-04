package plugins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPManifestFetcherFetchesHTTPSManifest(t *testing.T) {
	manifest := validManifest()
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	got, err := (HTTPManifestFetcher{Client: server.Client()}).Fetch(context.Background(), server.URL+"/gigi-plugin.json")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if got.ID != manifest.ID || got.SourceKind != SourceKindManifestURL || got.ManifestURL != server.URL+"/gigi-plugin.json" {
		t.Fatalf("manifest = %+v, want URL-enriched manifest", got)
	}
}

func TestHTTPManifestFetcherRejectsNonHTTPSURL(t *testing.T) {
	_, err := (HTTPManifestFetcher{}).Fetch(context.Background(), "http://example.test/gigi-plugin.json")
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("error = %v, want HTTPS requirement", err)
	}
}

func TestHTTPManifestFetcherRejectsOversizeManifest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 5)))
	}))
	defer server.Close()

	_, err := (HTTPManifestFetcher{Client: server.Client(), MaxBytes: 4}).Fetch(context.Background(), server.URL)
	if err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("error = %v, want byte limit", err)
	}
}

func TestHTTPManifestFetcherFetchesAttachmentManifest(t *testing.T) {
	manifest := validManifest()
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	got, err := (HTTPManifestFetcher{Client: server.Client()}).FetchAttachment(context.Background(), AttachmentSource{
		ID:       "attachment-id",
		URL:      server.URL + "/gigi-plugin.json?discord=cdn",
		Filename: "gigi-plugin.json",
		Size:     len(body),
	})
	if err != nil {
		t.Fatalf("FetchAttachment returned error: %v", err)
	}
	if got.ID != manifest.ID || got.SourceKind != SourceKindUploadedFile || got.ManifestURL != "" {
		t.Fatalf("manifest = %+v, want uploaded file source without URL", got)
	}
}

func TestHTTPManifestFetcherRejectsNonJSONAttachment(t *testing.T) {
	_, err := (HTTPManifestFetcher{}).FetchAttachment(context.Background(), AttachmentSource{
		URL:      "https://example.test/gigi-plugin.txt",
		Filename: "gigi-plugin.txt",
		Size:     10,
	})
	if err == nil || !strings.Contains(err.Error(), "JSON file") {
		t.Fatalf("error = %v, want JSON file requirement", err)
	}
}

func TestHTTPManifestFetcherRejectsAttachmentURLUserInfo(t *testing.T) {
	_, err := (HTTPManifestFetcher{}).FetchAttachment(context.Background(), AttachmentSource{
		URL:      "https://user:pass@example.test/gigi-plugin.json",
		Filename: "gigi-plugin.json",
		Size:     10,
	})
	if err == nil || !strings.Contains(err.Error(), "user info") {
		t.Fatalf("error = %v, want user info rejection", err)
	}
}
