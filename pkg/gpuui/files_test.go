package gpuui

import (
	"github.com/dymmer-code/dym-bridge/pkg/local"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"os"
	"path/filepath"
	"testing"
)

func TestFileOperationsStayInSelectedDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"", ".", "..", "../outside", "a/b", `a\b`} {
		if _, err := folderPath(dir, name, false); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	target, err := folderPath(dir, "New folder", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = local.Mkdir(target); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(target, "file"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	items := []sftp.Item{{Name: "New folder", Path: target, IsDir: true}}
	if err = deleteItems(dir, items, false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("directory survived")
	}
	outside := filepath.Join(t.TempDir(), "keep")
	if err = os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = deleteItems(dir, []sftp.Item{{Path: outside}}, false, nil); err == nil {
		t.Fatal("accepted outside deletion")
	}
	if _, err = os.Stat(outside); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteFolderCreateAndDelete(t *testing.T) {
	cfg := testSFTPServer(t)
	dir := t.TempDir()
	cfg.SFTP.RemotePath = dir
	var s session
	if err := s.connect(cfg); err != nil {
		t.Fatal(err)
	}
	defer s.close()
	target, err := folderPath(dir, "remote folder", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.files.Mkdir(target); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(target, "child"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := s.files.List(dir)
	if err != nil || len(items) != 1 {
		t.Fatalf("%v %v", items, err)
	}
	if err = deleteItems(dir, items, true, s.files); err != nil {
		t.Fatal(err)
	}
	items, err = s.files.List(dir)
	if err != nil || len(items) != 0 {
		t.Fatalf("%v %v", items, err)
	}
}
func TestDroppedFilesKeepPathsAndDirectories(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := droppedItems([]string{file, dir})
	if err != nil || len(items) != 2 || items[0].Size != 5 || !items[1].IsDir {
		t.Fatalf("%v %v", items, err)
	}
	if _, err = droppedItems([]string{filepath.Join(dir, "missing")}); err == nil {
		t.Fatal("missing file accepted")
	}
}
