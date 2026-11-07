package tunnel

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dymmer-code/dym-bridge/pkg/config"
	"golang.org/x/crypto/ssh"
)

// State represents the status of the tunnel connection.
type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateError        State = "error"
)

// StateListener is called whenever the tunnel connection state changes.
type StateListener func(state State, msg string)

// Manager coordinates the SSH connection and HTTP reverse proxy.
type Manager struct {
	mu          sync.RWMutex
	cfg         *config.Config
	sshConn     *SSHConnection
	proxy       *HTTPProxyServer
	state       State
	lastError   error
	listeners   []StateListener
	autoConnect bool
}

// NewManager creates a new tunnel manager.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:   cfg,
		state: StateDisconnected,
	}
}

// AddListener registers a callback to be notified of state transitions.
func (m *Manager) AddListener(l StateListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, l)
}

func (m *Manager) setState(st State, err error) {
	m.mu.Lock()
	m.state = st
	m.lastError = err
	listeners := append([]StateListener(nil), m.listeners...)
	m.mu.Unlock()

	msg := ""
	if err != nil {
		msg = err.Error()
	}
	for _, l := range listeners {
		l(st, msg)
	}
}

// Connect initiates the SSH connection and sets up the local reverse proxy.
func (m *Manager) Connect() error {
	m.mu.Lock()
	if m.state == StateConnected || m.state == StateConnecting {
		m.mu.Unlock()
		return nil
	}
	cfg := m.cfg
	m.mu.Unlock()

	m.setState(StateConnecting, nil)

	// 1. Establish SSH connection
	sshConn, err := ConnectSSH(&cfg.SSH)
	if err != nil {
		m.setState(StateError, err)
		return err
	}

	// Without a Host header (a provisioned resource) there is no web tunnel: the connection serves SFTP only.
	if strings.TrimSpace(cfg.WordPress.HostHeader) == "" {
		m.mu.Lock()
		m.sshConn = sshConn
		m.mu.Unlock()
		m.setState(StateConnected, nil)
		return nil
	}

	// 2. Setup HTTP Reverse Proxy
	dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return sshConn.Dial(network, addr)
	}

	proxy, err := NewHTTPProxyServer(&cfg.WordPress, dialer)
	if err != nil {
		_ = sshConn.Close()
		m.setState(StateError, err)
		return fmt.Errorf("failed to initialize local HTTP proxy: %w", err)
	}

	proxy.Start()

	m.mu.Lock()
	m.sshConn = sshConn
	m.proxy = proxy
	m.mu.Unlock()

	m.setState(StateConnected, nil)
	return nil
}

// Disconnect terminates the proxy server and SSH connection.
func (m *Manager) Disconnect() {
	m.mu.Lock()
	proxy := m.proxy
	sshConn := m.sshConn
	m.proxy = nil
	m.sshConn = nil
	m.mu.Unlock()

	if proxy != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = proxy.Stop(ctx)
		cancel()
	}

	if sshConn != nil {
		_ = sshConn.Close()
	}

	m.setState(StateDisconnected, nil)
}

// SSHClient returns the underlying ssh.Client if connected.
func (m *Manager) SSHClient() *ssh.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.sshConn != nil {
		return m.sshConn.Client()
	}
	return nil
}

// AdminURL returns the local URL to access WordPress Admin.
func (m *Manager) AdminURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.proxy != nil {
		return m.proxy.LocalURL()
	}
	if m.cfg != nil {
		return m.cfg.LocalAdminURL()
	}
	return "http://127.0.0.1:8080/wp-admin/"
}

// State returns the current tunnel state.
func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Config returns the current configuration.
func (m *Manager) Config() *config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// SetConfig updates the configuration.
func (m *Manager) SetConfig(cfg *config.Config) {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
}
