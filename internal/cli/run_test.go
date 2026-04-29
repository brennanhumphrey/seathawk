package cli

import (
	"strings"
	"testing"
)

func TestRootCommandIncludesRun(t *testing.T) {
	root := newRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "run" {
			return
		}
	}
	t.Fatal("root command does not include run command")
}

func TestRunCommandMetadata(t *testing.T) {
	cmd := newRunCmd()
	if cmd.Use != "run" {
		t.Fatalf("Use = %q, want run", cmd.Use)
	}
	short := strings.ToLower(cmd.Short)
	for _, want := range []string{"daemon", "auto-registration"} {
		if !strings.Contains(short, want) {
			t.Fatalf("Short = %q, want to contain %q", cmd.Short, want)
		}
	}
	long := strings.ToLower(cmd.Long)
	for _, want := range []string{"read-only", "--auto-register", "register attempt"} {
		if !strings.Contains(long, want) {
			t.Fatalf("Long = %q, want to contain %q", cmd.Long, want)
		}
	}
	if cmd.Flags().Lookup("auto-register") == nil {
		t.Fatal("run command does not define --auto-register")
	}
}
