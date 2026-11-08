package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dymmer-code/dym-bridge/pkg/secret"
	"gopkg.in/ini.v1"
)

var (
	ErrConfigNotFound = errors.New("fichero de configuración dym-bridge.ini no encontrado")
	ErrMissingSSHHost = errors.New("el campo [ssh].host es obligatorio")
	ErrMissingSSHUser = errors.New("el campo [ssh].user es obligatorio")
	ErrMissingAuth    = errors.New("se requiere autenticación: configure [ssh].key_file, [ssh].key_data o [ssh].password")
	ErrMissingWPHost  = errors.New("el campo [wordpress].host_header es obligatorio")
)

type SSHConfig struct {
	Host       string `ini:"host"`
	Port       int    `ini:"port"`
	User       string `ini:"user"`
	Password   string `ini:"password"`
	KeyFile    string `ini:"key_file"`
	KeyData    string `ini:"key_data"`
	Passphrase string `ini:"passphrase"`
	// HostKeys are the SHA-256 fingerprints of the keys the server may present (they come from the provisioning of
	// Dymmer). When there are any, a server with another key is refused; with none (a manual .ini) the first
	// connection is accepted as it always was.
	HostKeys []string `ini:"-"`
}

type WordPressConfig struct {
	HostHeader string `ini:"host_header"`
	RemotePort int    `ini:"remote_port"`
	LocalPort  int    `ini:"local_port"`
	AdminPath  string `ini:"admin_path"`
}

type SFTPConfig struct {
	RemotePath string `ini:"remote_path"`
}

type Config struct {
	Tunnels    []Tunnel        `ini:"-"`
	SSH        SSHConfig       `ini:"ssh"`
	WordPress  WordPressConfig `ini:"wordpress"`
	SFTP       SFTPConfig      `ini:"sftp"`
	LoadedPath string          `ini:"-"`
	// Provisioned is true when the configuration comes from Dymmer instead of a .ini file: the web tunnel is optional
	// then, because Dymmer does not give a Host header.
	Provisioned bool `ini:"-"`
}

// Load loads and parses an INI configuration file from the given path.
func Load(path string) (*Config, error) {
	cfgFile, err := ini.LoadSources(ini.LoadOptions{
		SpaceBeforeInlineComment: true,
		AllowBooleanKeys:         true,
	}, path)
	if err != nil {
		return nil, fmt.Errorf("error al leer archivo ini %q: %w", path, err)
	}

	cfg := &Config{
		LoadedPath: path,
	}

	if err := cfgFile.MapTo(cfg); err != nil {
		return nil, fmt.Errorf("error al mapear configuración: %w", err)
	}

	// Apply default values
	cfg.applyDefaults()

	// Resolve relative KeyFile path relative to the INI file's directory (if not encrypted)
	if cfg.SSH.KeyFile != "" && !secret.IsEncrypted(cfg.SSH.KeyFile) && !filepath.IsAbs(cfg.SSH.KeyFile) {
		iniDir := filepath.Dir(path)
		cfg.SSH.KeyFile = filepath.Clean(filepath.Join(iniDir, cfg.SSH.KeyFile))
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) applyDefaults() {
	c.SSH.Host = strings.TrimSpace(c.SSH.Host)
	c.SSH.User = strings.TrimSpace(c.SSH.User)
	c.SSH.Password = strings.TrimSpace(c.SSH.Password)
	c.SSH.KeyFile = strings.TrimSpace(c.SSH.KeyFile)
	c.SSH.KeyData = strings.TrimSpace(c.SSH.KeyData)
	c.SSH.Passphrase = strings.TrimSpace(c.SSH.Passphrase)
	c.WordPress.HostHeader = strings.TrimSpace(c.WordPress.HostHeader)
	c.WordPress.AdminPath = strings.TrimSpace(c.WordPress.AdminPath)
	c.SFTP.RemotePath = strings.TrimSpace(c.SFTP.RemotePath)

	if c.SSH.Port <= 0 {
		c.SSH.Port = 22
	}
	if c.WordPress.RemotePort <= 0 {
		c.WordPress.RemotePort = 80
	}
	if c.WordPress.LocalPort <= 0 {
		c.WordPress.LocalPort = 8080
	}
	if c.WordPress.AdminPath == "" {
		c.WordPress.AdminPath = "/wp-admin/"
	} else if !strings.HasPrefix(c.WordPress.AdminPath, "/") {
		c.WordPress.AdminPath = "/" + c.WordPress.AdminPath
	}
	if c.SFTP.RemotePath == "" {
		c.SFTP.RemotePath = "/var/www/wordpress"
	}
}

// HasEncryptedCredentials reports whether any SSH credential field contains encrypted ENC(...) data.
func (c *Config) HasEncryptedCredentials() bool {
	return secret.IsEncrypted(c.SSH.Password) ||
		secret.IsEncrypted(c.SSH.Passphrase) ||
		secret.IsEncrypted(c.SSH.KeyData) ||
		secret.IsEncrypted(c.SSH.KeyFile)
}

// Unlock decrypts all encrypted fields in memory using the provided master password.
func (c *Config) Unlock(masterPassword string) error {
	if secret.IsEncrypted(c.SSH.Password) {
		dec, err := secret.Decrypt(c.SSH.Password, masterPassword)
		if err != nil {
			return fmt.Errorf("error al desencriptar [ssh].password: %w", err)
		}
		c.SSH.Password = dec
	}

	if secret.IsEncrypted(c.SSH.Passphrase) {
		dec, err := secret.Decrypt(c.SSH.Passphrase, masterPassword)
		if err != nil {
			return fmt.Errorf("error al desencriptar [ssh].passphrase: %w", err)
		}
		c.SSH.Passphrase = dec
	}

	if secret.IsEncrypted(c.SSH.KeyData) {
		dec, err := secret.Decrypt(c.SSH.KeyData, masterPassword)
		if err != nil {
			return fmt.Errorf("error al desencriptar [ssh].key_data: %w", err)
		}
		c.SSH.KeyData = dec
	}

	if secret.IsEncrypted(c.SSH.KeyFile) {
		dec, err := secret.Decrypt(c.SSH.KeyFile, masterPassword)
		if err != nil {
			return fmt.Errorf("error al desencriptar [ssh].key_file: %w", err)
		}
		c.SSH.KeyFile = dec
		// Resolve relative path if needed now that it is decrypted
		if c.SSH.KeyFile != "" && !filepath.IsAbs(c.SSH.KeyFile) && c.LoadedPath != "" {
			iniDir := filepath.Dir(c.LoadedPath)
			c.SSH.KeyFile = filepath.Clean(filepath.Join(iniDir, c.SSH.KeyFile))
		}
	}

	return nil
}

// Validate checks that required fields are present and valid.
func (c *Config) Validate() error {
	if err := ValidateTunnels(c.Tunnels); err != nil {
		return err
	}
	if strings.TrimSpace(c.SSH.Host) == "" {
		return ErrMissingSSHHost
	}
	if strings.TrimSpace(c.SSH.User) == "" {
		return ErrMissingSSHUser
	}
	if strings.TrimSpace(c.SSH.KeyFile) == "" && strings.TrimSpace(c.SSH.KeyData) == "" && strings.TrimSpace(c.SSH.Password) == "" {
		return ErrMissingAuth
	}
	if !c.Provisioned && strings.TrimSpace(c.WordPress.HostHeader) == "" {
		return ErrMissingWPHost
	}
	return nil
}

// LocalAdminURL returns the full local URL for opening the WordPress admin in the browser.
func (c *Config) LocalAdminURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", c.WordPress.LocalPort, c.WordPress.AdminPath)
}
