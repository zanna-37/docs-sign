# docs-sign

A self-hosted, zero-knowledge PDF signing web application. Users upload PDF documents
and PNG signatures, place/move/resize/rotate signatures visually, and export a
**fully flattened** (rasterized) PDF from which the original signature image cannot be
recovered.

All user content (signatures, documents, exports) is encrypted at rest with per-user
keys. The server only ever holds decryption keys **in memory** for the duration of a
session and **never** writes plaintext to disk.

## Security model

- **Envelope encryption.** Each user has a random 256-bit Data Encryption Key (DEK) that
  encrypts every blob with AES-256-GCM. The DEK itself is wrapped twice:
  - by a key derived from the user's password (Argon2id), and
  - by a key derived from a one-time **recovery code** (shown once at first login).
- Disk only ever stores the wrapped DEKs plus ciphertext. The plaintext DEK lives only in
  a session-scoped, in-memory store with idle + absolute timeouts and is zeroized on
  logout/expiry. A server restart drops all DEKs — users simply log in again.
- **No recovery without secrets.** Losing both the password and the recovery code means the
  data is unrecoverable by design. Admins can manage users but **cannot** decrypt their data.

## Signing & flattening

The browser renders a transiently-decrypted copy of the PDF (over TLS) for WYSIWYG
placement. On export, the server decrypts in RAM, renders every page to a bitmap with
**PDFium** (embedded as a WebAssembly module via wazero — no cgo), composites the
scaled/rotated signature PNGs onto those bitmaps, and assembles an **image-only** output
PDF. Every page is rasterized, so no signature object survives in the output.

## Users

No public registration. On first run an **admin** account is created interactively.
The admin adds/removes users; each new user gets a temporary password and is forced to set
their own password (and is shown their recovery code) on first login.

## Stack

- Backend: Go (`net/http` + chi), `modernc.org/sqlite` (pure-Go metadata DB),
  filesystem-backed encrypted blob store.
- PDF: `github.com/klippa-app/go-pdfium` (WebAssembly/wazero).
- Frontend: React + TypeScript + Tailwind, built with Vite and embedded in the binary via
  `//go:embed`.
- Deployment: serves plain HTTP behind your TLS-terminating reverse proxy (nginx/Caddy/…).

## Build

```sh
make build        # builds frontend + single Go binary into ./bin/docs-sign
./bin/docs-sign --data ./data --addr 127.0.0.1:8080
```

See `make help` for all targets.
