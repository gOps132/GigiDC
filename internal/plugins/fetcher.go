package plugins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPManifestFetcher struct {
	Client   *http.Client
	MaxBytes int64
}

func (f HTTPManifestFetcher) Fetch(ctx context.Context, manifestURL string) (Manifest, error) {
	manifestURL = strings.TrimSpace(manifestURL)
	if err := validateManifestURL(manifestURL); err != nil {
		return Manifest{}, err
	}
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	maxBytes := f.MaxBytes
	if maxBytes <= 0 {
		maxBytes = maxManifestBytes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return Manifest{}, fmt.Errorf("create manifest request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Manifest{}, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Manifest{}, fmt.Errorf("manifest URL returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return Manifest{}, fmt.Errorf("manifest exceeds %d byte limit", maxBytes)
	}
	return DecodeManifestFromURL(body, manifestURL)
}
