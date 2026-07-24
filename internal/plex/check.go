package plex

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// mediaContainer captures the attributes ReelPing needs from a Plex
// MediaContainer XML response. The decoder ignores all other fields, and we
// never read child elements of /status/sessions (which contain viewer data).
type mediaContainer struct {
	XMLName           xml.Name `xml:"MediaContainer"`
	Size              int      `xml:"size,attr"`
	MachineIdentifier string   `xml:"machineIdentifier,attr"`
	Version           string   `xml:"version,attr"`
	FriendlyName      string   `xml:"friendlyName,attr"`
}

// Check runs the full multi-stage availability check and returns a classified
// result. It never blocks longer than roughly (stages × timeout).
func (c *Client) Check(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{}

	// Stage 2: DNS resolution (only when the host is a name, not an IP literal).
	if net.ParseIP(c.host) == nil {
		dnsCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
		_, err := net.DefaultResolver.LookupHost(dnsCtx, c.host)
		cancel()
		if err != nil {
			res.Stage = "dns"
			res.Classification = DNSFailure
			res.Detail = "Could not resolve the Plex hostname."
			res.LatencyMillis = time.Since(start).Milliseconds()
			return res
		}
	}

	// Stage 3: TCP connectivity. The dial error tells us host-vs-service.
	dialCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	conn, dialErr := (&net.Dialer{}).DialContext(dialCtx, "tcp", net.JoinHostPort(c.host, c.port))
	cancel()
	if dialErr != nil {
		if conn != nil {
			_ = conn.Close()
		}
		return c.classifyDialFailure(ctx, dialErr, start)
	}
	_ = conn.Close()
	res.HostReachable = true

	// Stage 4: HTTP request to /identity (no token required).
	ident, httpRes := c.fetchIdentity(ctx)
	if !httpRes.OK {
		httpRes.HostReachable = true
		httpRes.LatencyMillis = time.Since(start).Milliseconds()
		return httpRes
	}
	res = httpRes
	res.HostReachable = true
	res.MachineID = ident.MachineIdentifier
	res.ServerVersion = ident.Version

	// Stage 5: authenticated verification (identity + optional sessions).
	if c.opts.ExpectedMachineID != "" {
		if !strings.EqualFold(strings.TrimSpace(ident.MachineIdentifier), strings.TrimSpace(c.opts.ExpectedMachineID)) {
			res.Classification = IdentityMismatch
			res.OK = false
			res.Stage = "identity"
			res.Detail = "The server that responded is not the expected Plex server."
			res.LatencyMillis = time.Since(start).Milliseconds()
			return res
		}
		res.IdentityVerified = true
	}

	if c.opts.Token != "" {
		// Friendly name from the root endpoint (best-effort; auth-aware).
		if name := c.fetchFriendlyName(ctx); name != "" {
			res.ServerName = name
		}
		if c.opts.FetchSessions {
			count, ok, authErr := c.fetchSessionCount(ctx)
			if authErr == errAuthFailed {
				res.Classification = AuthenticationFailure
				res.OK = false
				res.Stage = "auth"
				res.Detail = "Plex rejected the configured token."
				res.LatencyMillis = time.Since(start).Milliseconds()
				return res
			}
			if ok {
				res.StreamCount = count
				res.StreamCountKnown = true
			}
		}
	}

	res.Classification = Online
	res.OK = true
	res.Stage = "complete"
	res.Detail = "Plex responded successfully."
	res.LatencyMillis = time.Since(start).Milliseconds()
	return res
}

var errAuthFailed = errors.New("plex authentication failed")

// classifyDialFailure turns a TCP dial error into a classification, optionally
// running supplemental host diagnostics to distinguish "service down" from
// "host down". ReelPing deliberately does NOT use ICMP (needs privileges and is
// often blocked); instead it interprets connection-refused vs timeout and,
// when enabled, probes other common ports.
func (c *Client) classifyDialFailure(ctx context.Context, dialErr error, start time.Time) CheckResult {
	res := CheckResult{Stage: "tcp", LatencyMillis: time.Since(start).Milliseconds()}

	var netErr net.Error
	timedOut := errors.As(dialErr, &netErr) && netErr.Timeout()
	refused := isConnRefused(dialErr)

	switch {
	case refused:
		// The host actively refused: it is up, Plex is not listening.
		res.Classification = PlexServiceUnreachable
		res.HostReachable = true
		res.Detail = "The Plex host is reachable but the Plex service is not accepting connections."
		return res
	case timedOut:
		res.Classification = RequestTimeout
		res.Detail = "The connection to Plex timed out."
	default:
		res.Classification = PlexServiceUnreachable
		res.Detail = "Could not connect to the Plex service."
	}

	if c.opts.SupplementalHostDiag {
		res.HostDiagRan = true
		if c.hostAnswersAnywhere(ctx) {
			res.HostReachable = true
			if res.Classification == RequestTimeout {
				res.Classification = PlexServiceUnreachable
				res.Detail = "The Plex host appears reachable, but the Plex service did not respond."
			}
		} else {
			res.HostReachable = false
			res.Classification = HostUnreachable
			res.Detail = "The Plex host did not respond on any probed port; it may be offline."
		}
	}
	return res
}

// hostAnswersAnywhere returns true if the host responds (connect or refuse) on
// any of a few common ports within a short budget — an unprivileged liveness
// heuristic used only as a supplemental diagnostic.
func (c *Client) hostAnswersAnywhere(ctx context.Context) bool {
	ports := []string{c.port, "443", "80"}
	seen := map[string]bool{}
	budget := c.opts.Timeout
	if budget > 2*time.Second {
		budget = 2 * time.Second
	}
	for _, p := range ports {
		if seen[p] {
			continue
		}
		seen[p] = true
		dctx, cancel := context.WithTimeout(ctx, budget)
		conn, err := (&net.Dialer{}).DialContext(dctx, "tcp", net.JoinHostPort(c.host, p))
		cancel()
		if conn != nil {
			_ = conn.Close()
		}
		if err == nil || isConnRefused(err) {
			return true
		}
	}
	return false
}

// fetchIdentity performs the HTTP GET /identity and returns the parsed
// container plus a CheckResult that carries any HTTP-stage error.
func (c *Client) fetchIdentity(ctx context.Context) (mediaContainer, CheckResult) {
	var mc mediaContainer
	req, err := c.newRequest(ctx, "/identity", false)
	if err != nil {
		return mc, CheckResult{Classification: UnknownFailure, Stage: "http", Detail: "Could not build the request."}
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return mc, httpErrorResult(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		// /identity does not require auth, so this is an unusual gateway result.
		return mc, CheckResult{Classification: ResponseError, Stage: "http",
			Detail: fmt.Sprintf("Plex returned HTTP %d.", resp.StatusCode)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mc, CheckResult{Classification: ResponseError, Stage: "http",
			Detail: fmt.Sprintf("Plex returned HTTP %d.", resp.StatusCode)}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return mc, CheckResult{Classification: InvalidResponse, Stage: "http", Detail: "Could not read the Plex response."}
	}
	if err := xml.Unmarshal(body, &mc); err != nil || mc.MachineIdentifier == "" {
		return mc, CheckResult{Classification: InvalidResponse, Stage: "http",
			Detail: "The response did not look like a Plex server."}
	}
	return mc, CheckResult{OK: true, Classification: Online, Stage: "http"}
}

// fetchFriendlyName reads the server's friendly name from the root endpoint.
func (c *Client) fetchFriendlyName(ctx context.Context) string {
	req, err := c.newRequest(ctx, "/", true)
	if err != nil {
		return ""
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return ""
	}
	var mc mediaContainer
	if xml.Unmarshal(body, &mc) == nil {
		return mc.FriendlyName
	}
	return ""
}

// fetchSessionCount reads only the size attribute of /status/sessions. It never
// parses per-session child data.
func (c *Client) fetchSessionCount(ctx context.Context) (int, bool, error) {
	req, err := c.newRequest(ctx, "/status/sessions", true)
	if err != nil {
		return 0, false, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return 0, false, nil // treat transient session errors as "unknown", not outage
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return 0, false, errAuthFailed
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, false, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, false, nil
	}
	var mc mediaContainer
	if xml.Unmarshal(body, &mc) != nil {
		return 0, false, nil
	}
	if mc.Size < 0 {
		mc.Size = 0
	}
	return mc.Size, true, nil
}

func (c *Client) newRequest(ctx context.Context, path string, withToken bool) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/xml")
	req.Header.Set("User-Agent", "ReelPing")
	req.Header.Set("X-Plex-Client-Identifier", clientIdentifier)
	req.Header.Set("X-Plex-Product", "ReelPing")
	if withToken && c.opts.Token != "" {
		req.Header.Set("X-Plex-Token", c.opts.Token)
	}
	return req, nil
}

// httpErrorResult classifies a transport-level HTTP error.
func httpErrorResult(err error) CheckResult {
	res := CheckResult{Stage: "http", OK: false}

	var certErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	if errors.As(err, &certErr) || errors.As(err, &unknownAuthority) || errors.As(err, &hostnameErr) {
		res.Classification = TLSFailure
		res.Detail = "The Plex HTTPS certificate could not be validated."
		return res
	}
	var recordErr *tls.RecordHeaderError
	if errors.As(err, &recordErr) {
		res.Classification = TLSFailure
		res.Detail = "TLS negotiation with Plex failed."
		return res
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		res.Classification = RequestTimeout
		res.Detail = "The Plex request timed out."
		return res
	}
	if isConnRefused(err) {
		res.Classification = PlexServiceUnreachable
		res.Detail = "The Plex service refused the connection."
		return res
	}
	res.Classification = PlexServiceUnreachable
	res.Detail = "The Plex request failed."
	return res
}
