package provider

import (
	"bytes"
	"context"
	"encoding/hex"
	"strings"
	"testing"
)

func TestAESGCMSealerRoundTripsWithAAD(t *testing.T) {
	ctx := context.Background()
	sealer := mustAESGCMSealer(t, "primary")
	plaintext := []byte("sk-live-test-secret")
	aad := []byte("provider:openai:owner:guild:123")

	sealed, err := sealer.Seal(ctx, plaintext, aad)
	if err != nil {
		t.Fatalf("Seal returned error: %v", err)
	}
	if sealed.KeyID != "primary" {
		t.Fatalf("KeyID = %q, want primary", sealed.KeyID)
	}
	if len(sealed.Nonce) == 0 {
		t.Fatal("Seal returned empty nonce")
	}
	if bytes.Contains(sealed.Ciphertext, plaintext) {
		t.Fatal("ciphertext contains plaintext")
	}

	opened, err := sealer.Open(ctx, sealed, aad)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if !bytes.Equal(opened, plaintext) {
		t.Fatalf("opened plaintext = %q, want %q", opened, plaintext)
	}
}

func TestAESGCMSealerRejectsAADMismatch(t *testing.T) {
	ctx := context.Background()
	sealer := mustAESGCMSealer(t, "primary")

	sealed, err := sealer.Seal(ctx, []byte("sk-live-test-secret"), []byte("aad-one"))
	if err != nil {
		t.Fatalf("Seal returned error: %v", err)
	}

	if _, err := sealer.Open(ctx, sealed, []byte("aad-two")); err == nil {
		t.Fatal("Open succeeded with mismatched AAD")
	}
}

func TestAESGCMSealerValidatesInputs(t *testing.T) {
	ctx := context.Background()

	if _, err := NewAESGCMSealer([]byte("short"), "primary"); err == nil {
		t.Fatal("NewAESGCMSealer accepted short key")
	}
	if _, err := NewAESGCMSealer(bytes.Repeat([]byte{1}, 32), " \t "); err == nil {
		t.Fatal("NewAESGCMSealer accepted empty keyID")
	}

	sealer := mustAESGCMSealer(t, "primary")
	if _, err := sealer.Seal(ctx, nil, []byte("aad")); err == nil {
		t.Fatal("Seal accepted nil plaintext")
	}
	if _, err := sealer.Seal(ctx, []byte{}, []byte("aad")); err == nil {
		t.Fatal("Seal accepted empty plaintext")
	}
	if _, err := sealer.Open(ctx, SealedSecret{}, []byte("aad")); err == nil {
		t.Fatal("Open accepted empty sealed secret")
	}
}

func TestFingerprintIsStableAndDoesNotRevealSecret(t *testing.T) {
	secret := "sk-live-test-secret"

	first, err := Fingerprint(secret)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	second, err := Fingerprint(secret)
	if err != nil {
		t.Fatalf("Fingerprint returned error on second call: %v", err)
	}

	if first != second {
		t.Fatalf("fingerprints differ: %q != %q", first, second)
	}
	if first == secret || strings.Contains(first, secret) {
		t.Fatalf("fingerprint reveals raw secret: %q", first)
	}
	if len(first) == 0 {
		t.Fatal("fingerprint is empty")
	}
	if _, err := hex.DecodeString(first); err != nil {
		t.Fatalf("fingerprint is not hex: %v", err)
	}
	if len(first) >= 64 {
		t.Fatalf("fingerprint = %q, want sha256 hex prefix shorter than full digest", first)
	}
}

func TestFingerprintRejectsEmptySecret(t *testing.T) {
	if _, err := Fingerprint(""); err == nil {
		t.Fatal("Fingerprint accepted empty secret")
	}
	if _, err := Fingerprint(" \t "); err == nil {
		t.Fatal("Fingerprint accepted whitespace-only secret")
	}
}

func mustAESGCMSealer(t *testing.T, keyID string) SecretSealer {
	t.Helper()
	sealer, err := NewAESGCMSealer(bytes.Repeat([]byte{7}, 32), keyID)
	if err != nil {
		t.Fatalf("NewAESGCMSealer returned error: %v", err)
	}
	return sealer
}
