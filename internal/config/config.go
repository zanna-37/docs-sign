// Package config parses command-line flags into a typed configuration.
package config

import (
	"flag"
	"time"

	"docs-sign/internal/crypto"
	"docs-sign/internal/pdfproc"
)

// Config holds runtime configuration.
type Config struct {
	DataDir        string
	Addr           string
	Dev            bool
	IdleTimeout    time.Duration
	SessionTimeout time.Duration
	ExportDPI      int
	MaxUploadBytes int64
	TrashRetention time.Duration
	KDF            crypto.KDFParams
}

// Parse reads configuration from the given argument list (excluding the program name).
func Parse(args []string) (*Config, error) {
	fs := flag.NewFlagSet("docs-sign", flag.ContinueOnError)
	var (
		dataDir    = fs.String("data", "./data", "data directory for the SQLite database and encrypted blobs")
		addr       = fs.String("addr", "127.0.0.1:8080", "HTTP listen address (serve plain HTTP behind a TLS-terminating reverse proxy)")
		dev        = fs.Bool("dev", false, "development mode: non-Secure cookies for use with the Vite dev server")
		idle       = fs.Duration("idle-timeout", 120*time.Minute, "session idle timeout")
		sessionTTL = fs.Duration("session-timeout", 24*time.Hour, "absolute session timeout")
		dpi        = fs.Int("dpi", pdfproc.DefaultDPI, "rasterization DPI for flattened exports")
		maxUpload  = fs.Int64("max-upload-mb", 128, "maximum upload size in megabytes")
		trashDays  = fs.Int("trash-days", 30, "days to keep items in the trash before permanent deletion")
	)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return &Config{
		DataDir:        *dataDir,
		Addr:           *addr,
		Dev:            *dev,
		IdleTimeout:    *idle,
		SessionTimeout: *sessionTTL,
		ExportDPI:      *dpi,
		MaxUploadBytes: *maxUpload * 1024 * 1024,
		TrashRetention: time.Duration(*trashDays) * 24 * time.Hour,
		KDF:            crypto.DefaultKDFParams(),
	}, nil
}
