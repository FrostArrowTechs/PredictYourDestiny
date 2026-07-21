package secret

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func testKey() string {
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func TestCipherRoundTripAndRandomNonce(t *testing.T) {
	c, err := New(testKey())
	if err != nil {
		t.Fatal(err)
	}
	first, err := c.Encrypt("provider-secret")
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Encrypt("provider-secret")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, prefix) {
		t.Fatal("ciphertext must be versioned and use a random nonce")
	}
	plain, err := c.Decrypt(first)
	if err != nil || plain != "provider-secret" {
		t.Fatalf("Decrypt() = %q, %v", plain, err)
	}
}

func TestCipherRejectsMissingKeyAndPlaintext(t *testing.T) {
	var c *Cipher
	if _, err := c.Encrypt("secret"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Encrypt error = %v", err)
	}
	configured, err := New(testKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := configured.Decrypt("plaintext"); err == nil {
		t.Fatal("plaintext was accepted")
	}
}
