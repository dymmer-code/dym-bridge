package provision

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Profiles is what the application keeps between runs: the server, the key of this device and the resources of the
// last provisioning. There is no token here: it was spent.
type Profiles struct {
	Server    string     `json:"server"`
	Device    Device     `json:"device"`
	Key       DeviceKey  `json:"key"`
	Resources []Resource `json:"resources"`
}

// DefaultPath is where the profiles are saved for the user.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dym-bridge", "profiles.json"), nil
}

// ErrNoProfiles means that this device was not set up yet.
var ErrNoProfiles = errors.New("this device is not set up: sign in with Dymmer first")

// Load reads the profiles at path.
func Load(path string) (*Profiles, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoProfiles
	}
	if err != nil {
		return nil, err
	}
	var p Profiles
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("the profiles file is damaged: %w", err)
	}
	return &p, nil
}

// Save writes the profiles with permissions for the user only, through a temporary file so a failure leaves the
// previous file intact.
func (p *Profiles) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".profiles-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Merge replaces the resources and the device with the ones of a new provisioning, keeping the key.
func (p *Profiles) Merge(f *File) {
	p.Device = f.Device
	p.Resources = f.Resources
}
