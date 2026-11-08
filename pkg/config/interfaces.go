package config

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Tunnel is an authorized loopback destination. Web metadata is optional so
// existing host/port-only provisioning remains compatible and is not guessed HTTP.
type Tunnel struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Scheme     string `json:"scheme,omitempty"`
	Path       string `json:"path,omitempty"`
	HostHeader string `json:"host_header,omitempty"`
}

var interfaceID = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var httpHost = regexp.MustCompile(`^(?:[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?|\[[0-9A-Fa-f:]+\])(?::[0-9]{1,5})?$`)

func ValidHTTPHost(host string) bool {
	if len(host) == 0 || len(host) > 253 || !httpHost.MatchString(host) {
		return false
	}
	u, err := url.Parse("http://" + host)
	if err != nil {
		return false
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		return err == nil && port > 0 && port <= 65535
	}
	return true
}
func (t Tunnel) IsInterface() bool { return t.ID != "" }
func (t Tunnel) Validate() error {
	if (t.Host != "127.0.0.1" && t.Host != "::1" && t.Host != "localhost") || t.Port < 1 || t.Port > 65535 {
		return fmt.Errorf("Invalid authorized tunnel destination")
	}
	if t.ID == "" && t.Name == "" && t.Scheme == "" && t.Path == "" && t.HostHeader == "" {
		return nil
	}
	if !interfaceID.MatchString(t.ID) || len(t.Name) > 100 || strings.TrimSpace(t.Name) == "" || strings.ContainsAny(t.Name, "\r\n\x00") || (t.Scheme != "http" && t.Scheme != "https") {
		return fmt.Errorf("Invalid web interface metadata")
	}
	for _, r := range t.Name {
		if r < 32 || r == 127 {
			return fmt.Errorf("Invalid interface name")
		}
	}
	if len(t.Path) > 2048 || !strings.HasPrefix(t.Path, "/") || strings.HasPrefix(t.Path, "//") {
		return fmt.Errorf("Invalid interface path")
	}
	for _, r := range t.Path {
		if r <= 32 || r == 127 || r == '\\' {
			return fmt.Errorf("Invalid interface path")
		}
	}
	u, err := url.Parse(t.Path)
	if err != nil || u.IsAbs() || u.Host != "" {
		return fmt.Errorf("Invalid interface path")
	}
	if t.HostHeader != "" && !ValidHTTPHost(t.HostHeader) {
		return fmt.Errorf("Invalid interface HTTP host")
	}
	return nil
}
func (t Tunnel) URL() string {
	return t.Scheme + "://" + net.JoinHostPort(t.Host, strconv.Itoa(t.Port)) + t.Path
}
func ValidateTunnels(tunnels []Tunnel) error {
	if len(tunnels) > 20 {
		return fmt.Errorf("Too many tunnel interfaces")
	}
	ids := map[string]bool{}
	for _, t := range tunnels {
		if err := t.Validate(); err != nil {
			return err
		}
		if t.IsInterface() {
			if ids[t.ID] {
				return fmt.Errorf("Duplicate interface ID: %s", t.ID)
			}
			ids[t.ID] = true
		}
	}
	return nil
}
