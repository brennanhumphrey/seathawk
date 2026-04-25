package vt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMinimalResponseTypesUnmarshal(t *testing.T) {
	fixtures := map[string]any{
		"fose_search_open_section.json": new(FoseSearchResponse),
		"studentdata_minimal.json":      new(StudentData),
		"preflight_success.json":        new(PreflightResponse),
	}

	for name, target := range fixtures {
		t.Run(name, func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if err := json.Unmarshal(contents, target); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}
		})
	}
}
