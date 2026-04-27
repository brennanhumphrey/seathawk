package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadReturnsErrorWhenDefaultHomeUnavailable(t *testing.T) {
	originalHome, hadHome := os.LookupEnv("HOME")
	t.Cleanup(func() {
		if hadHome {
			_ = os.Setenv("HOME", originalHome)
			return
		}
		_ = os.Unsetenv("HOME")
	})
	t.Setenv("HOME", "")

	_, err := Load("")
	if err == nil {
		t.Fatal("Load returned nil error")
	}
	if !strings.Contains(err.Error(), "resolve user home directory") {
		t.Fatalf("error = %v, want home directory context", err)
	}
}
