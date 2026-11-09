package audit

import (
	"encoding/json"
)

// buildChainInfo builds the chain information for the record
func (s *AuditService) buildChainInfo(record *AuditRecord) error {
	// Get the latest record for prev hash
	latest, err := s.store.GetLatestRecord()
	if err != nil && err != ErrAuditNotFound {
		return err
	}

	var prevHash string
	var sequence int64 = 1

	if latest != nil {
		prevHash = latest.ChainInfo.Hash
		sequence = latest.ChainInfo.Sequence + 1
	}

	// ChainVersion pins the hash algorithm/field set and must survive the
	// ChainInfo rebuild; 0 (unset, records built before the version field
	// existed) is stored/validated as version 1 (legacy).
	version := record.ChainInfo.ChainVersion
	if version == 0 {
		version = ChainVersionLegacy
	}
	record.ChainInfo = ChainInfo{
		PrevHash:     prevHash,
		Sequence:     sequence,
		ChainVersion: version,
	}

	// Calculate hash
	hash, err := s.calculateHash(record)
	if err != nil {
		return err
	}
	record.ChainInfo.Hash = hash

	// Sign if signer is available
	if s.signer != nil {
		signature, signerID, err := s.signer.Sign(record)
		if err != nil {
			return err
		}
		record.ChainInfo.Signature = signature
		record.ChainInfo.SignerID = signerID
	}

	return nil
}

// calculateHash calculates the hash of a record. The algorithm and input
// field set are pinned by the record's chain version (ARCH-007/ARCH-008):
// version 1 (and 0/unset, for records built before the version field
// existed) uses the legacy DJB2 digest over the original 7-field subset;
// version 2 uses SHA-256 over the canonical full-record encoding.
func (s *AuditService) calculateHash(record *AuditRecord) (string, error) {
	if record.ChainInfo.ChainVersion >= ChainVersionCanonical {
		data, err := canonicalHashInput(record)
		if err != nil {
			return "", err
		}
		return sha256Hex(data), nil
	}
	data, err := legacyHashInput(record)
	if err != nil {
		return "", err
	}
	return legacyHashBytes(data), nil
}

// legacyHashInput builds the version-1 hash input: the original 7-field
// subset (timestamp, event_type, actor_id, resource id, outcome, prev_hash,
// sequence, details). Kept verbatim so pre-ARCH-008 rows remain verifiable.
func legacyHashInput(record *AuditRecord) ([]byte, error) {
	data := map[string]interface{}{
		"timestamp":  record.Timestamp.UnixNano(),
		"event_type": record.EventType,
		"actor_id":   record.Actor.ID,
		"resource":   record.Resource.ID,
		"outcome":    record.Outcome,
		"prev_hash":  record.ChainInfo.PrevHash,
		"sequence":   record.ChainInfo.Sequence,
	}

	if record.Details != nil {
		data["details"] = record.Details
	}

	return json.Marshal(data)
}

// canonicalHashInput builds the version-2 hash input: every semantic field
// of the record, with empty/zero values included so the field set is closed
// (ARCH-007 D1). Any future change to this set or its encoding is a new
// chain version, not a silent drift.
func canonicalHashInput(record *AuditRecord) ([]byte, error) {
	details := normalizeHashPayloadMap(record.Details)
	if details == nil {
		details = map[string]interface{}{}
	}
	var changes interface{}
	if record.Changes != nil {
		before := normalizeHashPayloadMap(record.Changes.Before)
		if before == nil {
			before = map[string]interface{}{}
		}
		after := normalizeHashPayloadMap(record.Changes.After)
		if after == nil {
			after = map[string]interface{}{}
		}
		changes = map[string]interface{}{
			"before":     before,
			"after":      after,
			"diffFields": record.Changes.DiffFields,
		}
	}

	data := map[string]interface{}{
		"id":         record.ID,
		"timestamp":  record.Timestamp.UTC().UnixMilli(),
		"event_type": record.EventType,
		"category":   record.Category,
		"severity":   record.Severity,
		"actor": map[string]interface{}{
			"id":          record.Actor.ID,
			"type":        record.Actor.Type,
			"name":        record.Actor.Name,
			"email":       record.Actor.Email,
			"roles":       record.Actor.Roles,
			"ipAddress":   record.Actor.IPAddress,
			"userAgent":   record.Actor.UserAgent,
			"sessionId":   record.Actor.SessionID,
			"mfaUsed":     record.Actor.MFAUsed,
			"delegatedBy": record.Actor.DelegatedBy,
			"metadata":    record.Actor.Metadata,
		},
		"action": record.Action,
		"resource": map[string]interface{}{
			"type":        record.Resource.Type,
			"id":          record.Resource.ID,
			"name":        record.Resource.Name,
			"gameId":      record.Resource.GameID,
			"environment": record.Resource.Environment,
			"metadata":    record.Resource.Metadata,
		},
		"outcome":       record.Outcome,
		"error_message": record.ErrorMessage,
		"changes":       changes,
		"context": map[string]interface{}{
			"requestId":     record.Context.RequestID,
			"traceId":       record.Context.TraceID,
			"correlationId": record.Context.CorrelationID,
			"service":       record.Context.Service,
			"version":       record.Context.Version,
			"environment":   record.Context.Environment,
			"tags":          record.Context.Tags,
		},
		"details":   details,
		"prev_hash": record.ChainInfo.PrevHash,
		"sequence":  record.ChainInfo.Sequence,
	}

	return json.Marshal(data)
}

// normalizeHashPayloadMap returns a JSON-round-tripped copy of v so numeric
// values are in the database-normalized float64 form. canonicalHashInput uses
// it so the hash input is byte-identical whether it comes from the caller
// (e.g. int64) or from a DB read-back (float64): json.Marshal(int(100)) and
// json.Marshal(float64(100)) are both "100", and values that lose precision
// under float64 conversion are rounded identically on both sides. The input
// map is never mutated. json.Marshal cannot fail on these shapes unless a
// value is unmarshalable, in which case calculateHash fails later anyway;
// on round-trip failure the original value is returned as-is.
func normalizeHashPayloadMap(v map[string]interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return v
	}
	return out
}
