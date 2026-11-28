package openapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// Request-body media-type selection (SUI-007). The executor serializes a
// request body as JSON and sends the selected media type verbatim as
// Content-Type, so the executable set is exactly the JSON media types. The
// selection is deterministic for the same document — repeated imports of one
// multi-media-type operation always derive the same body slot (the contract
// is the public guarantee, docs/architecture/openapi-request-contract.md).

// DiagRequestBodyNoExecutableMedia marks an operation whose declared request
// body media types all fall outside the executor's serialization capability.
// Field is "body"; the message names the declared media types and the
// executable set, so a proposal records the reason instead of silently
// picking a body the executor cannot send.
const DiagRequestBodyNoExecutableMedia = "openapi_request_body_no_executable_media"

// executableRequestBodyMediaType is application/json, the only concrete media
// type the executor serializes a body as. Everything else (the RFC 6839
// structured-syntax suffix application/*+json) is recognized by
// IsExecutableRequestBodyMediaType.
const executableRequestBodyMediaType = "application/json"

// executableMediaTypeSet is the human-readable capability list used in the
// "no executable media type" reason and in the architecture doc.
const executableMediaTypeSet = "application/json and application/*+json"

// jsonMediaTypeSuffix is the RFC 6839 structured syntax suffix marking a media
// type whose payload is JSON regardless of the vendor tree it is served under
// (application/vnd.api+json, application/hal+json, ...).
const jsonMediaTypeSuffix = "+json"

// IsJSONMediaType reports whether a media type carries a JSON payload: exactly
// application/json, or any media type ending in the structured-syntax suffix
// +json (RFC 6839). Media type parameters (charset, boundary, ...) and case
// are ignored. An empty or wildcard type is not JSON.
func IsJSONMediaType(mediaType string) bool {
	mediaType = normalizeMediaType(mediaType)
	if mediaType == "" || mediaType == "*/*" {
		return false
	}
	if mediaType == executableRequestBodyMediaType {
		return true
	}
	return strings.HasSuffix(mediaType, jsonMediaTypeSuffix)
}

// IsExecutableRequestBodyMediaType reports whether the executor can serialize a
// request body of the given media type (SUI-007). The executor writes JSON
// bodies and sends the selected media type verbatim as Content-Type, so the
// executable set is exactly the JSON media types. form-urlencoded and
// multipart/form-data are declared but not yet executable (SUI-012 / SUI-013);
// any other media type is not executable either.
func IsExecutableRequestBodyMediaType(mediaType string) bool {
	return IsJSONMediaType(mediaType)
}

// normalizeMediaType lowercases a media type and drops its parameters so that
// "Application/JSON; charset=utf-8" compares as "application/json".
func normalizeMediaType(mediaType string) string {
	mediaType = strings.TrimSpace(strings.ToLower(mediaType))
	if index := strings.IndexByte(mediaType, ';'); index >= 0 {
		mediaType = strings.TrimSpace(mediaType[:index])
	}
	return mediaType
}

// SelectRequestBodyMediaType picks the request-body media type the executor
// will use from the declared content entries (SUI-007). It returns the chosen
// media type, or, when no declared media type is executable, an empty media
// type plus the reason recorded by the proposal diagnostic.
//
// The selection is deterministic for the same document: application/json wins,
// then the JSON-suffix media types, tie-broken lexicographically. Entries
// whose schema is absent take part in the selection like any other entry —
// the callers decide whether an entry without a schema counts (extraction
// skips schema-less entries first, keeping the legacy "no schema means no
// body" behavior).
func SelectRequestBodyMediaType(content map[string]RequestMedia) (mediaType string, reason string) {
	if len(content) == 0 {
		return "", "no request body media types declared"
	}
	declared := make([]string, 0, len(content))
	for key := range content {
		declared = append(declared, key)
	}
	sort.Strings(declared)

	type candidate struct {
		mediaType string
		rank      int
	}
	var candidates []candidate
	for _, key := range declared {
		if !IsExecutableRequestBodyMediaType(key) {
			continue
		}
		cand := candidate{mediaType: key, rank: 1}
		if normalizeMediaType(key) == executableRequestBodyMediaType {
			cand.rank = 0
		}
		candidates = append(candidates, cand)
	}
	if len(candidates) == 0 {
		return "", fmt.Sprintf(
			"no executable request body media type; declared [%s]; executable media types are %s",
			strings.Join(declared, ", "), executableMediaTypeSet)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].rank != candidates[j].rank {
			return candidates[i].rank < candidates[j].rank
		}
		return candidates[i].mediaType < candidates[j].mediaType
	})
	return candidates[0].mediaType, ""
}

// SelectExecutableMediaType picks the first executable media type of a
// declared, document-ordered list — Swagger 2.0 `consumes` semantics, where
// declaration order is meaningful. It returns "" with a reason when the list
// declares media types but none of them is executable; a missing list returns
// "", "" so the caller can fall through to a more specific declaration or a
// default.
func SelectExecutableMediaType(declared []string) (mediaType string, reason string) {
	for _, key := range declared {
		if IsExecutableRequestBodyMediaType(key) {
			return key, ""
		}
	}
	if len(declared) == 0 {
		return "", ""
	}
	return "", fmt.Sprintf(
		"no executable request body media type; declared [%s]; executable media types are %s",
		strings.Join(declared, ", "), executableMediaTypeSet)
}

// RequestBodyMediaDiagnostic builds the contract-level diagnostic recorded
// when an operation declares a request body whose media types are all outside
// the executor capability (SUI-007). Severity is warning: the operation still
// registers, but the reason is never dropped.
func RequestBodyMediaDiagnostic(reason string) spec.Diagnostic {
	return spec.Diagnostic{
		Code:     DiagRequestBodyNoExecutableMedia,
		Severity: spec.SeverityWarning,
		Field:    "body",
		Message:  reason,
	}
}
