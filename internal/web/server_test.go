package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/buildinfo"
)

func TestHealthz(t *testing.T) {
	server := NewServer(Options{
		Build: buildinfo.Info{Version: "test", Commit: "abc", BuildTime: "now"},
		Ready: func(context.Context) error {
			return nil
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"version":"test"`) {
		t.Fatalf("body missing version: %s", recorder.Body.String())
	}
}

func TestReadyzFailsClosed(t *testing.T) {
	server := NewServer(Options{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
