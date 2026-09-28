package sftp

import (
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Item represents a remote file or folder in the SFTP directory tree.
type Item struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime time.Time
	Mode    os.FileMode
}

// Client wraps an SFTP client connection.
type Client struct {
	sftpClient *sftp.Client
}

// NewClient creates a new SFTP client over an established SSH client.
func NewClient(sshClient *ssh.Client) (*Client, error) {
	if sshClient == nil {
		return nil, fmt.Errorf("cliente SSH no inicializado")
	}

	sc, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, fmt.Errorf("no se pudo iniciar subsistema SFTP: %w", err)
	}

	return &Client{sftpClient: sc}, nil
}

// Close closes the underlying SFTP client.
func (c *Client) Close() error {
	if c.sftpClient != nil {
		return c.sftpClient.Close()
	}
	return nil
}

// List returns the contents of the given directory, with folders first, sorted alphabetically.
func (c *Client) List(dirPath string) ([]Item, error) {
	if c.sftpClient == nil {
		return nil, fmt.Errorf("cliente SFTP no conectado")
	}

	entries, err := c.sftpClient.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("error al listar directorio %q: %w", dirPath, err)
	}

	var items []Item
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}

		itemPath := path.Join(dirPath, name)
		items = append(items, Item{
			Name:    name,
			Path:    itemPath,
			IsDir:   entry.IsDir(),
			Size:    entry.Size(),
			ModTime: entry.ModTime(),
			Mode:    entry.Mode(),
		})
	}

	SortItems(items)
	return items, nil
}

// SortItems sorts folders first, then alphabetical by name (case-insensitive).
func SortItems(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		// Directories first
		if items[i].IsDir && !items[j].IsDir {
			return true
		}
		if !items[i].IsDir && items[j].IsDir {
			return false
		}
		// Alphabetical case-insensitive
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
}

// Upload uploads a local file to the specified remote path.
func (c *Client) Upload(localPath, remoteFilePath string, progress ProgressCallback) error {
	if c.sftpClient == nil {
		return fmt.Errorf("cliente SFTP no conectado")
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("error al abrir archivo local %q: %w", localPath, err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("error al obtener tamaño de %q: %w", localPath, err)
	}

	// Create or truncate remote file
	remoteFile, err := c.sftpClient.Create(remoteFilePath)
	if err != nil {
		return fmt.Errorf("error al crear archivo remoto %q: %w", remoteFilePath, err)
	}
	defer remoteFile.Close()

	reader := NewCountingReader(localFile, stat.Size(), progress)
	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(remoteFile, reader, buf); err != nil {
		return fmt.Errorf("error al transferir archivo a %q: %w", remoteFilePath, err)
	}

	return nil
}

// Download downloads a remote file to a local destination path.
func (c *Client) Download(remoteFilePath, localFilePath string, progress ProgressCallback) error {
	if c.sftpClient == nil {
		return fmt.Errorf("cliente SFTP no conectado")
	}

	remoteFile, err := c.sftpClient.Open(remoteFilePath)
	if err != nil {
		return fmt.Errorf("error al abrir archivo remoto %q: %w", remoteFilePath, err)
	}
	defer remoteFile.Close()

	stat, err := remoteFile.Stat()
	if err != nil {
		return fmt.Errorf("error al consultar archivo remoto %q: %w", remoteFilePath, err)
	}

	localFile, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("error al crear archivo local %q: %w", localFilePath, err)
	}
	defer localFile.Close()

	reader := NewCountingReader(remoteFile, stat.Size(), progress)
	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(localFile, reader, buf); err != nil {
		return fmt.Errorf("error al descargar archivo a %q: %w", localFilePath, err)
	}

	return nil
}

// Mkdir creates a directory on the remote server.
func (c *Client) Mkdir(remoteDirPath string) error {
	if c.sftpClient == nil {
		return fmt.Errorf("cliente SFTP no conectado")
	}
	return c.sftpClient.Mkdir(remoteDirPath)
}

// Delete removes a file or directory on the remote server.
func (c *Client) Delete(remotePath string, isDir bool) error {
	if c.sftpClient == nil {
		return fmt.Errorf("cliente SFTP no conectado")
	}

	if isDir {
		return c.removeDirRecursive(remotePath)
	}
	return c.sftpClient.Remove(remotePath)
}

func (c *Client) removeDirRecursive(remoteDirPath string) error {
	entries, err := c.sftpClient.ReadDir(remoteDirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		subPath := path.Join(remoteDirPath, entry.Name())
		if entry.IsDir() {
			if err := c.removeDirRecursive(subPath); err != nil {
				return err
			}
		} else {
			if err := c.sftpClient.Remove(subPath); err != nil {
				return err
			}
		}
	}

	return c.sftpClient.RemoveDirectory(remoteDirPath)
}

// Rename renames or moves a file/folder on the remote server.
func (c *Client) Rename(oldPath, newPath string) error {
	if c.sftpClient == nil {
		return fmt.Errorf("cliente SFTP no conectado")
	}
	return c.sftpClient.Rename(oldPath, newPath)
}
