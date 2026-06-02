package storage

import "testing"

func TestPostgresTCPAddressDefaultsPort(t *testing.T) {
	got, err := postgresTCPAddress("postgres://gigi:gigi@db/gigi?sslmode=disable")
	if err != nil {
		t.Fatalf("postgresTCPAddress returned error: %v", err)
	}
	if got != "db:5432" {
		t.Fatalf("address = %q, want db:5432", got)
	}
}

func TestPostgresTCPAddressRejectsUnsupportedScheme(t *testing.T) {
	_, err := postgresTCPAddress("mysql://example")
	if err == nil {
		t.Fatal("expected unsupported scheme error")
	}
}
