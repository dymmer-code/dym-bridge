package provision

import (
	"path/filepath"
	"testing"
)

func TestNamedInterfacesSurviveProvisioningAndProfiles(t *testing.T) {
	p := &Profiles{Key: DeviceKey{Private: "encrypted-key"}, Resources: []Resource{{Address: "ssh.example.com", Port: 22, User: "client", HostKeys: []HostKey{{Fingerprint: "SHA256:test"}}, Tunnels: []Tunnel{
		{Host: "127.0.0.1", Port: 8080, ID: "wordpress", Name: "WordPress Admin", Scheme: "http", Path: "/wp-admin/", HostHeader: "site.example.com"},
		{Host: "127.0.0.1", Port: 3306},
	}}}}
	file := filepath.Join(t.TempDir(), "profiles.json")
	if err := p.Save(file); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loaded.Config(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Tunnels) != 2 || cfg.Tunnels[0].Name != "WordPress Admin" || cfg.Tunnels[0].HostHeader != "site.example.com" {
		t.Fatal(cfg.Tunnels)
	}
	loaded.Resources[0].Tunnels[0].Host = "evil.test"
	if _, err := loaded.Config(0); err == nil {
		t.Fatal("non-loopback destination accepted")
	}
	if cfg.Tunnels[0].Host != "127.0.0.1" {
		t.Fatal("config aliases saved profile")
	}
}
