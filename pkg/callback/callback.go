// Package callback hands the address that the browser opened (dym-bridge://callback?code=...) from the process the
// operating system started for it to the process that is waiting for the sign-in. Only one dym-bridge runs at a time
// (see pkg/lock), so the second one, started by the system with the address as its argument, passes it on and exits.
package callback

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Receiver waits for callbacks on the loopback interface. The port and a random secret are in a file that only the
// user can read, so only a process of the same user can deliver an address.
type Receiver struct {
	ln    net.Listener
	path  string
	nonce string
	uris  chan string
}

// DefaultPath is the file where a waiting process announces itself.
func DefaultPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dym-bridge", "callback"), nil
}

// Listen starts waiting and announces itself in the file at path.
func Listen(path string) (*Receiver, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	nb := make([]byte, 16)
	if _, err := rand.Read(nb); err != nil {
		_ = ln.Close()
		return nil, err
	}
	r := &Receiver{ln: ln, path: path, nonce: hex.EncodeToString(nb), uris: make(chan string, 4)}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		_ = ln.Close()
		return nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d\n%s\n", port, r.nonce)), 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	go r.serve()
	return r, nil
}

func (r *Receiver) serve() {
	for {
		c, err := r.ln.Accept()
		if err != nil {
			return
		}
		go r.handle(c)
	}
}

func (r *Receiver) handle(c net.Conn) {
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 0, 4096), 16<<10)
	if !sc.Scan() {
		return
	}
	given := sc.Text()
	if subtle.ConstantTimeCompare([]byte(given), []byte(r.nonce)) != 1 || !sc.Scan() {
		return
	}
	uri := strings.TrimSpace(sc.Text())
	if !strings.HasPrefix(uri, "dym-bridge://") {
		return
	}
	select {
	case r.uris <- uri:
		_, _ = c.Write([]byte("ok\n"))
	default:
	}
}

// Next waits for the next address delivered, or for the context to end.
func (r *Receiver) Next(ctx context.Context) (string, error) {
	select {
	case uri := <-r.uris:
		return uri, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Close stops waiting and removes the announcement.
func (r *Receiver) Close() error {
	_ = os.Remove(r.path)
	return r.ln.Close()
}

// ErrNobodyWaiting means no sign-in is in progress.
var ErrNobodyWaiting = errors.New("no sign-in is waiting for this address: start it from dym-bridge")

// Send delivers the address to the process that is waiting.
func Send(path, uri string) error {
	if !strings.HasPrefix(uri, "dym-bridge://") {
		return fmt.Errorf("%q is not an address of dym-bridge", uri)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ErrNobodyWaiting
	}
	parts := strings.Fields(string(data))
	if len(parts) != 2 {
		return ErrNobodyWaiting
	}
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+parts[0], 3*time.Second)
	if err != nil {
		return ErrNobodyWaiting
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprintf(c, "%s\n%s\n", parts[1], uri); err != nil {
		return err
	}
	answer, _ := bufio.NewReader(c).ReadString('\n')
	if strings.TrimSpace(answer) != "ok" {
		return errors.New("the waiting dym-bridge did not accept the address")
	}
	return nil
}
