package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/brennanhumphrey/seathawk/internal/register"
	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/watch"
)

func TestRootIncludesRegisterCommand(t *testing.T) {
	root := newRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "register" {
			return
		}
	}
	t.Fatal("root command does not include register command")
}

func TestRegisterAttemptRequiresConfirm(t *testing.T) {
	cmd := newRegisterAttemptCmd()
	cmd.SetArgs([]string{"1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("register attempt returned nil error without --confirm")
	}
	if !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("error = %q, want --confirm", err.Error())
	}
}

func TestPrintAttemptResult(t *testing.T) {
	var buf bytes.Buffer
	printAttemptResult(&buf, register.AttemptAddResult{
		Watch: store.Watch{ID: 7},
		Evaluation: watch.Evaluation{
			SectionStatus: watch.SectionOpen,
			Window:        watch.WindowReady,
		},
		Outcome:    register.OutcomeConfirmed,
		Registered: true,
		Message:    "registration confirmed in studentdata",
	})

	output := buf.String()
	for _, want := range []string{
		"watch_id: 7",
		"outcome: confirmed",
		"registered: true",
		"section_status: open",
		"registration_window: ready",
		"message: registration confirmed in studentdata",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}
