package provision

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"strings"

	"github.com/dymmer-code/dym-bridge/pkg/secret"
	"golang.org/x/crypto/ssh"
)

// DeviceKey is the key pair of this device. The private half never leaves the computer: it is kept encrypted with the
// master password, and only the public half is sent to Dymmer.
type DeviceKey struct {
	PublicKey string `json:"public_key"` // authorized_keys line
	Private   string `json:"private"`    // ENC(...), the private key in OpenSSH format
}

// GenerateDeviceKey creates an Ed25519 pair for this device and encrypts the private key with the master password.
func GenerateDeviceKey(comment, masterPassword string) (*DeviceKey, error) {
	if masterPassword == "" {
		return nil, fmt.Errorf("a master password is needed to protect the private key")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return nil, err
	}
	enc, err := secret.Encrypt(string(pem.EncodeToMemory(block)), masterPassword)
	if err != nil {
		return nil, err
	}
	return &DeviceKey{PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))), Private: enc}, nil
}

// Signer decrypts the private key in memory and returns the signer for SSH.
func (k *DeviceKey) Signer(masterPassword string) (ssh.Signer, error) {
	pemText, err := secret.Decrypt(k.Private, masterPassword)
	if err != nil {
		return nil, fmt.Errorf("the master password does not open the device key: %w", err)
	}
	return ssh.ParsePrivateKey([]byte(pemText))
}

// HostKeyCallback accepts a server only if its key is one of those Dymmer reported for it (compared by SHA-256
// fingerprint). A different key is refused with a message that says so: it can be a changed server or an attacker.
func HostKeyCallback(keys []HostKey) ssh.HostKeyCallback {
	return func(hostname string, _ net.Addr, key ssh.PublicKey) error {
		got := ssh.FingerprintSHA256(key)
		for _, k := range keys {
			if k.Fingerprint == got {
				return nil
			}
		}
		return fmt.Errorf("the key of %s (%s) is not the one Dymmer reported: do not continue, sign in again to refresh it", hostname, got)
	}
}
