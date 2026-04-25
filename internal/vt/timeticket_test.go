package vt

import (
	"strings"
	"testing"
)

func TestBuildTimeTicket(t *testing.T) {
	ticket := RegistrationTicket{
		StartDate: "2026-03-17T07:00:00:000000000-04:00",
		Ticket:    "2026-03-17T07:00:00:000000000-04:00|202606||bannerHash|signedHash",
	}

	got, err := BuildTimeTicket(ticket, "personID")
	if err != nil {
		t.Fatalf("BuildTimeTicket returned error: %v", err)
	}

	want := "2026-03-17T07:00:00:000000000-04:00|202606||bannerHash|signedHash|1773745200000|personID"
	if got != want {
		t.Fatalf("BuildTimeTicket = %q, want %q", got, want)
	}
	if fields := strings.Split(got, "|"); len(fields) != 7 {
		t.Fatalf("field count = %d, want 7", len(fields))
	}
}

func TestBuildTimeTicketErrors(t *testing.T) {
	valid := RegistrationTicket{
		StartDate: "2026-03-17T07:00:00:000000000-04:00",
		Ticket:    "a|b|c|d|e",
	}

	tests := []struct {
		name   string
		ticket RegistrationTicket
		persID string
	}{
		{name: "empty ticket", ticket: RegistrationTicket{StartDate: valid.StartDate}, persID: "personID"},
		{name: "bad ticket field count", ticket: RegistrationTicket{StartDate: valid.StartDate, Ticket: "a|b|c"}, persID: "personID"},
		{name: "empty person ID", ticket: valid, persID: ""},
		{name: "bad timestamp", ticket: RegistrationTicket{StartDate: "bad", Ticket: valid.Ticket}, persID: "personID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildTimeTicket(tt.ticket, tt.persID); err == nil {
				t.Fatal("BuildTimeTicket returned nil error")
			}
		})
	}
}
