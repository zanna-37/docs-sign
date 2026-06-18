// Package crypto implements the zero-knowledge envelope-encryption primitives used
// throughout docs-sign:
//
//   - Argon2id key derivation from passwords and recovery codes.
//   - AES-256-GCM key wrapping for the per-user Data Encryption Key (DEK).
//   - A chunked, authenticated stream cipher (AES-256-GCM per chunk) for blobs, so
//     large documents can be encrypted to / decrypted from disk with bounded memory
//     while still detecting truncation, reordering and tampering.
//
// The server only ever holds a plaintext DEK in memory; it is never written to disk.
package crypto

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// KeyLen is the AES key length (AES-256).
	KeyLen = 32
	// SaltLen is the length of a KDF salt.
	SaltLen = 16
	// DEKLen is the length of a Data Encryption Key.
	DEKLen = 32

	// DefaultChunkSize is the plaintext size of each blob chunk.
	DefaultChunkSize = 64 * 1024

	blobMagic = "DSB1"
)

// ErrDecrypt is returned for any authentication/decryption failure. It is intentionally
// opaque so callers cannot distinguish "wrong key" from "corrupted data".
var ErrDecrypt = errors.New("crypto: decryption failed (wrong key or corrupted data)")

// KDFParams holds Argon2id cost parameters.
type KDFParams struct {
	Time    uint32 // number of iterations
	Memory  uint32 // memory in KiB
	Threads uint8  // parallelism
}

// DefaultKDFParams returns sensible interactive-login Argon2id parameters.
func DefaultKDFParams() KDFParams {
	return KDFParams{Time: 3, Memory: 64 * 1024, Threads: 4}
}

// DeriveKey derives a 32-byte key from a secret and salt using Argon2id.
func DeriveKey(secret, salt []byte, p KDFParams) []byte {
	return argon2.IDKey(secret, salt, p.Time, p.Memory, p.Threads, KeyLen)
}

// RandomBytes returns n cryptographically random bytes.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// NewSalt returns a fresh random salt.
func NewSalt() ([]byte, error) { return RandomBytes(SaltLen) }

// GenerateDEK returns a fresh random Data Encryption Key.
func GenerateDEK() ([]byte, error) { return RandomBytes(DEKLen) }

// Zero overwrites b with zeros. Best-effort scrubbing of key material.
func Zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func aesgcm(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// WrapKey encrypts dek under kek using AES-256-GCM. Output layout: nonce || ciphertext.
func WrapKey(kek, dek []byte) ([]byte, error) {
	g, err := aesgcm(kek)
	if err != nil {
		return nil, err
	}
	nonce, err := RandomBytes(g.NonceSize())
	if err != nil {
		return nil, err
	}
	ct := g.Seal(nil, nonce, dek, nil)
	return append(nonce, ct...), nil
}

// UnwrapKey reverses WrapKey. A wrong kek yields ErrDecrypt (this is how password and
// recovery-code verification happens — there is no separate password hash on disk).
func UnwrapKey(kek, wrapped []byte) ([]byte, error) {
	g, err := aesgcm(kek)
	if err != nil {
		return nil, err
	}
	ns := g.NonceSize()
	if len(wrapped) < ns {
		return nil, ErrDecrypt
	}
	nonce, ct := wrapped[:ns], wrapped[ns:]
	dek, err := g.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	return dek, nil
}

var recoveryEnc = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateRecoveryCode returns a human-friendly recovery code for display (grouped with
// dashes) and its canonical form used for key derivation. The display string is shown to
// the user exactly once; only a DEK wrapped under it is ever persisted.
func GenerateRecoveryCode() (display, canonical string, err error) {
	raw, err := RandomBytes(20) // 160 bits -> 32 base32 chars
	if err != nil {
		return "", "", err
	}
	canonical = recoveryEnc.EncodeToString(raw)
	var b strings.Builder
	for i, r := range canonical {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return b.String(), canonical, nil
}

// NormalizeRecoveryCode canonicalizes user-entered recovery codes (strips spaces/dashes,
// uppercases) so they match the canonical form returned by GenerateRecoveryCode.
func NormalizeRecoveryCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func chunkAAD(index uint64, final bool) []byte {
	aad := make([]byte, 9)
	binary.BigEndian.PutUint64(aad[:8], index)
	if final {
		aad[8] = 1
	}
	return aad
}

// EncryptStream encrypts plaintext from r into w using chunked AES-256-GCM.
//
// Layout: magic(4) || chunkSize(uint32) then, repeated, nonce(12) || ctLen(uint32) || ct.
// Each chunk authenticates its index and a "final" flag in the AAD, so truncating,
// reordering, or appending chunks is detected on decrypt.
func EncryptStream(dek []byte, r io.Reader, w io.Writer) error {
	g, err := aesgcm(dek)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, blobMagic); err != nil {
		return err
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], DefaultChunkSize)
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}

	br := bufio.NewReaderSize(r, DefaultChunkSize)
	buf := make([]byte, DefaultChunkSize)
	var index uint64
	for {
		n, rerr := io.ReadFull(br, buf)
		if rerr != nil && rerr != io.EOF && rerr != io.ErrUnexpectedEOF {
			return rerr
		}
		final := n < len(buf)
		if !final {
			// Full chunk: peek to learn whether any data remains.
			if _, perr := br.Peek(1); perr == io.EOF {
				final = true
			} else if perr != nil {
				return perr
			}
		}
		if err := writeChunk(w, g, index, buf[:n], final); err != nil {
			return err
		}
		index++
		if final {
			return nil
		}
	}
}

func writeChunk(w io.Writer, g cipher.AEAD, index uint64, pt []byte, final bool) error {
	nonce, err := RandomBytes(g.NonceSize())
	if err != nil {
		return err
	}
	ct := g.Seal(nil, nonce, pt, chunkAAD(index, final))
	if _, err := w.Write(nonce); err != nil {
		return err
	}
	var l [4]byte
	binary.BigEndian.PutUint32(l[:], uint32(len(ct)))
	if _, err := w.Write(l[:]); err != nil {
		return err
	}
	_, err = w.Write(ct)
	return err
}

// DecryptStream reverses EncryptStream.
func DecryptStream(dek []byte, r io.Reader, w io.Writer) error {
	g, err := aesgcm(dek)
	if err != nil {
		return err
	}
	br := bufio.NewReader(r)

	magic := make([]byte, len(blobMagic))
	if _, err := io.ReadFull(br, magic); err != nil || string(magic) != blobMagic {
		return ErrDecrypt
	}
	var hdr [4]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil {
		return ErrDecrypt
	}

	nonce := make([]byte, g.NonceSize())
	var l [4]byte
	var index uint64
	for {
		if _, err := io.ReadFull(br, nonce); err != nil {
			return ErrDecrypt
		}
		if _, err := io.ReadFull(br, l[:]); err != nil {
			return ErrDecrypt
		}
		ct := make([]byte, binary.BigEndian.Uint32(l[:]))
		if _, err := io.ReadFull(br, ct); err != nil {
			return ErrDecrypt
		}
		// The encryptor marked the last chunk final exactly when the stream ended; mirror
		// that by checking for EOF here. A mismatch (truncation/append) fails the AAD.
		final := false
		if _, perr := br.Peek(1); perr == io.EOF {
			final = true
		}
		pt, err := g.Open(nil, nonce, ct, chunkAAD(index, final))
		if err != nil {
			return ErrDecrypt
		}
		if _, err := w.Write(pt); err != nil {
			return err
		}
		index++
		if final {
			return nil
		}
	}
}

// EncryptBytes is a convenience wrapper around EncryptStream for in-memory data.
func EncryptBytes(dek, plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := EncryptStream(dek, bytes.NewReader(plaintext), &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecryptBytes is a convenience wrapper around DecryptStream for in-memory data.
func DecryptBytes(dek, ciphertext []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := DecryptStream(dek, bytes.NewReader(ciphertext), &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
