package tunnel

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/dymmer-code/dym-bridge/pkg/config"
	"golang.org/x/crypto/ssh"
)

// SSHConnection encapsulates an active SSH client connection.
type SSHConnection struct {
	client *ssh.Client
	config *config.SSHConfig
	done   chan struct{}
}

// ConnectSSH establishes a new SSH connection using the provided configuration.
func ConnectSSH(cfg *config.SSHConfig) (*SSHConnection, error) {
	authMethods, err := buildAuthMethods(cfg)
	if err != nil {
		return nil, fmt.Errorf("error de autenticación SSH: %w", err)
	}

	clientConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Permits seamless initial connection
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	client, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar al servidor SSH en %s: %w", addr, err)
	}

	conn := &SSHConnection{
		client: client,
		config: cfg,
		done:   make(chan struct{}),
	}

	// Start background keepalive
	go conn.keepAliveLoop(30 * time.Second)

	return conn, nil
}

func buildAuthMethods(cfg *config.SSHConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if cfg.KeyFile != "" {
		keyBytes, err := os.ReadFile(cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("no se pudo leer el archivo de clave privada %q: %w", cfg.KeyFile, err)
		}

		var signer ssh.Signer
		if cfg.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(cfg.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}

		if err != nil {
			return nil, fmt.Errorf("formato de clave privada no válido en %q: %w", cfg.KeyFile, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no se configuró ningún método de autenticación válido")
	}

	return methods, nil
}

// Dial dials a network address on the remote host via the SSH client.
func (c *SSHConnection) Dial(network, addr string) (net.Conn, error) {
	if c.client == nil {
		return nil, fmt.Errorf("conexión SSH no establecida")
	}
	return c.client.Dial(network, addr)
}

// Client returns the underlying ssh.Client.
func (c *SSHConnection) Client() *ssh.Client {
	return c.client
}

// keepAliveLoop sends periodic global requests to prevent TCP timeouts.
func (c *SSHConnection) keepAliveLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			if c.client == nil {
				return
			}
			_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				// Fallback ping
				_, _, _ = c.client.SendRequest("ping", false, nil)
			}
		}
	}
}

// Close terminates the SSH connection and stops the keepalive loop.
func (c *SSHConnection) Close() error {
	select {
	case <-c.done:
		// already closed
	default:
		close(c.done)
	}

	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
