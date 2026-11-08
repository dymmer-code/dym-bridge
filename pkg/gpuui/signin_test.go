package gpuui

import (
	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"testing"
)

func TestSignInPasswordValidation(t *testing.T) {
	for _, tc := range []struct {
		password, confirm string
		first, valid      bool
	}{{"", "", true, true}, {"one", "two", true, false}, {"one", "one", true, true}, {"one", "", false, true}} {
		if got := validateSignInPassword(tc.password, tc.confirm, tc.first); (got == nil) != tc.valid {
			t.Fatalf("validation mismatch: %v", got)
		}
	}
}
func TestLoginServerFixedUnlessDeveloperOverride(t *testing.T) {
	b := &browser{}
	if b.serverURL() != DefaultServer {
		t.Fatal(b.serverURL())
	}
	b.profiles = &provision.Profiles{Server: "https://private.example"}
	if b.serverURL() != DefaultServer {
		t.Fatal(b.serverURL())
	}
	b.server = "https://override.example"
	if b.serverURL() != b.server {
		t.Fatal(b.serverURL())
	}
}
