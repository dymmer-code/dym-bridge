package provision

import (
	"context"
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

	"golang.org/x/crypto/ssh"
)

func TestRequestChallengeIsS256OfTheVerifier(t *testing.T) {
	r, err := NewRequest()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(r.Verifier))
	if r.Challenge != base64.RawURLEncoding.EncodeToString(sum[:]) {
		t.Fatal("the challenge is not the S256 of the verifier")
	}
	if len(r.Verifier) < 43 || len(r.Verifier) > 128 {
		t.Fatalf("verifier length %d outside RFC 7636", len(r.Verifier))
	}
	other, _ := NewRequest()
	if other.State == r.State || other.Verifier == r.Verifier {
		t.Fatal("requests must not repeat")
	}
}

func TestAuthorizeURL(t *testing.T) {
	r, _ := NewRequest()
	u, err := url.Parse(r.AuthorizeURL("https://dymmer.example/", RedirectURI))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Path != "/oauth/authorize" || q.Get("client_id") != "dym-bridge" || q.Get("redirect_uri") != "dym-bridge://callback" ||
		q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") != r.Challenge || q.Get("state") != r.State ||
		q.Get("response_type") != "code" || q.Get("scope") != "provision" {
		t.Fatalf("unexpected URL %s", u)
	}
}

func TestCode(t *testing.T) {
	r, _ := NewRequest()
	cb := func(extra string) string { return "dym-bridge://callback?" + extra }

	if code, err := r.Code(cb("code=abc&state=" + r.State)); err != nil || code != "abc" {
		t.Fatalf("got %q %v", code, err)
	}
	if _, err := r.Code(cb("code=abc&state=other")); err == nil || !strings.Contains(err.Error(), "state") {
		t.Fatalf("a code from another sign-in must be refused: %v", err)
	}
	if _, err := r.Code(cb("code=abc")); err == nil {
		t.Fatal("no state must be refused")
	}
	if _, err := r.Code(cb("error=access_denied&state=" + r.State)); err != ErrDenied {
		t.Fatalf("cancel must be ErrDenied, got %v", err)
	}
	if _, err := r.Code(cb("error=invalid_scope&state=" + r.State)); err == nil || err == ErrDenied {
		t.Fatalf("other errors are reported: %v", err)
	}
	if _, err := r.Code(cb("state=" + r.State)); err == nil {
		t.Fatal("a callback without code must fail")
	}
}

// fakeDymmer behaves like the endpoints of Dymmer (backlog dymmer #328/#329).
func fakeDymmer(t *testing.T, wantVerifier *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_ = r.ParseForm()
			if r.Form.Get("code") != "good" || r.Form.Get("code_verifier") != *wantVerifier || r.Form.Get("client_id") != "dym-bridge" {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer"}`))
		case "/oauth/provision":
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
				return
			}
			var in DeviceInfo
			_ = json.NewDecoder(r.Body).Decode(&in)
			if !strings.HasPrefix(in.PublicKey, "ssh-ed25519 ") {
				w.WriteHeader(422)
				_, _ = w.Write([]byte(`{"error":"invalid_public_key"}`))
				return
			}
			_, _ = w.Write([]byte(`{"version":1,"device":{"id":"d1","name":"` + in.DeviceName + `","fingerprint":"SHA256:x"},
			"resources":[{"delegation_id":"g1","host_id":"h1","name":"web","address":"ssh.example.com","port":2202,
			"user":"client","path":"/sites","expires_at":null,"host_keys":[{"fingerprint":"SHA256:abc","public_key":"ssh-ed25519 AAAA"}],
			"tunnels":[{"host":"127.0.0.1","port":8080}]}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestExchangeAndProvisionHappyPath(t *testing.T) {
	req, _ := NewRequest()
	srv := fakeDymmer(t, &req.Verifier)
	defer srv.Close()
	c := &Client{Server: srv.URL}

	token, err := c.Exchange(context.Background(), req, "good", RedirectURI)
	if err != nil || token != "tok" {
		t.Fatalf("exchange: %q %v", token, err)
	}
	key, err := GenerateDeviceKey("laptop", "master")
	if err != nil {
		t.Fatal(err)
	}
	file, err := c.Provision(context.Background(), token, DeviceInfo{PublicKey: key.PublicKey, DeviceName: "Laptop", System: "macOS"})
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Resources) != 1 || file.Resources[0].Address != "ssh.example.com" || file.Resources[0].Port != 2202 ||
		file.Resources[0].HostKeys[0].Fingerprint != "SHA256:abc" || file.Resources[0].Tunnels[0].Port != 8080 {
		t.Fatalf("unexpected file %+v", file)
	}
	if file.Resources[0].Label() != "web · client" {
		t.Fatalf("label %q", file.Resources[0].Label())
	}
}

func TestExchangeRefusals(t *testing.T) {
	req, _ := NewRequest()
	srv := fakeDymmer(t, &req.Verifier)
	defer srv.Close()
	c := &Client{Server: srv.URL}

	if _, err := c.Exchange(context.Background(), req, "bad", RedirectURI); err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("a bad code must fail with the reason: %v", err)
	}
	wrong := &Request{Verifier: "w" + req.Verifier}
	if _, err := c.Exchange(context.Background(), wrong, "good", RedirectURI); err == nil {
		t.Fatal("a wrong verifier must fail")
	}
}

func TestProvisionRefusals(t *testing.T) {
	req, _ := NewRequest()
	srv := fakeDymmer(t, &req.Verifier)
	defer srv.Close()
	c := &Client{Server: srv.URL}

	if _, err := c.Provision(context.Background(), "spent", DeviceInfo{PublicKey: "ssh-ed25519 AAAA"}); err == nil || !strings.Contains(err.Error(), "sign in again") {
		t.Fatalf("a spent token asks to sign in again: %v", err)
	}
	if _, err := c.Provision(context.Background(), "tok", DeviceInfo{PublicKey: "nope"}); err == nil || !strings.Contains(err.Error(), "invalid_public_key") {
		t.Fatalf("the reason is reported: %v", err)
	}
}

func TestProvisionRejectsUnknownFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":2,"device":{},"resources":[]}`))
	}))
	defer srv.Close()
	c := &Client{Server: srv.URL}
	if _, err := c.Provision(context.Background(), "tok", DeviceInfo{}); err == nil || !strings.Contains(err.Error(), "update") {
		t.Fatalf("a newer format asks to update: %v", err)
	}
}

func TestUnreachableServer(t *testing.T) {
	c := &Client{Server: "http://127.0.0.1:1"}
	if _, err := c.Exchange(context.Background(), &Request{}, "x", RedirectURI); err == nil || !strings.Contains(err.Error(), "could not reach") {
		t.Fatalf("got %v", err)
	}
}

func TestDeviceKey(t *testing.T) {
	key, err := GenerateDeviceKey("laptop", "master")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key.PublicKey, "ssh-ed25519 ") || strings.ContainsAny(key.PublicKey, "\r\n") {
		t.Fatalf("public key %q", key.PublicKey)
	}
	if !strings.HasPrefix(key.Private, "ENC(") || strings.Contains(key.Private, "PRIVATE KEY") {
		t.Fatal("the private key must be stored encrypted, never in clear")
	}
	signer, err := key.Signer("master")
	if err != nil {
		t.Fatal(err)
	}
	if string(ssh.MarshalAuthorizedKey(signer.PublicKey())) != key.PublicKey+"\n" {
		t.Fatal("the private key does not match the published one")
	}
	if _, err := key.Signer("wrong"); err == nil {
		t.Fatal("a wrong master password must fail")
	}
	if _, err := GenerateDeviceKey("x", ""); err == nil {
		t.Fatal("an empty master password must be refused")
	}
	again, _ := GenerateDeviceKey("laptop", "master")
	if again.PublicKey == key.PublicKey {
		t.Fatal("each device gets its own key")
	}
}

func TestHostKeyCallback(t *testing.T) {
	key, _ := GenerateDeviceKey("h", "m")
	signer, _ := key.Signer("m")
	good := []HostKey{{Fingerprint: ssh.FingerprintSHA256(signer.PublicKey())}}

	if err := HostKeyCallback(good)("h", nil, signer.PublicKey()); err != nil {
		t.Fatalf("the reported key must be accepted: %v", err)
	}
	other, _ := GenerateDeviceKey("o", "m")
	otherSigner, _ := other.Signer("m")
	if err := HostKeyCallback(good)("h", nil, otherSigner.PublicKey()); err == nil || !strings.Contains(err.Error(), "not the one Dymmer reported") {
		t.Fatalf("an unknown key must be refused: %v", err)
	}
	if err := HostKeyCallback(nil)("h", nil, signer.PublicKey()); err == nil {
		t.Fatal("no reported keys means nothing is trusted")
	}
}

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "profiles.json")
	if _, err := Load(path); err != ErrNoProfiles {
		t.Fatalf("missing file: %v", err)
	}
	key, _ := GenerateDeviceKey("l", "m")
	p := &Profiles{Server: "https://dymmer.example", Key: *key}
	p.Merge(&File{Device: Device{ID: "d1"}, Resources: []Resource{{Name: "web", Address: "a", Port: 22, User: "u"}}})
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "PRIVATE KEY") || strings.Contains(string(data), "access_token") {
		t.Fatal("neither a clear private key nor a token may be on disk")
	}
	got, err := Load(path)
	if err != nil || got.Device.ID != "d1" || got.Resources[0].Name != "web" || got.Key.PublicKey != key.PublicKey {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || err == ErrNoProfiles {
		t.Fatalf("a damaged file is an error: %v", err)
	}
}

func TestProfilesConfig(t *testing.T) {
	key, _ := GenerateDeviceKey("l", "m")
	p := &Profiles{Key: *key, Resources: []Resource{
		{Name: "web", Address: "ssh.example.com", Port: 2202, User: "client", Path: "/sites", HostKeys: []HostKey{{Fingerprint: "SHA256:abc"}}},
		{Name: "nokeys", Address: "x", User: "u"},
	}}
	cfg, err := p.Config(0)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SSH.Host != "ssh.example.com" || cfg.SSH.Port != 2202 || cfg.SSH.User != "client" || cfg.SFTP.RemotePath != "/sites" ||
		len(cfg.SSH.HostKeys) != 1 || !cfg.Provisioned || !cfg.HasEncryptedCredentials() {
		t.Fatalf("unexpected config %+v", cfg)
	}
	if err := cfg.Unlock("m"); err != nil || !strings.Contains(cfg.SSH.KeyData, "PRIVATE KEY") {
		t.Fatalf("the master password opens the key in memory: %v", err)
	}
	if _, err := p.Config(1); err == nil {
		t.Fatal("a resource without a server key is refused: nothing is trusted blindly")
	}
	if _, err := p.Config(5); err == nil {
		t.Fatal("an unknown resource is an error")
	}
}

type fakeReceiver struct{ uri chan string }

func (f fakeReceiver) Next(ctx context.Context) (string, error) {
	select {
	case u := <-f.uri:
		return u, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// browser simulates the user approving: it reads the state from the address that was opened and "calls back".
func browser(rcv fakeReceiver, code string, tamper bool) func(string) error {
	return func(authURL string) error {
		u, _ := url.Parse(authURL)
		state := u.Query().Get("state")
		if tamper {
			state = "other"
		}
		rcv.uri <- "dym-bridge://callback?code=" + code + "&state=" + state
		return nil
	}
}

func TestLoginEndToEnd(t *testing.T) {
	var verifier string
	srv := fakeDymmerCapturingVerifier(t, &verifier)
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "p", "profiles.json")
	rcv := fakeReceiver{uri: make(chan string, 1)}

	opts := LoginOptions{Server: srv.URL, ProfilesPath: path, MasterPassword: "master", DeviceName: "Laptop",
		Open: browser(rcv, "good", false), Receiver: rcv}
	p, err := Login(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Resources) != 1 || p.Device.ID != "d1" || !strings.HasPrefix(p.Key.PublicKey, "ssh-ed25519 ") {
		t.Fatalf("unexpected %+v", p)
	}
	saved, err := Load(path)
	if err != nil || saved.Resources[0].Address != "ssh.example.com" {
		t.Fatalf("not saved: %v", err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), `"tok"`) || strings.Contains(string(data), "PRIVATE KEY") {
		t.Fatal("no token and no clear private key on disk")
	}

	// Signing in again keeps the same device key (one key per device) and needs the same master password.
	first := p.Key.PublicKey
	rcv2 := fakeReceiver{uri: make(chan string, 1)}
	opts.Open, opts.Receiver = browser(rcv2, "good", false), rcv2
	p2, err := Login(context.Background(), opts)
	if err != nil || p2.Key.PublicKey != first {
		t.Fatalf("the key must be kept: %v", err)
	}
	opts.MasterPassword = "wrong"
	if _, err := Login(context.Background(), opts); err == nil {
		t.Fatal("a wrong master password must stop before anything is sent")
	}
}

func TestLoginRefusesAForeignCallback(t *testing.T) {
	var verifier string
	srv := fakeDymmerCapturingVerifier(t, &verifier)
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "profiles.json")
	rcv := fakeReceiver{uri: make(chan string, 1)}
	_, err := Login(context.Background(), LoginOptions{Server: srv.URL, ProfilesPath: path, MasterPassword: "m",
		Open: browser(rcv, "good", true), Receiver: rcv})
	if err == nil || !strings.Contains(err.Error(), "state") {
		t.Fatalf("got %v", err)
	}
	if _, err := Load(path); err != ErrNoProfiles {
		t.Fatal("nothing may be saved after a refused callback")
	}
}

func TestLoginTimesOutWithoutTheBrowser(t *testing.T) {
	rcv := fakeReceiver{uri: make(chan string)}
	_, err := Login(context.Background(), LoginOptions{Server: "http://127.0.0.1:1", ProfilesPath: filepath.Join(t.TempDir(), "p.json"),
		MasterPassword: "m", Open: func(string) error { return nil }, Receiver: rcv, Timeout: 100 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "no answer") {
		t.Fatalf("got %v", err)
	}
}

func TestLoginCancelledByTheUser(t *testing.T) {
	var verifier string
	srv := fakeDymmerCapturingVerifier(t, &verifier)
	defer srv.Close()
	rcv := fakeReceiver{uri: make(chan string, 1)}
	open := func(authURL string) error {
		u, _ := url.Parse(authURL)
		rcv.uri <- "dym-bridge://callback?error=access_denied&state=" + u.Query().Get("state")
		return nil
	}
	_, err := Login(context.Background(), LoginOptions{Server: srv.URL, ProfilesPath: filepath.Join(t.TempDir(), "p.json"),
		MasterPassword: "m", Open: open, Receiver: rcv})
	if err != ErrDenied {
		t.Fatalf("got %v", err)
	}
}

// fakeDymmerCapturingVerifier is the fake server, but it learns the verifier from the challenge in the authorization
// request like the real one: it checks S256(verifier) == challenge it was shown. Here the authorize step is the
// browser fake, so the verifier is checked by length and format only.
func fakeDymmerCapturingVerifier(t *testing.T, verifier *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_ = r.ParseForm()
			v := r.Form.Get("code_verifier")
			if r.Form.Get("code") != "good" || len(v) < 43 || r.Form.Get("redirect_uri") != RedirectURI {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			*verifier = v
			_, _ = w.Write([]byte(`{"access_token":"tok"}`))
		case "/oauth/provision":
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(401)
				return
			}
			_, _ = w.Write([]byte(`{"version":1,"device":{"id":"d1","name":"Laptop","fingerprint":"SHA256:x"},"resources":[{"delegation_id":"g1","host_id":"h1","name":"web","address":"ssh.example.com","port":2202,"user":"client","path":"/sites","host_keys":[{"fingerprint":"SHA256:abc"}],"tunnels":[]}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
}
