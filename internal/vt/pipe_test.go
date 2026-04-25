package vt

import "testing"

func TestParseCartEntry(t *testing.T) {
	got, err := ParseCartEntry("202606|default|60058|3||||AAEC 2104||N|||E|||||")
	if err != nil {
		t.Fatalf("ParseCartEntry returned error: %v", err)
	}

	if got.Term != "202606" || got.CartID != "default" || got.CRN != "60058" {
		t.Fatalf("unexpected identity fields: %+v", got)
	}
	if got.Hours != "3" || got.Course != "AAEC 2104" || got.GradeMode != "N" || got.RegInfo != "E" {
		t.Fatalf("unexpected course fields: %+v", got)
	}
}

func TestParseCartEntryPreservesOptionsOrDrop(t *testing.T) {
	got, err := ParseCartEntry("202606|default|60058|3||||AAEC 2104||N|||E|||||60900")
	if err != nil {
		t.Fatalf("ParseCartEntry returned error: %v", err)
	}
	if got.OptionsOrDrop != "60900" {
		t.Fatalf("OptionsOrDrop = %q, want 60900", got.OptionsOrDrop)
	}
}

func TestParseCartEntryErrors(t *testing.T) {
	tests := []string{
		"",
		"202606|default|60058",
		"202606|default|60058|3||||AAEC 2104||N|||E||||||extra",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseCartEntry(input); err == nil {
				t.Fatalf("ParseCartEntry(%q) returned nil error", input)
			}
		})
	}
}

func TestParseRegisteredSection(t *testing.T) {
	got, err := ParseRegisteredSection("60900|CS 3304||N|3|UG|joHHuW")
	if err != nil {
		t.Fatalf("ParseRegisteredSection returned error: %v", err)
	}

	if got.CRN != "60900" || got.Code != "CS 3304" || got.GradeMode != "N" || got.Hours != "3" || got.Career != "UG" {
		t.Fatalf("unexpected registered section: %+v", got)
	}
}

func TestParseRegisteredSectionErrors(t *testing.T) {
	tests := []string{
		"",
		"60900|CS 3304||N|3",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseRegisteredSection(input); err == nil {
				t.Fatalf("ParseRegisteredSection(%q) returned nil error", input)
			}
		})
	}
}
