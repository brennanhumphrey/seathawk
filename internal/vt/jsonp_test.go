package vt

import (
	"encoding/json"
	"testing"
)

func TestStripJSONP(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "setRecord", input: `setRecord({"ok":true})`, want: `{"ok":true}`},
		{name: "setCart with semicolon", input: `setCart({"cart":[]});`, want: `{"cart":[]}`},
		{name: "whitespace", input: " \npreflight({\"reg_course_errors\":{\"60058\":\"||\"}}); \t", want: `{"reg_course_errors":{"60058":"||"}}`},
		{name: "nested payload", input: `setRecord({"pers":{"id":"x"},"cart":["a|b"]})`, want: `{"pers":{"id":"x"},"cart":["a|b"]}`},
		{name: "parentheses inside string", input: `setRecord({"message":"permission required (department only)"})`, want: `{"message":"permission required (department only)"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StripJSONP(tt.input)
			if err != nil {
				t.Fatalf("StripJSONP returned error: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("StripJSONP = %s, want %s", got, tt.want)
			}
			if !json.Valid(got) {
				t.Fatalf("StripJSONP returned invalid JSON: %s", got)
			}
		})
	}
}

func TestStripJSONPErrors(t *testing.T) {
	tests := []string{
		"",
		`{"ok":true}`,
		`setRecord`,
		`setRecord()`,
		`setRecord({"ok":true}`,
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := StripJSONP(input); err == nil {
				t.Fatalf("StripJSONP(%q) returned nil error", input)
			}
		})
	}
}
