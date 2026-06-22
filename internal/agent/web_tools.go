package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	ToolWebSearch = "web.search"
	ToolWebFetch  = "web.fetch"

	defaultWebSearchURL   = "https://html.duckduckgo.com/html/?q="
	defaultBraveSearchURL = "https://api.search.brave.com/res/v1/web/search"
	maxWebSearchResults   = 10
	defaultWebFetchChars  = 10000
	maxWebResponseBytes   = 1 << 20
)

var (
	errBlockedHost             = errors.New("blocked host")
	errSearchProviderChallenge = errors.New("search provider challenge")
)

type hostResolver func(context.Context, string) ([]net.IP, error)

type WebSearchProvider interface {
	Search(context.Context, string, int) ([]SearchResult, string, error)
}

// WebSearchTool searches the web using the configured provider.
type WebSearchTool struct {
	Provider    WebSearchProvider
	Client      *http.Client
	BaseURL     string
	ResolveHost hostResolver
}

func (t WebSearchTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolWebSearch,
		Description: "Search the web for a query. Arguments: query, limit (optional).",
		Kind:        ToolKindRead,
		Capability:  "web.search",
	}
}

type SearchResult struct {
	Title string
	URL   string
	Body  string
}

func (t WebSearchTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	query := strings.TrimSpace(call.Args["query"])
	if query == "" {
		return ToolResult{}, fmt.Errorf("search query is required")
	}
	limit := parseWebLimit(call.Args["limit"], 5, maxWebSearchResults)

	results, providerName, err := t.searchProvider().Search(ctx, query, limit)
	if err != nil {
		return ToolResult{}, fmt.Errorf("web search failed: %w", err)
	}
	if len(results) == 0 {
		return ToolResult{Name: ToolWebSearch, Summary: fmt.Sprintf("No search results found for query: %q", query), Data: map[string]string{"count": "0", "provider": providerName}}, nil
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Search results for %q:\n", query))
	data := map[string]string{"count": strconv.Itoa(len(results)), "provider": providerName}
	for i, result := range results {
		index := strconv.Itoa(i + 1)
		summary.WriteString(fmt.Sprintf("%s. [%s](%s) - %s\n", index, result.Title, result.URL, result.Body))
		data["title_"+index] = result.Title
		data["url_"+index] = result.URL
		data["snippet_"+index] = result.Body
	}
	return ToolResult{Name: ToolWebSearch, Summary: summary.String(), Data: data}, nil
}

func (t WebSearchTool) searchProvider() WebSearchProvider {
	if t.Provider != nil {
		return t.Provider
	}
	return DuckDuckGoSearchProvider{Client: t.Client, BaseURL: t.BaseURL, ResolveHost: t.ResolveHost}
}

type DuckDuckGoSearchProvider struct {
	Client      *http.Client
	BaseURL     string
	ResolveHost hostResolver
}

func (p DuckDuckGoSearchProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, string, error) {
	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		baseURL = defaultWebSearchURL
	}
	searchURL := baseURL + url.QueryEscape(query)
	target, err := parseSafeWebURL(ctx, searchURL, p.ResolveHost)
	if err != nil {
		return nil, "duckduckgo", err
	}

	client := safeWebClient(p.Client, p.ResolveHost)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "duckduckgo", err
	}
	req.Header.Set("User-Agent", "GigiDC/1.0 (+https://github.com/gOps132/GigiDC)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "duckduckgo", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "duckduckgo", fmt.Errorf("search request returned status %d", resp.StatusCode)
	}
	results, err := parseDuckDuckGoHTML(io.LimitReader(resp.Body, maxWebResponseBytes), limit)
	return results, "duckduckgo", err
}

type BraveSearchProvider struct {
	APIKey      string
	Client      *http.Client
	BaseURL     string
	ResolveHost hostResolver
}

func (p BraveSearchProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, string, error) {
	apiKey := strings.TrimSpace(p.APIKey)
	if apiKey == "" {
		return nil, "brave", fmt.Errorf("brave search api key is required")
	}
	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		baseURL = defaultBraveSearchURL
	}
	target, err := parseSafeWebURL(ctx, baseURL, p.ResolveHost)
	if err != nil {
		return nil, "brave", err
	}
	values := target.Query()
	values.Set("q", query)
	values.Set("count", strconv.Itoa(parseWebLimit(strconv.Itoa(limit), 5, maxWebSearchResults)))
	target.RawQuery = values.Encode()

	client := safeWebClient(p.Client, p.ResolveHost)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "brave", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)
	req.Header.Set("User-Agent", "GigiDC/1.0 (+https://github.com/gOps132/GigiDC)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "brave", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "brave", fmt.Errorf("brave search returned status %d", resp.StatusCode)
	}

	var payload struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxWebResponseBytes)).Decode(&payload); err != nil {
		return nil, "brave", err
	}

	results := make([]SearchResult, 0, len(payload.Web.Results))
	for _, result := range payload.Web.Results {
		title := strings.TrimSpace(result.Title)
		resultURL := strings.TrimSpace(result.URL)
		if title == "" || !isHTTPResultURL(resultURL) {
			continue
		}
		results = append(results, SearchResult{
			Title: title,
			URL:   resultURL,
			Body:  strings.TrimSpace(result.Description),
		})
		if len(results) >= maxWebSearchResults {
			break
		}
	}
	return results, "brave", nil
}

type SearXNGSearchProvider struct {
	Client      *http.Client
	BaseURL     string
	ResolveHost hostResolver
}

func (p SearXNGSearchProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, string, error) {
	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		return nil, "searxng", fmt.Errorf("searxng base url is required")
	}
	target, err := searxngSearchURL(baseURL)
	if err != nil {
		return nil, "searxng", err
	}
	values := target.Query()
	values.Set("q", query)
	values.Set("format", "json")
	target.RawQuery = values.Encode()

	client := trustedSearchClient(p.Client, target.Hostname())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "searxng", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GigiDC/1.0 (+https://github.com/gOps132/GigiDC)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "searxng", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "searxng", fmt.Errorf("searxng search returned status %d", resp.StatusCode)
	}

	var payload struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxWebResponseBytes)).Decode(&payload); err != nil {
		return nil, "searxng", err
	}

	resultLimit := parseWebLimit(strconv.Itoa(limit), 5, maxWebSearchResults)
	results := make([]SearchResult, 0, len(payload.Results))
	for _, result := range payload.Results {
		title := strings.TrimSpace(result.Title)
		resultURL := strings.TrimSpace(result.URL)
		if title == "" || !isHTTPResultURL(resultURL) {
			continue
		}
		results = append(results, SearchResult{
			Title: title,
			URL:   resultURL,
			Body:  strings.TrimSpace(result.Content),
		})
		if len(results) >= resultLimit {
			break
		}
	}
	return results, "searxng", nil
}

func searxngSearchURL(baseURL string) (*url.URL, error) {
	if !strings.Contains(baseURL, "://") {
		baseURL = "https://" + baseURL
	}
	target, err := parseTrustedSearchURL(baseURL)
	if err != nil {
		return nil, err
	}
	cleanPath := strings.TrimRight(target.Path, "/")
	switch {
	case cleanPath == "":
		target.Path = "/search"
	case !strings.HasSuffix(cleanPath, "/search"):
		target.Path = cleanPath + "/search"
	default:
		target.Path = cleanPath
	}
	return target, nil
}

func trustedSearchClient(base *http.Client, allowedHost string) *http.Client {
	client := &http.Client{Timeout: 15 * time.Second}
	if base != nil {
		*client = *base
		if client.Timeout == 0 {
			client.Timeout = 15 * time.Second
		}
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		if _, err := parseTrustedSearchURL(req.URL.String()); err != nil {
			return err
		}
		if !strings.EqualFold(req.URL.Hostname(), allowedHost) {
			return fmt.Errorf("redirect host %q is not allowed", req.URL.Hostname())
		}
		return nil
	}
	return client
}

func parseTrustedSearchURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.User != nil {
		return nil, fmt.Errorf("userinfo is not allowed")
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("host is required")
	}
	return u, nil
}

func isHTTPResultURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

type FallbackSearchProvider struct {
	Primary  WebSearchProvider
	Fallback WebSearchProvider
}

func (p FallbackSearchProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, string, error) {
	if p.Primary == nil {
		if p.Fallback == nil {
			return nil, "", fmt.Errorf("web search provider is required")
		}
		return p.Fallback.Search(ctx, query, limit)
	}
	results, name, err := p.Primary.Search(ctx, query, limit)
	if err == nil || p.Fallback == nil {
		return results, name, err
	}
	return p.Fallback.Search(ctx, query, limit)
}

func parseDuckDuckGoHTML(r io.Reader, limit int) ([]SearchResult, error) {
	limit = parseWebLimit(strconv.Itoa(limit), 5, maxWebSearchResults)
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	var current *SearchResult
	challenged := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(results) >= limit {
			return
		}
		if n.Type == html.ElementNode && isDuckDuckGoChallengeNode(n) {
			challenged = true
		}
		if n.Type == html.ElementNode && n.Data == "div" && nodeClassContains(n, "result body") {
			if current != nil {
				results = append(results, *current)
			}
			current = &SearchResult{}
		}
		if current != nil && n.Type == html.ElementNode && n.Data == "a" {
			classVal := attr(n, "class")
			if strings.Contains(classVal, "result__a") {
				current.Title = getTextContent(n)
				current.URL = cleanDuckDuckGoURL(attr(n, "href"))
			}
			if strings.Contains(classVal, "result__snippet") {
				current.Body = getTextContent(n)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if current != nil && len(results) < limit {
		results = append(results, *current)
	}

	filtered := make([]SearchResult, 0, len(results))
	for _, result := range results {
		if result.Title != "" && result.URL != "" {
			filtered = append(filtered, result)
		}
	}
	if challenged && len(filtered) == 0 {
		return nil, errSearchProviderChallenge
	}
	return filtered, nil
}

func isDuckDuckGoChallengeNode(n *html.Node) bool {
	id := attr(n, "id")
	action := attr(n, "action")
	src := attr(n, "src")
	return id == "challenge-form" ||
		id == "img-form" ||
		strings.Contains(action, "/anomaly.js") ||
		strings.Contains(src, "/anomaly.js")
}

func cleanDuckDuckGoURL(href string) string {
	if strings.HasPrefix(href, "//duckduckgo.com/l/?") {
		href = "https:" + href
	}
	u, err := url.Parse(href)
	if err == nil && strings.Contains(u.Host, "duckduckgo.com") {
		if target := u.Query().Get("uddg"); target != "" {
			return target
		}
	}
	return href
}

// WebFetchTool fetches a web URL and returns plain readable text.
type WebFetchTool struct {
	Client      *http.Client
	ResolveHost hostResolver
}

func (t WebFetchTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolWebFetch,
		Description: "Fetch public text/HTML content from a URL. Arguments: url.",
		Kind:        ToolKindRead,
		Capability:  "web.fetch",
	}
}

func (t WebFetchTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	targetURL := strings.TrimSpace(call.Args["url"])
	if targetURL == "" {
		return ToolResult{}, fmt.Errorf("url is required")
	}
	if !strings.Contains(targetURL, "://") {
		targetURL = "https://" + targetURL
	}

	target, err := parseSafeWebURL(ctx, targetURL, t.ResolveHost)
	if err != nil {
		return ToolResult{}, fmt.Errorf("web fetch failed: %w", err)
	}
	text, truncated, err := t.fetch(ctx, target)
	if err != nil {
		return ToolResult{}, fmt.Errorf("web fetch failed: %w", err)
	}

	return ToolResult{
		Name:    ToolWebFetch,
		Summary: fmt.Sprintf("Fetched public content from %s", target.String()),
		Data: map[string]string{
			"url":       target.String(),
			"content":   text,
			"truncated": strconv.FormatBool(truncated),
		},
	}, nil
}

func (t WebFetchTool) fetch(ctx context.Context, target *url.URL) (string, bool, error) {
	client := safeWebClient(t.Client, t.ResolveHost)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", "GigiDC/1.0 (+https://github.com/gOps132/GigiDC)")

	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("fetch returned status %d", resp.StatusCode)
	}
	if !allowedWebContentType(resp.Header.Get("Content-Type")) {
		return "", false, fmt.Errorf("unsupported content type")
	}

	limited := io.LimitReader(resp.Body, maxWebResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", false, err
	}
	truncated := len(body) > maxWebResponseBytes
	if truncated {
		body = body[:maxWebResponseBytes]
	}

	text, err := htmlToText(body)
	if err != nil {
		return "", false, err
	}
	if len(text) > defaultWebFetchChars {
		text = text[:defaultWebFetchChars] + "\n... [TRUNCATED] ..."
		truncated = true
	}
	return text, truncated, nil
}

func htmlToText(body []byte) (string, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "head" || n.Data == "noscript") {
			return
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString("\n")
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	re := regexp.MustCompile(`\n{2,}`)
	return re.ReplaceAllString(sb.String(), "\n\n"), nil
}

func safeWebClient(base *http.Client, resolver hostResolver) *http.Client {
	client := &http.Client{Timeout: 15 * time.Second}
	if base != nil {
		*client = *base
		if client.Timeout == 0 {
			client.Timeout = 15 * time.Second
		}
	}
	if client.Transport == nil {
		client.Transport = safeWebTransport(resolver)
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		_, err := parseSafeWebURL(req.Context(), req.URL.String(), resolver)
		return err
	}
	return client
}

func safeWebTransport(resolver hostResolver) http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if err := validatePublicHost(ctx, host, resolver); err != nil {
			return nil, err
		}
		r := resolver
		if r == nil {
			r = lookupIP
		}
		ips, err := r(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if isBlockedIP(ip) {
				continue
			}
			dialer := net.Dialer{Timeout: 10 * time.Second}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}
		return nil, fmt.Errorf("%w %q", errBlockedHost, host)
	}
	return transport
}

func parseSafeWebURL(ctx context.Context, raw string, resolver hostResolver) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.User != nil {
		return nil, fmt.Errorf("userinfo is not allowed")
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("host is required")
	}
	if err := validatePublicHost(ctx, u.Hostname(), resolver); err != nil {
		return nil, err
	}
	return u, nil
}

func validatePublicHost(ctx context.Context, host string, resolver hostResolver) error {
	normalized := strings.Trim(strings.ToLower(host), "[]")
	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") || strings.HasSuffix(normalized, ".local") {
		return fmt.Errorf("%w %q", errBlockedHost, host)
	}
	if ip := net.ParseIP(normalized); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w %q", errBlockedHost, host)
		}
		return nil
	}
	if resolver == nil {
		resolver = lookupIP
	}
	ips, err := resolver(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("resolve host: no addresses")
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w %q", errBlockedHost, host)
		}
	}
	return nil
}

func lookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

func allowedWebContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(mediaType) {
	case "text/html", "text/plain", "application/xhtml+xml", "application/xml", "text/xml":
		return true
	default:
		return false
	}
}

func parseWebLimit(raw string, fallback int, max int) int {
	limit := fallback
	if value, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && value > 0 {
		limit = value
	}
	if limit > max {
		return max
	}
	if limit <= 0 {
		return fallback
	}
	return limit
}

func nodeClassContains(n *html.Node, value string) bool {
	return strings.Contains(attr(n, "class"), value)
}

func attr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func getTextContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}
