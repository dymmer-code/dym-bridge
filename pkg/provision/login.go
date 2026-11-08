package provision

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/dymmer-code/dym-bridge/pkg/secret"
	"github.com/dymmer-code/dym-bridge/pkg/version"
)

// Receiver delivers the address the browser opened when the user approved.
type Receiver interface {
	Next(ctx context.Context) (string, error)
}

// LoginOptions is everything one sign-in needs. Open and Receiver are the parts that depend on the system.
type LoginOptions struct {
	Server         string
	ProfilesPath   string
	MasterPassword string // protects the private key of this device
	DeviceName     string // defaults to the host name
	Open           func(url string) error
	Receiver       Receiver
	Client         *Client       // nil: a normal one for Server
	Timeout        time.Duration // how long to wait for the browser; default five minutes
	RedirectURI    string        // default RedirectURI
}

// Login signs in with Dymmer through the browser, publishes the public key of this device and saves the resources it
// received. The token is a local variable: it exists between the exchange and the provisioning call and nowhere else.
func Login(ctx context.Context, o LoginOptions) (*Profiles, error) {
	if o.Server == "" || o.Open == nil || o.Receiver == nil {
		return nil, errors.New("the sign-in is not fully set up")
	}
	redirect := o.RedirectURI
	if redirect == "" {
		redirect = RedirectURI
	}
	client := o.Client
	if client == nil {
		client = &Client{Server: o.Server}
	}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	name := o.DeviceName
	if name == "" {
		name, _ = os.Hostname()
		if name == "" {
			name = "device"
		}
	}

	profiles, err := Load(o.ProfilesPath)
	switch {
	case errors.Is(err, ErrNoProfiles):
		key, kerr := GenerateDeviceKey(name, o.MasterPassword)
		if kerr != nil {
			return nil, kerr
		}
		profiles = &Profiles{Server: o.Server, Key: *key}
	case err != nil:
		return nil, err
	default:
		// The master password must open the key this device already has; otherwise we would silently keep a key
		// nobody can use.
		if _, err := profiles.Key.Signer(o.MasterPassword); err != nil {
			return nil, err
		}
		if o.MasterPassword != "" && !secret.IsEncrypted(profiles.Key.Private) {
			encrypted, err := secret.Encrypt(profiles.Key.Private, o.MasterPassword)
			if err != nil {
				return nil, err
			}
			profiles.Key.Private = encrypted
		}
		profiles.Server = o.Server
	}

	req, err := NewRequest()
	if err != nil {
		return nil, err
	}
	if err := o.Open(req.AuthorizeURL(o.Server, redirect)); err != nil {
		return nil, fmt.Errorf("the browser could not be opened: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	callback, err := o.Receiver.Next(waitCtx)
	if err != nil {
		return nil, fmt.Errorf("no answer from the browser: %w", err)
	}
	code, err := req.Code(callback)
	if err != nil {
		return nil, err
	}
	token, err := client.Exchange(ctx, req, code, redirect)
	if err != nil {
		return nil, err
	}
	file, err := client.Provision(ctx, token, DeviceInfo{
		PublicKey:     profiles.Key.PublicKey,
		DeviceName:    name,
		System:        runtime.GOOS + " " + runtime.GOARCH,
		ClientVersion: version.Version,
	})
	token = "" // spent; nothing keeps it
	if err != nil {
		return nil, err
	}
	profiles.Merge(file)
	if err := profiles.Save(o.ProfilesPath); err != nil {
		return nil, fmt.Errorf("the resources were received but could not be saved: %w", err)
	}
	return profiles, nil
}
