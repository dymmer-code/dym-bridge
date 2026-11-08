package gpuui

import (
	"github.com/dymmer-code/dym-bridge/pkg/local"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/state"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testBrowser(t *testing.T) *browser {
	t.Helper()
	dir := t.TempDir()
	b := &browser{local: NewModel(dir), remote: NewModel("/"), status: state.NewSignal("Disconnected"), selection: state.NewSignal(""), transferText: state.NewSignal("No transfers"), progress: make(chan transferProgress, 1), updates: make(chan func(), 16), stopping: make(chan struct{}), profilesFile: filepath.Join(t.TempDir(), "profiles.json"), callbackFile: filepath.Join(t.TempDir(), "callback"), gpu: gogpu.NewApp(gogpu.DefaultConfig())}
	b.setTheme()
	b.ui = app.New(app.WithTheme(b.theme.AsTheme()))
	t.Cleanup(func() {
		close(b.stopping)
		if b.loginCancel != nil {
			b.loginCancel()
		}
		b.workers.Wait()
		b.session.close()
		b.ui.Window().Close()
	})
	return b
}
func finishOperation(t *testing.T, b *browser) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for b.busy {
		select {
		case fn := <-b.updates:
			fn()
		case <-deadline:
			t.Fatal("operation timed out")
		}
	}
	b.workers.Wait()
}
func TestBrowserConnectionTransfersAndEmptyDirectoryRefresh(t *testing.T) {
	b := testBrowser(t)
	cfg := testSFTPServer(t)
	cfg.SFTP.RemotePath = t.TempDir()
	b.connect(cfg)
	finishOperation(t, b)
	if !b.connected || b.remote.Path != cfg.SFTP.RemotePath {
		t.Fatalf("connect: %s", b.status.Get())
	}
	folder := filepath.Join(b.remote.Path, "new")
	b.mutateFiles(b.remote, "Creating", func() error { return b.session.files.Mkdir(folder) })
	finishOperation(t, b)
	if len(b.remote.Visible) != 1 {
		t.Fatal("new folder missing from listing")
	}
	b.remote.Toggle(0)
	items := b.remote.CheckedItems()
	b.deleteSelected(b.remote)
	if _, err := os.Stat(folder); err != nil {
		t.Fatal("delete confirmation deleted prematurely")
	}
	b.mutateFiles(b.remote, "Deleting", func() error { return deleteItems(b.remote.Path, items, true, b.session.files) })
	finishOperation(t, b)
	if len(b.remote.Visible) != 0 {
		t.Fatal("deleted folder remains in empty listing")
	}
	source := filepath.Join(b.local.Path, "file.txt")
	if err := os.WriteFile(source, []byte("roundtrip"), 0600); err != nil {
		t.Fatal(err)
	}
	b.navigate(b.local, b.local.Path)
	b.local.Toggle(0)
	b.transfer(true)
	finishOperation(t, b)
	if len(b.remote.Visible) != 1 {
		t.Fatal("uploaded file missing")
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	b.remote.Toggle(0)
	b.transfer(false)
	finishOperation(t, b)
	data, err := os.ReadFile(source)
	if err != nil || string(data) != "roundtrip" {
		t.Fatalf("%q %v", data, err)
	}
	b.navigateRemote(b.remote.Path)
	finishOperation(t, b)
	b.showSites()
	b.showConnection()
	b.showEncrypt()
	b.goToFolder(b.local)
	b.newFolder(b.local)
	b.signIn()
	b.startOver()
	b.rebuild()
	b.disconnect()
	finishOperation(t, b)
	if b.connected || len(b.remote.Visible) != 0 {
		t.Fatal("disconnect retained remote items")
	}
	b.dropFiles([]string{source})
	if b.busy {
		t.Fatal("disconnected drop started upload")
	}
	b.connectAction()
	finishOperation(t, b)
	if !b.connected {
		t.Fatal("reconnect failed")
	}
	b.dropFiles([]string{source})
	finishOperation(t, b)
	b.disconnect()
	finishOperation(t, b)
}
func TestLocalMutationRefreshesAfterPartialFailure(t *testing.T) {
	b := testBrowser(t)
	file := filepath.Join(b.local.Path, "file")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	b.navigate(b.local, b.local.Path)
	b.mutateFiles(b.local, "Deleting", func() error { return local.Delete(file, false) })
	finishOperation(t, b)
	if len(b.local.Visible) != 0 {
		t.Fatal("deleted file remains visible")
	}
	b.mutateFiles(b.local, "Invalid delete", func() error { return deleteItems(b.local.Path, []sftp.Item{{Path: "/outside"}}, false, nil) })
	finishOperation(t, b)
	if b.status.Get() == "Done" {
		t.Fatal("failure was hidden")
	}
	b.showSites()
}
