package storage

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"
)

type TCPReadyCheck struct {
	databaseURL string
	timeout     time.Duration
}

func NewTCPReadyCheck(databaseURL string, timeout time.Duration) TCPReadyCheck {
	return TCPReadyCheck{
		databaseURL: databaseURL,
		timeout:     timeout,
	}
}

func (c TCPReadyCheck) Ready(ctx context.Context) error {
	target, err := postgresTCPAddress(c.databaseURL)
	if err != nil {
		return err
	}

	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return fmt.Errorf("database is not reachable: %w", err)
	}
	return conn.Close()
}

func postgresTCPAddress(databaseURL string) (string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return "", fmt.Errorf("parse database URL: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return "", fmt.Errorf("unsupported database URL scheme %q", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("database URL host is required")
	}

	port := parsed.Port()
	if port == "" {
		port = "5432"
	}

	return net.JoinHostPort(host, port), nil
}
