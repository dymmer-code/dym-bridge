package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	Prefix     = "ENC("
	Suffix     = ")"
	magicHeader = "DYM1"
	saltBytes  = 16
	keyBytes   = 32     // AES-256
	iterations = 100000 // PBKDF2 iterations with SHA-256
)

var (
	ErrInvalidFormat       = errors.New("formato de texto cifrado no válido: debe comenzar con ENC( y terminar con )")
	ErrEmptyMasterPassword = errors.New("la clave maestra no puede estar vacía")
	ErrDecryptionFailed    = errors.New("clave maestra incorrecta o datos cifrados corruptos")
)

// IsEncrypted returns true if the value is enclosed within ENC(...)
func IsEncrypted(val string) bool {
	trimmed := strings.TrimSpace(val)
	return strings.HasPrefix(trimmed, Prefix) && strings.HasSuffix(trimmed, Suffix)
}

// Encrypt encrypts the given plaintext using AES-256-GCM and a key derived from masterPassword via PBKDF2.
// It returns a formatted string: ENC(<base64-payload>).
func Encrypt(plaintext, masterPassword string) (string, error) {
	if masterPassword == "" {
		return "", ErrEmptyMasterPassword
	}

	salt := make([]byte, saltBytes)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("error al generar sal criptográfica: %w", err)
	}

	derivedKey := pbkdf2.Key([]byte(masterPassword), salt, iterations, keyBytes, sha256.New)

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", fmt.Errorf("error al inicializar cifrador AES: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("error al inicializar modo GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("error al generar vector de inicialización: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Payload layout: [4 bytes Magic "DYM1"] + [16 bytes Salt] + [12 bytes Nonce] + [Ciphertext + 16 bytes Tag]
	buf := make([]byte, len(magicHeader)+len(salt)+len(nonce)+len(sealed))
	copy(buf[0:4], magicHeader)
	copy(buf[4:20], salt)
	copy(buf[20:32], nonce)
	copy(buf[32:], sealed)

	encoded := base64.StdEncoding.EncodeToString(buf)
	return fmt.Sprintf("%s%s%s", Prefix, encoded, Suffix), nil
}

// Decrypt verifies and decrypts an ENC(...) string using masterPassword.
// If the password is wrong or the ciphertext was altered, it returns ErrDecryptionFailed.
func Decrypt(encVal, masterPassword string) (string, error) {
	if !IsEncrypted(encVal) {
		return "", ErrInvalidFormat
	}
	if masterPassword == "" {
		return "", ErrEmptyMasterPassword
	}

	trimmed := strings.TrimSpace(encVal)
	inner := trimmed[len(Prefix) : len(trimmed)-len(Suffix)]

	data, err := base64.StdEncoding.DecodeString(inner)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	minLen := len(magicHeader) + saltBytes + 12 + 16 // 4 + 16 + 12 + 16 = 48 bytes
	if len(data) < minLen {
		return "", ErrDecryptionFailed
	}

	if string(data[0:4]) != magicHeader {
		return "", ErrDecryptionFailed
	}

	salt := data[4:20]
	nonce := data[20:32]
	sealed := data[32:]

	derivedKey := pbkdf2.Key([]byte(masterPassword), salt, iterations, keyBytes, sha256.New)

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}
