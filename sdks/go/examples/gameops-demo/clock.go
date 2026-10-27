package main

import "time"

// nowUnixMillis and unixMillisToRFC3339 are thin time wrappers so tests can
// verify the fixed-length seed without reimplementing ISO formatting.
func nowUnixMillis() int64 { return time.Now().UnixMilli() }

func unixMillisToRFC3339(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func timeParseRFC3339(v string) (int64, error) {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}
