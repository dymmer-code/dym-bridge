package gpuui

import (
	"fmt"
	"github.com/dymmer-code/dym-bridge/pkg/sites"
	"net/url"
	"path"
	"path/filepath"

	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/secret"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"github.com/dymmer-code/dym-bridge/pkg/tunnel"
)

// session is owned by the single background operation. Widgets never access its
// mutable connection while an operation is running.
type session struct {
	manager *tunnel.Manager
	files   *sftp.Client
	relays  map[string]*sites.Relay
}

func (s *session) close() {
	for _, relay := range s.relays {
		relay.Close()
	}
	s.relays = nil
	if s.files != nil {
		_ = s.files.Close()
		s.files = nil
	}
	if s.manager != nil {
		s.manager.Disconnect()
		s.manager = nil
	}
}
func (s *session) connect(cfg *config.Config) error {
	s.close()
	s.manager = tunnel.NewManager(cfg)
	if err := s.manager.Connect(); err != nil {
		s.close()
		return err
	}
	files, err := sftp.NewClient(s.manager.SSHClient())
	if err != nil {
		s.close()
		return err
	}
	s.files = files
	return nil
}

type transferClient interface {
	Upload(string, string, sftp.ProgressCallback) error
	Download(string, string, sftp.ProgressCallback) error
	UploadDir(string, string, func(string, int64), sftp.ProgressCallback) error
	DownloadDir(string, string, func(string, int64), sftp.ProgressCallback) error
}
type transferProgress struct {
	Name             string
	Bytes, Size      int64
	Completed, Total int
}

func transferItems(client transferClient, upload bool, items []sftp.Item, target string, notify func(transferProgress)) error {
	progress := transferProgress{Total: len(items)}
	report := func(done, total int64) { progress.Bytes = done; progress.Size = total; notify(progress) }
	start := func(name string, size int64) { progress.Name = name; report(0, size) }
	for _, item := range items {
		start(item.Name, item.Size)
		var err error
		if upload {
			dest := path.Join(target, item.Name)
			if item.IsDir {
				err = client.UploadDir(item.Path, dest, start, report)
			} else {
				err = client.Upload(item.Path, dest, report)
			}
		} else {
			dest := filepath.Join(target, item.Name)
			if item.IsDir {
				err = client.DownloadDir(item.Path, dest, start, report)
			} else {
				err = client.Download(item.Path, dest, report)
			}
		}
		if err != nil {
			return fmt.Errorf("%s: %w", item.Name, err)
		}
		progress.Completed++
		notify(progress)
	}
	return nil
}
func encryptValue(value, password, confirm string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("Enter a value to encrypt")
	}
	if password == "" {
		return "", fmt.Errorf("Enter a master password")
	}
	if password != confirm {
		return "", fmt.Errorf("The passwords do not match")
	}
	result, err := secret.Encrypt(value, password)
	if err != nil {
		return "", fmt.Errorf("Could not encrypt the value")
	}
	return result, nil
}
func (m *Model) CheckedItems() []sftp.Item {
	var items []sftp.Item
	for _, item := range m.Visible {
		if m.IsSelected(item.Path) {
			items = append(items, item)
		}
	}
	return items
}

func (s *session) siteURL(raw string) (string, error) {
	if s.manager == nil || s.manager.SSHClient() == nil {
		return "", fmt.Errorf("Connect before opening a site")
	}
	if _, err := sites.Validate(sites.Link{Name: "site", URL: raw}); err != nil {
		return "", err
	}
	u, _ := url.Parse(raw)
	key := u.Scheme + "://" + u.Host
	if s.relays == nil {
		s.relays = map[string]*sites.Relay{}
	}
	relay := s.relays[key]
	if relay == nil {
		var err error
		relay, err = sites.NewRelay(raw, s.manager.SSHClient().DialContext)
		if err != nil {
			return "", err
		}
		s.relays[key] = relay
	}
	return relay.URL(raw), nil
}
