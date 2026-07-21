/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package controllers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

// ---------------------------------------------------------------------------
// validateOutboundURL
// ---------------------------------------------------------------------------

func TestValidateOutboundURL(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		allowed     []string
		wantErr     bool
		errContains string
	}{
		// ── scheme checks ───────────────────────────────────────────────────
		{
			name:    "https with no allowlist passes",
			raw:     "https://example.com/config.yaml",
			allowed: nil,
			wantErr: false,
		},
		{
			name:        "http is rejected",
			raw:         "http://example.com/config.yaml",
			allowed:     nil,
			wantErr:     true,
			errContains: "https",
		},
		{
			name:        "file scheme is rejected",
			raw:         "file:///etc/passwd",
			allowed:     nil,
			wantErr:     true,
			errContains: "https",
		},
		{
			name:        "link-local http is rejected on scheme",
			raw:         "http://169.254.169.254/latest/meta-data/",
			allowed:     nil,
			wantErr:     true,
			errContains: "https",
		},
		{
			name:        "malformed URL is rejected",
			raw:         "://bad",
			allowed:     nil,
			wantErr:     true,
			errContains: "invalid URL",
		},

		// ── empty allowlist (scheme-only enforcement) ────────────────────────
		{
			name:    "any https host permitted when allowlist is empty",
			raw:     "https://arbitrary.host.example.com/path",
			allowed: []string{},
			wantErr: false,
		},

		// ── allowlist: exact match ───────────────────────────────────────────
		{
			name:    "exact match passes",
			raw:     "https://myidp.example.com/discovery",
			allowed: []string{"myidp.example.com"},
			wantErr: false,
		},
		{
			name:        "non-matching host rejected",
			raw:         "https://evil.example.com/discovery",
			allowed:     []string{"myidp.example.com"},
			wantErr:     true,
			errContains: "permitted hosts list",
		},
		{
			name:        "subdomain does not match exact entry",
			raw:         "https://sub.myidp.example.com/discovery",
			allowed:     []string{"myidp.example.com"},
			wantErr:     true,
			errContains: "permitted hosts list",
		},

		// ── allowlist: suffix match ──────────────────────────────────────────
		{
			name:    "subdomain matches dot-prefixed suffix",
			raw:     "https://idp.internal.corp/discovery",
			allowed: []string{".internal.corp"},
			wantErr: false,
		},
		{
			name:    "deeper subdomain matches dot-prefixed suffix",
			raw:     "https://a.b.internal.corp/discovery",
			allowed: []string{".internal.corp"},
			wantErr: false,
		},
		{
			name:        "unrelated host does not match dot-prefixed suffix",
			raw:         "https://evil.corp/discovery",
			allowed:     []string{".internal.corp"},
			wantErr:     true,
			errContains: "permitted hosts list",
		},
		{
			name:        "suffix match does not permit bare domain without subdomain",
			raw:         "https://internal.corp/discovery",
			allowed:     []string{".internal.corp"},
			wantErr:     true,
			errContains: "permitted hosts list",
		},

		// ── allowlist: multiple entries ──────────────────────────────────────
		{
			name:    "first entry matches",
			raw:     "https://idp.example.com/discovery",
			allowed: []string{"idp.example.com", "config.example.com"},
			wantErr: false,
		},
		{
			name:    "second entry matches",
			raw:     "https://config.example.com/iag.yaml",
			allowed: []string{"idp.example.com", "config.example.com"},
			wantErr: false,
		},
		{
			name:        "neither entry matches",
			raw:         "https://other.example.com/iag.yaml",
			allowed:     []string{"idp.example.com", "config.example.com"},
			wantErr:     true,
			errContains: "permitted hosts list",
		},

		// ── port in URL is ignored for host matching ─────────────────────────
		{
			name:    "https with port passes exact match",
			raw:     "https://myidp.example.com:8443/discovery",
			allowed: []string{"myidp.example.com"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOutboundURL(tt.raw, tt.allowed)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// safeCheckRedirect
// ---------------------------------------------------------------------------

func TestSafeCheckRedirect(t *testing.T) {
	tests := []struct {
		name        string
		redirectTo  string
		allowed     []string
		wantErr     bool
		errContains string
	}{
		{
			name:       "https redirect to same allowed host passes",
			redirectTo: "https://myidp.example.com/new-path",
			allowed:    []string{"myidp.example.com"},
			wantErr:    false,
		},
		{
			name:        "redirect to http is blocked",
			redirectTo:  "http://myidp.example.com/new-path",
			allowed:     []string{"myidp.example.com"},
			wantErr:     true,
			errContains: "redirect blocked",
		},
		{
			name:        "redirect to link-local http is blocked",
			redirectTo:  "http://169.254.169.254/latest/meta-data/",
			allowed:     []string{"myidp.example.com"},
			wantErr:     true,
			errContains: "redirect blocked",
		},
		{
			name:        "redirect to off-allowlist https host is blocked",
			redirectTo:  "https://evil.example.com/steal",
			allowed:     []string{"myidp.example.com"},
			wantErr:     true,
			errContains: "redirect blocked",
		},
		{
			name:       "redirect passes when allowlist is empty (scheme-only)",
			redirectTo: "https://anywhere.example.com/path",
			allowed:    nil,
			wantErr:    false,
		},
		{
			name:        "redirect to http blocked even with empty allowlist",
			redirectTo:  "http://anywhere.example.com/path",
			allowed:     nil,
			wantErr:     true,
			errContains: "redirect blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := safeCheckRedirect(tt.allowed)

			req, _ := http.NewRequest("GET", tt.redirectTo, nil)
			// via contains one prior request to simulate a single redirect hop
			via := []*http.Request{{}}

			err := fn(req, via)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// handleWebEntryMerge — URL validation and redirect enforcement
// Uses httptest.Server so no real network calls are made.
// ---------------------------------------------------------------------------

func TestHandleWebEntryMerge_URLValidation(t *testing.T) {
	// redirectServer issues a plain-http 302 to another plain-http target.
	// The initial URL is http://, so validateOutboundURL blocks it before
	// any network call is made — no TLS trust issues.
	httpTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "version: \"22.07\"")
	}))
	defer httpTarget.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, httpTarget.URL+"/config.yaml", http.StatusFound)
	}))
	defer redirectServer.Close()

	tests := []struct {
		name        string
		url         string
		allowed     []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "plain http URL is rejected before fetch",
			url:         "http://example.com/config.yaml",
			allowed:     nil,
			wantErr:     true,
			errContains: "web configuration URL rejected",
		},
		{
			name:        "empty URL is rejected",
			url:         "",
			allowed:     nil,
			wantErr:     true,
			errContains: "missing the Url",
		},
		{
			name:        "off-allowlist host is rejected before fetch",
			url:         "https://evil.example.com/config.yaml",
			allowed:     []string{"safe.example.com"},
			wantErr:     true,
			errContains: "web configuration URL rejected",
		},
		{
			// Initial URL is http:// so validateOutboundURL fires before
			// any connection is attempted — no TLS cert trust required.
			name:        "http redirect source is blocked before fetch",
			url:         redirectServer.URL + "/config.yaml",
			allowed:     nil,
			wantErr:     true,
			errContains: "web configuration URL rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			master := make(map[string]interface{})
			_, err := handleWebEntryMerge(nil, noopNSN(), tt.url, nil, master, tt.allowed)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getDiscoveryData — URL validation before fetch
// ---------------------------------------------------------------------------

func TestGetDiscoveryData_URLValidation(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		allowed     []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "http discovery endpoint is rejected",
			endpoint:    "http://idp.example.com/.well-known/openid-configuration",
			allowed:     nil,
			wantErr:     true,
			errContains: "OIDC discovery endpoint rejected",
		},
		{
			name:        "off-allowlist discovery endpoint is rejected",
			endpoint:    "https://evil.example.com/.well-known/openid-configuration",
			allowed:     []string{"idp.example.com"},
			wantErr:     true,
			errContains: "OIDC discovery endpoint rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &IAGOidcReg{DiscoveryEndpoint: tt.endpoint}
			_, err := getDiscoveryData(entry, false, tt.allowed)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Vector B: host-pinning in getDiscoveryData
// An attacker-controlled discovery document returns token_endpoint and
// registration_endpoint on a different host. getDiscoveryData must reject
// the response before any credential is sent.
// ---------------------------------------------------------------------------

func TestVectorB_TokenEndpointHostPinning(t *testing.T) {
	// Attacker collector — records any body it receives.
	var collectedBody string
	attackerServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		collectedBody = string(b)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"access_token":"stolen"}`)
	}))
	defer attackerServer.Close()

	// The discovery server returns a well-formed discovery document whose
	// token_endpoint and registration_endpoint point at the attacker's host.
	// Both servers are TLS so the scheme check passes; host-pinning must
	// catch the mismatch.
	discoveryServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"token_endpoint": "%s/token",
			"registration_endpoint": "%s/register"
		}`, attackerServer.URL, attackerServer.URL)
	}))
	defer discoveryServer.Close()

	// discoveryServer and attackerServer both listen on 127.0.0.1 but on
	// different ports. hostOf() uses Hostname() which strips the port, so
	// both resolve to "127.0.0.1" — host-pinning would pass.
	// To exercise the real cross-host case we use a discovery document that
	// returns endpoints on a clearly different hostname.
	discoveryServerCrossHost := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
			"token_endpoint": "https://evil.example.com/token",
			"registration_endpoint": "https://evil.example.com/register"
		}`)
	}))
	defer discoveryServerCrossHost.Close()

	// Use the test server's own TLS client so the self-signed cert is trusted.
	// We reach into doRequest via insecure=true to bypass cert validation,
	// which is sufficient for this test — we are testing host-pinning logic,
	// not TLS trust.
	entry := &IAGOidcReg{DiscoveryEndpoint: discoveryServerCrossHost.URL + "/.well-known/openid-configuration"}

	_, err := getDiscoveryData(entry, true /* insecure */, nil)

	if err == nil {
		t.Fatal("expected host-pinning to block the cross-host token_endpoint, got nil error")
	}
	if !strings.Contains(err.Error(), "does not match discovery host") {
		t.Fatalf("expected host-pinning error, got: %v", err)
	}
	if collectedBody != "" {
		t.Fatalf("credentials were sent to attacker server before host-pinning check: %s", collectedBody)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// noopNSN returns an empty NamespacedName suitable for tests that do not
// exercise the secret-header path of handleWebEntryMerge.
func noopNSN() types.NamespacedName {
	return types.NamespacedName{}
}
