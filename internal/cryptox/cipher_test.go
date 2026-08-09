package cryptox

import (
	"testing"
)

func TestCipherRoundTripAndTamperDetection(t *testing.T) {
	t.Parallel()
	c, err := Load(t.TempDir(), "test-only-master-key-with-enough-entropy")
	if err != nil {
		t.Fatal(err)
	}

	const secret = "SID=secret; SAPISID=top-secret"
	sealed, err := c.Encrypt(secret)
	if err != nil {
		t.Fatal(err)
	}
	if sealed == secret {
		t.Fatal("ciphertext unexpectedly equals plaintext")
	}
	plain, err := c.Decrypt(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if plain != secret {
		t.Fatalf("got %q, want %q", plain, secret)
	}

	tampered := sealed[:len(sealed)-1] + "A"
	if tampered != sealed {
		if _, err := c.Decrypt(tampered); err == nil {
			t.Fatal("tampered ciphertext was accepted")
		}
	}
}
