package gpuui

import (
	"github.com/dymmer-code/dym-bridge/pkg/config"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestProvisionedSitesPreparedOnConnect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "wordpress.example" {
			t.Errorf("host %s", r.Host)
		}
		_, _ = io.WriteString(w, r.URL.RequestURI())
	}))
	defer upstream.Close()
	host, portString, _ := net.SplitHostPort(upstream.Listener.Addr().String())
	port, _ := strconv.Atoi(portString)
	cfg := testSFTPServer(t)
	cfg.Tunnels = []config.Tunnel{{Host: host, Port: port, ID: "wordpress", Name: "WordPress Admin", Scheme: "http", Path: "/wp-admin/?test=1", HostHeader: "wordpress.example"}, {Host: host, Port: 3306}}
	links := interfaceLinks(cfg)
	if len(links) != 1 || links[0].Name != "WordPress Admin" {
		t.Fatal(links)
	}
	var session session
	if err := session.connect(cfg); err != nil {
		t.Fatal(err)
	}
	defer session.close()
	if len(session.relays) != 1 {
		t.Fatal("interface listener was not prepared during connect")
	}
	localURL, err := session.siteLinkURL(links[0])
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(localURL)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(data) != "/wp-admin/?test=1" {
		t.Fatal(string(data))
	}
	session.close()
	if _, err := http.Get(localURL); err == nil {
		t.Fatal("local endpoint still active after disconnect")
	}
	cfg.Tunnels = nil
	if err := session.connect(cfg); err != nil {
		t.Fatal(err)
	}
	if len(session.relays) != 0 {
		t.Fatal("previous connection interfaces leaked")
	}
}
