package sftp

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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
		return nil, fmt.Errorf("SSH client not initialized")
	}

	sc, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, fmt.Errorf("failed to start SFTP subsystem: %w", err)
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
		return nil, fmt.Errorf("SFTP client not connected")
	}

	entries, err := c.sftpClient.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory %q: %w", dirPath, err)
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
		return fmt.Errorf("SFTP client not connected")
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file %q: %w", localPath, err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get size for %q: %w", localPath, err)
	}

	// Create or truncate remote file
	remoteFile, err := c.sftpClient.Create(remoteFilePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %q: %w", remoteFilePath, err)
	}
	defer remoteFile.Close()

	reader := NewCountingReader(localFile, stat.Size(), progress)
	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(remoteFile, reader, buf); err != nil {
		return fmt.Errorf("failed to transfer file to %q: %w", remoteFilePath, err)
	}

	return nil
}

// Download downloads a remote file to a local destination path.
func (c *Client) Download(remoteFilePath, localFilePath string, progress ProgressCallback) error {
	if c.sftpClient == nil {
		return fmt.Errorf("SFTP client not connected")
	}

	remoteFile, err := c.sftpClient.Open(remoteFilePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file %q: %w", remoteFilePath, err)
	}
	defer remoteFile.Close()

	stat, err := remoteFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat remote file %q: %w", remoteFilePath, err)
	}

	localFile, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to create local file %q: %w", localFilePath, err)
	}
	defer localFile.Close()

	reader := NewCountingReader(remoteFile, stat.Size(), progress)
	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(localFile, reader, buf); err != nil {
		return fmt.Errorf("failed to download file to %q: %w", localFilePath, err)
	}

	return nil
}

// UploadDir recursively uploads a local directory tree to a target remote directory.
func (c *Client) UploadDir(localDirPath, remoteDirPath string, onFileStart func(relPath string, size int64), progress ProgressCallback) error {
	if c.sftpClient == nil {
		return fmt.Errorf("SFTP client not connected")
	}

	if err := c.sftpClient.MkdirAll(remoteDirPath); err != nil {
		return fmt.Errorf("failed to create remote directory %q: %w", remoteDirPath, err)
	}

	entries, err := os.ReadDir(localDirPath)
	if err != nil {
		return fmt.Errorf("failed to read local directory %q: %w", localDirPath, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}

		subLocal := filepath.Join(localDirPath, name)
		subRemote := path.Join(remoteDirPath, name)

		if entry.IsDir() {
			if err := c.UploadDir(subLocal, subRemote, onFileStart, progress); err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if onFileStart != nil {
				onFileStart(name, info.Size())
			}
			if err := c.Upload(subLocal, subRemote, progress); err != nil {
				return err
			}
		}
	}

	return nil
}

// DownloadDir recursively downloads a remote directory tree to a target local directory.
func (c *Client) DownloadDir(remoteDirPath, localDirPath string, onFileStart func(relPath string, size int64), progress ProgressCallback) error {
	if c.sftpClient == nil {
		return fmt.Errorf("SFTP client not connected")
	}

	if err := os.MkdirAll(localDirPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory %q: %w", localDirPath, err)
	}

	entries, err := c.sftpClient.ReadDir(remoteDirPath)
	if err != nil {
		return fmt.Errorf("failed to read remote directory %q: %w", remoteDirPath, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}

		subRemote := path.Join(remoteDirPath, name)
		subLocal := filepath.Join(localDirPath, name)

		if entry.IsDir() {
			if err := c.DownloadDir(subRemote, subLocal, onFileStart, progress); err != nil {
				return err
			}
		} else {
			if onFileStart != nil {
				onFileStart(name, entry.Size())
			}
			if err := c.Download(subRemote, subLocal, progress); err != nil {
				return err
			}
		}
	}

	return nil
}

// Mkdir creates a directory on the remote server.
func (c *Client) Mkdir(remoteDirPath string) error {
	if c.sftpClient == nil {
		return fmt.Errorf("SFTP client not connected")
	}
	return c.sftpClient.Mkdir(remoteDirPath)
}

// Delete removes a file or directory on the remote server.
func (c *Client) Delete(remotePath string, isDir bool) error {
	if c.sftpClient == nil {
		return fmt.Errorf("SFTP client not connected")
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
