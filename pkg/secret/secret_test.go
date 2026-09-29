package secret

import (
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	masterPwd := "MiClaveMaestraSuperSegura123!"
	plaintexts := []string{
		"SecretoClienteWordPress2026",
		"",
		"clave con espacios y caracteres especiales: ñ, á, é, í, ó, ú, @#$%^&*()_+",
		"-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACDHfLz9aX3o0G4m5K6z8y2P6wQ9fT1bN2c3D4e5F6g7HwAAAJh7VwYZe1cG\n-----END OPENSSH PRIVATE KEY-----\n",
	}

	for _, pt := range plaintexts {
		enc, err := Encrypt(pt, masterPwd)
		if err != nil {
			t.Fatalf("Encrypt error for %q: %v", pt, err)
		}

		if !IsEncrypted(enc) {
			t.Fatalf("expected IsEncrypted=true for %q", enc)
		}

		if !strings.HasPrefix(enc, "ENC(") || !strings.HasSuffix(enc, ")") {
			t.Fatalf("invalid envelope format: %s", enc)
		}

		// Decrypt with correct password
		dec, err := Decrypt(enc, masterPwd)
		if err != nil {
			t.Fatalf("Decrypt error: %v", err)
		}

		if dec != pt {
			t.Fatalf("plaintext mismatch: got %q, want %q", dec, pt)
		}

		// Decrypt with incorrect password
		_, err = Decrypt(enc, "ClaveIncorrecta")
		if err == nil {
			t.Fatalf("expected error when decrypting with wrong password, got nil")
		}
		if err != ErrDecryptionFailed {
			t.Fatalf("expected ErrDecryptionFailed, got: %v", err)
		}
	}
}

func TestEmptyMasterPassword(t *testing.T) {
	_, err := Encrypt("test", "")
	if err != ErrEmptyMasterPassword {
		t.Fatalf("expected ErrEmptyMasterPassword, got %v", err)
	}

	_, err = Decrypt("ENC(AAAA)", "")
	if err != ErrEmptyMasterPassword {
		t.Fatalf("expected ErrEmptyMasterPassword, got %v", err)
	}
}

func TestInvalidFormat(t *testing.T) {
	_, err := Decrypt("plain_password_not_enc", "pwd")
	if err != ErrInvalidFormat {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestCorruptedCiphertext(t *testing.T) {
	masterPwd := "password123"
	enc, err := Encrypt("secret", masterPwd)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	// Corrupt inside
	corrupted := enc[:len(enc)-4] + "AAAA)"
	_, err = Decrypt(corrupted, masterPwd)
	if err != ErrDecryptionFailed {
		t.Fatalf("expected ErrDecryptionFailed on corrupted data, got %v", err)
	}
}
