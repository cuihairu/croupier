package main

import "strconv"

func itoa(v int) string { return strconv.Itoa(v) }

func atoi(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

// millisNow returns wall-clock Unix milliseconds.
func millisNow() int64 { return nowUnixMillis() }

// millisISO renders Unix milliseconds as an RFC3339 UTC string.
func millisISO(ms int64) string { return unixMillisToRFC3339(ms) }
