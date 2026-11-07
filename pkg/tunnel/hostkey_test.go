package tunnel

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/dymmer-code/dym-bridge/pkg/config"
	"golang.org/x/crypto/ssh"
)

// sshServer is a minimal in-process server that accepts any public key; it exposes the key it presents as its host key.
func sshServer(t *testing.T) (host string, port int, fingerprint string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	conf := &ssh.ServerConfig{PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
		return &ssh.Permissions{}, nil
	}}
	conf.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				sc, chans, reqs, err := ssh.NewServerConn(c, conf)
				if err != nil {
					_ = c.Close()
					return
				}
				defer func() { _ = sc.Close() }()
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					_ = ch.Reject(ssh.Prohibited, "no")
				}
			}()
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port, ssh.FingerprintSHA256(signer.PublicKey())
}

func TestConnectSSHChecksReportedHostKeys(t *testing.T) {
	host, port, fingerprint := sshServer(t)
	keyPEM, _ := generateTestEd25519Key(t)
	cfg := func(keys ...string) *config.SSHConfig {
		return &config.SSHConfig{Host: host, Port: port, User: "u", KeyData: string(keyPEM), HostKeys: keys}
	}

	conn, err := ConnectSSH(cfg(fingerprint))
	if err != nil {
		t.Fatalf("the reported key must be accepted: %v", err)
	}
	_ = conn.Close()

	if _, err := ConnectSSH(cfg("SHA256:" + strings.Repeat("A", 43))); err == nil || !strings.Contains(err.Error(), "not the one Dymmer reported") {
		t.Fatalf("another server key must be refused: %v", err)
	}

	// A manual .ini has no reported keys and keeps working as before.
	conn, err = ConnectSSH(cfg())
	if err != nil {
		t.Fatalf("without reported keys the first connection is accepted: %v", err)
	}
	_ = conn.Close()
}

func TestManagerWithoutHostHeaderServesSFTPOnly(t *testing.T) {
	host, port, fingerprint := sshServer(t)
	keyPEM, _ := generateTestEd25519Key(t)
	m := NewManager(&config.Config{
		Provisioned: true,
		SSH:         config.SSHConfig{Host: host, Port: port, User: "u", KeyData: string(keyPEM), HostKeys: []string{fingerprint}},
	})
	if err := m.Connect(); err != nil {
		t.Fatal(err)
	}
	defer m.Disconnect()
	if m.State() != StateConnected || m.SSHClient() == nil {
		t.Fatalf("state %v", m.State())
	}
	if strings.Contains(m.AdminURL(), strconv.Itoa(port)) {
		t.Fatal("no web tunnel was expected")
	}
}
