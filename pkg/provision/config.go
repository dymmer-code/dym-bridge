package provision

import (
	"fmt"

	"github.com/dymmer-code/dym-bridge/pkg/config"
)

// Config builds connection settings without changing key protection. Encrypted
// keys are unlocked in memory; unencrypted keys connect directly. Dymmer-provided
// fingerprints pin the SSH server identity.
func (p *Profiles) Config(index int) (*config.Config, error) {
	if index < 0 || index >= len(p.Resources) {
		return nil, fmt.Errorf("there is no resource number %d", index+1)
	}
	r := p.Resources[index]

	fingerprints := make([]string, 0, len(r.HostKeys))
	for _, k := range r.HostKeys {
		fingerprints = append(fingerprints, k.Fingerprint)
	}
	if len(fingerprints) == 0 {
		return nil, fmt.Errorf("Dymmer gave no server key for %s: sign in again", r.Label())
	}

	path := r.Path
	if path == "" {
		path = "/"
	}
	cfg := &config.Config{
		Provisioned: true,
		Tunnels:     append([]config.Tunnel(nil), r.Tunnels...),
		SSH: config.SSHConfig{
			Host:     r.Address,
			Port:     r.Port,
			User:     r.User,
			KeyData:  p.Key.Private,
			HostKeys: fingerprints,
		},
		SFTP: config.SFTPConfig{RemotePath: path},
	}
	if cfg.SSH.Port <= 0 {
		cfg.SSH.Port = 22
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
