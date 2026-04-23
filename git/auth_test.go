package git

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestGetAuth_NoKeyConfigured(t *testing.T) {
	resetViper(t)
	// sshprivatekey not set → returns nil, nil

	auth, err := GetAuth()
	if err != nil {
		t.Fatalf("GetAuth() error = %v, want nil", err)
	}
	if auth != nil {
		t.Fatalf("GetAuth() = %v, want nil", auth)
	}
}

func TestGetAuth_ExplicitlyEmpty(t *testing.T) {
	resetViper(t)
	viper.Set("sshprivatekey", "")

	auth, err := GetAuth()
	if err != nil {
		t.Fatalf("GetAuth() error = %v, want nil", err)
	}
	if auth != nil {
		t.Fatalf("GetAuth() = %v, want nil", auth)
	}
}

func TestGetAuth_MissingFile(t *testing.T) {
	resetViper(t)
	viper.Set("sshprivatekey", "/nonexistent/totally/missing/key")

	_, err := GetAuth()
	if err == nil {
		t.Fatal("GetAuth() expected error for missing file, got nil")
	}
}

func TestGetAuth_InvalidKeyContent(t *testing.T) {
	resetViper(t)

	f, err := os.CreateTemp(t.TempDir(), "sshkey-*")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	_, _ = f.WriteString("this is not a valid SSH private key")
	f.Close()

	viper.Set("sshprivatekey", f.Name())

	// File exists but content is not a valid key → error from ssh.NewPublicKeysFromFile
	_, err = GetAuth()
	if err == nil {
		t.Fatal("GetAuth() expected error for invalid key content, got nil")
	}
}
