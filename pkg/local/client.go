package local

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dymmer-code/dym-bridge/pkg/sftp"
)

// DefaultLocalDir returns the user's Downloads directory, or Home, or current directory.
func DefaultLocalDir() string {
	home, err := os.UserHomeDir()
	if err == nil {
		downloads := filepath.Join(home, "Downloads")
		if info, err := os.Stat(downloads); err == nil && info.IsDir() {
			return downloads
		}
		return home
	}
	cwd, err := os.Getwd()
	if err == nil {
		return cwd
	}
	return "."
}

// CommonDirectories returns quick-access folders (Downloads, Desktop, Documents, Home).
func CommonDirectories() map[string]string {
	res := make(map[string]string)
	home, err := os.UserHomeDir()
	if err != nil {
		return res
	}

	res["Home"] = home
	for _, folder := range []string{"Downloads", "Desktop", "Documents"} {
		p := filepath.Join(home, folder)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			res[folder] = p
		}
	}
	return res
}

// ListDirectory returns the sorted items of a local directory.
func ListDirectory(dirPath string) ([]sftp.Item, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read local directory %q: %w", dirPath, err)
	}

	var items []sftp.Item
	for _, e := range entries {
		name := e.Name()
		// Skip hidden files if desired, or keep them visible. Let's include everything except . and ..
		if name == "." || name == ".." {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		items = append(items, sftp.Item{
			Name:    name,
			Path:    filepath.Join(dirPath, name),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Mode:    info.Mode(),
		})
	}

	sftp.SortItems(items)
	return items, nil
}

// Mkdir creates a directory locally.
func Mkdir(path string) error {
	return os.MkdirAll(path, 0755)
}

// Delete removes a file or directory locally.
func Delete(path string, isDir bool) error {
	if isDir {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// Rename moves or renames a local file or directory.
func Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
