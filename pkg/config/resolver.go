package config

import (
	"os"
	"path/filepath"
	"strings"
)

const DefaultConfigFileName = "dym-bridge.ini"

// FindConfigFile searches for the configuration file in standard locations.
func FindConfigFile(cliPath string) (string, error) {
	if cliPath != "" {
		if fileExists(cliPath) {
			abs, err := filepath.Abs(cliPath)
			if err == nil {
				return abs, nil
			}
			return cliPath, nil
		}
		return "", ErrConfigNotFound
	}

	candidates := CandidatePaths()
	for _, candidate := range candidates {
		if fileExists(candidate) {
			abs, err := filepath.Abs(candidate)
			if err == nil {
				return abs, nil
			}
			return candidate, nil
		}
	}

	return "", ErrConfigNotFound
}

// CandidatePaths returns the list of paths checked for the configuration file in order of precedence.
func CandidatePaths() []string {
	var candidates []string

	// 1. Same directory as the executable
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(exeDir, DefaultConfigFileName))

		// If running inside a macOS .app bundle (.../Contents/MacOS/dym-bridge),
		// also check the directory containing the .app bundle itself.
		if strings.Contains(exeDir, ".app/Contents/MacOS") {
			bundleDir := filepath.Dir(filepath.Dir(filepath.Dir(exeDir)))
			candidates = append(candidates, filepath.Join(bundleDir, DefaultConfigFileName))
		}
	}

	// 2. Current working directory
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, DefaultConfigFileName))
	}

	// 3. User configuration directory ~/.config/dym-bridge/dym-bridge.ini
	if homeDir, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(homeDir, ".config", "dym-bridge", DefaultConfigFileName))
	}

	return candidates
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// SampleConfigTemplate returns a formatted sample dym-bridge.ini content.
func SampleConfigTemplate() string {
	return `# dym-bridge.ini - Configuración de conexión cliente
[ssh]
host = ssh.ejemplo.com
port = 22
user = cliente_wp
# Autenticación: clave privada (recomendada) o contraseña
key_file = id_ed25519
# passphrase = (si la clave privada está protegida)
# password = SecretoOpcionalSiNoUsaClave

[wordpress]
# Dominio de WordPress (para cabecera HTTP Host y evitar bloqueos canónicos)
host_header = cliente.com
# Puerto en el servidor remoto donde escucha el servidor web en localhost (por defecto 80)
remote_port = 80
# Puerto local donde se expondrá el túnel (por defecto 8080)
local_port = 8080
# Ruta del panel de administración
admin_path = /wp-admin/

[sftp]
# Directorio remoto inicial en el servidor
remote_path = /var/www/html/wp-content
`
}
