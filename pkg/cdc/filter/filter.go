// Package filter provides business-neutral selectors for database change events.
package filter

import (
	"context"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/pkg/cdc"
)

// Selector matches source metadata and DML operation. An empty field matches any
// value; '*' is supported as a component-local wildcard. Database and Table are
// matched independently so a wildcard never crosses a qualified-name boundary.
type Selector struct {
	SourceID   string          `json:"sourceId,omitempty"`
	SourceType string          `json:"sourceType,omitempty"`
	Database   string          `json:"database,omitempty"`
	Table      string          `json:"table,omitempty"`
	Operations []cdc.Operation `json:"operations,omitempty"`
}

// Filter is default-deny: an event must match at least one Include selector,
// then must match no Exclude selector. An empty Include list denies all events.
type Filter struct {
	Include []Selector `json:"include,omitempty"`
	Exclude []Selector `json:"exclude,omitempty"`
}

// Match reports whether the event passes the filter.
func (f Filter) Match(event cdc.ChangeEvent) bool {
	if len(f.Include) == 0 || !anyMatch(f.Include, event) {
		return false
	}
	return !anyMatch(f.Exclude, event)
}

func anyMatch(selectors []Selector, event cdc.ChangeEvent) bool {
	for _, selector := range selectors {
		if selector.matches(event) {
			return true
		}
	}
	return false
}

func (s Selector) matches(event cdc.ChangeEvent) bool {
	if !componentMatch(s.SourceID, event.SourceID) ||
		!componentMatch(s.SourceType, event.SourceType) ||
		!componentMatch(s.Database, event.Database) ||
		!componentMatch(s.Table, event.Table) {
		return false
	}
	if len(s.Operations) == 0 {
		return true
	}
	for _, operation := range s.Operations {
		if operation == event.Operation {
			return true
		}
	}
	return false
}

// componentMatch supports exact matches or a single '*' wildcard. This keeps
// selector semantics predictable and avoids filesystem/path escaping rules.
func componentMatch(pattern, value string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}
	parts := strings.Split(pattern, "*")
	if len(parts) != 2 {
		return false
	}
	return strings.HasPrefix(value, parts[0]) && strings.HasSuffix(value, parts[1]) &&
		len(value) >= len(parts[0])+len(parts[1])
}

// PushdownTables returns an exact table allowlist that is safe to send to a
// connector. It returns nil when selectors are too broad or source-specific for
// a generic adapter to translate safely. The caller must still apply Match to
// every delivered event because source-side filtering is only an optimization.
func (f Filter) PushdownTables(sourceID, sourceType string) []string {
	if len(f.Include) == 0 {
		return []string{}
	}
	set := make(map[string]struct{}, len(f.Include))
	for _, selector := range f.Include {
		if !sourceConstraintMatches(selector.SourceID, sourceID) || !sourceConstraintMatches(selector.SourceType, sourceType) {
			continue
		}
		if selector.Database == "" || selector.Table == "" || strings.Contains(selector.Database, "*") || strings.Contains(selector.Table, "*") {
			return nil
		}
		set[selector.Database+"."+selector.Table] = struct{}{}
	}
	if len(set) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(set))
	for table := range set {
		out = append(out, table)
	}
	sort.Strings(out)
	return out
}

func sourceConstraintMatches(constraint, value string) bool {
	return constraint == "" || constraint == "*" || constraint == value
}

// Events forwards only matching events. It exits on context cancellation and
// closes the output channel; it does not own or close the input channel.
func (f Filter) Events(ctx context.Context, input <-chan cdc.ChangeEvent) <-chan cdc.ChangeEvent {
	output := make(chan cdc.ChangeEvent)
	go func() {
		defer close(output)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-input:
				if !ok {
					return
				}
				if !f.Match(event) {
					continue
				}
				select {
				case output <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return output
}
