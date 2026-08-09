package normalizer

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// SourcePriority defines the priority order for semantic sources.
// Higher priority sources override lower priority ones.
// platform_review > sdk_explicit > openapi_rest
var SourcePriority = map[spec.SemanticSource]int{
	spec.SemanticSourceOpenAPIRest:    1,
	spec.SemanticSourceSDKExplicit:    2,
	spec.SemanticSourcePlatformReview: 3,
}

// SemanticProvenanceTracker tracks field-level provenance and detects conflicts.
type SemanticProvenanceTracker struct {
	provenance map[string]*spec.SemanticProvenance
	conflicts  []spec.SemanticConflict
}

// NewSemanticProvenanceTracker creates a new provenance tracker.
func NewSemanticProvenanceTracker() *SemanticProvenanceTracker {
	return &SemanticProvenanceTracker{
		provenance: make(map[string]*spec.SemanticProvenance),
	}
}

// TrackString records a string field value from a specific source and detects conflicts.
// Returns true if the value was accepted (no conflict or higher priority).
func (t *SemanticProvenanceTracker) TrackString(
	field string,
	value string,
	source spec.SemanticSource,
	sourceDigest string,
	updatedBy string,
) bool {
	return t.trackRaw(field, json.RawMessage(strconv.Quote(value)), source, sourceDigest, updatedBy)
}

// TrackUint records an unsigned integer field value from a specific source.
func (t *SemanticProvenanceTracker) TrackUint(
	field string,
	value uint,
	source spec.SemanticSource,
	sourceDigest string,
	updatedBy string,
) bool {
	return t.trackRaw(field, json.RawMessage(strconv.FormatUint(uint64(value), 10)), source, sourceDigest, updatedBy)
}

func (t *SemanticProvenanceTracker) trackRaw(
	field string,
	valueJSON json.RawMessage,
	source spec.SemanticSource,
	sourceDigest string,
	updatedBy string,
) bool {

	existing, exists := t.provenance[field]
	if !exists {
		// First time seeing this field
		t.provenance[field] = &spec.SemanticProvenance{
			Field:        field,
			Source:       source,
			SourceDigest: sourceDigest,
			Confidence:   confidenceForSource(source),
			Status:       "effective",
			Value:        valueJSON,
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
			UpdatedBy:    updatedBy,
		}
		return true
	}

	// Check if values are the same
	if valuesEqual(existing.Value, valueJSON) {
		// Same value, update provenance but keep effective
		existing.SourceDigest = sourceDigest
		existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		existing.UpdatedBy = updatedBy
		return true
	}

	// Different values - check priority
	existingPriority := SourcePriority[existing.Source]
	newPriority := SourcePriority[source]

	if newPriority > existingPriority {
		// New source has higher priority - override
		t.addConflict(field, existing.Source, existing.Value, source, valueJSON)
		existing.OverriddenValue = existing.Value
		existing.Value = valueJSON
		existing.Source = source
		existing.SourceDigest = sourceDigest
		existing.Confidence = confidenceForSource(source)
		existing.Status = "effective"
		existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		existing.UpdatedBy = updatedBy
		return true
	} else if newPriority < existingPriority {
		// New source has lower priority - mark as overridden
		// Record the conflict
		t.addConflict(field, existing.Source, existing.Value, source, valueJSON)
		return false
	} else {
		// Same priority - conflict!
		existing.Status = "conflict"
		t.addConflict(field, existing.Source, existing.Value, source, valueJSON)
		return false
	}
}

// addConflict records a conflict between two sources.
func (t *SemanticProvenanceTracker) addConflict(
	field string,
	source1 spec.SemanticSource,
	value1 json.RawMessage,
	source2 spec.SemanticSource,
	value2 json.RawMessage,
) {
	// Check if conflict already exists for this field
	for i, conflict := range t.conflicts {
		if conflict.Field == field {
			// Update existing conflict
			t.conflicts[i].Values[source2] = value2
			return
		}
	}

	// Create new conflict
	t.conflicts = append(t.conflicts, spec.SemanticConflict{
		Field: field,
		Values: map[spec.SemanticSource]json.RawMessage{
			source1: value1,
			source2: value2,
		},
	})
}

// GetProvenance returns the current provenance map.
func (t *SemanticProvenanceTracker) GetProvenance() map[string]*spec.SemanticProvenance {
	return t.provenance
}

// GetConflicts returns the list of unresolved conflicts.
func (t *SemanticProvenanceTracker) GetConflicts() []spec.SemanticConflict {
	return t.conflicts
}

// HasConflicts returns true if there are unresolved conflicts.
func (t *SemanticProvenanceTracker) HasConflicts() bool {
	return len(t.conflicts) > 0
}

// ResolveConflict resolves a conflict by choosing a source.
func (t *SemanticProvenanceTracker) ResolveConflict(
	field string,
	chosenSource spec.SemanticSource,
	resolvedBy string,
) bool {
	for i, conflict := range t.conflicts {
		if conflict.Field == field {
			chosenValue, ok := conflict.Values[chosenSource]
			if !ok {
				return false
			}

			// Update provenance with chosen value
			if prov, exists := t.provenance[field]; exists {
				prov.Value = chosenValue
				prov.Source = chosenSource
				prov.Confidence = confidenceForSource(chosenSource)
				prov.Status = "effective"
				prov.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				prov.UpdatedBy = resolvedBy
			}

			// Mark conflict as resolved
			t.conflicts[i].Resolution = chosenSource
			t.conflicts[i].ResolvedAt = time.Now().UTC().Format(time.RFC3339)
			t.conflicts[i].ResolvedBy = resolvedBy
			return true
		}
	}
	return false
}

// confidenceForSource returns the confidence level for a source.
func confidenceForSource(source spec.SemanticSource) string {
	switch source {
	case spec.SemanticSourcePlatformReview, spec.SemanticSourceSDKExplicit:
		return "high"
	case spec.SemanticSourceOpenAPIRest:
		return "low"
	default:
		return "low"
	}
}

// valuesEqual compares two JSON values for equality.
func valuesEqual(a, b json.RawMessage) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) == 0 || len(b) == 0 {
		return false
	}

	canonicalA, err := canonicalJSON(a)
	if err != nil {
		return false
	}
	canonicalB, err := canonicalJSON(b)
	if err != nil {
		return false
	}
	return bytes.Equal(canonicalA, canonicalB)
}

// canonicalJSON normalizes object key order without decoding into untyped values.
func canonicalJSON(value json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return nil, nil
	}

	switch trimmed[0] {
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &object); err != nil {
			return nil, err
		}
		for key, item := range object {
			canonical, err := canonicalJSON(item)
			if err != nil {
				return nil, err
			}
			object[key] = canonical
		}
		return json.Marshal(object)
	case '[':
		var array []json.RawMessage
		if err := json.Unmarshal(trimmed, &array); err != nil {
			return nil, err
		}
		for index, item := range array {
			canonical, err := canonicalJSON(item)
			if err != nil {
				return nil, err
			}
			array[index] = canonical
		}
		return json.Marshal(array)
	default:
		var compact bytes.Buffer
		if err := json.Compact(&compact, trimmed); err != nil {
			return nil, err
		}
		return compact.Bytes(), nil
	}
}
