package gpuui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNavigatePreservesDirectoryOnError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	m := NewModel(root)
	if err := m.Navigate(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Navigate(filepath.Join(root, "missing")); err == nil {
		t.Fatal("expected error")
	}
	if m.Path != root || len(m.Items) != 1 {
		t.Fatal("failed navigation changed current directory")
	}
}

func TestFilterAndSelection(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"Alpha.txt", "beta.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := NewModel(root)
	if err := m.Navigate(root); err != nil {
		t.Fatal(err)
	}
	m.Filter("ALP")
	if len(m.Visible) != 1 || m.Visible[0].Name != "Alpha.txt" {
		t.Fatal(m.Visible)
	}
	m.Select(0)
	if m.Selected() == nil || m.Selected().Name != "Alpha.txt" {
		t.Fatal("wrong selection")
	}
	m.Filter("beta")
	if m.Selected() != nil {
		t.Fatal("stale selection after filtering")
	}
}

func TestDemoNavigationAndStress(t *testing.T) {
	m := NewDemoModel()
	if err := m.Navigate("/wp-content/themes"); err != nil {
		t.Fatal(err)
	}
	if m.Path != "/wp-content/themes" {
		t.Fatal(m.Path)
	}
	before := m.Path
	if err := m.Navigate("/not-present"); err == nil {
		t.Fatal("unknown demo folder must fail")
	}
	if m.Path != before {
		t.Fatal("invalid path changed model")
	}
	m.Stress()
	if len(m.Visible) != 10000 {
		t.Fatal(len(m.Visible))
	}
	m.Filter("asset-09999")
	if len(m.Visible) != 1 {
		t.Fatal(len(m.Visible))
	}
}

func TestSelectAllVisibleAndClear(t *testing.T) {
	m := NewDemoModel()
	m.Filter("themes")
	m.ToggleAll()
	if m.SelectionCount() != 1 || !m.IsSelected(m.Visible[0].Path) {
		t.Fatal("must select only visible items")
	}
	m.ToggleAll()
	if m.SelectionCount() != 0 {
		t.Fatal("toggle must clear selection")
	}
	m.Filter("")
	m.ToggleAll()
	if m.SelectionCount() != 4 {
		t.Fatal("must select all four entries")
	}
	m.Select(0)
	if m.SelectionCount() != 1 {
		t.Fatal("single click must replace selection")
	}
	m.Filter("uploads")
	if m.SelectionCount() != 0 {
		t.Fatal("filter must clear stale selection")
	}
}
