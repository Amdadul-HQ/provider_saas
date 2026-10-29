package vault

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
)

func TestCipherRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	c, err := NewCipher(key, 1)
	if err != nil {
		t.Fatal(err)
	}
	tenant := uuid.New()
	ct, nonce, ver, err := c.Encrypt([]byte(`{"username":"a","password":"b"}`), tenant[:])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, []byte("password")) {
		t.Fatal("ciphertext contains plaintext")
	}
	pt, err := c.Decrypt(ct, nonce, tenant[:], ver)
	if err != nil || string(pt) != `{"username":"a","password":"b"}` {
		t.Fatalf("decrypt = %q, %v", pt, err)
	}
	other := uuid.New()
	if _, err := c.Decrypt(ct, nonce, other[:], ver); err == nil {
		t.Fatal("decrypted with another tenant's AAD")
	}
	if _, err := c.Decrypt(ct, nonce, tenant[:], 2); err == nil {
		t.Fatal("unknown version accepted")
	}
	// rotation: new current key, old key still decrypts
	c2, _ := NewCipher(bytes.Repeat([]byte{9}, 32), 2)
	c2.AddKey(1, key)
	if _, err := c2.Decrypt(ct, nonce, tenant[:], 1); err != nil {
		t.Fatal("rotation: old key failed", err)
	}
	if _, err := NewCipher([]byte("short"), 1); err == nil {
		t.Fatal("short key accepted")
	}
}
