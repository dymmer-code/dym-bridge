package tunnel

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/dymmer-code/dym-bridge/pkg/config"
	"golang.org/x/crypto/ssh"
)

func generateTestEd25519Key(t *testing.T) ([]byte, []byte) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	privBytes, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}

	pemBytes := pem.EncodeToMemory(privBytes)
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to create ssh public key: %v", err)
	}

	return pemBytes, sshPub.Marshal()
}

func TestBuildAuthMethods_KeyData(t *testing.T) {
	keyPEM, _ := generateTestEd25519Key(t)

	cfg := &config.SSHConfig{
		User:    "testuser",
		KeyData: string(keyPEM),
	}

	methods, err := buildAuthMethods(cfg)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestBuildAuthMethods_KeyFile(t *testing.T) {
	keyPEM, _ := generateTestEd25519Key(t)
	tempDir := t.TempDir()
	keyFile := filepath.Join(tempDir, "id_ed25519")
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	cfg := &config.SSHConfig{
		User:    "testuser",
		KeyFile: keyFile,
	}

	methods, err := buildAuthMethods(cfg)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestBuildAuthMethods_Password(t *testing.T) {
	cfg := &config.SSHConfig{
		User:     "testuser",
		Password: "password123",
	}

	methods, err := buildAuthMethods(cfg)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}
