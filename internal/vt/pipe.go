package vt

import (
	"fmt"
	"strings"
)

// CartEntry is one 18-field pipe-delimited entry from the default cart.
//
// VT returns cart rows as strings instead of JSON objects. The field names here
// come directly from observed browser data and docs/VT_API_REFERENCE.md. Many
// fields are usually empty, but their positions still matter.
type CartEntry struct {
	Term         string
	CartID       string
	CRN          string
	Hours        string
	Course       string
	GradeMode    string
	RegInfo      string
	PermissionNo string
	SwapCRN      string
	Status       string
	WaitListOkay string
	// OptionsOrDrop is dual-use in VT's format. It may contain encoded JSON
	// options for a normal add, or the drop CRN when staging a swap.
	OptionsOrDrop string
}

// ParseCartEntry parses VT's 18-field pipe-delimited cart entry format.
func ParseCartEntry(value string) (CartEntry, error) {
	parts := strings.Split(value, "|")
	if len(parts) != 18 {
		return CartEntry{}, fmt.Errorf("cart entry has %d fields, expected 18", len(parts))
	}

	return CartEntry{
		Term:   parts[0],
		CartID: parts[1],
		CRN:    parts[2],
		Hours:  parts[3],
		// The skipped indexes are meet/resource/add-path/equivalency fields that
		// are not needed yet but still count toward the 18-field shape.
		Course:        parts[7],
		GradeMode:     parts[9],
		RegInfo:       parts[12],
		PermissionNo:  parts[13],
		SwapCRN:       parts[14],
		Status:        parts[15],
		WaitListOkay:  parts[16],
		OptionsOrDrop: parts[17],
	}, nil
}

// RegisteredSection is one pipe-delimited entry from studentdata reg.<term>.
//
// These records represent the student's actual registered sections for a term.
// That makes them more authoritative than cart or shockabsorber status output.
type RegisteredSection struct {
	CRN       string
	Code      string
	GradeMode string
	Hours     string
	Career    string
}

// ParseRegisteredSection parses VT's registered-section pipe-delimited format.
//
// The known useful fields are in the first six positions. VT may append an
// extra misc identifier, so this parser accepts additional fields.
func ParseRegisteredSection(value string) (RegisteredSection, error) {
	parts := strings.Split(value, "|")
	if len(parts) < 6 {
		return RegisteredSection{}, fmt.Errorf("registered section has %d fields, expected at least 6", len(parts))
	}

	return RegisteredSection{
		CRN:       parts[0],
		Code:      parts[1],
		GradeMode: parts[3],
		Hours:     parts[4],
		Career:    parts[5],
	}, nil
}
