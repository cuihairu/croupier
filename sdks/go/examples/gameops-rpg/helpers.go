package main

import (
	"sort"
	"strconv"
)

// Collection of small domain helpers shared by executors and the store.

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func padLeft(s string, n int, pad byte) string {
	for len(s) < n {
		s = string(pad) + s
	}
	return s
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func ternaryInt(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
}

func parseIDTail(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] < '0' || s[i] > '9' {
			if i == len(s)-1 {
				return 0
			}
			return atoi(s[i+1:], 0)
		}
	}
	return atoi(s, 0)
}

// itoa64 keeps seed literal counts readable.
func itoa64(v int64) string { return strconv.FormatInt(v, 10) }

// sortSliceBy implements the gameops-style list sort semantics: orderBy names
// a record field, orderDirection asc/desc. Unknown keys leave the current
// order untouched (deterministic). It mutates items in place.
func sortSliceBy[T any](m map[string]any, items []T) {
	key := valueString(m, "orderBy")
	if key == "" || len(items) == 0 {
		return
	}
	desc := valueString(m, "orderDirection") == "desc"
	sort.SliceStable(items, func(i, j int) bool {
		vi := recordFieldString(items[i], key)
		vj := recordFieldString(items[j], key)
		if vi == vj {
			return false
		}
		if desc {
			return vi > vj
		}
		return vi < vj
	})
}
