package sites

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type Dialer func(context.Context, string, string) (net.Conn, error)
type Relay struct {
	server    *http.Server
	transport *http.Transport
	base      string
}

func NewRelay(raw string, dial Dialer) (*Relay, error) {
	if _, err := Validate(Link{Name: "site", URL: raw}); err != nil {
		return nil, err
	}
	if dial == nil {
		return nil, fmt.Errorf("SSH connection required")
	}
	target, _ := url.Parse(raw)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	base := "http://" + listener.Addr().String()
	transport := &http.Transport{DialContext: dial, Proxy: nil, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 30 * time.Second}
	proxy := &httputil.ReverseProxy{Transport: transport, Rewrite: func(pr *httputil.ProxyRequest) {
		pr.Out.URL.Scheme = target.Scheme
		pr.Out.URL.Host = target.Host
		pr.Out.Host = target.Host
		pr.SetXForwarded()
		pr.Out.Header.Set("X-Forwarded-Host", target.Host)
		pr.Out.Header.Set("X-Forwarded-Proto", target.Scheme)
	}, ModifyResponse: func(resp *http.Response) error {
		if location := resp.Header.Get("Location"); location != "" {
			u, err := url.Parse(location)
			if err != nil {
				return err
			}
			if u.Host != "" {
				if !strings.EqualFold(u.Host, target.Host) {
					return fmt.Errorf("Redirect to another site is not configured for this tunnel")
				}
				u.Scheme = "http"
				u.Host = listener.Addr().String()
				resp.Header.Set("Location", u.String())
			}
		}
		// Domain cookies must belong to the local proxy's host. Keep all other
		// attributes, including Secure and HttpOnly, as supplied by the server.
		cookies := resp.Header.Values("Set-Cookie")
		resp.Header.Del("Set-Cookie")
		for _, cookie := range cookies {
			parts := strings.Split(cookie, ";")
			kept := parts[:1]
			for _, part := range parts[1:] {
				if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(part)), "domain=") {
					kept = append(kept, part)
				}
			}
			resp.Header.Add("Set-Cookie", strings.Join(kept, ";"))
		}
		return nil
	}, ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "Site unavailable through the SSH tunnel", http.StatusBadGateway)
	}}
	r := &Relay{transport: transport, base: base, server: &http.Server{Handler: proxy, ReadHeaderTimeout: 10 * time.Second}}
	go func() { _ = r.server.Serve(listener) }()
	return r, nil
}
func (r *Relay) URL(reference string) string {
	u, _ := url.Parse(reference)
	base, _ := url.Parse(r.base)
	if u == nil {
		return r.base
	}
	u.Scheme = base.Scheme
	u.Host = base.Host
	u.User = nil
	return u.String()
}
func (r *Relay) Close() { _ = r.server.Close(); r.transport.CloseIdleConnections() }
