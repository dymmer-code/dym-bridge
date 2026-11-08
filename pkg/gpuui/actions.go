package gpuui

import (
	"fmt"
	"path"
	"strings"

	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/local"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"github.com/gogpu/ui/core/button"
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
		b.updates <- func() {
			// A final progress callback can race the frame that receives completion.
			// Discard it before publishing the terminal status.
			select {
			case <-b.progress:
			default:
			}
			b.busy = false
			done(err)
			b.rebuild()
		}
		b.gpu.RequestRedraw()
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
	source, target := b.local, b.remote.Path
	verb := "Uploading"
	if !upload {
		source, target = b.remote, b.local.Path
		verb = "Downloading"
	}
	items := source.CheckedItems()
	if len(items) == 0 {
		return
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
	content := []widget.Widget{p.HBox(b.text(title, 18).Bold(), p.Expanded(p.Box()), b.action("Back", false, back)).CrossAlign(p.CrossAxisCenter), b.rule()}
	content = append(content, children...)
	form := p.VBox(content...).Gap(12).Padding(24)
	b.ui.SetRoot(p.VBox(p.HBox(p.Expanded(p.Box()), p.Box(form).Width(600), p.Expanded(p.Box())), p.Expanded(p.Box())).Background(b.theme.Colors.Surface))
	b.gpu.RequestRedraw()
}
func (b *browser) showConnection() {
	if b.busy {
		return
	}
	file := b.field("Configuration file (.ini)", false)
	file.SetText(b.configPath)
	message := state.NewSignal("Choose an existing configuration or a saved resource.")
	children := []widget.Widget{b.text("Configuration file", 12), file, p.TextFn(func() string { return message.Get() }).FontSize(12).MaxLines(2), b.action("Connect", true, func() {
		cfg, err := config.Load(strings.TrimSpace(file.Text()))
		if err != nil {
			message.Set("Could not load configuration. Check the path and INI settings.")
			return
		}
		b.configPath = strings.TrimSpace(file.Text())
		b.connect(cfg)
	})}
	if filename, err := provision.DefaultPath(); err == nil {
		if profiles, err := provision.Load(filename); err == nil {
			children = append(children, b.rule(), b.text("Saved resources", 13))
			for i, r := range profiles.Resources {
				children = append(children, b.action(r.Label(), false, func() {
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
	children = append(children, b.rule(), b.text("Advanced", 13),
		b.action("Encrypt configuration value…", false, func() {
			b.configPath = file.Text()
			b.showEncrypt()
		}))
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
func (b *browser) showEncrypt() {
	value := b.field("Value to encrypt", false)
	password := b.field("Master password", true)
	confirm := b.field("Confirm password", true)
	result := b.field("ENC(…)", false)
	message := state.NewSignal("Create an ENC(…) value for credentials in your INI configuration.")
	b.showFormWithBack("Encrypt configuration value", b.showConnection, b.text("Value", 12), value, b.text("Master password", 12), password, b.text("Confirm master password", 12), confirm,
		p.TextFn(func() string { return message.Get() }).FontSize(12).MaxLines(2),
		b.action("Encrypt", true, func() {
			enc, err := encryptValue(value.Text(), password.Text(), confirm.Text())
			if err != nil {
				message.Set(err.Error())
				result.SetText("")
				return
			}
			result.SetText(enc)
			message.Set("Encrypted value ready to copy")
		}), b.text("Encrypted value", 12), result, b.action("Copy", false, func() {
			if result.Text() == "" {
				return
			}
			if err := b.gpu.ClipboardWrite(result.Text()); err != nil {
				message.Set("Could not copy to clipboard")
				return
			}
			message.Set("Copied to clipboard")
		}, button.DisabledFn(func() bool { return result.Text() == "" })))
}
