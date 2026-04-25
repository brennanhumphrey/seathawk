package vt

import (
	"fmt"
	"regexp"
	"strings"
)

var jsonpPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\(([\s\S]*)\)[;\s]*$`)

// StripJSONP extracts the JSON payload from a sisproxy JSONP response.
//
// sisproxy does not return plain JSON. It wraps payloads in browser callback
// functions such as setRecord({...}), setCart({...}), or preflight({...}).
// fose and shockabsorber do not use this wrapper.
func StripJSONP(input string) ([]byte, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty JSONP response")
	}

	matches := jsonpPattern.FindStringSubmatch(input)
	if matches == nil {
		return nil, fmt.Errorf("malformed JSONP response")
	}
	if strings.TrimSpace(matches[1]) == "" {
		return nil, fmt.Errorf("empty JSONP payload")
	}

	// Return bytes so callers can pass the result directly to json.Unmarshal.
	return []byte(matches[1]), nil
}
