// Package provision sets the application up from a Dymmer account: sign in through the browser (authorization code
// with PKCE, RFC 8252), publish the public key of this device and receive the resources it may reach.
//
// The token Dymmer hands out is for one call only: it is used for the provisioning request and dropped. It is never
// written to disk and no refresh exists.
package provision

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// ClientID is how this application is registered in Dymmer.
	ClientID = "dym-bridge"
	// RedirectURI is the private-use scheme registered with the operating system (RFC 8252, section 7.1).
	RedirectURI = "dym-bridge://callback"
	// Scope is the only scope: the provisioning call.
	Scope = "provision"
)

// Request is the state of one sign-in: it lives in memory until the callback arrives.
type Request struct {
	Verifier  string
	Challenge string
	State     string
}

// NewRequest creates the PKCE verifier (RFC 7636), its S256 challenge and an unguessable state.
func NewRequest() (*Request, error) {
	verifier, err := randomString(32)
	if err != nil {
		return nil, err
	}
	state, err := randomString(16)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(verifier))
	return &Request{
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
		State:     state,
	}, nil
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthorizeURL is the page to open in the system browser.
func (r *Request) AuthorizeURL(server, redirectURI string) string {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {Scope},
		"state":                 {r.State},
		"code_challenge":        {r.Challenge},
		"code_challenge_method": {"S256"},
	}
	return strings.TrimRight(server, "/") + "/oauth/authorize?" + q.Encode()
}

// ErrDenied is returned when the user cancelled in the browser.
var ErrDenied = errors.New("the authorization was cancelled")

// Code reads the callback the browser delivered (the URI the operating system handed to the application) and returns
// the authorization code. The state must be the one of the request: another application that registered the same
// scheme cannot make this one accept its code.
func (r *Request) Code(callback string) (string, error) {
	u, err := url.Parse(callback)
	if err != nil {
		return "", fmt.Errorf("the callback is not a valid address: %w", err)
	}
	q := u.Query()
	if q.Get("state") != r.State {
		return "", errors.New("the callback does not belong to this sign-in (state mismatch)")
	}
	if e := q.Get("error"); e != "" {
		if e == "access_denied" {
			return "", ErrDenied
		}
		return "", fmt.Errorf("Dymmer refused the authorization: %s", e)
	}
	code := q.Get("code")
	if code == "" {
		return "", errors.New("the callback has no code")
	}
	return code, nil
}

// Client talks to a Dymmer server.
type Client struct {
	Server string
	HTTP   *http.Client
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// Token is the one-use token. It is never saved: use it once and let it go.
type Token string

// Exchange trades the authorization code for the token, proving the verifier.
func (c *Client) Exchange(ctx context.Context, r *Request, code, redirectURI string) (Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {ClientID},
		"redirect_uri":  {redirectURI},
		"code_verifier": {r.Verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/oauth/token"), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	status, err := c.do(req, &out)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK || out.AccessToken == "" {
		return "", fmt.Errorf("Dymmer did not give a token (%s): sign in again", reason(out.Error, status))
	}
	return Token(out.AccessToken), nil
}

func (c *Client) url(path string) string { return strings.TrimRight(c.Server, "/") + path }

func reason(code string, status int) string {
	if code != "" {
		return code
	}
	return fmt.Sprintf("HTTP %d", status)
}

func (c *Client) do(req *http.Request, out any) (int, error) {
	resp, err := c.http().Do(req)
	if err != nil {
		return 0, fmt.Errorf("could not reach Dymmer: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if len(body) > 0 {
		_ = json.Unmarshal(body, out) // an error body that is not JSON is reported by its status
	}
	return resp.StatusCode, nil
}
