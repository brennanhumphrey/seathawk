package vt

import (
	"fmt"
	"strings"
)

// RegistrationTicket is the registration-window ticket returned by studentdata.
//
// Ticket is not the final shockabsorber time_ticket. VT returns a 5-field
// server ticket here; BuildTimeTicket appends the two client-side fields that
// shockabsorber expects.
type RegistrationTicket struct {
	Term      string   `json:"term"`
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
	Ticket    string   `json:"ticket"`
	PinReq    bool     `json:"pinreq"`
	Actions   []string `json:"actions"`
}

// BuildTimeTicket appends the client-side timestamp and person ID fields to a
// server-provided registration ticket.
//
// Final shape:
// <start_iso>|<term>|<career>|<banner_hash>|<signed_hash>|<start_ms>|<pers_id>
//
// The function validates the server-provided Ticket has 5 fields because the
// completed shockabsorber time_ticket must have 7 fields.
func BuildTimeTicket(ticket RegistrationTicket, persID string) (string, error) {
	if strings.TrimSpace(ticket.Ticket) == "" {
		return "", fmt.Errorf("registration ticket is empty")
	}
	if strings.TrimSpace(persID) == "" {
		return "", fmt.Errorf("person ID is empty")
	}

	parts := strings.Split(ticket.Ticket, "|")
	if len(parts) != 5 {
		return "", fmt.Errorf("registration ticket has %d fields, expected 5", len(parts))
	}

	// VT uses start_date, not current time, for the millisecond field appended to
	// the ticket. That mirrors the browser's time-ticket construction.
	start, err := ParseTimestamp(ticket.StartDate)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s|%d|%s", ticket.Ticket, start.UnixMilli(), persID), nil
}
