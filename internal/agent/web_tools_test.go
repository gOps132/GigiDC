package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestWebFetchBlocksLocalhostBeforeRequest(t *testing.T) {
	tool := WebFetchTool{
		Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("request should not reach transport")
			return nil, nil
		})},
		ResolveHost: staticResolver(map[string][]net.IP{
			"localhost": {net.ParseIP("127.0.0.1")},
		}),
	}

	_, err := tool.Execute(context.Background(), Request{}, ToolCall{
		Name: ToolWebFetch,
		Args: map[string]string{"url": "http://localhost/"},
	})
	if err == nil || !strings.Contains(err.Error(), "blocked host") {
		t.Fatalf("expected blocked host error, got %v", err)
	}
}

func TestWebFetchBlocksPrivateRedirect(t *testing.T) {
	tool := WebFetchTool{
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Hostname() {
			case "public.test":
				return textResponse(http.StatusFound, "text/html", "", map[string]string{
					"Location": "http://10.0.0.1/private",
				}), nil
			case "10.0.0.1":
				t.Fatal("private redirect target should not be requested")
			}
			return nil, errors.New("unexpected request")
		})},
		ResolveHost: staticResolver(map[string][]net.IP{
			"public.test": {net.ParseIP("93.184.216.34")},
			"10.0.0.1":    {net.ParseIP("10.0.0.1")},
		}),
	}

	_, err := tool.Execute(context.Background(), Request{}, ToolCall{
		Name: ToolWebFetch,
		Args: map[string]string{"url": "http://public.test/"},
	})
	if err == nil || !strings.Contains(err.Error(), "blocked host") {
		t.Fatalf("expected blocked host error, got %v", err)
	}
}

func TestWebFetchCapsResponseBytesBeforeParsing(t *testing.T) {
	body := "<html><body><h1>Start</h1>" + strings.Repeat("x", maxWebResponseBytes+1024) + "</body></html>"
	tool := WebFetchTool{
		Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return textResponse(http.StatusOK, "text/html; charset=utf-8", body, nil), nil
		})},
		ResolveHost: staticResolver(map[string][]net.IP{
			"public.test": {net.ParseIP("93.184.216.34")},
		}),
	}

	result, err := tool.Execute(context.Background(), Request{}, ToolCall{
		Name: ToolWebFetch,
		Args: map[string]string{"url": "http://public.test/"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(result.Data["content"]); got > maxWebResponseBytes {
		t.Fatalf("expected capped content, got %d bytes", got)
	}
	if result.Data["truncated"] != "true" {
		t.Fatalf("expected truncated marker, got %q", result.Data["truncated"])
	}
}

func TestWebSearchClampsLimit(t *testing.T) {
	var html strings.Builder
	html.WriteString("<html><body>")
	for i := 0; i < maxWebSearchResults+5; i++ {
		html.WriteString(`<div class="result body"><a class="result__a" href="https://example.com/item">Title</a><a class="result__snippet">Snippet</a></div>`)
	}
	html.WriteString("</body></html>")

	tool := WebSearchTool{
		Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return textResponse(http.StatusOK, "text/html", html.String(), nil), nil
		})},
		BaseURL: "https://search.test/?q=",
		ResolveHost: staticResolver(map[string][]net.IP{
			"search.test": {net.ParseIP("93.184.216.34")},
		}),
	}

	result, err := tool.Execute(context.Background(), Request{}, ToolCall{
		Name: ToolWebSearch,
		Args: map[string]string{"query": "test", "limit": "999"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data["count"] != "10" {
		t.Fatalf("expected clamped count 10, got %s", result.Data["count"])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func staticResolver(entries map[string][]net.IP) func(context.Context, string) ([]net.IP, error) {
	return func(_ context.Context, host string) ([]net.IP, error) {
		if ips, ok := entries[host]; ok {
			return ips, nil
		}
		return nil, errors.New("host not found")
	}
}

func textResponse(status int, contentType string, body string, headers map[string]string) *http.Response {
	header := make(http.Header)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		header.Set(key, value)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
