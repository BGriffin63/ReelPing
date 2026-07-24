package plex

import (
	"errors"
	"strings"
	"syscall"
)

// isConnRefused reports whether err is a "connection refused" error. It works
// cross-platform: syscall.ECONNREFUSED is defined on both Unix and Windows
// (mapped to WSAECONNREFUSED). A string fallback covers wrapped/custom errors.
func isConnRefused(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "actively refused") // Windows wording
}
