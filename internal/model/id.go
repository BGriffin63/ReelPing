package model

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"strings"
	"time"
)

var b32 = base32.NewEncoding("0123456789abcdefghjkmnpqrstvwxyz").WithPadding(base32.NoPadding)

// NewID returns a lexicographically time-sortable identifier: 6 bytes of
// milliseconds since epoch followed by 5 random bytes, base32-encoded. Because
// the time prefix sorts with the value, records keyed by ID iterate in
// chronological order, and reverse iteration yields newest-first — while the ID
// is still globally unique and safe to expose (no secrets, URL-safe).
func NewID() string {
	var buf [11]byte
	ms := uint64(time.Now().UnixMilli())
	// 6 bytes of milliseconds (enough until year ~10889).
	buf[0] = byte(ms >> 40)
	buf[1] = byte(ms >> 32)
	buf[2] = byte(ms >> 24)
	buf[3] = byte(ms >> 16)
	buf[4] = byte(ms >> 8)
	buf[5] = byte(ms)
	_, _ = rand.Read(buf[6:])
	return b32.EncodeToString(buf[:])
}

// ShortID returns a short display form of an ID (first 8 chars, uppercased).
func ShortID(id string) string {
	if len(id) > 8 {
		id = id[:8]
	}
	return strings.ToUpper(id)
}

// randToken returns n random bytes base32-encoded (used for opaque tokens).
func randToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b32.EncodeToString(b)
}

// NewSessionID returns a 256-bit random opaque session identifier.
func NewSessionID() string { return randToken(32) }

// NewCSRFToken returns a 128-bit random CSRF token.
func NewCSRFToken() string { return randToken(16) }

var _ = binary.BigEndian // reserved for future numeric keys
