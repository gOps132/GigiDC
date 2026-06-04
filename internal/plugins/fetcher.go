package plugins

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
	if err := validateManifestURL(manifestURL); err != nil {
		return Manifest{}, err
	}
	body, err := f.fetchBytes(ctx, manifestURL, f.maxBytes())
	if err != nil {
		return Manifest{}, err
	}
	return DecodeManifestFromURL(body, manifestURL)
}

func (f HTTPManifestFetcher) FetchAttachment(ctx context.Context, attachment AttachmentSource) (Manifest, error) {
	if err := validateAttachmentSource(attachment, f.maxBytes()); err != nil {
		return Manifest{}, err
	}
	body, err := f.fetchBytes(ctx, strings.TrimSpace(attachment.URL), f.maxBytes())
	if err != nil {
		return Manifest{}, err
	}
	return DecodeManifestFromAttachment(body)
}

func (f HTTPManifestFetcher) fetchBytes(ctx context.Context, sourceURL string, maxBytes int64) ([]byte, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

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
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("manifest exceeds %d byte limit", maxBytes)
	}
	return body, nil
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
	if attachment.Size > 0 && int64(attachment.Size) > maxBytes {
		return fmt.Errorf("manifest exceeds %d byte limit", maxBytes)
	}
	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(attachment.Filename)), ".json") {
		return fmt.Errorf("manifest attachment must be a JSON file")
	}
	return nil
}
