package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// handleWriteFailure applies the configured write-failure policy (ARCH-007
// D4). Strict (the default) fails the call; best-effort records the gap at
// WARN, fires a rate-limited notifier alert and returns the record with
// ChainInfo.WriteFailed set so callers can proceed without an error.
func (s *AuditService) handleWriteFailure(record *AuditRecord, cause error) (*AuditRecord, error) {
	s.mu.RLock()
	policy := s.writeFailurePolicy
	notifier := s.notifier
	s.mu.RUnlock()

	if policy != WritePolicyBestEffort {
		return nil, fmt.Errorf("failed to store audit record: %w", cause)
	}

	slog.Default().Warn("audit store write failed; best-effort mode continues",
		"error", cause, "eventType", record.EventType,
		"sequence", record.ChainInfo.Sequence)
	record.ChainInfo.WriteFailed = true
	s.alertWriteGap(notifier, record)
	return record, nil
}

// ReportWriteGap surfaces an audit-store write gap to the operator without
// failing the already-completed side effect (ARCH-008 D4 caller contract).
// It logs the gap at WARN and fires the rate-limited notifier alert. Callers
// that have already executed a business side effect (function invocation,
// approval state transition) MUST use this instead of propagating the write
// error as a 5xx, because a 5xx would imply the side effect did not happen
// and could invite a blind retry. A silent discard (fmt.Printf / _ =) is the
// anti-pattern this replaces.
func (s *AuditService) ReportWriteGap(ctx context.Context, eventType AuditEventType, cause error) {
	if s == nil {
		return
	}
	slog.Default().Warn("audit store write failed after side effect; gap surfaced",
		"eventType", eventType, "error", cause)
	s.mu.RLock()
	notifier := s.notifier
	s.mu.RUnlock()
	record := &AuditRecord{
		ID:        generateAuditID(),
		Timestamp: time.Now().UTC(),
		EventType: eventType,
		Outcome:   "failure",
		ChainInfo: ChainInfo{WriteFailed: true},
		Details:   map[string]interface{}{"gap": true},
	}
	if cause != nil {
		record.Details["error"] = cause.Error()
	}
	s.alertWriteGap(notifier, record)
}

// alertWriteGap fires the write-gap alert through the notifier, rate-limited
// to one alert per gapAlertInterval so an extended outage does not spam the
// notify channel.
func (s *AuditService) alertWriteGap(notifier AuditNotifier, record *AuditRecord) {
	if notifier == nil {
		return
	}
	s.gapAlertMu.Lock()
	now := time.Now()
	if !s.lastGapAlert.IsZero() && now.Sub(s.lastGapAlert) < s.gapAlertInterval {
		s.gapAlertMu.Unlock()
		return
	}
	s.lastGapAlert = now
	s.gapAlertMu.Unlock()

	_ = notifier.NotifyAudit(context.Background(), record)
}
