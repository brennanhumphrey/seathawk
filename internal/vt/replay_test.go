package vt

import (
	"net/url"
	"strings"
	"testing"
)

func TestBuildSingleReplayURL(t *testing.T) {
	got, err := BuildSingleReplayURL("202606", "60058")
	if err != nil {
		t.Fatalf("BuildSingleReplayURL returned error: %v", err)
	}

	want := "api/?page=sisproxy&action=register&term_code=202606&crn=60058&wait_crn=&swap_crn="
	if got != want {
		t.Fatalf("BuildSingleReplayURL = %q, want %q", got, want)
	}
	if strings.Contains(got, "authtoken") {
		t.Fatalf("replay URL contains authtoken: %s", got)
	}
	if !strings.HasPrefix(got, "api/?") {
		t.Fatalf("replay URL should be relative: %s", got)
	}
}

func TestBuildSwapReplayURL(t *testing.T) {
	got, err := BuildSwapReplayURL("202606", "60058", "60900")
	if err != nil {
		t.Fatalf("BuildSwapReplayURL returned error: %v", err)
	}

	want := "api/?page=sisproxy&action=register&term_code=202606&crn=60058%2C60900&wait_crn=&swap_crn="
	if got != want {
		t.Fatalf("BuildSwapReplayURL = %q, want %q", got, want)
	}
	if outer := url.QueryEscape(got); !strings.Contains(outer, "60058%252C60900") {
		t.Fatalf("outer-encoded replay URL = %q, want double-encoded comma", outer)
	}
}

func TestBuildReplayURLErrors(t *testing.T) {
	tests := []struct {
		name string
		fn   func() (string, error)
	}{
		{name: "single empty term", fn: func() (string, error) { return BuildSingleReplayURL("", "60058") }},
		{name: "single empty CRN", fn: func() (string, error) { return BuildSingleReplayURL("202606", "") }},
		{name: "swap empty term", fn: func() (string, error) { return BuildSwapReplayURL("", "60058", "60900") }},
		{name: "swap empty add CRN", fn: func() (string, error) { return BuildSwapReplayURL("202606", "", "60900") }},
		{name: "swap empty drop CRN", fn: func() (string, error) { return BuildSwapReplayURL("202606", "60058", "") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.fn(); err == nil {
				t.Fatal("replay builder returned nil error")
			}
		})
	}
}
