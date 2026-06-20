package plugins

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()
	client := clientForPublicHost(t, server)
	manifestURL := publicManifestURL("/gigi-plugin.json")

	got, err := (HTTPManifestFetcher{Client: client}).Fetch(context.Background(), manifestURL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if got.ID != manifest.ID || got.SourceKind != SourceKindManifestURL || got.ManifestURL != manifestURL {
		t.Fatalf("manifest = %+v, want URL-enriched manifest", got)
	}
}

func TestHTTPManifestFetcherRejectsNonHTTPSURL(t *testing.T) {
	_, err := (HTTPManifestFetcher{}).Fetch(context.Background(), "http://example.test/gigi-plugin.json")
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("error = %v, want HTTPS requirement", err)
	}
}

func TestHTTPManifestFetcherRejectsUnsafeLiteralManifestURLHosts(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return manifestHTTPResponse(t, "application/json"), nil
	})}
	tests := []struct {
		name        string
		manifestURL string
	}{
		{name: "localhost", manifestURL: "https://localhost/gigi-plugin.json"},
		{name: "loopback IPv4", manifestURL: "https://127.0.0.1/gigi-plugin.json"},
		{name: "loopback IPv6", manifestURL: "https://[::1]/gigi-plugin.json"},
		{name: "private IPv4", manifestURL: "https://10.0.0.8/gigi-plugin.json"},
		{name: "link-local IPv4", manifestURL: "https://169.254.169.254/gigi-plugin.json"},
		{name: "link-local IPv6", manifestURL: "https://[fe80::1]/gigi-plugin.json"},
		{name: "unspecified IPv4", manifestURL: "https://0.0.0.0/gigi-plugin.json"},
		{name: "unspecified IPv6", manifestURL: "https://[::]/gigi-plugin.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (HTTPManifestFetcher{Client: client}).Fetch(context.Background(), tt.manifestURL)
			if err == nil || !strings.Contains(err.Error(), "manifest URL host") {
				t.Fatalf("error = %v, want literal host rejection", err)
			}
		})
	}
}

func TestHTTPManifestFetcherDefaultClientInstallsSafeDialer(t *testing.T) {
	client := (HTTPManifestFetcher{}).client(validateFetchManifestURL)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", client.Transport)
	}
	if transport.DialContext == nil {
		t.Fatal("default fetcher transport must validate resolved remote addresses")
	}
	if transport.Proxy != nil {
		t.Fatal("default fetcher transport must not use environment proxy")
	}
}

func TestValidatePublicRemoteAddrRejectsUnsafeResolvedAddresses(t *testing.T) {
	tests := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("10.0.0.8"),
		net.ParseIP("169.254.169.254"),
		net.ParseIP("0.0.0.0"),
		net.ParseIP("::1"),
		net.ParseIP("fe80::1"),
	}

	for _, ip := range tests {
		t.Run(ip.String(), func(t *testing.T) {
			err := validatePublicRemoteAddr(&net.TCPAddr{IP: ip, Port: 443})
			if err == nil || !strings.Contains(err.Error(), "resolved") {
				t.Fatalf("error = %v, want unsafe resolved address rejection", err)
			}
		})
	}
}

func TestValidatePublicRemoteAddrAllowsPublicResolvedAddress(t *testing.T) {
	if err := validatePublicRemoteAddr(&net.TCPAddr{IP: net.ParseIP("93.184.216.34"), Port: 443}); err != nil {
		t.Fatalf("validatePublicRemoteAddr returned error: %v", err)
	}
}

func TestHTTPManifestFetcherRejectsUnsafeManifestRedirects(t *testing.T) {
	tests := []struct {
		name     string
		location string
		want     string
	}{
		{name: "non-HTTPS", location: "http://example.test/gigi-plugin.json", want: "HTTPS"},
		{name: "private literal host", location: "https://10.0.0.8/gigi-plugin.json", want: "manifest URL host"},
		{name: "query", location: "https://example.test/gigi-plugin.json?token=value", want: "query"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", tt.location)
				w.WriteHeader(http.StatusFound)
			}))
			defer server.Close()

			_, err := (HTTPManifestFetcher{Client: clientForPublicHost(t, server)}).Fetch(context.Background(), publicManifestURL("/redirect"))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want redirect rejection containing %q", err, tt.want)
			}
		})
	}
}

func TestHTTPManifestFetcherRejectsOversizeManifest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Repeat("x", 5)))
	}))
	defer server.Close()

	_, err := (HTTPManifestFetcher{Client: clientForPublicHost(t, server), MaxBytes: 4}).Fetch(context.Background(), publicManifestURL("/gigi-plugin.json"))
	if err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("error = %v, want byte limit", err)
	}
}

func TestHTTPManifestFetcherRejectsNonJSONManifestContentType(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(mustMarshalManifest(t))
	}))
	defer server.Close()

	_, err := (HTTPManifestFetcher{Client: clientForPublicHost(t, server)}).Fetch(context.Background(), publicManifestURL("/gigi-plugin.json"))
	if err == nil || !strings.Contains(err.Error(), "Content-Type") {
		t.Fatalf("error = %v, want Content-Type rejection", err)
	}
}

func TestHTTPManifestFetcherAcceptsEmptyManifestContentType(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "")
		_, _ = w.Write(mustMarshalManifest(t))
	}))
	defer server.Close()

	_, err := (HTTPManifestFetcher{Client: clientForPublicHost(t, server)}).Fetch(context.Background(), publicManifestURL("/gigi-plugin.json"))
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
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
		ID:          "attachment-id",
		URL:         server.URL + "/gigi-plugin.json?discord=cdn",
		Filename:    "gigi-plugin.json",
		ContentType: "application/json",
		Size:        len(body),
	})
	if err != nil {
		t.Fatalf("FetchAttachment returned error: %v", err)
	}
	if got.ID != manifest.ID || got.SourceKind != SourceKindUploadedFile || got.ManifestURL != "" {
		t.Fatalf("manifest = %+v, want uploaded file source without URL", got)
	}
}

func TestHTTPManifestFetcherRejectsUnsafeAttachmentRedirects(t *testing.T) {
	tests := []struct {
		name     string
		location string
		want     string
	}{
		{name: "non-HTTPS", location: "http://example.test/gigi-plugin.json", want: "HTTPS"},
		{name: "private literal host", location: "https://10.0.0.8/gigi-plugin.json", want: "attachment URL host"},
		{name: "user info", location: "https://user:pass@example.test/gigi-plugin.json", want: "user info"},
		{name: "fragment", location: "https://example.test/gigi-plugin.json#secret", want: "fragment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", tt.location)
				w.WriteHeader(http.StatusFound)
			}))
			defer server.Close()

			_, err := (HTTPManifestFetcher{Client: server.Client()}).FetchAttachment(context.Background(), AttachmentSource{
				URL:         server.URL + "/gigi-plugin.json?discord=cdn",
				Filename:    "gigi-plugin.json",
				ContentType: "application/json",
				Size:        10,
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want redirect rejection containing %q", err, tt.want)
			}
		})
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

func TestHTTPManifestFetcherRejectsNonJSONAttachmentContentType(t *testing.T) {
	_, err := (HTTPManifestFetcher{}).FetchAttachment(context.Background(), AttachmentSource{
		URL:         "https://example.test/gigi-plugin.json",
		Filename:    "gigi-plugin.json",
		ContentType: "text/plain",
		Size:        10,
	})
	if err == nil || !strings.Contains(err.Error(), "Content-Type") {
		t.Fatalf("error = %v, want attachment Content-Type rejection", err)
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func clientForPublicHost(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()

	baseTransport, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("server client transport = %T, want *http.Transport", server.Client().Transport)
	}
	transport := baseTransport.Clone()
	dialer := &net.Dialer{}
	serverAddr := server.Listener.Addr().String()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, serverAddr)
	}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	transport.TLSClientConfig.InsecureSkipVerify = true

	return &http.Client{Transport: transport}
}

func publicManifestURL(path string) string {
	return "https://example.test" + path
}

func manifestHTTPResponse(t *testing.T, contentType string) *http.Response {
	t.Helper()

	header := make(http.Header)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(mustMarshalManifest(t))),
	}
}

func mustMarshalManifest(t *testing.T) []byte {
	t.Helper()

	body, err := json.Marshal(validManifest())
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return body
}
