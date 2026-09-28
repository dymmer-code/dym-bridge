package tunnel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/dymmer-code/dym-bridge/pkg/config"
)

// DialerFunc abstracts network dialing (e.g. through an SSH connection or mock).
type DialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// HTTPProxyServer is a local reverse proxy for accessing WordPress through the SSH tunnel.
type HTTPProxyServer struct {
	server     *http.Server
	listener   net.Listener
	cfg        *config.WordPressConfig
	actualPort int
}

// NewHTTPProxyServer initializes an HTTP reverse proxy server.
func NewHTTPProxyServer(cfg *config.WordPressConfig, dialer DialerFunc) (*HTTPProxyServer, error) {
	listenAddr := fmt.Sprintf("127.0.0.1:%d", cfg.LocalPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el puerto local %d para el proxy: %w", cfg.LocalPort, err)
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port

	transport := &http.Transport{
		DialContext:           dialer,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	remoteTarget := fmt.Sprintf("127.0.0.1:%d", cfg.RemotePort)

	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = remoteTarget

			// Crucial: Set Host header to the WordPress configured domain
			req.Host = cfg.HostHeader
			req.Header.Set("X-Forwarded-Host", cfg.HostHeader)
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Real-IP", "127.0.0.1")
		},
		ModifyResponse: func(resp *http.Response) error {
			// Rewrite Location header in redirects (e.g. 301/302 from WordPress)
			location := resp.Header.Get("Location")
			if location != "" {
				rewritten := rewriteLocationHeader(location, cfg.HostHeader, actualPort)
				resp.Header.Set("Location", rewritten)
			}
			return nil
		},
	}

	srv := &http.Server{
		Handler:      proxy,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	return &HTTPProxyServer{
		server:     srv,
		listener:   listener,
		cfg:        cfg,
		actualPort: actualPort,
	}, nil
}

// Start begins listening for requests in a background goroutine.
func (p *HTTPProxyServer) Start() {
	go func() {
		_ = p.server.Serve(p.listener)
	}()
}

// Stop gracefully shuts down the local reverse proxy.
func (p *HTTPProxyServer) Stop(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}

// ActualPort returns the port the local proxy is listening on.
func (p *HTTPProxyServer) ActualPort() int {
	return p.actualPort
}

// LocalURL returns the full local URL to access WordPress Admin.
func (p *HTTPProxyServer) LocalURL() string {
	adminPath := p.cfg.AdminPath
	if !strings.HasPrefix(adminPath, "/") {
		adminPath = "/" + adminPath
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s", p.actualPort, adminPath)
}

// rewriteLocationHeader transforms remote canonical URLs in redirects into local proxy URLs.
func rewriteLocationHeader(rawLocation, targetHost string, localPort int) string {
	locURL, err := url.Parse(rawLocation)
	if err != nil {
		return rawLocation
	}

	// If the redirect points to the WordPress domain (with or without custom port)
	hostOnly := locURL.Hostname()
	if strings.EqualFold(hostOnly, targetHost) || strings.EqualFold(locURL.Host, targetHost) {
		locURL.Scheme = "http"
		locURL.Host = fmt.Sprintf("127.0.0.1:%d", localPort)
		return locURL.String()
	}

	return rawLocation
}
