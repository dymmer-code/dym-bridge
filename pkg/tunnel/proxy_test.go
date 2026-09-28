package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/dymmer-code/dym-bridge/pkg/config"
)

func TestRewriteLocationHeader(t *testing.T) {
	tests := []struct {
		name       string
		location   string
		targetHost string
		localPort  int
		expected   string
	}{
		{
			name:       "https domain redirect to wp-admin",
			location:   "https://cliente.com/wp-admin/",
			targetHost: "cliente.com",
			localPort:  8080,
			expected:   "http://127.0.0.1:8080/wp-admin/",
		},
		{
			name:       "redirect with query params",
			location:   "https://cliente.com/wp-login.php?redirect_to=%2Fwp-admin%2F",
			targetHost: "cliente.com",
			localPort:  8080,
			expected:   "http://127.0.0.1:8080/wp-login.php?redirect_to=%2Fwp-admin%2F",
		},
		{
			name:       "external domain is preserved untouched",
			location:   "https://wordpress.org/support/",
			targetHost: "cliente.com",
			localPort:  8080,
			expected:   "https://wordpress.org/support/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteLocationHeader(tt.location, tt.targetHost, tt.localPort)
			if got != tt.expected {
				t.Errorf("rewriteLocationHeader() = %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestHTTPProxyServer(t *testing.T) {
	var receivedHost string
	var receivedXForwardedHost string
	var receivedXForwardedProto string

	// Upstream mock server simulating remote WordPress
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		receivedXForwardedHost = r.Header.Get("X-Forwarded-Host")
		receivedXForwardedProto = r.Header.Get("X-Forwarded-Proto")

		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://cliente.com/wp-admin/", http.StatusFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("WordPress Admin OK"))
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	upstreamPort, _ := strconv.Atoi(upstreamURL.Port())

	cfg := &config.WordPressConfig{
		HostHeader: "cliente.com",
		RemotePort: upstreamPort,
		LocalPort:  0, // Automatically pick free local port
		AdminPath:  "/wp-admin/",
	}

	// Direct dialer to mock upstream
	dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", upstreamURL.Host)
	}

	proxy, err := NewHTTPProxyServer(cfg, dialer)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}
	proxy.Start()
	defer func() { _ = proxy.Stop(context.Background()) }()

	proxyBaseURL := fmt.Sprintf("http://127.0.0.1:%d", proxy.ActualPort())

	// Test 1: Request forwarded and headers injected
	resp, err := http.Get(proxyBaseURL + "/wp-admin/")
	if err != nil {
		t.Fatalf("request through proxy failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if string(body) != "WordPress Admin OK" {
		t.Errorf("unexpected body: %s", string(body))
	}
	if receivedHost != "cliente.com" {
		t.Errorf("expected Host header %q, got %q", "cliente.com", receivedHost)
	}
	if receivedXForwardedHost != "cliente.com" {
		t.Errorf("expected X-Forwarded-Host %q, got %q", "cliente.com", receivedXForwardedHost)
	}
	if receivedXForwardedProto != "https" {
		t.Errorf("expected X-Forwarded-Proto https, got %q", receivedXForwardedProto)
	}

	// Test 2: Redirect rewriting
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // do not follow redirect automatically
		},
	}
	respRedirect, err := client.Get(proxyBaseURL + "/redirect")
	if err != nil {
		t.Fatalf("request to redirect endpoint failed: %v", err)
	}
	_ = respRedirect.Body.Close()

	loc := respRedirect.Header.Get("Location")
	expectedLoc := fmt.Sprintf("http://127.0.0.1:%d/wp-admin/", proxy.ActualPort())
	if loc != expectedLoc {
		t.Errorf("expected rewritten location %q, got %q", expectedLoc, loc)
	}
}
