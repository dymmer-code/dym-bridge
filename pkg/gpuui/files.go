package gpuui

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dymmer-code/dym-bridge/pkg/local"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	p "github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/state"
)

func folderPath(dir, name string, remote bool) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return "", fmt.Errorf("Enter a folder name without path separators")
	}
	if remote {
		return path.Join(dir, name), nil
	}
	return filepath.Join(dir, name), nil
}
func deleteItems(dir string, items []sftp.Item, remote bool, client *sftp.Client) error {
	for _, item := range items {
		parent := filepath.Dir(item.Path)
		base := filepath.Clean(dir)
		if remote {
			parent = path.Dir(item.Path)
			base = path.Clean(dir)
		}
		if parent != base || item.Path == base {
			return fmt.Errorf("Item is outside the selected folder")
		}
	}
	for _, item := range items {
		var err error
		if remote {
			if client == nil {
				return fmt.Errorf("Not connected")
			}
			err = client.Delete(item.Path, item.IsDir)
		} else {
			err = local.Delete(item.Path, item.IsDir)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (b *browser) mutateFiles(m *Model, message string, work func() error) {
	var items []sftp.Item
	listed := false
	b.runOperation(message, func() error {
		err := work()
		var listErr error
		if m == b.remote {
			items, listErr = b.session.files.List(m.Path)
		} else {
			items, listErr = local.ListDirectory(m.Path)
		}
		listed = listErr == nil
		if err != nil {
			return err
		}
		return listErr
	}, func(err error) {
		if listed {
			m.Items = items
			m.Filter("")
		}
		if err != nil {
			b.status.Set(message + ": " + err.Error())
		} else {
			b.status.Set("Done")
		}
	})
}
func (b *browser) newFolder(m *Model) {
	if b.busy || (m == b.remote && !b.connected) {
		return
	}
	name := b.field("Folder name", false)
	message := state.NewSignal("")
	b.showForm("New folder", b.text(m.Path, 12).Path(), name, p.TextFn(func() string { return message.Get() }).FontSize(12), b.action("Create", true, func() {
		target, err := folderPath(m.Path, name.Text(), m == b.remote)
		if err != nil {
			message.Set(err.Error())
			return
		}
		b.mutateFiles(m, "Creating folder…", func() error {
			if m == b.remote {
				return b.session.files.Mkdir(target)
			}
			return local.Mkdir(target)
		})
	}))
}
func (b *browser) deleteSelected(m *Model) {
	if b.busy || (m == b.remote && !b.connected) {
		return
	}
	items := m.CheckedItems()
	if len(items) == 0 {
		return
	}
	b.showForm("Delete selected items", b.text(fmt.Sprintf("Permanently delete %d item(s) and their contents?", len(items)), 14), b.text(m.Path, 12).Path(), b.action("Delete permanently", true, func() {
		b.mutateFiles(m, "Deleting…", func() error { return deleteItems(m.Path, items, m == b.remote, b.session.files) })
	}))
}
func (b *browser) goToFolder(m *Model) {
	entry := b.field("Folder path", false)
	entry.SetText(m.Path)
	b.showForm("Go to folder", entry, b.action("Open", true, func() { b.navigate(m, strings.TrimSpace(entry.Text())) }))
}
func droppedItems(paths []string) ([]sftp.Item, error) {
	items := make([]sftp.Item, 0, len(paths))
	for _, name := range paths {
		info, err := os.Stat(name)
		if err != nil {
			return nil, err
		}
		items = append(items, sftp.Item{Name: filepath.Base(name), Path: name, IsDir: info.IsDir(), Size: info.Size(), Mode: info.Mode(), ModTime: info.ModTime()})
	}
	return items, nil
}
func (b *browser) dropFiles(paths []string) {
	if b.busy || !b.connected {
		b.status.Set("Connect before uploading files")
		return
	}
	items, err := droppedItems(paths)
	if err != nil {
		b.status.Set(err.Error())
		return
	}
	b.transferSelection(true, items)
}
