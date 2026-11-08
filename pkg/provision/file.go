package provision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dymmer-code/dym-bridge/pkg/config"
)

// SupportedVersion is the version of the provisioning file this application understands.
const SupportedVersion = 1

// HostKey is a key the server was reported to have, so that the first connection is not trusted blindly.
type HostKey struct {
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"public_key"`
}

// Tunnel is a local-only destination on the host that may be reached through the SSH connection.
type Tunnel = config.Tunnel

// Resource is one place the user may enter: a host, with the system user and the path they were given.
type Resource struct {
	DelegationID string     `json:"delegation_id"`
	HostID       string     `json:"host_id"`
	Name         string     `json:"name"`
	Address      string     `json:"address"`
	Port         int        `json:"port"`
	User         string     `json:"user"`
	Path         string     `json:"path"`
	ExpiresAt    *time.Time `json:"expires_at"`
	HostKeys     []HostKey  `json:"host_keys"`
	Tunnels      []Tunnel   `json:"tunnels"`
}

// Device is how Dymmer knows this computer.
type Device struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
}

// File is what the provisioning call answers.
type File struct {
	Version   int        `json:"version"`
	Device    Device     `json:"device"`
	Resources []Resource `json:"resources"`
}

// DeviceInfo is what the application sends: the public key, a name and the system. Never a private key.
type DeviceInfo struct {
	PublicKey  string `json:"public_key"`
	DeviceName string `json:"device_name"`
	System     string `json:"system"`
	// ClientVersion is the version of the application, so the owner can see which devices are old.
	ClientVersion string `json:"client_version,omitempty"`
}

// Provision makes the single provisioning call with the token and returns the file. The token is spent whatever the
// answer is: to try again the user signs in again.
func (c *Client) Provision(ctx context.Context, token Token, info DeviceInfo) (*File, error) {
	body, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/oauth/provision"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+string(token))

	var out struct {
		File
		Error string `json:"error"`
	}
	status, err := c.do(req, &out)
	if err != nil {
		return nil, err
	}
	switch {
	case status == http.StatusUnauthorized:
		return nil, errors.New("the sign-in expired or was already used: sign in again")
	case status != http.StatusOK:
		return nil, fmt.Errorf("Dymmer refused the provisioning (%s)", reason(out.Error, status))
	case out.Version != SupportedVersion:
		return nil, fmt.Errorf("this version of dym-bridge does not understand provisioning format %d: update it", out.Version)
	}
	return &out.File, nil
}

// Label is the name a user sees for the resource.
func (r Resource) Label() string {
	if r.Name == "" {
		return r.Address
	}
	return strings.TrimSpace(r.Name + " · " + r.User)
}
