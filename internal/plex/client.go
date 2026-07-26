package plex

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/BGriffin63/reelping/internal/security"
)

// maxResponseBytes bounds every Plex response body we read.
const maxResponseBytes = 1 << 20 // 1 MiB

// maxRedirects bounds redirect following.
const maxRedirects = 3

// clientIdentifier is sent as X-Plex-Client-Identifier.
const clientIdentifier = "reelping-monitor"

// Options configures a Plex client / check.
type Options struct {
	BaseURL           string
	Token             string
	ExpectedMachineID string
	VerifyTLS         bool
	Timeout           time.Duration
	// SupplementalHostDiag enables best-effort host reachability diagnostics.
	SupplementalHostDiag bool
	// FetchSessions enables the authenticated /status/sessions stream count.
	FetchSessions bool
}

// Client performs Plex checks. It builds a bounded http.Client per check so TLS
// verification and timeouts always reflect the current options.
type Client struct {
	opts Options
	host string
	port string
}

// New builds a client from options, parsing and validating the base URL.
func New(opts Options) (*Client, error) {
	normalized, err := security.ValidatePlexBaseURL(opts.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Plex URL: %w", err)
	}
	opts.BaseURL = normalized
	u, err := url.Parse(normalized)
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	return &Client{opts: opts, host: host, port: port}, nil
}

// httpClient builds a bounded HTTP client with a strict redirect policy.
func (c *Client) httpClient() *http.Client {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   c.opts.Timeout,
			KeepAlive: -1,
		}).DialContext,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: !c.opts.VerifyTLS}, //nolint:gosec // toggle is an explicit admin choice
		TLSHandshakeTimeout:   c.opts.Timeout,
		ResponseHeaderTimeout: c.opts.Timeout,
		ExpectContinueTimeout: time.Second,
		MaxIdleConns:          1,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{
		Timeout:   c.opts.Timeout,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("refusing redirect to unsupported scheme %q", req.URL.Scheme)
			}
			// Do not carry the Plex token across redirects to other hosts.
			return nil
		},
	}
}

// endpoint builds an absolute URL for a Plex path under the base URL. The token
// is NEVER included here; it is sent as a header.
func (c *Client) endpoint(path string) string {
	base := c.opts.BaseURL
	return base + path
}
