package gpuui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dymmer-code/dym-bridge/pkg/callback"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"github.com/dymmer-code/dym-bridge/pkg/secret"
	p "github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/widget"
)

const DefaultServer = "https://dymmer.com"

func (b *browser) serverURL() string {
	if b.server != "" {
		return b.server
	}
	return DefaultServer
}
func validateSignInPassword(password, confirm string, first bool) error {
	if first && password != confirm {
		return fmt.Errorf("The passwords do not match")
	}
	return nil
}
func (b *browser) signIn() {
	if b.busy {
		return
	}
	filename, err := b.profilePath()
	if err != nil {
		b.status.Set(err.Error())
		return
	}
	profiles, loadErr := provision.Load(filename)
	first := errors.Is(loadErr, provision.ErrNoProfiles) || (loadErr == nil && !secret.IsEncrypted(profiles.Key.Private))
	password := b.field("Master password", true)
	confirm := b.field("Repeat master password", true)

	message := state.NewSignal("")
	label := "Master password (optional)"
	help := "The master password protects the private SSH key used to connect to your server. Leave it blank to store the key unencrypted on this computer."
	if !first && loadErr == nil {
		label = "Master password"
		help = "The master password protects your private SSH key. This device’s key is already encrypted, so enter its existing password."
	}
	children := []widget.Widget{b.text(label, 12), password, b.helpText(help)}
	if first {
		children = append(children, b.text("Repeat master password", 12), confirm)
	}
	children = append(children,
		p.TextFn(func() string { return message.Get() }).FontSize(12).MaxLines(3),
		b.action("Continue in browser", true, func() {
			if err := validateSignInPassword(password.Text(), confirm.Text(), first); err != nil {
				message.Set(err.Error())
				return
			}
			master := password.Text()
			password.SetText("")
			confirm.SetText("")
			b.runSignIn(master)
		}), b.action("Forgot password? Start over…", false, b.startOver))
	b.showForm("Sign in with Dymmer", children...)
}
func (b *browser) runSignIn(master string) {
	filename, err := b.profilePath()
	if err != nil {
		b.status.Set(err.Error())
		return
	}
	cbPath, err := b.callbackPath()
	if err != nil {
		b.status.Set(err.Error())
		return
	}
	receiver, err := callback.Listen(cbPath)
	if err != nil {
		b.status.Set(err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	b.loginCancel = cancel
	var mu sync.Mutex
	authURL := ""
	current := func() string { mu.Lock(); defer mu.Unlock(); return authURL }
	b.showFormWithBack("Waiting for the browser", cancel,
		b.text("Approve this computer in your browser, then come back.", 13),
		p.HBox(b.action("Open again", false, func() {
			if u := current(); u != "" {
				_ = b.openBrowser(u)
			}
		}), b.action("Copy link", false, func() {
			if u := current(); u != "" {
				_ = b.gpu.ClipboardWrite(u)
			}
		}), b.action("Cancel", false, cancel)).Gap(8))
	b.busy = true
	server := b.serverURL()
	b.workers.Add(1)
	go func() {
		defer b.workers.Done()
		profiles, err := provision.Login(ctx, provision.LoginOptions{Server: server, ProfilesPath: filename, MasterPassword: master, Receiver: receiver, Open: func(u string) error { mu.Lock(); authURL = u; mu.Unlock(); return b.openBrowser(u) }})
		_ = receiver.Close()
		cancel()
		b.post(func() {
			b.busy = false
			b.loginCancel = nil
			if err != nil {
				b.status.Set("Sign-in failed: " + err.Error())
				b.rebuild()
				return
			}
			b.profiles = profiles
			b.status.Set(fmt.Sprintf("Signed in · %d resources", len(profiles.Resources)))
			b.showConnection()
		})
	}()
}
func (b *browser) startOver() {
	if b.busy {
		return
	}
	b.showForm("Start over", b.text("Forget this computer’s saved key and resources?", 14), b.text("The old public key remains in Dymmer until you revoke it there.", 12), b.action("Forget and sign in again", true, func() {
		filename, err := b.profilePath()
		if err == nil {
			err = os.Remove(filename)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			b.status.Set(err.Error())
			b.rebuild()
			return
		}
		b.session.close()
		b.connected = false
		b.cfg = nil
		b.profiles = nil
		b.siteLinks = nil
		b.manualSites = nil
		b.remote.Items = nil
		b.remote.Filter("")
		b.signIn()
	}))
}

func (b *browser) profilePath() (string, error) {
	if b.profilesFile != "" {
		return b.profilesFile, nil
	}
	return provision.DefaultPath()
}
func (b *browser) callbackPath() (string, error) {
	if b.callbackFile != "" {
		return b.callbackFile, nil
	}
	return callback.DefaultPath()
}
func (b *browser) openBrowser(uri string) error {
	if b.openURL != nil {
		return b.openURL(uri)
	}
	return provision.OpenBrowser(uri)
}
