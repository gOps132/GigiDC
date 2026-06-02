package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func NewID(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		if prefix == "" {
			return hex.EncodeToString(raw[:])
		}
		return prefix + "_" + hex.EncodeToString(raw[:])
	}
	if prefix == "" {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
