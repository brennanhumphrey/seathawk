package cli

import (
	"strings"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "short", input: "abcd", want: "****"},
		{name: "eight", input: "abcdefgh", want: "********"},
		{name: "long", input: "abcdefghijkl", want: "abcd****ijkl"},
		{name: "trim", input: " abcdefghijkl ", want: "abcd****ijkl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskSecret(tt.input); got != tt.want {
				t.Fatalf("maskSecret(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReadCapturedCredentialsFromStdin(t *testing.T) {
	input := `{"authtoken":"token","pers_id":"person","pers_id_proof":"proof"}`
	got, err := readCapturedCredentials("-", strings.NewReader(input))
	if err != nil {
		t.Fatalf("readCapturedCredentials returned error: %v", err)
	}
	if got.Authtoken != "token" || got.PersID != "person" || got.PersIDProof != "proof" {
		t.Fatalf("unexpected payload: %+v", got)
	}
}
