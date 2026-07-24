// Package plex implements ReelPing's multi-stage Plex availability check and a
// minimal, defensive Plex API client. It never uses a shell, never follows
// redirects to non-HTTP(S) schemes, bounds every response, and sends the Plex
// token only as an X-Plex-Token header (never in a URL).
package plex

// Classification is a normalised outcome category for a check.
type Classification string

const (
	Online                 Classification = "online"
	PlexServiceUnreachable Classification = "plex_service_unreachable"
	HostUnreachable        Classification = "host_unreachable"
	RequestTimeout         Classification = "request_timeout"
	DNSFailure             Classification = "dns_failure"
	TLSFailure             Classification = "tls_failure"
	AuthenticationFailure  Classification = "authentication_failure"
	IdentityMismatch       Classification = "identity_mismatch"
	InvalidResponse        Classification = "invalid_response"
	ResponseError          Classification = "response_error"
	UnknownFailure         Classification = "unknown_failure"
)

// PlainLanguage returns a public-safe, human description of a classification,
// suitable for a Discord message. It never leaks URLs, IPs, or internals.
func (c Classification) PlainLanguage() string {
	switch c {
	case Online:
		return "Online"
	case PlexServiceUnreachable:
		return "Plex service unreachable"
	case HostUnreachable:
		return "Host unreachable"
	case RequestTimeout:
		return "Request timed out"
	case DNSFailure:
		return "DNS lookup failed"
	case TLSFailure:
		return "TLS/HTTPS error"
	case AuthenticationFailure:
		return "Plex authentication failed"
	case IdentityMismatch:
		return "Unexpected server identity"
	case InvalidResponse:
		return "Unrecognised response"
	case ResponseError:
		return "Unexpected response status"
	default:
		return "Unknown error"
	}
}

// IsUp reports whether the classification represents a healthy server.
func (c Classification) IsUp() bool { return c == Online }

// CheckResult is the outcome of one availability check. All string fields are
// safe to persist; Detail is already sanitised (no secrets, no full URLs).
type CheckResult struct {
	Classification   Classification `json:"classification"`
	OK               bool           `json:"ok"`
	Stage            string         `json:"stage"`
	LatencyMillis    int64          `json:"latency_millis"`
	MachineID        string         `json:"machine_id,omitempty"`
	ServerName       string         `json:"server_name,omitempty"`
	ServerVersion    string         `json:"server_version,omitempty"`
	IdentityVerified bool           `json:"identity_verified"`
	StreamCount      int            `json:"stream_count"`
	StreamCountKnown bool           `json:"stream_count_known"`
	HostReachable    bool           `json:"host_reachable"`
	HostDiagRan      bool           `json:"host_diag_ran"`
	Detail           string         `json:"detail"`
}
