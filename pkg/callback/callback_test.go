package callback

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSendReachesTheWaitingProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d", "callback")
	r, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("announcement mode %v", info.Mode().Perm())
	}
	const uri = "dym-bridge://callback?code=abc&state=xyz"
	if err := Send(path, uri); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := r.Next(ctx)
	if err != nil || got != uri {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestSendWithNobodyWaiting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callback")
	if err := Send(path, "dym-bridge://callback?code=1"); err != ErrNobodyWaiting {
		t.Fatalf("no file: %v", err)
	}
	r, _ := Listen(path)
	_ = r.Close()
	if err := Send(path, "dym-bridge://callback?code=1"); err != ErrNobodyWaiting {
		t.Fatalf("closed receiver removes its announcement: %v", err)
	}
}

func TestOtherAddressesAreRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callback")
	r, _ := Listen(path)
	defer func() { _ = r.Close() }()
	if err := Send(path, "https://evil.example/"); err == nil {
		t.Fatal("only dym-bridge addresses are sent")
	}
}

func TestWithoutTheSecretNothingIsAccepted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "callback")
	r, _ := Listen(path)
	defer func() { _ = r.Close() }()
	data, _ := os.ReadFile(path)
	port := strings.Fields(string(data))[0]

	c, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = c.Write([]byte("wrong-secret\ndym-bridge://callback?code=evil\n"))
	_ = c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if got, err := r.Next(ctx); err == nil {
		t.Fatalf("an address without the secret was accepted: %q", got)
	}
}

func TestNextEndsWithTheContext(t *testing.T) {
	r, _ := Listen(filepath.Join(t.TempDir(), "callback"))
	defer func() { _ = r.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Next(ctx); err == nil {
		t.Fatal("a cancelled context ends the wait")
	}
}
