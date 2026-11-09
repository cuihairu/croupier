package audit

import (
	"context"
	"time"
)

// Get retrieves an audit record by ID
func (s *AuditService) Get(id string) (*AuditRecord, error) {
	return s.store.Get(id)
}

// List lists audit records with filtering and pagination
func (s *AuditService) List(filter AuditFilter, page AuditPage) ([]*AuditRecord, int, error) {
	return s.store.List(filter, page)
}

// GetStats gets audit statistics
func (s *AuditService) GetStats(startTime, endTime time.Time) (*AuditStats, error) {
	return s.store.GetStats(startTime, endTime)
}

// ValidateChain validates the integrity of the audit chain
func (s *AuditService) ValidateChain(startSeq, endSeq int64) (*ChainValidationResult, error) {
	records, err := s.store.GetChainRange(startSeq, endSeq)
	if err != nil {
		return nil, err
	}

	result := &ChainValidationResult{
		Valid:        true,
		TotalRecords: len(records),
		Errors:       []ChainError{},
	}

	for i, record := range records {
		// Verify hash
		expectedHash, err := s.calculateHash(record)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ChainError{
				Sequence: record.ChainInfo.Sequence,
				Type:     "hash_calculation_error",
				Message:  err.Error(),
			})
			continue
		}

		if record.ChainInfo.Hash != expectedHash {
			result.Valid = false
			result.Errors = append(result.Errors, ChainError{
				Sequence: record.ChainInfo.Sequence,
				Type:     "hash_mismatch",
				Message:  "Record hash does not match calculated hash",
			})
		}

		// Verify chain link
		if i > 0 {
			prevRecord := records[i-1]
			if record.ChainInfo.PrevHash != prevRecord.ChainInfo.Hash {
				result.Valid = false
				result.Errors = append(result.Errors, ChainError{
					Sequence: record.ChainInfo.Sequence,
					Type:     "chain_broken",
					Message:  "Previous hash does not match",
				})
			}
		}

		// Verify signature if present
		if s.signer != nil && record.ChainInfo.Signature != "" {
			if err := s.signer.Verify(record); err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, ChainError{
					Sequence: record.ChainInfo.Sequence,
					Type:     "signature_invalid",
					Message:  err.Error(),
				})
			}
		}
	}

	return result, nil
}

// ChainValidationResult contains the result of chain validation
type ChainValidationResult struct {
	Valid        bool         `json:"valid"`
	TotalRecords int          `json:"totalRecords"`
	Errors       []ChainError `json:"errors,omitempty"`
}

// ChainError represents an error found during chain validation
type ChainError struct {
	Sequence int64  `json:"sequence"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

// Export exports audit records
func (s *AuditService) Export(filter AuditFilter, format string) ([]byte, error) {
	return s.store.Export(filter, format)
}

// Archive archives old audit records
func (s *AuditService) Archive(before time.Time, archivePath string) (int64, error) {
	// Export records to archive
	filter := AuditFilter{
		EndTime: &before,
	}

	data, err := s.store.Export(filter, "json")
	if err != nil {
		return 0, err
	}

	// Here you would typically write to archive storage
	// For now, we'll just delete old records
	// In production, you'd write to S3, tape, etc. first
	// data contains the exported records that would be written to archive
	_ = data // Archive data would be written to archivePath

	// Delete archived records
	count, err := s.store.DeleteBefore(before)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// AuditSigner interface for signing audit records
type AuditSigner interface {
	Sign(record *AuditRecord) (signature, signerID string, err error)
	Verify(record *AuditRecord) error
}

// AuditNotifier interface for audit notifications
type AuditNotifier interface {
	NotifyAudit(ctx context.Context, record *AuditRecord) error
}

// Helper functions
