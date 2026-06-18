package crypto

import (
	"bytes"
	"testing"
)

func TestDeriveKeyDeterministic(t *testing.T) {
	salt := bytes.Repeat([]byte{0x01}, SaltLen)
	p := DefaultKDFParams()
	k1 := DeriveKey([]byte("hunter2"), salt, p)
	k2 := DeriveKey([]byte("hunter2"), salt, p)
	if !bytes.Equal(k1, k2) {
		t.Fatal("derivation not deterministic")
	}
	if len(k1) != KeyLen {
		t.Fatalf("key len = %d, want %d", len(k1), KeyLen)
	}
	if k3 := DeriveKey([]byte("hunter3"), salt, p); bytes.Equal(k1, k3) {
		t.Fatal("different passwords produced same key")
	}
}

func TestWrapUnwrap(t *testing.T) {
	kek := bytes.Repeat([]byte{0x02}, KeyLen)
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := WrapKey(kek, dek)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnwrapKey(kek, wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("unwrapped DEK mismatch")
	}
	// Wrong key must fail.
	badKek := bytes.Repeat([]byte{0x03}, KeyLen)
	if _, err := UnwrapKey(badKek, wrapped); err != ErrDecrypt {
		t.Fatalf("expected ErrDecrypt, got %v", err)
	}
	// Tampered ciphertext must fail.
	wrapped[len(wrapped)-1] ^= 0xff
	if _, err := UnwrapKey(kek, wrapped); err != ErrDecrypt {
		t.Fatalf("expected ErrDecrypt on tamper, got %v", err)
	}
}

func TestRecoveryCodeRoundTrip(t *testing.T) {
	display, canonical, err := GenerateRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	if NormalizeRecoveryCode(display) != canonical {
		t.Fatalf("normalize(%q)=%q, want %q", display, NormalizeRecoveryCode(display), canonical)
	}
	// Lowercase + extra spaces should still normalize correctly.
	messy := "  " + display + "  "
	if NormalizeRecoveryCode(messy) != canonical {
		t.Fatal("messy recovery code did not normalize")
	}
	// A wrapped DEK should unwrap with a key derived from the canonical code.
	salt, _ := NewSalt()
	kek := DeriveKey([]byte(canonical), salt, DefaultKDFParams())
	dek, _ := GenerateDEK()
	wrapped, _ := WrapKey(kek, dek)
	kek2 := DeriveKey([]byte(NormalizeRecoveryCode(display)), salt, DefaultKDFParams())
	got, err := UnwrapKey(kek2, wrapped)
	if err != nil || !bytes.Equal(got, dek) {
		t.Fatal("recovery-derived key failed to unwrap DEK")
	}
}

func TestStreamRoundTrip(t *testing.T) {
	dek, _ := GenerateDEK()
	sizes := []int{0, 1, 100, DefaultChunkSize - 1, DefaultChunkSize, DefaultChunkSize + 1, 3*DefaultChunkSize + 7}
	for _, sz := range sizes {
		plain := make([]byte, sz)
		for i := range plain {
			plain[i] = byte(i * 7)
		}
		ct, err := EncryptBytes(dek, plain)
		if err != nil {
			t.Fatalf("size %d encrypt: %v", sz, err)
		}
		got, err := DecryptBytes(dek, ct)
		if err != nil {
			t.Fatalf("size %d decrypt: %v", sz, err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("size %d round-trip mismatch", sz)
		}
	}
}

func TestStreamWrongKey(t *testing.T) {
	dek, _ := GenerateDEK()
	other, _ := GenerateDEK()
	ct, _ := EncryptBytes(dek, []byte("top secret signature bytes"))
	if _, err := DecryptBytes(other, ct); err != ErrDecrypt {
		t.Fatalf("expected ErrDecrypt with wrong key, got %v", err)
	}
}

func TestStreamTruncationDetected(t *testing.T) {
	dek, _ := GenerateDEK()
	plain := make([]byte, 3*DefaultChunkSize)
	ct, _ := EncryptBytes(dek, plain)
	// Drop the last 50 bytes to simulate truncation.
	truncated := ct[:len(ct)-50]
	if _, err := DecryptBytes(dek, truncated); err != ErrDecrypt {
		t.Fatalf("expected ErrDecrypt on truncation, got %v", err)
	}
}

func TestStreamTamperDetected(t *testing.T) {
	dek, _ := GenerateDEK()
	ct, _ := EncryptBytes(dek, bytes.Repeat([]byte("x"), 5000))
	ct[len(ct)/2] ^= 0x01
	if _, err := DecryptBytes(dek, ct); err != ErrDecrypt {
		t.Fatalf("expected ErrDecrypt on tamper, got %v", err)
	}
}
