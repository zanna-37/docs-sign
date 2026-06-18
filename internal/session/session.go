// Package session keeps decrypted Data Encryption Keys in memory only, keyed by an opaque
// session token carried in a cookie. Sessions expire on idle and absolute timeouts and are
// scrubbed (DEK zeroized) on logout, expiry, or user deletion. Nothing here is persisted —
// a server restart drops every session and users simply log in again.
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"docs-sign/internal/crypto"
)

// Session is a live, authenticated session holding the user's plaintext DEK.
type Session struct {
	Token              string
	UserID             string
	Username           string
	IsAdmin            bool
	MustChangePassword bool
	dek                []byte
	createdAt          time.Time
	lastSeen           time.Time
}

// DEK returns a copy of the session's Data Encryption Key for use by a single operation.
func (s *Session) DEK() []byte {
	cp := make([]byte, len(s.dek))
	copy(cp, s.dek)
	return cp
}

// Manager is a concurrency-safe in-memory session store.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	idleTTL  time.Duration
	absTTL   time.Duration
	now      func() time.Time
}

// NewManager creates a session manager with the given idle and absolute timeouts.
func NewManager(idleTTL, absTTL time.Duration) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		idleTTL:  idleTTL,
		absTTL:   absTTL,
		now:      time.Now,
	}
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("session: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Create starts a new session for a user. The provided dek is copied; the caller may scrub
// its own copy afterward.
func (m *Manager) Create(userID, username string, isAdmin, mustChange bool, dek []byte) *Session {
	dekCopy := make([]byte, len(dek))
	copy(dekCopy, dek)
	now := m.now()
	s := &Session{
		Token:              newToken(),
		UserID:             userID,
		Username:           username,
		IsAdmin:            isAdmin,
		MustChangePassword: mustChange,
		dek:                dekCopy,
		createdAt:          now,
		lastSeen:           now,
	}
	m.mu.Lock()
	m.sessions[s.Token] = s
	m.mu.Unlock()
	return s
}

func (m *Manager) expired(s *Session, now time.Time) bool {
	return now.Sub(s.lastSeen) > m.idleTTL || now.Sub(s.createdAt) > m.absTTL
}

// Get returns the session for a token, refreshing its idle timer. Returns false if the
// token is unknown or expired (expired sessions are scrubbed and removed).
func (m *Manager) Get(token string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[token]
	if !ok {
		return nil, false
	}
	now := m.now()
	if m.expired(s, now) {
		crypto.Zero(s.dek)
		delete(m.sessions, token)
		return nil, false
	}
	s.lastSeen = now
	return s, true
}

// SetMustChangePassword updates the flag on a live session (after a forced password change).
func (m *Manager) SetMustChangePassword(token string, v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[token]; ok {
		s.MustChangePassword = v
	}
}

// Delete ends a session and scrubs its DEK.
func (m *Manager) Delete(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[token]; ok {
		crypto.Zero(s.dek)
		delete(m.sessions, token)
	}
}

// DeleteByUser ends all sessions for a user (used when an account is disabled or deleted).
func (m *Manager) DeleteByUser(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for token, s := range m.sessions {
		if s.UserID == userID {
			crypto.Zero(s.dek)
			delete(m.sessions, token)
		}
	}
}

// StartJanitor runs periodic eviction of expired sessions until ctx is cancelled.
func (m *Manager) StartJanitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				m.purgeAll()
				return
			case <-ticker.C:
				m.evictExpired()
			}
		}
	}()
}

func (m *Manager) evictExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	for token, s := range m.sessions {
		if m.expired(s, now) {
			crypto.Zero(s.dek)
			delete(m.sessions, token)
		}
	}
}

func (m *Manager) purgeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for token, s := range m.sessions {
		crypto.Zero(s.dek)
		delete(m.sessions, token)
	}
}
