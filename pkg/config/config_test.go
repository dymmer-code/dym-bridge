package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	tempDir := t.TempDir()
	iniPath := filepath.Join(tempDir, "dym-bridge.ini")
	keyPath := filepath.Join(tempDir, "test_id_ed25519")

	_ = os.WriteFile(keyPath, []byte("dummy-key"), 0600)

	content := `
[ssh]
host = server.test.com
port = 2222
user = testuser
key_file = test_id_ed25519

[wordpress]
host_header = testclient.com
remote_port = 8080
local_port = 9090
admin_path = /wp-admin/

[sftp]
remote_path = /var/www/test
`
	if err := os.WriteFile(iniPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test ini: %v", err)
	}

	cfg, err := Load(iniPath)
	if err != nil {
		t.Fatalf("expected successful load, got: %v", err)
	}

	if cfg.SSH.Host != "server.test.com" || cfg.SSH.Port != 2222 || cfg.SSH.User != "testuser" {
		t.Errorf("unexpected ssh config: %+v", cfg.SSH)
	}

	// Verify relative key_file was resolved against iniDir
	expectedKeyPath := filepath.Join(tempDir, "test_id_ed25519")
	if cfg.SSH.KeyFile != expectedKeyPath {
		t.Errorf("expected key_file %q, got %q", expectedKeyPath, cfg.SSH.KeyFile)
	}

	if cfg.WordPress.HostHeader != "testclient.com" || cfg.WordPress.LocalPort != 9090 || cfg.WordPress.RemotePort != 8080 {
		t.Errorf("unexpected wordpress config: %+v", cfg.WordPress)
	}

	if cfg.LocalAdminURL() != "http://127.0.0.1:9090/wp-admin/" {
		t.Errorf("unexpected local admin url: %s", cfg.LocalAdminURL())
	}
}

func TestLoadDefaults(t *testing.T) {
	tempDir := t.TempDir()
	iniPath := filepath.Join(tempDir, "dym-bridge.ini")

	content := `
[ssh]
host = minimal.com
user = minuser
password = secret

[wordpress]
host_header = minimal.com
`
	if err := os.WriteFile(iniPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test ini: %v", err)
	}

	cfg, err := Load(iniPath)
	if err != nil {
		t.Fatalf("expected successful load, got: %v", err)
	}

	if cfg.SSH.Port != 22 {
		t.Errorf("expected default ssh port 22, got %d", cfg.SSH.Port)
	}
	if cfg.WordPress.RemotePort != 80 {
		t.Errorf("expected default remote port 80, got %d", cfg.WordPress.RemotePort)
	}
	if cfg.WordPress.LocalPort != 8080 {
		t.Errorf("expected default local port 8080, got %d", cfg.WordPress.LocalPort)
	}
	if cfg.WordPress.AdminPath != "/wp-admin/" {
		t.Errorf("expected default admin path /wp-admin/, got %s", cfg.WordPress.AdminPath)
	}
	if cfg.SFTP.RemotePath != "/var/www/html/wp-content" {
		t.Errorf("expected default sftp path, got %s", cfg.SFTP.RemotePath)
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		ini     string
		wantErr error
	}{
		{
			name: "missing ssh host",
			ini: `
[ssh]
user = usr
password = pwd
[wordpress]
host_header = test.com
`,
			wantErr: ErrMissingSSHHost,
		},
		{
			name: "missing ssh user",
			ini: `
[ssh]
host = myhost
password = pwd
[wordpress]
host_header = test.com
`,
			wantErr: ErrMissingSSHUser,
		},
		{
			name: "missing auth",
			ini: `
[ssh]
host = myhost
user = usr
[wordpress]
host_header = test.com
`,
			wantErr: ErrMissingAuth,
		},
		{
			name: "missing wordpress host_header",
			ini: `
[ssh]
host = myhost
user = usr
password = pwd
[wordpress]
`,
			wantErr: ErrMissingWPHost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			iniPath := filepath.Join(tempDir, "test.ini")
			_ = os.WriteFile(iniPath, []byte(tt.ini), 0644)

			_, err := Load(iniPath)
			if err != tt.wantErr {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestFindConfigFileExplicit(t *testing.T) {
	tempDir := t.TempDir()
	iniPath := filepath.Join(tempDir, "custom.ini")
	_ = os.WriteFile(iniPath, []byte("[ssh]\n"), 0644)

	found, err := FindConfigFile(iniPath)
	if err != nil {
		t.Fatalf("expected to find custom.ini: %v", err)
	}
	if found != iniPath {
		t.Errorf("expected %s, got %s", iniPath, found)
	}

	_, err = FindConfigFile(filepath.Join(tempDir, "nonexistent.ini"))
	if err != ErrConfigNotFound {
		t.Errorf("expected ErrConfigNotFound, got: %v", err)
	}
}
