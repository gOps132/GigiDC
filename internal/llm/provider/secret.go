package provider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

type SecretSealer interface {
	Seal(ctx context.Context, plaintext, aad []byte) (SealedSecret, error)
	Open(ctx context.Context, sealed SealedSecret, aad []byte) ([]byte, error)
}

type SealedSecret struct {
	Ciphertext []byte
	Nonce      []byte
	KeyID      string
}

type AESGCMSealer struct {
	gcm   cipher.AEAD
	keyID string
}

func NewAESGCMSealer(key []byte, keyID string) (*AESGCMSealer, error) {
	if len(key) != 32 {
		return nil, errors.New("aes-gcm key must be 32 bytes")
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, errors.New("key id is required")
	}

	block, err := aes.NewCipher(append([]byte(nil), key...))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &AESGCMSealer{gcm: gcm, keyID: keyID}, nil
}

func (s *AESGCMSealer) Seal(ctx context.Context, plaintext, aad []byte) (SealedSecret, error) {
	if err := ctx.Err(); err != nil {
		return SealedSecret{}, err
	}
	if len(plaintext) == 0 {
		return SealedSecret{}, errors.New("plaintext is required")
	}

	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return SealedSecret{}, err
	}

	ciphertext := s.gcm.Seal(nil, nonce, plaintext, aad)
	return SealedSecret{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		KeyID:      s.keyID,
	}, nil
}

func (s *AESGCMSealer) Open(ctx context.Context, sealed SealedSecret, aad []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sealed.KeyID != s.keyID {
		return nil, errors.New("sealed secret key id mismatch")
	}
	if len(sealed.Nonce) != s.gcm.NonceSize() {
		return nil, errors.New("sealed secret nonce is invalid")
	}
	if len(sealed.Ciphertext) == 0 {
		return nil, errors.New("sealed secret ciphertext is required")
	}

	plaintext, err := s.gcm.Open(nil, sealed.Nonce, sealed.Ciphertext, aad)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func Fingerprint(secret string) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("secret is required")
	}
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])[:16], nil
}
