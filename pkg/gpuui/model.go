// Package gpuui contains the isolated GoGPU desktop experiment.
package gpuui

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/dymmer-code/dym-bridge/pkg/local"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
)

// Model is a read-only directory browser. Failed navigation preserves the last view.
type Model struct {
	Path           string
	Items, Visible []sftp.Item
	Query          string
	selected       int
	selectedPaths  map[string]bool
	Demo           bool
}

func NewModel(dir string) *Model { return &Model{Path: dir, selected: -1} }
func NewDemoModel() *Model {
	m := NewModel("/wp-content")
	m.Demo = true
	_ = m.Navigate(m.Path)
	return m
}
func (m *Model) Navigate(dir string) error {
	var items []sftp.Item
	var err error
	if m.Demo {
		items, err = demoDirectory(path.Clean(dir))
	} else {
		items, err = local.ListDirectory(dir)
	}
	if err != nil {
		return err
	}
	m.Path = dir
	m.Items = items
	m.Filter("")
	return nil
}
func (m *Model) Filter(query string) {
	m.Query = query
	m.selected = -1
	m.selectedPaths = nil
	m.Visible = nil
	for _, item := range m.Items {
		if strings.Contains(strings.ToLower(item.Name), strings.ToLower(query)) {
			m.Visible = append(m.Visible, item)
		}
	}
}
func (m *Model) Select(index int) {
	m.selected = index
	m.selectedPaths = map[string]bool{}
	if item := m.Selected(); item != nil {
		m.selectedPaths[item.Path] = true
	}
}
func (m *Model) IsSelected(path string) bool { return m.selectedPaths[path] }
func (m *Model) SelectionCount() int         { return len(m.selectedPaths) }
func (m *Model) ToggleAll() {
	clear := len(m.selectedPaths) == len(m.Visible)
	m.selected = -1
	m.selectedPaths = map[string]bool{}
	if !clear {
		for _, item := range m.Visible {
			m.selectedPaths[item.Path] = true
		}
	}
}
func (m *Model) Selected() *sftp.Item {
	if m.selected < 0 || m.selected >= len(m.Visible) {
		return nil
	}
	item := m.Visible[m.selected]
	return &item
}
func (m *Model) Stress() {
	m.Items = make([]sftp.Item, 10000)
	for i := range m.Items {
		name := fmt.Sprintf("asset-%05d.jpg", i)
		m.Items[i] = sftp.Item{Name: name, Path: path.Join("/stress-test", name), Size: int64(12000 + i), ModTime: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)}
	}
	m.Path = "/stress-test"
	m.Filter("")
}
func demoDirectory(dir string) ([]sftp.Item, error) {
	tree := map[string][]string{
		"/":                             {"wp-content/"},
		"/wp-content":                   {"plugins/", "themes/", "uploads/", "index.php"},
		"/wp-content/plugins":           {"analytics/", "security/", "index.php"},
		"/wp-content/themes":            {"dymmer/", "index.php"},
		"/wp-content/themes/dymmer":     {"assets/", "style.css", "functions.php", "screenshot.png"},
		"/wp-content/uploads":           {"2026/"},
		"/wp-content/uploads/2026":      {"hero.webp", "team.jpg", "brand.svg"},
		"/wp-content/plugins/analytics": {}, "/wp-content/plugins/security": {}, "/wp-content/themes/dymmer/assets": {},
	}
	names, ok := tree[dir]
	if !ok {
		return nil, fmt.Errorf("esta carpeta no existe en la demostración")
	}
	items := make([]sftp.Item, 0, len(names))
	for _, name := range names {
		isDir := strings.HasSuffix(name, "/")
		name = strings.TrimSuffix(name, "/")
		items = append(items, sftp.Item{Name: name, Path: path.Join(dir, name), IsDir: isDir, Size: 24576, ModTime: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)})
	}
	sftp.SortItems(items)
	return items, nil
}
