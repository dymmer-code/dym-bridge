package sites

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestValidationAndStore(t *testing.T) {
	for _, raw := range []string{"file:///tmp/a", "javascript:alert(1)", "https://user:pass@example.org/", "/admin", "http://"} {
		if _, err := Validate(Link{Name: "Site", URL: raw}); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	filename := filepath.Join(t.TempDir(), "sites.json")
	links := []Link{{Name: "Dashboard", URL: "http://127.0.0.1:3000/admin?q=a#tab"}}
	if err := Save(filename, "connection-a", links); err != nil {
		t.Fatal(err)
	}
	if err := Save(filename, "connection-b", []Link{}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := Load(filename, "connection-a")
	if err != nil || !ok || len(got) != 1 || got[0] != links[0] {
		t.Fatal(got, ok, err)
	}
	got, ok, err = Load(filename, "connection-b")
	if err != nil || !ok || len(got) != 0 {
		t.Fatal(got, ok, err)
	}
	_, ok, err = Load(filename, "missing")
	if err != nil || ok {
		t.Fatal(ok, err)
	}
}
func TestTunnelUsesInjectedDialerAndRewritesRedirect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "internal.test:8080" {
			t.Errorf("host: %s", r.Host)
		}
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "http://internal.test:8080/login?q=1")
			w.WriteHeader(302)
			return
		}
		_, _ = io.WriteString(w, r.URL.RequestURI())
	}))
	defer upstream.Close()
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr != "internal.test:8080" {
			t.Errorf("address: %s", addr)
		}
		return (&net.Dialer{}).DialContext(ctx, network, upstream.Listener.Addr().String())
	}
	relay, err := NewRelay("http://internal.test:8080/start?q=ok#tab", dial)
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	resp, err := http.Get(relay.URL("/start?q=ok"))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(data) != "/start?q=ok" {
		t.Fatal(string(data))
	}
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err = client.Get(relay.URL("/redirect"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.Header.Get("Location") != relay.URL("/login?q=1") {
		t.Fatal(resp.Header.Get("Location"))
	}
	relay.Close()
	if _, err := http.Get(relay.URL("/")); err == nil {
		t.Fatal("relay still listening")
	}
}

func TestInvalidSavePreservesExistingSites(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sites.json")
	original := []Link{{Name: "One", URL: "https://example.org/"}}
	if err := Save(file, "key", original); err != nil {
		t.Fatal(err)
	}
	if err := Save(file, "key", []Link{{Name: "One", URL: "file:///tmp/a"}}); err == nil {
		t.Fatal("accepted invalid link")
	}
	if err := Save(file, "key", []Link{{Name: "Same", URL: "http://a/"}, {Name: "same", URL: "http://b/"}}); err == nil {
		t.Fatal("accepted duplicate names")
	}
	got, _, err := Load(file, "key")
	if err != nil || len(got) != 1 || got[0] != original[0] {
		t.Fatal(got, err)
	}
}
func TestRelayRejectsUntrustedTLS(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "must not be reached") }))
	defer upstream.Close()
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	relay, err := NewRelay(upstream.URL, dial)
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	resp, err := http.Get(relay.URL("/"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("untrusted upstream certificate accepted: %d", resp.StatusCode)
	}
}
func TestCrossOriginRedirectDoesNotEscapeTunnel(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://elsewhere.invalid/")
		w.WriteHeader(302)
	}))
	defer upstream.Close()
	relay, err := NewRelay(upstream.URL, func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	resp, err := http.Get(relay.URL("/"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway || resp.Header.Get("Location") != "" {
		t.Fatalf("redirect escaped: %v", resp)
	}
}
