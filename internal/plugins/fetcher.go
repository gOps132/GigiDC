package plugins

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type AttachmentSource struct {
	ID          string
	URL         string
	Filename    string
	ContentType string
	Size        int
}

type HTTPManifestFetcher struct {
	Client   *http.Client
	MaxBytes int64
}

func (f HTTPManifestFetcher) Fetch(ctx context.Context, manifestURL string) (Manifest, error) {
	manifestURL = strings.TrimSpace(manifestURL)
	if err := validateFetchManifestURL(manifestURL); err != nil {
		return Manifest{}, err
	}
	body, err := f.fetchBytes(ctx, manifestURL, f.maxBytes(), validateFetchManifestURL, true)
	if err != nil {
		return Manifest{}, err
	}
	return DecodeManifestFromURL(body, manifestURL)
}

func (f HTTPManifestFetcher) FetchAttachment(ctx context.Context, attachment AttachmentSource) (Manifest, error) {
	if err := validateAttachmentSource(attachment, f.maxBytes()); err != nil {
		return Manifest{}, err
	}
	body, err := f.fetchBytes(ctx, strings.TrimSpace(attachment.URL), f.maxBytes(), validateFetchAttachmentURL, false)
	if err != nil {
		return Manifest{}, err
	}
	return DecodeManifestFromAttachment(body)
}

func (f HTTPManifestFetcher) fetchBytes(ctx context.Context, sourceURL string, maxBytes int64, validateRedirect func(string) error, requireJSONContentType bool) ([]byte, error) {
	client := f.client(validateRedirect)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create manifest request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("manifest URL returned status %d", resp.StatusCode)
	}
	if requireJSONContentType {
		if err := validateJSONContentType(resp.Header.Get("Content-Type"), "manifest Content-Type"); err != nil {
			return nil, err
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("manifest exceeds %d byte limit", maxBytes)
	}
	return body, nil
}

func (f HTTPManifestFetcher) client(validateRedirect func(string) error) *http.Client {
	client := f.Client
	if client == nil {
		client = &http.Client{
			Timeout:   5 * time.Second,
			Transport: safeManifestTransport(),
		}
	}
	if validateRedirect == nil {
		return client
	}

	clone := *client
	originalCheckRedirect := clone.CheckRedirect
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := validateRedirect(req.URL.String()); err != nil {
			return fmt.Errorf("redirect target rejected: %w", err)
		}
		if originalCheckRedirect != nil {
			return originalCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}
	return &clone
}

func safeManifestTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		if err := validatePublicRemoteAddr(conn.RemoteAddr()); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return conn, nil
	}
	return transport
}

func (f HTTPManifestFetcher) maxBytes() int64 {
	if f.MaxBytes <= 0 {
		return maxManifestBytes
	}
	return f.MaxBytes
}

func validateAttachmentSource(attachment AttachmentSource, maxBytes int64) error {
	if strings.TrimSpace(attachment.URL) == "" {
		return fmt.Errorf("attachment URL is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(attachment.URL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("attachment URL must be HTTPS")
	}
	if parsed.User != nil {
		return fmt.Errorf("attachment URL must not include user info")
	}
	if err := validateJSONContentType(attachment.ContentType, "attachment Content-Type"); err != nil {
		return err
	}
	if attachment.Size > 0 && int64(attachment.Size) > maxBytes {
		return fmt.Errorf("manifest exceeds %d byte limit", maxBytes)
	}
	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(attachment.Filename)), ".json") {
		return fmt.Errorf("manifest attachment must be a JSON file")
	}
	return nil
}

func validateFetchAttachmentURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("attachment URL is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("attachment URL must be an HTTPS URL")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("attachment URL must not include user info or fragment")
	}
	return validatePublicLiteralHost("attachment URL", parsed.Hostname())
}

func validateFetchManifestURL(value string) error {
	if err := validateManifestURL(value); err != nil {
		return err
	}
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("manifest URL must be an HTTPS URL")
	}
	return validatePublicLiteralHost("manifest URL", parsed.Hostname())
}

func validatePublicLiteralHost(label string, host string) error {
	host = strings.TrimSpace(host)
	normalizedHost := strings.TrimSuffix(strings.ToLower(host), ".")
	if normalizedHost == "localhost" {
		return fmt.Errorf("%s host must not be localhost, loopback, private, link-local, or unspecified IP", label)
	}

	addrHost := host
	if zoneIndex := strings.LastIndex(addrHost, "%"); zoneIndex >= 0 {
		addrHost = addrHost[:zoneIndex]
	}
	addr, err := netip.ParseAddr(addrHost)
	if err != nil {
		return nil
	}
	if isUnsafeManifestAddr(addr) {
		return fmt.Errorf("%s host must not be localhost, loopback, private, link-local, or unspecified IP", label)
	}
	return nil
}

func validatePublicRemoteAddr(remote net.Addr) error {
	if remote == nil {
		return fmt.Errorf("manifest URL resolved address could not be verified")
	}
	if tcpAddr, ok := remote.(*net.TCPAddr); ok {
		addr, ok := netip.AddrFromSlice(tcpAddr.IP)
		if !ok || isUnsafeManifestAddr(addr) {
			return fmt.Errorf("manifest URL resolved to localhost, loopback, private, link-local, or unspecified IP")
		}
		return nil
	}
	host, _, err := net.SplitHostPort(remote.String())
	if err != nil {
		host = remote.String()
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || isUnsafeManifestAddr(addr) {
		return fmt.Errorf("manifest URL resolved to localhost, loopback, private, link-local, or unspecified IP")
	}
	return nil
}

func isUnsafeManifestAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() || addr.IsMulticast()
}

func validateJSONContentType(value string, label string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return fmt.Errorf("%s must be JSON", label)
	}
	mediaType = strings.ToLower(mediaType)
	if mediaType == "application/json" || mediaType == "text/json" || strings.HasSuffix(mediaType, "+json") {
		return nil
	}
	return fmt.Errorf("%s must be JSON", label)
}
