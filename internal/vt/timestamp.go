package vt

import (
	"fmt"
	"regexp"
	"time"
)

var vtTimestampPattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}):(\d+)([-+]\d{2}:\d{2})$`)

// ParseTimestamp parses VT's non-standard registration timestamp format.
//
// VT writes fractional seconds after a colon, for example:
// 2026-03-17T07:00:00:000000000-04:00
//
// RFC3339 expects a period before fractional seconds, so this normalizes the
// value before handing it to time.Parse.
func ParseTimestamp(value string) (time.Time, error) {
	matches := vtTimestampPattern.FindStringSubmatch(value)
	if matches == nil {
		return time.Time{}, fmt.Errorf("malformed VT timestamp %q", value)
	}

	normalized := matches[1] + "." + matches[2] + matches[3]
	t, err := time.Parse(time.RFC3339Nano, normalized)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse VT timestamp: %w", err)
	}

	return t, nil
}
