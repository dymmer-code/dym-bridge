// Package sites stores named HTTP(S) destinations and exposes them through an
// injected SSH dialer. It does not open direct connections to destination hosts.
package sites

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/dymmer-code/dym-bridge/pkg/config"
)

type Link struct {
	HostHeader string `json:"host_header,omitempty"`
	Name       string `json:"name"`
	URL        string `json:"url"`
}

func Validate(link Link) (Link, error) {
	if link.HostHeader != "" && !config.ValidHTTPHost(link.HostHeader) {
		return link, fmt.Errorf("Invalid HTTP host header")
	}
	link.Name = strings.TrimSpace(link.Name)
	link.URL = strings.TrimSpace(link.URL)
	if link.Name == "" {
		return link, fmt.Errorf("Enter a site name")
	}
	u, err := url.Parse(link.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return link, fmt.Errorf("Enter an HTTP or HTTPS URL without embedded credentials")
	}
	return link, nil
}
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "dym-bridge", "sites.json"), err
}
func read(filename string) (map[string][]Link, error) {
	data, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return map[string][]Link{}, nil
	}
	if err != nil {
		return nil, err
	}
	links := map[string][]Link{}
	err = json.Unmarshal(data, &links)
	if links == nil && err == nil {
		err = fmt.Errorf("Invalid sites configuration")
	}
	return links, err
}
func Load(filename, key string) ([]Link, bool, error) {
	all, err := read(filename)
	if err != nil {
		return nil, false, err
	}
	links, ok := all[key]
	return links, ok, nil
}
func Save(filename, key string, links []Link) error {
	clean := make([]Link, len(links))
	names := map[string]bool{}
	for i, link := range links {
		v, err := Validate(link)
		if err != nil {
			return err
		}
		if names[strings.ToLower(v.Name)] {
			return fmt.Errorf("Site names must be unique")
		}
		names[strings.ToLower(v.Name)] = true
		clean[i] = v
	}
	all, err := read(filename)
	if err != nil {
		return err
	}
	all[key] = clean
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(filename), ".sites-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filename)
}
