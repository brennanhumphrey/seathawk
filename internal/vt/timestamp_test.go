package vt

import "testing"

func TestParseTimestamp(t *testing.T) {
	got, err := ParseTimestamp("2026-03-17T07:00:00:000000000-04:00")
	if err != nil {
		t.Fatalf("ParseTimestamp returned error: %v", err)
	}
	if got.UnixMilli() != 1773745200000 {
		t.Fatalf("UnixMilli = %d, want 1773745200000", got.UnixMilli())
	}
	if got.Format("-07:00") != "-04:00" {
		t.Fatalf("offset = %s, want -04:00", got.Format("-07:00"))
	}
}

func TestParseTimestampDifferentOffset(t *testing.T) {
	got, err := ParseTimestamp("2026-03-17T07:00:00:123456789+02:30")
	if err != nil {
		t.Fatalf("ParseTimestamp returned error: %v", err)
	}
	if got.Nanosecond() != 123456789 {
		t.Fatalf("Nanosecond = %d, want 123456789", got.Nanosecond())
	}
	if got.Format("-07:00") != "+02:30" {
		t.Fatalf("offset = %s, want +02:30", got.Format("-07:00"))
	}
}

func TestParseTimestampErrors(t *testing.T) {
	tests := []string{
		"",
		"2026-03-17T07:00:00.000000000-04:00",
		"2026-03-17T07:00:00-04:00",
		"not a timestamp",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseTimestamp(input); err == nil {
				t.Fatalf("ParseTimestamp(%q) returned nil error", input)
			}
		})
	}
}
