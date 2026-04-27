package cli

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	watchsvc "github.com/brennanhumphrey/seathawk/internal/watch"
)

func TestRootCommandIncludesWatch(t *testing.T) {
	root := newRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "watch" {
			return
		}
	}
	t.Fatal("root command does not include watch command")
}

func TestParseWatchID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "valid", input: "123", want: 123},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "not number", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWatchID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseWatchID returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWatchID returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseWatchID = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPrintWatchList(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	var buf bytes.Buffer

	printWatchList(&buf, []store.Watch{
		{
			ID:            1,
			Term:          "202609",
			Mode:          "swap",
			AddCRN:        "60058",
			DropCRN:       sql.NullString{String: "60900", Valid: true},
			Active:        true,
			LastSeenStat:  sql.NullString{String: "C", Valid: true},
			NextPollAt:    sql.NullTime{Time: now, Valid: true},
			LastAttemptAt: sql.NullTime{},
		},
	})

	out := buf.String()
	for _, want := range []string{"ID", "ACTIVE", "MODE", "202609", "60058", "60900", "C", "2026-04-26T12:00:00Z", "-"} {
		if !strings.Contains(out, want) {
			t.Fatalf("printWatchList output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintWatchListEmpty(t *testing.T) {
	var buf bytes.Buffer
	printWatchList(&buf, nil)
	if got := strings.TrimSpace(buf.String()); got != "no watches found" {
		t.Fatalf("empty output = %q, want no watches found", got)
	}
}

func TestPrintWatchSummary(t *testing.T) {
	var buf bytes.Buffer
	printWatchSummary(&buf, store.Watch{
		ID:      7,
		Term:    "202609",
		Mode:    "add",
		AddCRN:  "60058",
		Active:  true,
		DropCRN: sql.NullString{},
	})

	out := buf.String()
	for _, want := range []string{"id: 7", "active: true", "mode: add", "term: 202609", "add_crn: 60058", "drop_crn: -"} {
		if !strings.Contains(out, want) {
			t.Fatalf("printWatchSummary output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintEvaluationSummary(t *testing.T) {
	var buf bytes.Buffer
	printEvaluationSummary(&buf, watchsvc.Evaluation{
		SectionStatus: watchsvc.SectionFull,
		SectionTitle:  "Software Engineering",
		Window:        watchsvc.WindowNotOpen,
		Notes:         []watchsvc.Note{watchsvc.NoteSectionFull, watchsvc.NoteWindowNotOpen},
	})

	out := buf.String()
	for _, want := range []string{"section_status: full", "section_title: Software Engineering", "registration_window: not_open", "section is currently full", "registration window is not open yet"} {
		if !strings.Contains(out, want) {
			t.Fatalf("printEvaluationSummary output missing %q:\n%s", want, out)
		}
	}
}

func TestWatchCommandRequiredFlags(t *testing.T) {
	addCmd := newWatchAddCmd()
	addCmd.SetArgs([]string{"--term", "202609"})
	if err := addCmd.ValidateRequiredFlags(); err == nil {
		t.Fatal("add command missing --crn returned nil error")
	}

	swapCmd := newWatchSwapCmd()
	swapCmd.SetArgs([]string{"--term", "202609", "--add-crn", "60058"})
	if err := swapCmd.ValidateRequiredFlags(); err == nil {
		t.Fatal("swap command missing --drop-crn returned nil error")
	}
}
