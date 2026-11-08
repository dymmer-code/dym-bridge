package gpuui

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dymmer-code/dym-bridge/pkg/callback"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/state"
)

func TestDesktopSignInPublishesPublicKeyAndKeepsEncryptedProfile(t *testing.T) {
	b := &browser{profilesFile: filepath.Join(t.TempDir(), "profiles.json"), callbackFile: filepath.Join(t.TempDir(), "callback"), updates: make(chan func(), 16), stopping: make(chan struct{}), status: state.NewSignal(""), gpu: gogpu.NewApp(gogpu.DefaultConfig())}
	b.setTheme()
	b.ui = app.New(app.WithTheme(b.theme.AsTheme()))
	defer b.ui.Window().Close()
	var challenge string
	var published provision.DeviceInfo
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_ = r.ParseForm()
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if r.Form.Get("code") != "test-code" || base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
				http.Error(w, "invalid PKCE", 400)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"ephemeral-test-token","token_type":"Bearer"}`))
		case "/oauth/provision":
			if r.Header.Get("Authorization") != "Bearer ephemeral-test-token" {
				http.Error(w, "unauthorized", 401)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&published); err != nil {
				http.Error(w, "invalid device", 400)
				return
			}
			if !strings.HasPrefix(published.PublicKey, "ssh-ed25519 ") {
				http.Error(w, "missing public key", 422)
				return
			}
			_, _ = w.Write([]byte(`{"version":1,"device":{"id":"test-device"},"resources":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	b.server = srv.URL
	b.openURL = func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		challenge = u.Query().Get("code_challenge")
		return callback.Send(b.callbackFile, "dym-bridge://callback?code=test-code&state="+url.QueryEscape(u.Query().Get("state")))
	}
	b.runSignIn("test-master-password")
	if !b.busy {
		t.Fatal("sign-in was not started")
	}
	select {
	case done := <-b.updates:
		done()
	case <-time.After(10 * time.Second):
		t.Fatal("sign-in did not complete")
	}
	b.workers.Wait()
	if b.busy || b.profiles == nil || b.profiles.Device.ID != "test-device" {
		t.Fatalf("sign-in failed: %s", b.status.Get())
	}
	if b.profiles.Key.PublicKey != published.PublicKey {
		t.Fatal("different device key published")
	}
	if _, err := b.profiles.Key.Signer("test-master-password"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(b.profilesFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"ephemeral-test-token", "PRIVATE KEY", "test-master-password"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("persisted secret %s", secret)
		}
	}
	if _, err := os.Stat(b.callbackFile); !os.IsNotExist(err) {
		t.Fatal("callback listener was not cleaned up")
	}
}
