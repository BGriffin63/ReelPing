package plex

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const identityXML = `<?xml version="1.0" encoding="UTF-8"?>` +
	`<MediaContainer size="0" claimed="1" machineIdentifier="MACHINE123" version="1.40.0.1"></MediaContainer>`

func newClient(t *testing.T, opts Options) *Client {
	t.Helper()
	c, err := New(opts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return c
}

func TestCheckOnline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(identityXML))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	res := newClient(t, Options{BaseURL: srv.URL, Timeout: 2 * time.Second, VerifyTLS: false}).Check(context.Background())
	if !res.OK || res.Classification != Online {
		t.Fatalf("expected online, got %+v", res)
	}
	if res.MachineID != "MACHINE123" {
		t.Fatalf("expected machine id, got %q", res.MachineID)
	}
}

func TestCheckInvalidResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("this is not plex"))
	}))
	defer srv.Close()
	res := newClient(t, Options{BaseURL: srv.URL, Timeout: 2 * time.Second}).Check(context.Background())
	if res.OK || res.Classification != InvalidResponse {
		t.Fatalf("expected invalid_response, got %+v", res)
	}
}

func TestCheckResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	res := newClient(t, Options{BaseURL: srv.URL, Timeout: 2 * time.Second}).Check(context.Background())
	if res.Classification != ResponseError {
		t.Fatalf("expected response_error, got %+v", res)
	}
}

func TestCheckOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<MediaContainer machineIdentifier="X">`))
		big := strings.Repeat("A", 3<<20)
		_, _ = w.Write([]byte(big))
	}))
	defer srv.Close()
	res := newClient(t, Options{BaseURL: srv.URL, Timeout: 3 * time.Second}).Check(context.Background())
	// Truncated body must not crash; it fails to parse -> invalid_response.
	if res.OK {
		t.Fatalf("oversized/garbage should not be OK, got %+v", res)
	}
}

func TestCheckTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(identityXML))
	}))
	defer srv.Close()
	res := newClient(t, Options{BaseURL: srv.URL, Timeout: 100 * time.Millisecond}).Check(context.Background())
	if res.OK {
		t.Fatalf("expected timeout failure, got %+v", res)
	}
	if res.Classification != RequestTimeout && res.Classification != PlexServiceUnreachable {
		t.Fatalf("expected timeout-ish classification, got %s", res.Classification)
	}
}

func TestCheckConnRefused(t *testing.T) {
	// Find a definitely-closed port.
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	_ = l.Close()
	res := newClient(t, Options{BaseURL: "http://" + addr, Timeout: time.Second}).Check(context.Background())
	if res.Classification != PlexServiceUnreachable {
		t.Fatalf("expected plex_service_unreachable, got %+v", res)
	}
	if !res.HostReachable {
		t.Fatalf("connection-refused implies host reachable")
	}
}

func TestCheckIdentityMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(identityXML))
	}))
	defer srv.Close()
	res := newClient(t, Options{BaseURL: srv.URL, Timeout: 2 * time.Second, ExpectedMachineID: "DIFFERENT"}).Check(context.Background())
	if res.Classification != IdentityMismatch {
		t.Fatalf("expected identity_mismatch, got %+v", res)
	}
}

func TestCheckAuthFailureOnSessions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity":
			_, _ = w.Write([]byte(identityXML))
		case "/status/sessions":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			_, _ = w.Write([]byte(`<MediaContainer friendlyName="Home"/>`))
		}
	}))
	defer srv.Close()
	res := newClient(t, Options{BaseURL: srv.URL, Timeout: 2 * time.Second, Token: "bad", FetchSessions: true}).Check(context.Background())
	if res.Classification != AuthenticationFailure {
		t.Fatalf("expected authentication_failure, got %+v", res)
	}
}

func TestCheckStreamCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") == "" && r.URL.Path != "/identity" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/identity":
			_, _ = w.Write([]byte(identityXML))
		case "/status/sessions":
			_, _ = w.Write([]byte(`<MediaContainer size="3"/>`))
		default:
			_, _ = w.Write([]byte(`<MediaContainer friendlyName="Home Server"/>`))
		}
	}))
	defer srv.Close()
	res := newClient(t, Options{BaseURL: srv.URL, Timeout: 2 * time.Second, Token: "good", FetchSessions: true}).Check(context.Background())
	if !res.OK {
		t.Fatalf("expected ok, got %+v", res)
	}
	if !res.StreamCountKnown || res.StreamCount != 3 {
		t.Fatalf("expected 3 streams, got %+v", res)
	}
	if res.ServerName != "Home Server" {
		t.Fatalf("expected friendly name, got %q", res.ServerName)
	}
}

func TestCheckTLSFailure(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(identityXML))
	}))
	defer srv.Close()
	// VerifyTLS true against a self-signed cert -> tls_failure.
	res := newClient(t, Options{BaseURL: srv.URL, Timeout: 2 * time.Second, VerifyTLS: true}).Check(context.Background())
	if res.Classification != TLSFailure {
		t.Fatalf("expected tls_failure, got %+v", res)
	}
	// VerifyTLS false -> should succeed.
	res = newClient(t, Options{BaseURL: srv.URL, Timeout: 2 * time.Second, VerifyTLS: false}).Check(context.Background())
	if !res.OK {
		t.Fatalf("expected ok with TLS verification disabled, got %+v", res)
	}
}

func TestCheckDNSFailure(t *testing.T) {
	res := newClient(t, Options{BaseURL: "http://nonexistent-host.invalid:32400", Timeout: 2 * time.Second}).Check(context.Background())
	if res.Classification != DNSFailure {
		t.Fatalf("expected dns_failure, got %+v", res)
	}
}

func TestTokenNeverInURL(t *testing.T) {
	var sawTokenInURL bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "Token") || strings.Contains(r.URL.String(), "secret") {
			sawTokenInURL = true
		}
		_, _ = w.Write([]byte(identityXML))
	}))
	defer srv.Close()
	_ = newClient(t, Options{BaseURL: srv.URL, Timeout: 2 * time.Second, Token: "secret", FetchSessions: true}).Check(context.Background())
	if sawTokenInURL {
		t.Fatalf("token must never appear in the request URL")
	}
	_ = fmt.Sprint()
}
