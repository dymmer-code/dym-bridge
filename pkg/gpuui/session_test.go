package gpuui

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/secret"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"github.com/gogpu/ui/state"
	pkgsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"path/filepath"
	"testing"
)

type transferFake struct {
	calls []string
	fail  bool
}

func (f *transferFake) Upload(a, b string, p sftp.ProgressCallback) error {
	f.calls = append(f.calls, "up:"+a+":"+b)
	if f.fail {
		return errors.New("offline")
	}
	p(4, 4)
	return nil
}
func (f *transferFake) Download(a, b string, p sftp.ProgressCallback) error {
	f.calls = append(f.calls, "down:"+a+":"+b)
	p(4, 4)
	return nil
}
func (f *transferFake) UploadDir(a, b string, start func(string, int64), p sftp.ProgressCallback) error {
	start("child.txt", 4)
	return f.Upload(a, b, p)
}
func (f *transferFake) DownloadDir(a, b string, start func(string, int64), p sftp.ProgressCallback) error {
	start("child.txt", 4)
	return f.Download(a, b, p)
}
func TestTransferSelection(t *testing.T) {
	for _, upload := range []bool{true, false} {
		f := &transferFake{}
		var last transferProgress
		items := []sftp.Item{{Name: "a.txt", Path: "/source/a.txt"}, {Name: "folder", Path: "/source/folder", IsDir: true}}
		err := transferItems(f, upload, items, "/target", func(p transferProgress) { last = p })
		if err != nil || len(f.calls) != 2 || last.Completed != 2 || last.Total != 2 {
			t.Fatalf("%v %v %+v", err, f.calls, last)
		}
		if upload && f.calls[0] != "up:/source/a.txt:/target/a.txt" {
			t.Fatal(f.calls)
		}
		if !upload && f.calls[0] != "down:/source/a.txt:"+filepath.Join("/target", "a.txt") {
			t.Fatal(f.calls)
		}
	}
}
func TestTransferStopsOnError(t *testing.T) {
	f := &transferFake{fail: true}
	err := transferItems(f, true, []sftp.Item{{Name: "a", Path: "/a"}, {Name: "b", Path: "/b"}}, "/target", func(transferProgress) {})
	if err == nil || len(f.calls) != 1 {
		t.Fatalf("%v %v", err, f.calls)
	}
}
func TestCheckedItemsIgnoreFocus(t *testing.T) {
	m := NewDemoModel()
	m.Toggle(0)
	m.Focus(1)
	got := m.CheckedItems()
	if len(got) != 1 || got[0].Path != m.Visible[0].Path {
		t.Fatal(got)
	}
}
func TestEncryptForm(t *testing.T) {
	for _, v := range [][3]string{{"", "a", "a"}, {"secret", "", ""}, {"secret", "a", "b"}} {
		if _, err := encryptValue(v[0], v[1], v[2]); err == nil {
			t.Fatal(v)
		}
	}
	enc, err := encryptValue("hello", "password", "password")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := secret.Decrypt(enc, "password")
	if err != nil || plain != "hello" {
		t.Fatal(plain, err)
	}
}

// The real session and transfer adapters are exercised against a loopback SSH/
// SFTP server, with disposable local directories and a pinned ephemeral host key.
func TestSessionRoundTrip(t *testing.T) {
	cfg := testSFTPServer(t)
	remote := t.TempDir()
	cfg.SFTP.RemotePath = remote
	var s session
	if err := s.connect(cfg); err != nil {
		t.Fatal(err)
	}
	defer s.close()
	source := t.TempDir()
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "folder"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "folder", "hello.txt"), []byte("hello over SFTP"), 0600); err != nil {
		t.Fatal(err)
	}
	items := []sftp.Item{{Name: "folder", Path: filepath.Join(source, "folder"), IsDir: true}}
	if err := transferItems(s.files, true, items, remote, func(transferProgress) {}); err != nil {
		t.Fatal(err)
	}
	listed, err := s.files.List(remote)
	if err != nil || len(listed) != 1 {
		t.Fatalf("%v %v", listed, err)
	}
	if err := transferItems(s.files, false, listed, target, func(transferProgress) {}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(target, "folder", "hello.txt"))
	if err != nil || string(data) != "hello over SFTP" {
		t.Fatalf("%q %v", data, err)
	}
	s.close()
	if s.files != nil || s.manager != nil {
		t.Fatal("connection not cleared")
	}
	if err := s.connect(cfg); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
}
func testSFTPServer(t *testing.T) *config.Config {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	server := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil }}
	server.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				c, chans, reqs, err := ssh.NewServerConn(conn, server)
				if err != nil {
					_ = conn.Close()
					return
				}
				defer c.Close()
				go ssh.DiscardRequests(reqs)
				for incoming := range chans {
					channel, requests, err := incoming.Accept()
					if err != nil {
						return
					}
					go func() {
						defer channel.Close()
						for req := range requests {
							if req.Type == "subsystem" {
								_ = req.Reply(true, nil)
								srv, err := pkgsftp.NewServer(channel)
								if err == nil {
									_ = srv.Serve()
									_ = srv.Close()
								}
								return
							}
							_ = req.Reply(false, nil)
						}
					}()
				}
			}()
		}
	}()
	return &config.Config{Provisioned: true, SSH: config.SSHConfig{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, User: "test", Password: "test", HostKeys: []string{ssh.FingerprintSHA256(signer.PublicKey())}}}
}

func TestTransferActionsFollowCheckboxesAndConnection(t *testing.T) {
	b := &browser{local: NewDemoModel(), remote: NewDemoModel(), uploadLabel: state.NewSignal(""), downloadLabel: state.NewSignal(""), uploadDisabled: state.NewSignal(false), downloadDisabled: state.NewSignal(false)}
	b.local.Toggle(0)
	b.remote.Toggle(1)
	b.updateActions()
	if !b.uploadDisabled.Get() || !b.downloadDisabled.Get() {
		t.Fatal("disconnected transfers enabled")
	}
	b.connected = true
	b.updateActions()
	if b.uploadDisabled.Get() || b.downloadDisabled.Get() || b.uploadLabel.Get() != "Upload (1)" || b.downloadLabel.Get() != "Download (1)" {
		t.Fatal("checked items not reflected")
	}
	b.busy = true
	b.updateActions()
	if !b.uploadDisabled.Get() || !b.downloadDisabled.Get() {
		t.Fatal("concurrent transfer enabled")
	}
	b.busy = false
	b.local.Toggle(0)
	b.updateActions()
	if !b.uploadDisabled.Get() || b.downloadDisabled.Get() {
		t.Fatal("selection removal not reflected")
	}
}
func TestFailedConnectionClearsSession(t *testing.T) {
	cfg := testSFTPServer(t)
	cfg.SSH.HostKeys = []string{"SHA256:wrong-key"}
	var s session
	if err := s.connect(cfg); err == nil {
		t.Fatal("unexpected connection")
	}
	if s.manager != nil || s.files != nil {
		t.Fatal("failed session retained")
	}
}
