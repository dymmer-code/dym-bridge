package gpuui

import (
	"fmt"
	"path"
	"strings"

	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/local"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"github.com/gogpu/ui/core/scrollview"
	"github.com/gogpu/ui/core/textfield"
	p "github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/theme/fluent"
	"github.com/gogpu/ui/widget"
)

// All widget mutations run on the UI thread. Progress is bounded to one latest
// value, so fast SFTP callbacks cannot flood the event queue or retain files.
func (b *browser) drainUpdates() {
	select {
	case p := <-b.progress:
		b.transferText.Set(formatProgress(p))
	default:
	}
	select {
	case fn := <-b.updates:
		fn()
	default:
	}
}
func formatProgress(p transferProgress) string {
	return fmt.Sprintf("%d/%d items · %s · %s / %s", p.Completed, p.Total, p.Name, sftp.FormatFileSize(p.Bytes), sftp.FormatFileSize(p.Size))
}
func (b *browser) reportProgress(p transferProgress) {
	select {
	case <-b.progress:
	default:
	}
	b.progress <- p
	b.gpu.RequestRedraw()
}
func (b *browser) runOperation(message string, work func() error, done func(error)) {
	if b.busy {
		return
	}
	b.busy = true
	b.status.Set(message)
	b.rebuild()
	b.workers.Add(1)
	go func() {
		defer b.workers.Done()
		err := work()
		b.post(func() {
			// A final progress callback can race the frame that receives completion.
			// Discard it before publishing the terminal status.
			select {
			case <-b.progress:
			default:
			}
			b.busy = false
			done(err)
			b.rebuild()
		})
	}()
}
func (b *browser) connectAction() {
	if b.busy {
		return
	}
	if b.cfg == nil {
		b.showConnection()
		return
	}
	b.connect(b.cfg)
}
func (b *browser) connect(cfg *config.Config) {
	if b.busy {
		return
	}
	if cfg.HasEncryptedCredentials() {
		b.showUnlock(cfg)
		return
	}
	b.connectionEpoch++
	epoch := b.connectionEpoch
	// Keep configuration immutable for the worker and for later reconnects.
	copyCfg := *cfg
	var items []sftp.Item
	b.connected = false
	b.remote.Items = nil
	b.remote.Filter("")
	b.runOperation("Establishing SSH connection…", func() error {
		if err := b.session.connect(&copyCfg); err != nil {
			return err
		}
		var err error
		items, err = b.session.files.List(copyCfg.SFTP.RemotePath)
		if err != nil {
			b.session.close()
		}
		return err
	}, func(err error) {
		b.cfg = &copyCfg
		b.loadSites()
		if err != nil {
			b.status.Set("Connection failed: " + err.Error())
			return
		}
		b.connected = true
		client := b.session.manager.SSHClient()
		go func() {
			err := client.Wait()
			b.post(func() {
				if b.connectionEpoch != epoch || !b.connected {
					return
				}
				b.connected = false
				b.remote.Items = nil
				b.remote.Filter("")
				message := "Connection ended"
				if err != nil {
					message += " · " + err.Error()
				}
				b.status.Set(message)
				b.rebuild()
			})
		}()
		b.remote.Path = copyCfg.SFTP.RemotePath
		b.remote.Items = items
		b.remote.Filter("")
		b.status.Set("SSH connection active and secure")
		if copyCfg.WordPress.HostHeader != "" {
			b.status.Set("Tunnel active and secure")
		}
	})
}
func (b *browser) disconnect() {
	if b.busy {
		return
	}
	b.connectionEpoch++
	b.runOperation("Disconnecting…", func() error { b.session.close(); return nil }, func(error) {
		b.connected = false
		b.remote.Items = nil
		b.remote.Filter("")
		b.status.Set("Disconnected")
	})
}
func (b *browser) navigateRemote(dir string) {
	if !b.connected || b.busy {
		return
	}
	root := path.Clean(b.cfg.SFTP.RemotePath)
	dir = path.Clean(dir)
	if dir != root && !strings.HasPrefix(dir, strings.TrimSuffix(root, "/")+"/") {
		dir = root
	}
	var items []sftp.Item
	b.runOperation("Opening folder…", func() error { var err error; items, err = b.session.files.List(dir); return err }, func(err error) {
		if err != nil {
			b.status.Set("Could not open folder: " + err.Error())
			return
		}
		b.remote.Path = dir
		b.remote.Items = items
		b.remote.Filter("")
		b.status.Set("Folder opened · " + dir)
	})
}
func (b *browser) transfer(upload bool) {
	if b.busy || !b.connected {
		return
	}
	source := b.local
	if !upload {
		source = b.remote
	}
	items := source.CheckedItems()
	if len(items) == 0 {
		return
	}
	b.transferSelection(upload, items)
}
func (b *browser) transferSelection(upload bool, items []sftp.Item) {
	if len(items) == 0 || b.busy || !b.connected {
		return
	}
	source, target, verb := b.local, b.remote.Path, "Uploading"
	if !upload {
		source, target, verb = b.remote, b.local.Path, "Downloading"
	}
	localPath, remotePath := b.local.Path, b.remote.Path
	var localItems, remoteItems []sftp.Item
	var refreshErr error
	b.transfersOpen = true
	b.transferText.Set(fmt.Sprintf("%s %d items…", verb, len(items)))
	b.runOperation(verb+"…", func() error {
		err := transferItems(b.session.files, upload, items, target, b.reportProgress)
		localItems, refreshErr = local.ListDirectory(localPath)
		var remoteErr error
		remoteItems, remoteErr = b.session.files.List(remotePath)
		if refreshErr == nil {
			refreshErr = remoteErr
		}
		return err
	}, func(err error) {
		// Refresh even after partial failure, but retain the selection for retry.
		if refreshErr == nil {
			b.local.Items = localItems
			b.local.Visible = localItems
			b.remote.Items = remoteItems
			b.remote.Visible = remoteItems
		}
		if err != nil {
			b.status.Set("Transfer failed: " + err.Error())
			b.transferText.Set("Failed · " + err.Error())
			return
		}
		source.selectedPaths = nil
		message := fmt.Sprintf("Transfer complete · %d items", len(items))
		b.status.Set(message)
		b.transferText.Set(message)
		if refreshErr != nil {
			b.status.Set(message + " · Refresh failed: " + refreshErr.Error())
		}
	})
}
func (b *browser) field(label string, password bool) *textfield.Widget {
	opts := []textfield.Option{textfield.Placeholder(label), textfield.A11yLabel(label), textfield.PainterOpt(fluent.TextFieldPainter{Theme: b.theme})}
	if password {
		opts = append(opts, textfield.InputTypeOpt(textfield.TypePassword))
	}
	return textfield.New(opts...)
}

// Forms use the regular focus-aware widget tree: the library's current dialog
// surface only dispatches events to its action buttons, not to editable content.
func (b *browser) showForm(title string, children ...widget.Widget) {
	b.showFormWithBack(title, b.rebuild, children...)
}
func (b *browser) showFormWithBack(title string, back func(), children ...widget.Widget) {
	if b.busy {
		return
	}
	content := []widget.Widget{p.HBox(b.text(title, 18).Bold(), p.Expanded(p.Box()), b.iconAction("Back", backIcon, back)).CrossAlign(p.CrossAxisCenter), b.rule()}
	content = append(content, children...)
	form := p.VBox(content...).Gap(12).Padding(24)
	b.ui.SetRoot(p.VBox(p.Expanded(scrollview.New(p.HBox(p.Expanded(p.Box()), p.Box(form).Width(600), p.Expanded(p.Box()))))).Background(b.theme.Colors.Surface))
	b.gpu.RequestRedraw()
}
func (b *browser) showConnection() {
	if b.busy {
		return
	}
	message := state.NewSignal("")
	children := []widget.Widget{}
	if filename, err := b.profilePath(); err == nil {
		if profiles, err := provision.Load(filename); err == nil {
			b.profiles = profiles
			for i, r := range profiles.Resources {
				children = append(children, b.iconAction(r.Label(), connectIcon, func() {
					cfg, err := profiles.Config(i)
					if err != nil {
						message.Set(err.Error())
						return
					}
					b.connect(cfg)
				}))
			}
		}
	}
	if b.configPath != "" {
		if cfg, err := config.Load(b.configPath); err == nil {
			children = append(children, b.iconAction(cfg.SSH.Host, connectIcon, func() { b.connect(cfg) }))
		} else {
			message.Set("Could not load the configuration. Choose a valid INI file.")
		}
	}
	if len(children) > 0 {
		children = append([]widget.Widget{b.text("Saved resources", 13)}, children...)
	} else {
		children = append(children, b.action("Choose INI file…", true, b.browseINI), b.action("Create INI file…", false, b.saveSampleINI),
			b.helpText("Dymmer can automatically set up your key and connection resources."),
			b.action("Sign in with Dymmer", false, b.signIn))
	}
	children = append(children, p.TextFn(func() string { return message.Get() }).FontSize(12).Color(b.theme.Colors.OnSurfaceSecond).MaxLines(2))
	b.showForm("Connection", children...)
}
func (b *browser) showUnlock(cfg *config.Config) {
	password := b.field("Master password", true)
	message := state.NewSignal("Unlock credentials in memory to connect.")
	b.showForm("Unlock connection", b.text("Master password", 12), password, p.TextFn(func() string { return message.Get() }).FontSize(12), b.action("Unlock and connect", true, func() {
		unlocked := *cfg
		if err := unlocked.Unlock(password.Text()); err != nil {
			message.Set("Incorrect master password or damaged credentials.")
			return
		}
		password.SetText("")
		b.connect(&unlocked)
	}))
}
